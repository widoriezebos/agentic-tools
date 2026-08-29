// Package narratordigest owns the durable highlights and lowlights that a
// returning human has not yet received. The story is a records-owned log; a
// machine-local cursor advances only after a Stop payload is emitted.
package narratordigest

import (
	"bytes"
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

type Entry struct {
	Kind       string
	Text       string
	SourceType string
	SourceID   string
}

type PendingDigest struct {
	Message      string `json:"message"`
	Cursor       int64  `json:"cursor"`
	PrefixSHA256 string `json:"prefixSha256"`
}

type cursorRecord struct {
	Schema       int    `json:"schema"`
	Cursor       int64  `json:"cursor"`
	PrefixSHA256 string `json:"prefixSha256"`
}

func stateDirectory(kind stateroot.Kind) string {
	relative, err := stateroot.RelativeRoot(kind)
	if err != nil {
		panic(err)
	}
	return filepath.FromSlash(relative)
}

func Path(repoRoot string) string {
	return filepath.Join(repoRoot, stateDirectory(stateroot.Records), "narrator-digest.log")
}

func CursorPath(repoRoot string) string {
	return filepath.Join(repoRoot, stateDirectory(stateroot.Steward), "narrator-digest-cursor.json")
}

func lockPath(repoRoot string) string {
	return filepath.Join(repoRoot, stateDirectory(stateroot.Steward), "narrator-digest.flock")
}

type digestLock struct{ file *os.File }

func acquire(repoRoot string) (*digestLock, error) {
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
	return &digestLock{file: file}, nil
}

func (l *digestLock) release() {
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	_ = l.file.Close()
}

func flatten(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

func sourceMarker(entry Entry) string {
	return "(source: " + flatten(entry.SourceType) + " " + flatten(entry.SourceID) + ")"
}

// Append writes one line per event and deduplicates exact event retries.
func Append(repoRoot string, entries []Entry, now time.Time) error {
	if len(entries) == 0 {
		return nil
	}
	lock, err := acquire(repoRoot)
	if err != nil {
		return err
	}
	defer lock.release()
	path := Path(repoRoot)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	body := string(existing)
	for _, entry := range entries {
		kind := strings.ToUpper(flatten(entry.Kind))
		text := flatten(entry.Text)
		marker := sourceMarker(entry)
		if (kind != "HIGHLIGHT" && kind != "LOWLIGHT") || text == "" || entry.SourceType == "" || entry.SourceID == "" {
			return fmt.Errorf("narrator digest entry requires highlight/lowlight text and a source")
		}
		signature := kind + " — " + text + " " + marker
		if strings.Contains(body, signature) {
			continue
		}
		body += fmt.Sprintf("%s %s — %s %s\n", now.UTC().Format(time.RFC3339), kind, text, marker)
	}
	if body == string(existing) {
		return nil
	}
	durable, err := atomicfile.WriteText(path, body, repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("narrator digest published with directory durability unknown")
	}
	return nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func loadCursor(repoRoot string) (cursorRecord, error) {
	data, err := os.ReadFile(CursorPath(repoRoot))
	if os.IsNotExist(err) {
		return cursorRecord{Schema: 1, PrefixSHA256: digest(nil)}, nil
	}
	if err != nil {
		return cursorRecord{}, err
	}
	var cursor cursorRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil {
		return cursorRecord{}, fmt.Errorf("malformed narrator digest cursor: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return cursorRecord{}, fmt.Errorf("malformed narrator digest cursor: trailing JSON content")
	}
	if cursor.Schema != 1 || cursor.Cursor < 0 || len(cursor.PrefixSHA256) != 64 {
		return cursorRecord{}, fmt.Errorf("malformed narrator digest cursor")
	}
	return cursor, nil
}

// Pending returns the digest bytes after the last emitted check-in cursor.
func Pending(repoRoot string) (PendingDigest, error) {
	lock, err := acquire(repoRoot)
	if err != nil {
		return PendingDigest{}, err
	}
	defer lock.release()
	data, err := os.ReadFile(Path(repoRoot))
	if os.IsNotExist(err) {
		data = nil
	} else if err != nil {
		return PendingDigest{}, err
	}
	cursor, err := loadCursor(repoRoot)
	if err != nil {
		return PendingDigest{}, err
	}
	if cursor.Cursor > int64(len(data)) || digest(data[:cursor.Cursor]) != cursor.PrefixSHA256 {
		return PendingDigest{}, fmt.Errorf("narrator digest changed before the last check-in cursor")
	}
	pending := strings.TrimSpace(string(data[cursor.Cursor:]))
	message := ""
	if pending != "" {
		message = "NARRATOR DIGEST since last check-in:\n" + pending
	}
	return PendingDigest{Message: message, Cursor: int64(len(data)), PrefixSHA256: digest(data)}, nil
}

// Advance records that exactly one pending prefix reached the check-in.
func Advance(repoRoot string, cursor int64, prefixSHA256 string) error {
	lock, err := acquire(repoRoot)
	if err != nil {
		return err
	}
	defer lock.release()
	data, err := os.ReadFile(Path(repoRoot))
	if err != nil {
		return err
	}
	current, err := loadCursor(repoRoot)
	if err != nil {
		return err
	}
	if cursor < current.Cursor || cursor > int64(len(data)) || len(prefixSHA256) != 64 || digest(data[:cursor]) != prefixSHA256 {
		return fmt.Errorf("narrator digest cursor advance does not name the emitted prefix")
	}
	record := cursorRecord{Schema: 1, Cursor: cursor, PrefixSHA256: prefixSHA256}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(CursorPath(repoRoot), string(encoded)+"\n", repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("narrator digest cursor published with directory durability unknown")
	}
	return nil
}
