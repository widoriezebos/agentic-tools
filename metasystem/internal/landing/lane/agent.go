package lane

// The keeper's landing-agent step (simple lane §1, rail 2): one landing
// agent per computer, started on demand. One Step per steward cycle: the
// agent is started when the lane is registered and not paused, its queue is
// not empty, no hold stands and none is alive; an idle lane runs no model.
// Only the lane checkout's own steward starts it, so the lane's own engine
// supervises the lane's agent. Each ended launch is reaped once through Reap
// (the outage feed).

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// AgentState is what the keeper remembers of the landing agent it last
// started, shared by every steward of the host under the lane flock.
type AgentState struct {
	Launch    string `json:"launch,omitempty"`
	StartedAt string `json:"startedAt,omitempty"`
	// ReapedAt is when the launch's end was reaped; empty while it runs or
	// before its reap.
	ReapedAt string `json:"reapedAt,omitempty"`
	// StartingAt is a start in progress, claimed under the flock while the
	// launcher runs outside it.
	StartingAt string `json:"startingAt,omitempty"`
	// Fingerprint is the lane as its agent can change it when the launch
	// started. Barren counts the launches in a row that ended with the lane
	// unchanged, BarrenLaunches names them, and BarrenFingerprint is the
	// lane at the last such end.
	Fingerprint       string   `json:"fingerprint,omitempty"`
	Barren            int      `json:"barren,omitempty"`
	BarrenLaunches    []string `json:"barrenLaunches,omitempty"`
	BarrenFingerprint string   `json:"barrenFingerprint,omitempty"`
}

// barrenLimit is how many launches in a row may end with the lane unchanged
// before the keeper starts no more by itself.
const barrenLimit = 2

// AgentTick is the keeper cadence while the landing lane waits for work or a hold.
const AgentTick = 15 * time.Second

func agentStatePath(home string) string {
	return filepath.Join(HostDir(home), "landing-agent-keeper.json")
}

// AgentStatePath is the keeper record used as evidence for a barren hold.
func AgentStatePath(home string) string { return agentStatePath(home) }

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
	// Fingerprint reads the lane as its agent can change it (its queue,
	// results and pushes): barrenLimit launches in a row that end with it
	// unchanged hold the next start until it changes. nil holds nothing.
	Fingerprint func(root string) (string, error)
	// BarrenStop records the existing barren hold, or its clearing, at its owner.
	BarrenStop func(record Record, state AgentState) error
	// Observe reconciles the lane's own stop question on every keeper tick.
	Observe func(Record) error
	// Prepare records the work selection before launch, outside the home lock.
	Prepare func(Record) error
	// Explicit is a start asked for by name (landing run), not the keeper's
	// own. Only a recorded person selection may clear a barren count.
	Explicit bool
	// PersonSelection reads an atomic selection snapshot without taking the queue lock.
	PersonSelection func(Record) bool
	// Continuation names a recorded batch admitted through the current fences.
	// It reads snapshots only; the home lock must never acquire the queue lock.
	Continuation func(Record) string
	Helmed       func(string) bool
	// Waiting counts waiting lines in the registered installation for the start log.
	Waiting func(install, checkout string) (int, error)
}

// startClaim bounds a start in progress that another step honours: a start
// the launcher never finished is taken over after it.
const startClaim = 10 * time.Minute

// AgentOutcome is what one keeper step found or did.
type AgentOutcome string

// The keeper step's outcomes.
const (
	// AgentNotKept: no lane is registered, or this steward does not keep it.
	AgentNotKept AgentOutcome = "not-kept"
	// AgentStarted: the step started a landing agent.
	AgentStarted AgentOutcome = "started"
	// AgentRunning: a landing agent runs, or its start is under way.
	AgentRunning AgentOutcome = "running"
	// AgentIdle: the lane's queue is empty.
	AgentIdle AgentOutcome = "idle"
	// AgentPaused: a person paused the lane.
	AgentPaused AgentOutcome = "paused"
	// AgentHeld: a hold stands, or what decides the start can't be read.
	AgentHeld AgentOutcome = "held"
	// AgentFailed: the start or the keeper's record failed.
	AgentFailed AgentOutcome = "failed"
)

// AgentRun is one keeper step's result: its outcome, the launch it started
// or found running, the wake it started for, and the line a steward prints.
type AgentRun struct {
	Outcome AgentOutcome
	Launch  string
	Root    string
	Reasons []string
	Line    string
}

func agentRun(outcome AgentOutcome, root, line string) AgentRun {
	return AgentRun{Outcome: outcome, Root: root, Line: line}
}

// Step observes the lane once and wakes its agent when it has work. It
// returns the line a steward prints when it changes; empty when no lane is
// registered or this steward does not keep it.
func (k AgentKeeper) Step() string { return k.Run().Line }

// Run is Step with its outcome. The decision is taken under the lane flock;
// the wake read and the start run outside it, so a person's landing stop
// never waits for them, and the start is re-checked under the flock before
// and after.
func (k AgentKeeper) Run() AgentRun {
	var root string
	var result AgentRun
	var registered Record
	var barren *AgentState
	proceed := false
	if err := withLock(k.Home, func() error {
		record, ok, err := Read(k.Home)
		if err != nil || !ok {
			result = agentRun(AgentNotKept, "", "")
			return err
		}
		root, registered = record.Root, record
		decision := k
		decision.BarrenStop = func(_ Record, state AgentState) error { barren = &state; return nil }
		result, proceed = decision.decide(record)
		return nil
	}); err != nil {
		return agentRun(AgentFailed, root, "the landing agent's keeper can't read the lane: "+err.Error())
	}
	// The lock order is install lock before home lock, never the reverse.
	// Stop records and question synchronization may take the install lock;
	// their inputs are read under the home lock and written after releasing it.
	if barren != nil && k.BarrenStop != nil {
		if err := k.BarrenStop(registered, *barren); err != nil {
			return agentRun(AgentFailed, root, "the landing agent's keeper can't write its stop: "+err.Error())
		}
	}
	if k.own(registered) && !gone(root) && k.Observe != nil {
		if err := k.Observe(registered); err != nil && !(k.Explicit && k.PersonSelection != nil && k.PersonSelection(registered)) {
			return agentRun(AgentHeld, root, "the lane's stop question cannot be reconciled: "+err.Error())
		}
	}
	if k.own(registered) && !gone(root) && k.Prepare != nil {
		if err := k.Prepare(registered); err != nil {
			if k.Observe != nil {
				_ = k.Observe(registered)
			}
			return agentRun(AgentHeld, root, err.Error())
		}
	}
	if !proceed {
		return result
	}
	if reason, held := k.held(root); held {
		return agentRun(AgentHeld, root, reason)
	}
	scope := k.scope(registered)
	wake := ReadWake(registered, k.Sources)
	wake.BatchID = scope
	if len(wake.Reasons) == 0 {
		line := "the landing lane at " + root + " is idle; no landing agent runs"
		if len(wake.Unread) > 0 {
			line += " (unread: " + strings.Join(wake.Unread, "; ") + ")"
		}
		return agentRun(AgentIdle, root, line)
	}
	waiting := 0
	if k.Waiting != nil {
		var err error
		waiting, err = k.Waiting(registered.Install, registered.Root)
		if err != nil {
			return agentRun(AgentHeld, root, "the landing agent cannot read its waiting lines: "+err.Error())
		}
	}
	// Claim the start under the flock, re-checking what may have changed
	// while the wake was read.
	claimed := false
	if err := withLock(k.Home, func() error {
		record, ok, err := Read(k.Home)
		if err != nil || !ok || record != registered {
			result = agentRun(AgentHeld, root, "the landing agent at "+root+" is not started: the lane changed while its wake was read")
			return err
		}
		// The holds are read again and the claim is recorded under the
		// flock, so two stewards never both start an agent.
		if stopped, stop := k.recheck(record); stop {
			result = stopped
			return nil
		}
		if reason, held := k.held(root); held {
			result = agentRun(AgentHeld, root, reason)
			return nil
		}
		if k.scope(record) != scope {
			result = agentRun(AgentHeld, root, "the recorded selection changed before launch")
			return nil
		}
		current, err := ReadAgentState(k.Home)
		if err != nil {
			return err
		}
		current.StartingAt = k.Now().UTC().Format(time.RFC3339)
		claimed = true
		return written(writeJSON(k.Home, agentStatePath(k.Home), current))
	}); err != nil {
		return agentRun(AgentFailed, root, "the landing agent's keeper can't claim the start: "+err.Error())
	}
	if !claimed {
		return result
	}
	fingerprint := ""
	if k.Fingerprint != nil {
		fingerprint, _ = k.Fingerprint(root)
	}
	id, startErr := k.Start(root, wake)
	if err := withLock(k.Home, func() error {
		current, err := ReadAgentState(k.Home)
		if err != nil {
			return err
		}
		current.StartingAt = ""
		if startErr != nil {
			result = agentRun(AgentFailed, root, fmt.Sprintf("the landing agent at %s could not start for %s: %v", root, strings.Join(wake.Reasons, ", "), startErr))
			return written(writeJSON(k.Home, agentStatePath(k.Home), current))
		}
		current = AgentState{Launch: id, StartedAt: k.Now().UTC().Format(time.RFC3339), Fingerprint: fingerprint,
			Barren: current.Barren, BarrenLaunches: current.BarrenLaunches, BarrenFingerprint: current.BarrenFingerprint}
		result = AgentRun{Outcome: AgentStarted, Launch: id, Root: root, Reasons: wake.Reasons,
			Line: fmt.Sprintf("lane agent started by the keeper: %d waiting line(s); %s at %s: %s", waiting, id, root, strings.Join(wake.Reasons, ", "))}
		// A changed fence, registration or selection stops the new launch.
		fresh, present, readErr := Read(k.Home)
		changedLane := readErr != nil || !present || fresh != registered
		currentScope := k.scope(registered)
		if by, paused := pausedClosed(k.Home); (paused && currentScope == "" || currentScope != scope || changedLane) && k.Cancel != nil {
			result.Outcome = AgentPaused
			if err := k.Cancel(id); err != nil {
				result.Outcome = AgentFailed
				result.Line = fmt.Sprintf("the landing agent %s started at %s as the lane was paused by %s, and could not be stopped: %v; run: metasystem work stop %s", id, root, by, err, id)
				if changedLane || !paused {
					result.Line = fmt.Sprintf("the landing agent %s changed its registered lane or selected work while starting and could not be stopped: %v; run: metasystem work stop %s", id, err, id)
				}
			} else {
				result.Line = fmt.Sprintf("the landing agent %s started at %s as the lane was paused by %s, and was stopped again", id, root, by)
				if changedLane || !paused {
					result.Outcome = AgentHeld
					result.Line = "the landing agent " + id + " was stopped because its registered lane or selected work changed while it started"
				}
			}
		}
		return written(writeJSON(k.Home, agentStatePath(k.Home), current))
	}); err != nil {
		return AgentRun{Outcome: AgentFailed, Launch: id, Root: root, Line: fmt.Sprintf("the landing agent at %s: its keeper record could not be written: %v", root, err)}
	}
	return result
}

func (k AgentKeeper) scope(record Record) string {
	if k.Continuation == nil {
		return ""
	}
	return k.Continuation(record)
}

func pausedLine(root, by string) string {
	return fmt.Sprintf("the landing agent at %s is not started: the lane is paused by %s; metasystem landing start resumes it", root, by)
}

// recheck is what may stop a start between the decision and the launch: the
// pause, and an agent that runs or is being started.
func (k AgentKeeper) recheck(record Record) (AgentRun, bool) {
	root := record.Root
	if by, paused := pausedClosed(k.Home); paused && k.scope(record) == "" {
		return agentRun(AgentPaused, root, pausedLine(root, by)), true
	}
	if k.Helmed != nil && k.Helmed(root) && k.scope(record) == "" {
		return agentRun(AgentHeld, root, "the landing checkout is at the helm; only a recorded person selection may continue"), true
	}
	id, running, err := k.Running()
	if err != nil {
		return agentRun(AgentHeld, root, fmt.Sprintf("the landing agent at %s is not started: whether one runs can't be read (%v)", root, err)), true
	}
	if running {
		return AgentRun{Outcome: AgentRunning, Launch: id, Root: root, Line: fmt.Sprintf("the landing agent %s is running at %s", id, root)}, true
	}
	at, starting, err := AgentStarting(k.Home, k.Now())
	if err != nil {
		return agentRun(AgentHeld, root, err.Error()), true
	}
	if starting {
		return agentRun(AgentRunning, root, fmt.Sprintf("the landing agent at %s is starting (since %s)", root, localClock(at))), true
	}
	return AgentRun{}, false
}

// decide is the step's part under the flock: whether this steward may start
// the agent now, after reaping an ended launch and reading the holds.
func (k AgentKeeper) decide(record Record) (AgentRun, bool) {
	root := record.Root
	if gone(root) {
		return agentRun(AgentHeld, root, goneRefusal(Record{Root: root}).Message), false
	}
	if by, paused := pausedClosed(k.Home); paused && !k.own(record) {
		return agentRun(AgentPaused, root, pausedLine(root, by)), false
	}
	if !k.own(record) {
		return agentRun(AgentNotKept, root, ""), false
	}
	// An ended launch is reaped at once, paused or not.
	state, _ := ReadAgentState(k.Home)
	if _, running, err := k.Running(); err == nil && !running && state.Launch != "" && state.ReapedAt == "" {
		for _, reap := range k.Reap {
			if err := reap(state.Launch); err != nil {
				return agentRun(AgentHeld, root, fmt.Sprintf("the landing agent at %s is not started: the end of %s could not be recorded (%v)", root, state.Launch, err)), false
			}
		}
		state.ReapedAt = k.Now().UTC().Format(time.RFC3339)
		k.countBarren(&state, root)
		if err := written(writeJSON(k.Home, agentStatePath(k.Home), state)); err != nil {
			return agentRun(AgentFailed, root, "the landing agent's keeper can't write its record: "+err.Error()), false
		}
	}
	if by, paused := pausedClosed(k.Home); paused && k.scope(record) == "" {
		return agentRun(AgentPaused, root, pausedLine(root, by)), false
	}
	if reason, held, err := k.barrenHold(record); err != nil {
		return agentRun(AgentFailed, root, "the landing agent's keeper can't write its record: "+err.Error()), false
	} else if held {
		return agentRun(AgentHeld, root, reason), false
	}
	if stopped, stop := k.recheck(record); stop {
		return stopped, false
	}
	return AgentRun{}, true
}

// countBarren counts an ended launch that left the lane as it found it, and
// clears the count when the lane changed during the launch.
func (k AgentKeeper) countBarren(state *AgentState, root string) {
	if k.Fingerprint == nil || state.Fingerprint == "" {
		return
	}
	now, err := k.Fingerprint(root)
	if err != nil || now != state.Fingerprint {
		state.Barren, state.BarrenLaunches, state.BarrenFingerprint = 0, nil, ""
		return
	}
	state.Barren++
	state.BarrenLaunches = append(state.BarrenLaunches, state.Launch)
	state.BarrenFingerprint = now
}

// barrenHold holds the keeper's own start after barrenLimit launches in a
// row ended with the lane unchanged, until the lane changes or an explicit run retries a recorded person selection.
func (k AgentKeeper) barrenHold(record Record) (string, bool, error) {
	root := record.Root
	state, _ := ReadAgentState(k.Home)
	if k.Fingerprint == nil || state.Barren == 0 {
		return "", false, nil
	}
	clear := k.Explicit && k.PersonSelection != nil && k.PersonSelection(record)
	if !clear && state.Barren >= barrenLimit {
		now, err := k.Fingerprint(root)
		clear = err == nil && now != state.BarrenFingerprint
	}
	if clear {
		state.Barren, state.BarrenLaunches, state.BarrenFingerprint = 0, nil, ""
		if k.BarrenStop != nil {
			if err := k.BarrenStop(record, state); err != nil {
				return "", false, err
			}
		}
		return "", false, written(writeJSON(k.Home, agentStatePath(k.Home), state))
	}
	if state.Barren < barrenLimit {
		return "", false, nil
	}
	if k.BarrenStop != nil {
		if err := k.BarrenStop(record, state); err != nil {
			return "", true, err
		}
	}
	return fmt.Sprintf("the landing agent at %s is not started: its last %d runs (%s) ended with the lane unchanged; a person must select waiting goals with metasystem landing run --goals GOALS, or prove main with metasystem landing prove --trunk when none is eligible",
		root, state.Barren, strings.Join(state.BarrenLaunches, ", ")), true, nil
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
// has not finished, and since when. A keeper record that cannot be read is an error naming
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

// RepairAgentRecord replaces a keeper record that cannot be read, at a
// person's landing start, keeping nothing (its launch, if any, is left to
// the launch store); a readable record is left as it is.
func RepairAgentRecord(home string) error {
	return withLock(home, func() error {
		if _, err := ReadAgentState(home); err != nil {
			return written(writeJSON(home, agentStatePath(home), AgentState{}))
		}
		return nil
	})
}

// own says whether this steward keeps the registered lane (ownsLane).
func (k AgentKeeper) own(record Record) bool { return ownsLane(k.Self, record) }

// OwnsLane says whether the steward of self keeps the registered lane, as
// the keeper decides it: the steward's lane-silent signal reads the lane
// only there.
func OwnsLane(self string, record Record) bool { return ownsLane(self, record) }

// ownsLane says whether the steward of self keeps the registered lane: its
// checkout is the lane's checkout or its installation, as landing set
// recorded them (K-a), never guessed.
func ownsLane(self string, record Record) bool {
	layout, err := record.Layout()
	if self == "" || err != nil {
		return false
	}
	here := resolved(self)
	return here == resolved(string(layout.Checkout)) || here == resolved(string(layout.Install))
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
