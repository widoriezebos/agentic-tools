package run

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

const (
	StopSignalNone = "none"
	StopSignalTerm = "term"
	StopSignalKill = "kill"

	StopResultStopped     = "stopped"
	StopResultAlreadyGone = "already-gone"
	StopResultNotStopped  = "not-stopped"
)

// StopOutcome is the durable account of one monitored-run stop attempt.
type StopOutcome struct {
	RunID         string `json:"runId"`
	InitialStatus string `json:"initialStatus"`
	Status        string `json:"status"`
	Custody       string `json:"custody"`
	PID           int64  `json:"pid,omitempty"`
	PIDStartedAt  int64  `json:"pidStartedAt,omitempty"`
	PIDStartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID        string `json:"bootId,omitempty"`
	PGID          int64  `json:"pgid,omitempty"`
	Signal        string `json:"signal"`
	Result        string `json:"result"`
	Reason        string `json:"reason"`
}

// Creation holds the generation and durable claim for one run creator.
// It is deliberately independent of the transition lock and can span a
// process start.
type Creation struct {
	Generation int64
	claim      *stopfence.Claim
}

func (c *Creation) Close() error {
	if c == nil {
		return nil
	}
	return c.claim.Close()
}

// StoppedError is the two-line refusal shared by run creation paths.
type StoppedError struct {
	Root   string
	Record stopfence.Record
	Thing  string
	Raced  bool
}

func (e *StoppedError) Error() string {
	first, err := stopfence.ClosedDescription(e.Record, e.Root)
	if err != nil {
		return "cannot render stopped-run refusal: " + err.Error()
	}
	if e.Raced {
		first += fmt.Sprintf("; while %s started, it has been ended", e.Thing)
	}
	command, err := stopfence.ClosedCommand(e.Record, e.Root)
	if err != nil {
		return "cannot render stopped-run refusal: " + err.Error()
	}
	return first + "\nat an agent-free terminal, run: " + command
}

// RearmedError reports the generation race in which a stop completed and a
// human arm reopened the checkout before a creator's second fence read.
type RearmedError struct {
	Root  string
	Thing string
}

func (e *RearmedError) Error() string {
	return fmt.Sprintf("the checkout %s was stopped and armed again while %s started; %s has been ended; the caller may retry", e.Root, e.Thing, e.Thing)
}

func (s *Store) readFence() (stopfence.Record, error) {
	if s.FenceRead != nil {
		return s.FenceRead(s.Root)
	}
	return stopfence.Read(s.Root)
}

func stoppedAt(root string, record stopfence.Record) error {
	return &StoppedError{Root: root, Record: record}
}

// BeginCreation reads the fence and publishes a completion-barrier claim.
func (s *Store) BeginCreation(verb string) (*Creation, error) {
	record, err := s.readFence()
	if err != nil {
		return nil, fmt.Errorf("stop fence cannot be read: %w", err)
	}
	if record.State == stopfence.StateClosed {
		return nil, stoppedAt(s.Root, record)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return nil, fmt.Errorf("creation claim identity cannot be proven")
	}
	claim, err := stopfence.Creating(s.Root, verb, record.Generation, exact.Ref())
	if err != nil {
		return nil, err
	}
	return &Creation{Generation: record.Generation, claim: claim}, nil
}

func (s *Store) verifyOpenGeneration(generation int64) (stopfence.Record, error) {
	record, err := s.readFence()
	if err != nil {
		return record, fmt.Errorf("stop fence cannot be read: %w", err)
	}
	if record.State == stopfence.StateClosed {
		return record, &StoppedError{Root: s.Root, Record: record}
	}
	if record.Generation != generation {
		return record, &RearmedError{Root: s.Root}
	}
	return record, nil
}

// CompleteLaunch performs the creator's second fence read after the wrapper
// has started. A raced launch is ended through the ordinary run stop path.
func (s *Store) CompleteLaunch(id string, generation int64) error {
	_, verificationErr := s.verifyOpenGeneration(generation)
	if verificationErr == nil {
		return nil
	}
	outcome, stopErr := s.stopWithNote(id, "stopped by metasystem stop")
	if stopErr != nil {
		return stopErr
	}
	if outcome.Result == StopResultNotStopped {
		return fmt.Errorf("run %s survived the stop race: %s", id, outcome.Reason)
	}
	var stopped *StoppedError
	if errors.As(verificationErr, &stopped) {
		return &StoppedError{Root: s.Root, Record: stopped.Record, Thing: "run " + id, Raced: true}
	}
	var rearmed *RearmedError
	if errors.As(verificationErr, &rearmed) {
		return &RearmedError{Root: s.Root, Thing: "run " + id}
	}
	return fmt.Errorf("%v; run %s has been ended", verificationErr, id)
}

func (s *Store) failForeignCreation(id string) error {
	return s.withLock(func() error {
		record, err := s.Read(id)
		if err != nil || record == nil {
			return err
		}
		if Terminal(record.Status) {
			return nil
		}
		note := "stopped by metasystem stop"
		var result AssessResult
		if record.Status == StatusLaunching {
			return s.terminalize(record, StatusLaunchFailed, nil, &note, &result)
		}
		return s.terminalizeWithVerdict(record, StatusEndedUnknown, nil, &note, &result)
	})
}

func (s *Store) completeForeignCreation(id string, generation int64) error {
	_, verificationErr := s.verifyOpenGeneration(generation)
	if verificationErr == nil {
		return nil
	}
	if err := s.failForeignCreation(id); err != nil {
		return err
	}
	var stopped *StoppedError
	if errors.As(verificationErr, &stopped) {
		return &StoppedError{Root: s.Root, Record: stopped.Record, Thing: "run " + id, Raced: true}
	}
	var rearmed *RearmedError
	if errors.As(verificationErr, &rearmed) {
		return &RearmedError{Root: s.Root, Thing: "run " + id}
	}
	return fmt.Errorf("%v; run %s record has been failed", verificationErr, id)
}

var stopSignal = func(pgid int64, signal syscall.Signal) error {
	return syscall.Kill(int(-pgid), signal)
}

type stopMechanism struct {
	proof           func(int64, string) (bool, bool)
	signal          func(int64, syscall.Signal) error
	escalate        bool
	proofBeforeGone bool
}

// Stop ends one monitored run without ever signalling adopted custody.
func (s *Store) Stop(id string) (StopOutcome, error) {
	return s.stopWithNote(id, "stopped by metasystem stop")
}

func (s *Store) stopWithNote(id, note string) (StopOutcome, error) {
	var outcome StopOutcome
	err := s.withLock(func() error {
		record, err := s.Read(id)
		if err != nil {
			return err
		}
		if record == nil {
			return fmt.Errorf("no run record %s", id)
		}
		held := s.stopHeld(record, note, stopMechanism{
			proof: s.groupOwnsNonce, signal: stopSignal, escalate: true,
		})
		outcome = held.StopOutcome
		if held.err != nil || held.Result == StopResultNotStopped {
			return held.err
		}
		// The stop outcome is also the family renderer's source of truth.
		// Refresh the status after conclusion so the printed verdict cannot
		// disagree with the record just written.
		concluded, err := s.Read(id)
		if err != nil {
			return err
		}
		if concluded == nil {
			return fmt.Errorf("no run record %s after stop", id)
		}
		outcome.Status = concluded.Status
		return nil
	})
	return outcome, err
}

type heldStopOutcome struct {
	StopOutcome
	err error
}

func outcomeFor(record *Record) StopOutcome {
	outcome := StopOutcome{RunID: record.RunId, InitialStatus: record.Status, Status: record.Status, Custody: record.Custody, Signal: StopSignalNone}
	if record.Pid != nil {
		outcome.PID = *record.Pid
	}
	if record.PidStartedAt != nil {
		outcome.PIDStartedAt = *record.PidStartedAt
	}
	outcome.PIDStartTicks = record.PidStartTicks
	outcome.BootID = record.BootID
	if record.Pgid != nil {
		outcome.PGID = *record.Pgid
	}
	return outcome
}

func (s *Store) stopHeld(record *Record, note string, mechanism stopMechanism) heldStopOutcome {
	outcome := heldStopOutcome{StopOutcome: outcomeFor(record)}
	if Terminal(record.Status) {
		outcome.Result, outcome.Reason = StopResultAlreadyGone, "record is terminal"
		return outcome
	}
	if record.Status == StatusLaunching {
		var assessed AssessResult
		outcome.err = s.terminalize(record, StatusLaunchFailed, nil, &note, &assessed)
		outcome.Result, outcome.Reason = StopResultStopped, "launch failed"
		return outcome
	}
	if record.Custody != CustodyWrapped {
		if record.Pid != nil && record.PidStartedAt != nil {
			ref := identity.Ref{Pid: *record.Pid, StartedAtSec: *record.PidStartedAt, StartTicks: record.PidStartTicks, BootID: record.BootID}
			if identity.AliveRef(s.prober(), ref) == identity.Dead && (record.Pgid == nil || s.groupEmpty(*record.Pgid)) {
				outcome.err = s.concludeStoppedHeld(record, note)
				outcome.Result, outcome.Reason = StopResultAlreadyGone, "leader and group already gone"
				return outcome
			}
		}
		outcome.Result = StopResultNotStopped
		outcome.Reason = "not the metasystem's process, not signalled"
		return outcome
	}
	if record.Pgid == nil || *record.Pgid <= 1 {
		outcome.Result, outcome.Reason = StopResultNotStopped, "wrapped run has no signalable recorded group"
		return outcome
	}
	pgid := *record.Pgid
	if !mechanism.proofBeforeGone && s.groupEmpty(pgid) {
		outcome.err = s.concludeStoppedHeld(record, note)
		outcome.Result, outcome.Reason = StopResultAlreadyGone, "group already gone"
		return outcome
	}
	owned, provable := mechanism.proof(pgid, record.LaunchNonce)
	if !provable || !owned {
		outcome.Result = StopResultNotStopped
		if !provable {
			outcome.Reason = "group ownership is unproven"
		} else {
			outcome.Reason = "group ownership is disproven"
		}
		return outcome
	}
	if err := mechanism.signal(pgid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		outcome.Result, outcome.Reason, outcome.err = StopResultNotStopped, "TERM failed", err
		return outcome
	}
	outcome.Signal = StopSignalTerm
	if !s.waitGroupEmpty(pgid, scaledStopDuration(5*time.Second)) {
		if !mechanism.escalate {
			outcome.Result, outcome.Reason = StopResultNotStopped, "group survived TERM"
			return outcome
		}
		owned, provable = mechanism.proof(pgid, record.LaunchNonce)
		if !provable || !owned {
			outcome.Result, outcome.Reason = StopResultNotStopped, "group ownership could not be re-proven before KILL"
			return outcome
		}
		if err := mechanism.signal(pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			outcome.Result, outcome.Reason, outcome.err = StopResultNotStopped, "KILL failed", err
			return outcome
		}
		outcome.Signal = StopSignalKill
		if !s.waitGroupEmpty(pgid, scaledStopDuration(500*time.Millisecond)) {
			outcome.Result, outcome.Reason = StopResultNotStopped, "group survived KILL"
			return outcome
		}
	}
	outcome.err = s.concludeStoppedHeld(record, note)
	outcome.Result = StopResultStopped
	if outcome.Signal == StopSignalKill {
		outcome.Reason = "TERM ignored"
	} else {
		outcome.Reason = "TERM"
	}
	return outcome
}

func (s *Store) concludeStoppedHeld(record *Record, note string) error {
	if Terminal(record.Status) {
		return nil
	}
	var assessed AssessResult
	if record.Status == StatusLaunching {
		return s.terminalize(record, StatusLaunchFailed, nil, &note, &assessed)
	}
	// Once the wrapper group is gone, preserve terminal evidence that arrived
	// while the orderly stop was in flight. An absent sidecar is not evidence:
	// that case still takes the stop-specific ended-unknown fallback and note.
	current := record
	assess := record.Status == StatusDraining
	if record.Status == StatusRunning && record.Evidence.Mode == EvidenceSidecar {
		sidecar, err := s.readSidecar(record)
		// A malformed or unreadable sidecar is still evidence owned by the
		// conclusion engine: provisional records its precise diagnostic and
		// concludes ended-unknown. Only a genuinely absent sidecar takes the
		// stop-specific fallback note below.
		assess = sidecar != nil || err != nil
	}
	if assess {
		if err := s.assessHeld(record.RunId, &assessed); err != nil {
			return err
		}
		var err error
		current, err = s.Read(record.RunId)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("no run record %s", record.RunId)
		}
		if Terminal(current.Status) {
			return nil
		}
	}
	return s.terminalizeWithVerdict(current, StatusEndedUnknown, nil, &note, &assessed)
}

func (s *Store) waitGroupEmpty(pgid int64, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for {
		if s.groupEmpty(pgid) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		poll := scaledStopDuration(100 * time.Millisecond)
		if poll > time.Until(deadline) {
			poll = time.Until(deadline)
		}
		time.Sleep(poll)
	}
}

func scaledStopDuration(base time.Duration) time.Duration {
	scale := int64(1000)
	if raw := os.Getenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			scale = parsed
		}
	}
	d := time.Duration((int64(base)*scale + 999) / 1000)
	if d < time.Millisecond {
		return time.Millisecond
	}
	return d
}

func (s *Store) groupOwnsNonce(pgid int64, nonce string) (owned, provable bool) {
	pids, err := s.allPids()
	if err != nil {
		return false, false
	}
	observations := 0
	for _, pid := range pids {
		group, err := s.getpgid(pid)
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return false, false
		}
		if group != pgid {
			continue
		}
		exact, state, _ := s.prober().Probe(pid)
		if state != identity.Alive || !exact.ArgvKnown {
			return false, false
		}
		observations++
		if strings.Contains(strings.Join(exact.Argv, " "), nonce) {
			return true, true
		}
	}
	return false, observations > 0
}
