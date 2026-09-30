package lane

// The keeper's landing-agent step (landing-lane-runtime-redesign §3, unit
// A-a; D1: one landing agent per computer, started on demand). One Step per
// steward cycle: the agent is started when the lane is registered and not
// paused, no hold stands, and a wake reason holds; an idle lane runs no
// model. Only the lane checkout's own steward starts it, so the lane's own
// engine supervises the lane's agent. The budget, usage and custody gates
// (K-g, K-f) join as Holds, and each ended launch is reaped once through
// Reap (the outage feed now; usage reconciliation with K-g).

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// AgentState is what the keeper remembers of the landing agent it last
// started, shared by every steward of the host under the lane flock.
type AgentState struct {
	Launch    string   `json:"launch,omitempty"`
	StartedAt string   `json:"startedAt,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
	// ReapedAt is when the launch's end was reaped; empty while it runs or
	// before its reap.
	ReapedAt string `json:"reapedAt,omitempty"`
}

func agentStatePath(home string) string {
	return filepath.Join(HostDir(home), "landing-agent-keeper.json")
}

// ReadAgentState is the keeper's record of the agent it last started; the
// zero state when none is kept.
func ReadAgentState(home string) (AgentState, error) {
	var state AgentState
	if _, err := readJSON(agentStatePath(home), &state); err != nil {
		return AgentState{}, err
	}
	return state, nil
}

// AgentKeeper wakes the host lane's landing agent.
type AgentKeeper struct {
	Home string
	Now  func() time.Time
	// Self is the checkout this steward keeps; only the lane checkout's own
	// steward (its top or its module root) starts the agent.
	Self string
	// Sources are the wake's reads, the same landing status --json uses.
	Sources WakeSources
	// Holds are the conditions besides the pause that hold a start: each
	// returns why it holds, empty when it does not; an error holds too,
	// because unknown is never a go.
	Holds []func(root string) (string, error)
	// Running names a landing launch that has not ended on this computer.
	Running func() (id string, running bool, err error)
	// Start starts the landing agent in the lane checkout for the wake and
	// returns its launch id.
	Start func(root string, wake Wake) (string, error)
	// Reap runs once for each ended launch the keeper started, before the
	// next start; a failed reap holds the next start and is tried again.
	Reap []func(id string) error
}

// Step observes the lane once and wakes its agent when it has work. It
// returns the line a steward prints when it changes; empty when no lane is
// registered.
func (k AgentKeeper) Step() string {
	line := ""
	err := withLock(k.Home, func() error {
		record, ok, err := Read(k.Home)
		if err != nil || !ok {
			return err
		}
		line = k.step(record.Root)
		return nil
	})
	if err != nil {
		return "the landing agent's keeper can't read the lane: " + err.Error()
	}
	return line
}

func (k AgentKeeper) step(root string) string {
	if gone(root) {
		return goneRefusal(Record{Root: root}).Message
	}
	if by, paused := pausedClosed(k.Home); paused {
		return fmt.Sprintf("the landing agent at %s is not started: the lane is paused by %s; metasystem landing start resumes it", root, by)
	}
	if !k.own(root) {
		return ""
	}
	id, running, err := k.Running()
	if err != nil {
		return fmt.Sprintf("the landing agent at %s is not started: whether one runs can't be read (%v)", root, err)
	}
	if running {
		return fmt.Sprintf("the landing agent %s is running at %s", id, root)
	}
	state, err := ReadAgentState(k.Home)
	if err != nil {
		return fmt.Sprintf("the landing agent at %s is not started: its keeper record can't be read (%v)", root, err)
	}
	if state.Launch != "" && state.ReapedAt == "" {
		for _, reap := range k.Reap {
			if err := reap(state.Launch); err != nil {
				return fmt.Sprintf("the landing agent at %s is not started: the end of %s could not be recorded (%v)", root, state.Launch, err)
			}
		}
		state.ReapedAt = k.Now().UTC().Format(time.RFC3339)
		if err := writeJSON(k.Home, agentStatePath(k.Home), state); err != nil {
			return "the landing agent's keeper can't write its record: " + err.Error()
		}
	}
	for _, hold := range k.Holds {
		reason, err := hold(root)
		if err != nil {
			return fmt.Sprintf("the landing agent at %s is not started: whether it may start can't be read (%v)", root, err)
		}
		if reason != "" {
			return fmt.Sprintf("the landing agent at %s is not started: %s", root, reason)
		}
	}
	wake := ReadWake(root, k.Now(), k.Sources)
	if len(wake.Reasons) == 0 {
		line := "the landing lane at " + root + " is idle; no landing agent runs"
		if len(wake.Unread) > 0 {
			line += " (unread: " + strings.Join(wake.Unread, "; ") + ")"
		}
		return line
	}
	id, err = k.Start(root, wake)
	if err != nil {
		return fmt.Sprintf("the landing agent at %s could not start for %s: %v", root, strings.Join(wake.Reasons, ", "), err)
	}
	state = AgentState{Launch: id, StartedAt: k.Now().UTC().Format(time.RFC3339), Reasons: wake.Reasons}
	if err := writeJSON(k.Home, agentStatePath(k.Home), state); err != nil {
		return fmt.Sprintf("the landing agent %s started at %s, but its keeper record could not be written: %v", id, root, err)
	}
	return fmt.Sprintf("woke the landing agent %s at %s: %s", id, root, strings.Join(wake.Reasons, ", "))
}

// own says whether this steward keeps the lane at root: its checkout is the
// lane's top or its module root.
func (k AgentKeeper) own(root string) bool {
	if k.Self == "" {
		return false
	}
	self := resolved(k.Self)
	return self == resolved(root) || self == resolved(batch.ModuleRoot(root))
}

// pausedClosed reads the pause failing closed (K2): a pause record that
// cannot be read counts as a pause.
func pausedClosed(home string) (string, bool) {
	var pause Pause
	ok, err := readJSON(pausePath(home), &pause)
	if err != nil {
		return "an unreadable pause record (" + pausePath(home) + ")", true
	}
	if !ok {
		return "", false
	}
	if pause.By == "" {
		return "an unnamed person", true
	}
	return pause.By, true
}
