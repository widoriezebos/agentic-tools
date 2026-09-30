package batchowner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// ownerLogCap bounds the owner's log: past it the file rolls to one previous
// generation (landing-owner.log.1), so the two hold at most twice this.
const ownerLogCap = 1 << 20

// OwnerLogPath is the landing owner's append-only log in its supervision
// directory: every red, start wait and error it reports.
func OwnerLogPath(root string) string {
	return filepath.Join(batch.ModuleRoot(root), "artifacts", "agents", "supervision", "landing-owner.log")
}

// NewOwnerLog is the owner's bounded log at root. A write that fails is
// dropped: the log never stops the owner.
func NewOwnerLog(root string) io.Writer { return newOwnerLog(root, ownerLogCap) }

func newOwnerLog(root string, limit int64) io.Writer {
	return &ownerLog{path: OwnerLogPath(root), limit: limit}
}

type ownerLog struct {
	mu    sync.Mutex
	path  string
	limit int64
}

func (log *ownerLog) Write(line []byte) (int, error) {
	log.mu.Lock()
	defer log.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(log.path), 0o755); err != nil {
		return len(line), nil
	}
	if info, err := os.Stat(log.path); err == nil && info.Size()+int64(len(line)) > log.limit {
		_ = os.Rename(log.path, log.path+".1")
	}
	file, err := os.OpenFile(log.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return len(line), nil
	}
	_, _ = file.Write(line)
	_ = file.Close()
	return len(line), nil
}

// TickErrors keeps the owner's last reported error for landing status: a
// report writes it, and a pass that reported nothing removes it.
type TickErrors struct {
	mu    sync.Mutex
	root  string
	noted bool
}

func NewTickErrors(root string) *TickErrors { return &TickErrors{root: root} }

// Begin starts a pass.
func (ticks *TickErrors) Begin() {
	if ticks == nil {
		return
	}
	ticks.mu.Lock()
	defer ticks.mu.Unlock()
	ticks.noted = false
}

// Note records one batch's error as the owner's last.
func (ticks *TickErrors) Note(id string, err error) {
	if ticks == nil || err == nil {
		return
	}
	ticks.mu.Lock()
	defer ticks.mu.Unlock()
	line := err.Error()
	if id != "" {
		line = "batch " + id + ": " + line
	}
	ticks.noted = true
	_, _ = atomicfile.WriteText(lane.TickErrorPath(ticks.root), line+"\n", ticks.root)
}

// End closes a pass: one that reported nothing clears the last error.
func (ticks *TickErrors) End() {
	if ticks == nil {
		return
	}
	ticks.mu.Lock()
	defer ticks.mu.Unlock()
	if ticks.noted {
		return
	}
	_ = os.Remove(lane.TickErrorPath(ticks.root))
}

// ownerReport is the owner's report sink: a JSON line on its log and the
// owner's last tick error.
func ownerReport(log io.Writer, ticks *TickErrors) func(string, error) {
	return func(id string, err error) {
		line, _ := json.Marshal(map[string]any{"component": "landing-owner", "batch": id, "error": err.Error()})
		fmt.Fprintln(log, string(line))
		ticks.Note(id, err)
	}
}
