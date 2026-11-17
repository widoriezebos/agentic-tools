package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// RunnerProgress is the resident runner's current work and completed ticks.
type RunnerProgress struct {
	Pid            int64      `json:"pid"`
	PidStartedAt   int64      `json:"pidStartedAt"`
	StartTicks     int64      `json:"startTicks"`
	BootID         string     `json:"bootId"`
	Generation     int        `json:"generation"`
	Capable        int        `json:"capable"`
	Operation      string     `json:"operation"`
	OperationSince time.Time  `json:"operationSince"`
	State          string     `json:"state"`
	Reason         string     `json:"reason,omitempty"`
	Deadline       *time.Time `json:"deadline,omitempty"`
	Done           int        `json:"done"`
	ProgressAt     time.Time  `json:"progressAt"`
	FailedTicks    int        `json:"failedTicks"`
	OKTicks        int        `json:"okTicks"`
	Cycle          int        `json:"cycle"`
	At             time.Time  `json:"at"`
}

func runnerProgressPath(root string) string {
	return filepath.Join(runnerDir(root), "runner-progress.json")
}

// Progress publishes without taking any of the runner's work locks.
// A nil publisher leaves manual commands silent.
type Progress struct {
	mu              sync.Mutex
	record          RunnerProgress
	path            string
	now             func() time.Time
	write           func(string, string) error
	written, warned bool
}

func newProgress(root string, self identity.Exact, generation int, now func() time.Time, write func(string, string) error) *Progress {
	return &Progress{path: runnerProgressPath(root), now: now, write: write, record: RunnerProgress{
		Pid: self.Pid, PidStartedAt: self.StartedAt.Unix(), StartTicks: self.StartTicks,
		BootID: self.BootID, Generation: generation, Capable: 1, State: "working",
	}}
}

// Begin starts an operation with no completed items.
func (p *Progress) Begin(operation string) {
	p.update(func(now time.Time) {
		p.record.Operation, p.record.OperationSince = operation, now
		p.record.State, p.record.Reason, p.record.Deadline, p.record.Done = "working", "", nil, 0
	})
}

// Item counts work that has returned, preserving the current state.
func (p *Progress) Item() { p.update(func(time.Time) { p.record.Done++ }) }

// Wait records a wait and its owner's deadline, when it has one.
func (p *Progress) Wait(reason string, within time.Duration) {
	p.update(func(now time.Time) {
		p.record.State, p.record.Reason, p.record.Deadline = "waiting", reason, nil
		if within > 0 && (reason == "idle" || reason == "external") {
			deadline := now.Add(within)
			p.record.Deadline = &deadline
		}
	})
}

// Working ends the current wait.
func (p *Progress) Working() {
	p.update(func(time.Time) { p.record.State, p.record.Reason, p.record.Deadline = "working", "", nil })
}

func (p *Progress) ticked(err error) {
	p.update(func(time.Time) {
		if err != nil {
			p.record.FailedTicks++
		} else {
			p.record.FailedTicks = 0
			p.record.OKTicks++
		}
		p.record.Done++
	})
}

func (p *Progress) cycled() { p.update(func(time.Time) { p.record.Cycle++ }) }

func (p *Progress) update(change func(time.Time)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	before, now := p.record, p.now().UTC()
	change(now)
	if !p.written || before.Operation != p.record.Operation || before.State != p.record.State ||
		before.Reason != p.record.Reason || before.Done != p.record.Done {
		p.record.ProgressAt = now
	}
	p.record.At, p.written = now, true
	data, err := json.Marshal(p.record)
	if err == nil {
		err = p.write(p.path, string(data)+"\n")
	}
	if err != nil && !p.warned {
		fmt.Fprintf(os.Stderr, "publish steward runner progress: %v\n", err)
	}
	p.warned = err != nil
}

// ProgressItems publishes completed work and waits within a runner operation.
// Only the resident loop supplies a publisher.
type ProgressItems interface {
	Item()
	Wait(reason string, within time.Duration)
	Working()
}

// itemCounter counts the previous iteration before starting the next one.
// Defer done when the loop can return before reaching its end.
type itemCounter struct {
	item    func()
	pending bool
}

func (c *itemCounter) next() { c.done(); c.pending = true }
func (c *itemCounter) done() {
	if c.pending && c.item != nil {
		c.item()
	}
	c.pending = false
}

// RunnerActivity describes only progress belonging to this live runner.
func RunnerActivity(root string, runner RunnerRecord) (activity string, since time.Time, ok bool) {
	var record RunnerProgress
	if err := readJSON(runnerProgressPath(root), &record); err != nil ||
		record.Pid != runner.Pid || record.PidStartedAt != runner.PidStartedAt ||
		record.StartTicks != runner.StartTicks || record.BootID != runner.BootID {
		return "", time.Time{}, false
	}
	state := record.State
	if record.State == "waiting" {
		state += " " + record.Reason
	}
	return fmt.Sprintf("%s (%s, %d done)", record.Operation, state, record.Done), record.OperationSince, true
}
