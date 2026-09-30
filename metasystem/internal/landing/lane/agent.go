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
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
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
	// StartingAt is a start in progress, claimed under the flock while the
	// launcher runs outside it.
	StartingAt string `json:"startingAt,omitempty"`
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
	// Cancel stops a launch that started while the lane was paused
	// meanwhile; nil leaves it to the pause's own reach.
	Cancel func(id string) error
	// Exclusive runs the start's claim under the batch owner's ensure lock
	// of the lane at root, the lock every owner start takes; nil runs it
	// as is.
	Exclusive func(root string, fn func() error) error
}

// AgentCooldown is how long the keeper waits before it wakes a fresh agent
// for the very reasons the last one ended with: an agent that ended while
// its reasons still hold would otherwise be relaunched every cycle. Changed
// reasons wake at once. (The full budget is K-g's.)
const AgentCooldown = time.Hour

// startClaim bounds a start in progress that another step honours: a start
// the launcher never finished is taken over after it.
const startClaim = 10 * time.Minute

// Step observes the lane once and wakes its agent when it has work. It
// returns the line a steward prints when it changes; empty when no lane is
// registered or this steward does not keep it. The decision is taken under
// the lane flock; the wake read and the start run outside it, so a person's
// landing stop never waits for them, and the start is re-checked under the
// flock before and after.
func (k AgentKeeper) Step() string {
	var root, line string
	var registered Record
	var state AgentState
	proceed := false
	if err := withLock(k.Home, func() error {
		record, ok, err := Read(k.Home)
		if err != nil || !ok {
			return err
		}
		root, registered = record.Root, record
		line, state, proceed = k.decide(record)
		return nil
	}); err != nil {
		return "the landing agent's keeper can't read the lane: " + err.Error()
	}
	if !proceed {
		return line
	}
	now := k.Now()
	wake := ReadWake(registered, now, k.Sources)
	if len(wake.Reasons) == 0 {
		line := "the landing lane at " + root + " is idle; no landing agent runs"
		if len(wake.Unread) > 0 {
			line += " (unread: " + strings.Join(wake.Unread, "; ") + ")"
		}
		return line
	}
	if ended, err := time.Parse(time.RFC3339, state.ReapedAt); err == nil && slices.Equal(state.Reasons, wake.Reasons) && now.Sub(ended) < AgentCooldown {
		return fmt.Sprintf("the landing agent at %s is not started again yet: the last one ended at %s with the same reasons (%s); it wakes when they change or at %s",
			root, LocalText(state.ReapedAt), strings.Join(wake.Reasons, ", "), localClock(ended.Add(AgentCooldown)))
	}
	// Claim the start under the flock, re-checking what may have changed
	// while the wake was read.
	claimed := false
	if err := withLock(k.Home, func() error {
		record, ok, err := Read(k.Home)
		if err != nil || !ok || record.Root != root {
			line = "the landing agent at " + root + " is not started: the lane changed while its wake was read"
			return err
		}
		exclusive := k.Exclusive
		if exclusive == nil {
			exclusive = func(_ string, fn func() error) error { return fn() }
		}
		// Under the owner's own ensure lock: the holds (the batch owner
		// among them) are read again and the claim is recorded before any
		// owner start can look, so an owner and the agent never both start.
		return exclusive(root, func() error {
			if reason, stop := k.recheck(root); stop {
				line = reason
				return nil
			}
			if reason, held := k.held(root); held {
				line = reason
				return nil
			}
			current, err := ReadAgentState(k.Home)
			if err != nil {
				return err
			}
			current.StartingAt = k.Now().UTC().Format(time.RFC3339)
			claimed = true
			return writeJSON(k.Home, agentStatePath(k.Home), current)
		})
	}); err != nil {
		return "the landing agent's keeper can't claim the start: " + err.Error()
	}
	if !claimed {
		return line
	}
	id, startErr := k.Start(root, wake)
	if err := withLock(k.Home, func() error {
		current, err := ReadAgentState(k.Home)
		if err != nil {
			return err
		}
		current.StartingAt = ""
		if startErr != nil {
			line = fmt.Sprintf("the landing agent at %s could not start for %s: %v", root, strings.Join(wake.Reasons, ", "), startErr)
			return writeJSON(k.Home, agentStatePath(k.Home), current)
		}
		current = AgentState{Launch: id, StartedAt: k.Now().UTC().Format(time.RFC3339), Reasons: wake.Reasons}
		line = fmt.Sprintf("woke the landing agent %s at %s: %s", id, root, strings.Join(wake.Reasons, ", "))
		// A pause that came while it started ends it at once; its reasons
		// then hold no cooldown.
		if by, paused := pausedClosed(k.Home); paused && k.Cancel != nil {
			current.Reasons = nil
			if err := k.Cancel(id); err != nil {
				line = fmt.Sprintf("the landing agent %s started at %s as the lane was paused by %s, and could not be stopped: %v; run: metasystem work stop %s", id, root, by, err, id)
			} else {
				line = fmt.Sprintf("the landing agent %s started at %s as the lane was paused by %s, and was stopped again", id, root, by)
			}
		}
		return writeJSON(k.Home, agentStatePath(k.Home), current)
	}); err != nil {
		return fmt.Sprintf("the landing agent at %s: its keeper record could not be written: %v", root, err)
	}
	return line
}

// recheck is what may stop a start between the decision and the launch: the
// pause, and an agent that runs or is being started.
func (k AgentKeeper) recheck(root string) (string, bool) {
	if by, paused := pausedClosed(k.Home); paused {
		return fmt.Sprintf("the landing agent at %s is not started: the lane is paused by %s; metasystem landing start resumes it", root, by), true
	}
	id, running, err := k.Running()
	if err != nil {
		return fmt.Sprintf("the landing agent at %s is not started: whether one runs can't be read (%v)", root, err), true
	}
	if running {
		return fmt.Sprintf("the landing agent %s is running at %s", id, root), true
	}
	at, starting, err := AgentStarting(k.Home, k.Now())
	if err != nil {
		return err.Error(), true
	}
	if starting {
		return fmt.Sprintf("the landing agent at %s is starting (since %s)", root, localClock(at)), true
	}
	return "", false
}

// decide is the step's part under the flock: whether this steward may start
// the agent now, after reaping an ended launch and reading the holds.
func (k AgentKeeper) decide(record Record) (string, AgentState, bool) {
	root := record.Root
	if gone(root) {
		return goneRefusal(Record{Root: root}).Message, AgentState{}, false
	}
	if by, paused := pausedClosed(k.Home); paused {
		return fmt.Sprintf("the landing agent at %s is not started: the lane is paused by %s; metasystem landing start resumes it", root, by), AgentState{}, false
	}
	if !k.own(record) {
		return "", AgentState{}, false
	}
	if reason, stop := k.recheck(root); stop {
		return reason, AgentState{}, false
	}
	state, _ := ReadAgentState(k.Home)
	if state.Launch != "" && state.ReapedAt == "" {
		for _, reap := range k.Reap {
			if err := reap(state.Launch); err != nil {
				return fmt.Sprintf("the landing agent at %s is not started: the end of %s could not be recorded (%v)", root, state.Launch, err), state, false
			}
		}
		state.ReapedAt = k.Now().UTC().Format(time.RFC3339)
		if err := writeJSON(k.Home, agentStatePath(k.Home), state); err != nil {
			return "the landing agent's keeper can't write its record: " + err.Error(), state, false
		}
	}
	if reason, held := k.held(root); held {
		return reason, state, false
	}
	return "", state, true
}

// held reads the holds: the first that holds, or cannot be read, stops the
// start.
func (k AgentKeeper) held(root string) (string, bool) {
	for _, hold := range k.Holds {
		reason, err := hold(root)
		if err != nil {
			return fmt.Sprintf("the landing agent at %s is not started: whether it may start can't be read (%v)", root, err), true
		}
		if reason != "" {
			return fmt.Sprintf("the landing agent at %s is not started: %s", root, reason), true
		}
	}
	return "", false
}

// AgentStarting says whether the keeper claimed a landing agent start that
// has not finished, and since when: an owner start honours it as it honours
// a running agent. A keeper record that cannot be read is an error naming
// the file and its repair, which every caller holds on.
func AgentStarting(home string, now time.Time) (time.Time, bool, error) {
	state, err := ReadAgentState(home)
	if err != nil {
		return time.Time{}, false, errors.New(UnreadableAgentRecord(home))
	}
	at, err := time.Parse(time.RFC3339, state.StartingAt)
	if err != nil || now.Sub(at) >= startClaim {
		return time.Time{}, false, nil
	}
	return at, true, nil
}

// UnreadableAgentRecord says, for a person, that the landing agent's keeper
// record cannot be read and what repairs it.
func UnreadableAgentRecord(home string) string {
	return "the landing agent's keeper record " + agentStatePath(home) + " can't be read, so nothing starts\nrun: metasystem landing start"
}

// ClearAgentCooldown forgets the reasons the last agent ended with: a
// person's landing start wakes the agent at once when work is there. A
// record that cannot be read is replaced at the person's word, keeping
// nothing (its launch, if any, is left to the launch store).
func ClearAgentCooldown(home string) error {
	return withLock(home, func() error {
		state, err := ReadAgentState(home)
		if err != nil {
			return writeJSON(home, agentStatePath(home), AgentState{})
		}
		if len(state.Reasons) == 0 {
			return nil
		}
		state.Reasons = nil
		return writeJSON(home, agentStatePath(home), state)
	})
}

// own says whether this steward keeps the registered lane: its checkout is
// the lane's checkout or its installation, as landing set recorded them
// (K-a), never guessed.
func (k AgentKeeper) own(record Record) bool {
	layout, err := record.Layout()
	if k.Self == "" || err != nil {
		return false
	}
	self := resolved(k.Self)
	return self == resolved(string(layout.Checkout)) || self == resolved(string(layout.Install))
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
