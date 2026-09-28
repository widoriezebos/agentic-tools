package applaunch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Outcome is what a stop proved.
type Outcome string

const (
	// StoppedNow: every recorded process is dead, the group has no member,
	// and a probed form's probe is dark.
	StoppedNow Outcome = "stopped-now"
	// AlreadyStopped: there was no run to stop.
	AlreadyStopped Outcome = "already-stopped"
	// StillRunning: the wait ran out with something still alive. Nothing is
	// reported as stopped that is not.
	StillRunning Outcome = "still-running"
	// Unprovable: a recorded process whose identity cannot be proved either
	// way. It is refused by name and never signalled.
	Unprovable Outcome = "unprovable"
)

// StopResult is what a stop did and what it proved.
type StopResult struct {
	Outcome Outcome
	Record  *Record
	Lines   []string
	// Proven says the run is over and its record may be removed once its
	// evidence has been copied.
	Proven bool
}

// StopOptions are the readers and senders one stop needs.
type StopOptions struct {
	Prober Prober
	Group  GroupReader
	Send   identity.SignalFunc
	Probe  func(Contract, string) error
	Now    func() time.Time
	Sleep  func(time.Duration)
	// Wait overrides the contract's stopMs.
	Wait time.Duration
	// RunStop runs the contract's own stop command. The engine's own runner
	// is used when this is nil.
	RunStop     func(command Command, facts Facts, record Record) error
	ProjectRoot string
	Environment []string
}

func (o StopOptions) prober() Prober {
	if o.Prober != nil {
		return o.Prober
	}
	return identity.KernelProber{}
}

func (o StopOptions) group() GroupReader {
	if o.Group != nil {
		return o.Group
	}
	return KernelGroup
}

func (o StopOptions) sleep(d time.Duration) {
	if o.Sleep != nil {
		o.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (o StopOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Stop ends one run and proves it. It removes no record: the caller copies
// the run's evidence first, because an ended record is the last thing that
// says what the run did and what its check found.
func Stop(stateRoot, key string, contract Contract, o StopOptions) (StopResult, error) {
	status, err := Read(stateRoot, key, contract, ReadOptions{Prober: o.Prober, Group: o.Group, Probe: o.Probe, Now: o.Now})
	if err != nil {
		return StopResult{}, err
	}
	result := StopResult{Record: status.Record}
	switch status.State {
	case Stopped:
		result.Outcome, result.Proven = AlreadyStopped, true
		result.Lines = []string{"no run is recorded for " + key}
		return result, nil
	case Unreadable:
		result.Outcome = Unprovable
		result.Lines = []string{"the run record for " + key + " cannot be read: " + status.Problem}
		return result, nil
	case Uninspectable:
		result.Outcome = Unprovable
		result.Lines = []string{"a recorded process of run " + key + " is uninspectable; nothing was signalled"}
		return result, nil
	}
	record := *status.Record
	// A stop has three waits, each derived from the contract's stopMs: the
	// application's own chance to end its descendants, the supervisor's
	// chance to end the group it leads, and the engine's last resort. Each
	// returns the moment what it waits for is true, so the budget costs
	// nothing when the application stops when it is asked to.
	unit := time.Duration(contract.StopWaitMS()) * time.Millisecond
	wait := o.Wait
	if wait <= 0 {
		wait = 3*unit + 2*time.Second
	}
	deadline := o.now().Add(wait)

	if !contract.Stop.Empty() {
		facts := FactsFor(record.Address)
		run := o.RunStop
		if run == nil {
			run = o.execStop
		}
		if err := run(*contract.Stop, facts, record); err != nil {
			result.Lines = append(result.Lines, "the contract's stop command failed: "+err.Error())
		} else {
			result.Lines = append(result.Lines, "ran the contract's stop command")
		}
	} else {
		result.Lines = append(result.Lines, o.endTree(record, unit, deadline)...)
	}

	result.Lines = append(result.Lines, o.waitForDeath(record, deadline)...)
	proof, lines := o.proveStopped(record, contract)
	result.Lines = append(result.Lines, lines...)
	switch {
	case proof == Unprovable:
		result.Outcome = Unprovable
	case proof == StoppedNow:
		result.Outcome, result.Proven = StoppedNow, true
		if record.Ended == nil {
			// A KILL to the group ends the supervisor too, so the ended
			// record is the stop caller's, written from the record the
			// supervisor wrote before it spawned anything.
			ended := &Ended{At: o.now().UTC().Format(time.RFC3339), ExitStatus: "stopped"}
			_ = UpdateRecord(stateRoot, key, func(current *Record) { current.Ended = ended })
			record.Ended = ended
			result.Record = &record
		}
	default:
		result.Outcome = StillRunning
	}
	return result, nil
}

// endTree is the default stop: the application by its re-proven ref, then,
// while anything of the supervisor's own group remains, the supervisor,
// which is the only process that may signal that group.
func (o StopOptions) endTree(record Record, unit time.Duration, deadline time.Time) []string {
	prober := o.prober()
	var lines []string
	ref, recorded, err := record.ChildRef()
	switch {
	case !recorded:
		lines = append(lines, "no application is recorded for this run; nothing was signalled")
	case err != nil:
		lines = append(lines, "the recorded application identity will not parse; it is refused by name and nothing was signalled")
	default:
		switch signalErr := signal(prober, ref, syscall.SIGTERM, o.Send); {
		case signalErr == nil:
			lines = append(lines, "asked the application to stop (TERM, identity re-proven)")
		case errors.Is(signalErr, identity.ErrGone):
			lines = append(lines, "the recorded application (pid "+strconv.FormatInt(ref.Pid, 10)+") is gone; its exact identity no longer matches and nothing was signalled")
		case errors.Is(signalErr, identity.ErrUninspectable):
			lines = append(lines, "the recorded application (pid "+strconv.FormatInt(ref.Pid, 10)+") is uninspectable; it is refused by name and nothing was signalled")
		default:
			lines = append(lines, "the application could not be signalled: "+signalErr.Error())
		}
	}
	// Give the application the chance to end its own descendants first.
	o.waitFor(earlier(o.now().Add(unit), deadline), func() bool { return o.treeQuiet(record) })
	if o.treeQuiet(record) {
		return lines
	}
	supervisorRef, err := record.SupervisorRef()
	if err != nil {
		return append(lines, "the recorded supervisor identity will not parse; the group is refused by name")
	}
	if identity.AliveRef(prober, supervisorRef) != identity.Alive {
		members, _ := o.group()(record.Group)
		lines = append(lines, "the supervisor is gone; an inspection of group "+strconv.FormatInt(record.Group, 10)+" finds:")
		lines = append(lines, memberLines(members)...)
		lines = append(lines, "none of it was signalled: only the group's living leader can prove the group is ours")
		return lines
	}
	switch signalErr := signal(prober, supervisorRef, syscall.SIGTERM, o.Send); {
	case signalErr == nil:
		lines = append(lines, "asked the supervisor to end its own group (TERM, identity re-proven)")
	case errors.Is(signalErr, identity.ErrGone):
		lines = append(lines, "the recorded supervisor is gone; nothing was signalled")
	default:
		lines = append(lines, "the supervisor could not be signalled: "+signalErr.Error())
	}
	// And the supervisor its own chance to end the group it leads.
	o.waitFor(earlier(o.now().Add(unit+time.Second), deadline), func() bool { return o.everythingDead(record) })
	return lines
}

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// waitForDeath escalates to KILL at the deadline: the application first, then
// the supervisor, each by its re-proven ref and never by a bare number.
func (o StopOptions) waitForDeath(record Record, deadline time.Time) []string {
	var lines []string
	prober := o.prober()
	o.waitFor(deadline, func() bool { return o.everythingDead(record) })
	if o.everythingDead(record) {
		return lines
	}
	if ref, recorded, err := record.ChildRef(); recorded && err == nil && identity.AliveRef(prober, ref) == identity.Alive {
		if signal(prober, ref, syscall.SIGKILL, o.Send) == nil {
			lines = append(lines, "the application ignored TERM; it was ended (KILL, identity re-proven)")
		}
	}
	if ref, err := record.SupervisorRef(); err == nil && identity.AliveRef(prober, ref) == identity.Alive {
		if signal(prober, ref, syscall.SIGKILL, o.Send) == nil {
			lines = append(lines, "the supervisor was ended (KILL, identity re-proven)")
		}
	}
	o.waitFor(o.now().Add(2*time.Second), func() bool { return o.everythingDead(record) })
	return lines
}

func (o StopOptions) waitFor(deadline time.Time, done func() bool) {
	for o.now().Before(deadline) {
		if done() {
			return
		}
		o.sleep(100 * time.Millisecond)
	}
}

func (o StopOptions) treeQuiet(record Record) bool {
	prober := o.prober()
	if ref, recorded, err := record.ChildRef(); recorded && err == nil && identity.AliveRef(prober, ref) == identity.Alive {
		return false
	}
	members, err := o.group()(record.Group)
	if err != nil {
		return false
	}
	supervisor, refErr := record.SupervisorRef()
	if refErr != nil {
		return len(members) == 0
	}
	return len(livingBesides(members, supervisor.Pid)) == 0
}

func (o StopOptions) everythingDead(record Record) bool {
	prober := o.prober()
	if ref, recorded, err := record.ChildRef(); recorded && err == nil && identity.AliveRef(prober, ref) != identity.Dead {
		return false
	}
	if ref, err := record.SupervisorRef(); err == nil && identity.AliveRef(prober, ref) != identity.Dead {
		return false
	}
	members, err := o.group()(record.Group)
	return err == nil && len(members) == 0
}

// proveStopped is the whole of the proof: every recorded ref dead, the
// supervisor's among them, the group with no member, and, for the http and
// tcp forms only, the probe dark. A log pattern cannot unmatch, so for the
// log and none forms death alone is the proof.
func (o StopOptions) proveStopped(record Record, contract Contract) (Outcome, []string) {
	prober := o.prober()
	var lines []string
	outcome := StoppedNow
	if ref, recorded, err := record.ChildRef(); recorded {
		if err != nil {
			return Unprovable, append(lines, "the recorded application identity will not parse")
		}
		switch identity.AliveRef(prober, ref) {
		case identity.Alive:
			lines, outcome = append(lines, "the application is still running"), StillRunning
		case identity.Unknown:
			return Unprovable, append(lines, "the application's identity is uninspectable; it is not reported as stopped")
		}
	}
	if ref, err := record.SupervisorRef(); err == nil {
		switch identity.AliveRef(prober, ref) {
		case identity.Alive:
			lines, outcome = append(lines, "the supervisor is still running"), StillRunning
		case identity.Unknown:
			return Unprovable, append(lines, "the supervisor's identity is uninspectable; it is not reported as stopped")
		}
	}
	members, err := o.group()(record.Group)
	if err != nil {
		return Unprovable, append(lines, "group "+strconv.FormatInt(record.Group, 10)+" could not be inspected; it is not reported as stopped")
	}
	if len(members) > 0 {
		lines = append(lines, "group "+strconv.FormatInt(record.Group, 10)+" still has members:")
		lines = append(lines, memberLines(members)...)
		outcome = StillRunning
	}
	if contract.Probed() && o.Probe != nil && record.Address != "" {
		if o.Probe(contract, record.Address) == nil {
			lines = append(lines, "the readiness probe still answers at "+record.Address)
			outcome = StillRunning
		} else {
			lines = append(lines, "the readiness probe is dark")
		}
	}
	if outcome == StoppedNow {
		lines = append(lines, "every recorded process is dead and group "+strconv.FormatInt(record.Group, 10)+" has no member")
	}
	return outcome, lines
}

func memberLines(members []Member) []string {
	lines := make([]string, 0, len(members))
	for _, member := range members {
		lines = append(lines, "  pid "+strconv.FormatInt(member.Pid, 10)+" "+member.Ref)
	}
	if len(lines) == 0 {
		lines = append(lines, "  nothing")
	}
	return lines
}

// execStop runs the contract's own stop command, bounded by the contract's
// stopMs, in the run's directory with the run's facts in its environment.
func (o StopOptions) execStop(command Command, facts Facts, record Record) error {
	argv := facts.Argv(&command)
	if len(argv) == 0 {
		return errors.New("the stop command has no argument vector")
	}
	process := exec.Command(argv[0], argv[1:]...)
	process.Dir = filepath.Join(o.ProjectRoot, filepath.FromSlash(command.CWD))
	process.Env = facts.Environment(o.Environment, record.StateRoot, record.Log)
	if log, err := os.OpenFile(record.Log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		defer log.Close()
		process.Stdout, process.Stderr = log, log
	}
	if err := process.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(stopCommandTimeout):
		_ = process.Process.Kill()
		return errors.New("the stop command did not finish in time")
	}
}

// stopCommandTimeout bounds the contract's own stop command. Every wait has
// a deadline and an owner; this one's owner is the stop that called it.
const stopCommandTimeout = 60 * time.Second
