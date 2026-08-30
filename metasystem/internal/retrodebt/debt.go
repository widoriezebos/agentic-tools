// Package retrodebt owns the durable obligation raised when expensive work
// completes. A debt remembers the exact receipt-ledger prefix present at birth;
// only a later RECEIPT row whose type is retro can discharge it.
package retrodebt

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"golang.org/x/sys/unix"
)

const Schema = 1

const (
	KindArc        = "arc-goal"
	KindObligation = "governed-obligation"
)

type Entry struct {
	ID                  string `json:"id"`
	Kind                string `json:"kind"`
	Source              string `json:"source"`
	RaisedAt            string `json:"raisedAt"`
	ReceiptOffset       int64  `json:"receiptOffset"`
	ReceiptPrefixSHA256 string `json:"receiptPrefixSha256"`
	DischargedBy        string `json:"dischargedBy,omitempty"`
}

type State struct {
	Schema     int     `json:"schema"`
	Generation uint64  `json:"generation"`
	Entries    []Entry `json:"entries"`
}

func stateDirectory(kind stateroot.Kind) string {
	relative, err := stateroot.RelativeRoot(kind)
	if err != nil {
		panic(err)
	}
	return filepath.FromSlash(relative)
}

func Path(repoRoot string) string {
	return filepath.Join(repoRoot, stateDirectory(stateroot.Registers), "retro-debt.json")
}

func lockPath(repoRoot string) string {
	return filepath.Join(repoRoot, stateDirectory(stateroot.Steward), "retro-debt.flock")
}

func receiptPath(repoRoot string) string {
	return filepath.Join(repoRoot, stateDirectory(stateroot.Receipts), "receipts.log")
}

type debtLock struct{ file *os.File }

func acquire(repoRoot string) (*debtLock, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath(repoRoot)), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lockPath(repoRoot), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return &debtLock{file: file}, nil
}

func (l *debtLock) release() {
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	_ = l.file.Close()
}

func load(repoRoot string) (State, error) {
	data, err := os.ReadFile(Path(repoRoot))
	if os.IsNotExist(err) {
		return State{Schema: Schema}, nil
	}
	if err != nil {
		return State{}, err
	}
	var state State
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("malformed retro debt record: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return State{}, fmt.Errorf("malformed retro debt record: trailing JSON content")
	}
	if state.Schema != Schema {
		return State{}, fmt.Errorf("malformed retro debt record: schema=%d, want %d", state.Schema, Schema)
	}
	seen := map[string]bool{}
	for _, entry := range state.Entries {
		if entry.ID == "" || entry.Source == "" || entry.RaisedAt == "" || entry.ReceiptOffset < 0 ||
			len(entry.ReceiptPrefixSHA256) != 64 || (entry.Kind != KindArc && entry.Kind != KindObligation) || seen[entry.ID] {
			return State{}, fmt.Errorf("malformed retro debt record: invalid entry %q", entry.ID)
		}
		seen[entry.ID] = true
	}
	return state, nil
}

func save(repoRoot string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(Path(repoRoot), string(data)+"\n", repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("retro debt record published with directory durability unknown")
	}
	return nil
}

func readReceipts(repoRoot string) ([]byte, error) {
	data, err := os.ReadFile(receiptPath(repoRoot))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Raise records one idempotent debt at the receipt ledger's current edge.
func Raise(repoRoot, kind, source string, now time.Time) (Entry, error) {
	if source == "" || (kind != KindArc && kind != KindObligation) {
		return Entry{}, fmt.Errorf("retro debt requires a known kind and nonempty source")
	}
	lock, err := acquire(repoRoot)
	if err != nil {
		return Entry{}, err
	}
	defer lock.release()
	state, err := load(repoRoot)
	if err != nil {
		return Entry{}, err
	}
	id := kind + ":" + source
	for _, entry := range state.Entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	receipts, err := readReceipts(repoRoot)
	if err != nil {
		return Entry{}, fmt.Errorf("read receipt ledger before raising retro debt: %w", err)
	}
	entry := Entry{
		ID: id, Kind: kind, Source: source, RaisedAt: now.UTC().Format(time.RFC3339),
		ReceiptOffset: int64(len(receipts)), ReceiptPrefixSHA256: digest(receipts),
	}
	state.Generation++
	state.Entries = append(state.Entries, entry)
	if err := save(repoRoot, state); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Open reconciles receipt-only discharges and returns the still-open debts.
func Open(repoRoot string) ([]Entry, error) {
	lock, err := acquire(repoRoot)
	if err != nil {
		return nil, err
	}
	defer lock.release()
	state, err := load(repoRoot)
	if err != nil {
		return nil, err
	}
	receipts, err := readReceipts(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("read receipt ledger while reconciling retro debt: %w", err)
	}
	changed := false
	for index := range state.Entries {
		entry := &state.Entries[index]
		if entry.DischargedBy != "" {
			continue
		}
		if entry.ReceiptOffset > int64(len(receipts)) || digest(receipts[:entry.ReceiptOffset]) != entry.ReceiptPrefixSHA256 {
			return nil, fmt.Errorf("receipt ledger prefix changed beneath retro debt %s; restore the append-only ledger", entry.ID)
		}
		if ref := firstRetroReceipt(receipts[entry.ReceiptOffset:]); ref != "" {
			entry.DischargedBy = ref
			changed = true
		}
	}
	if changed {
		state.Generation++
		if err := save(repoRoot, state); err != nil {
			return nil, err
		}
	}
	var open []Entry
	for _, entry := range state.Entries {
		if entry.DischargedBy == "" {
			open = append(open, entry)
		}
	}
	return open, nil
}

func firstRetroReceipt(data []byte) string {
	for _, raw := range strings.Split(string(data), "\n") {
		fields := strings.Split(raw, "|")
		if len(fields) < 4 || fields[2] != "RECEIPT" || fields[3] != "type=retro" {
			continue
		}
		epoch := fields[0]
		if epoch == "" {
			continue
		}
		sum := sha1.Sum([]byte(raw))
		return epoch + ":" + hex.EncodeToString(sum[:])
	}
	return ""
}
