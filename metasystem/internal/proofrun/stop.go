package proofrun

import (
	"fmt"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type StopResult string
type StopSignal string

const (
	StopSignalNone StopSignal = "none"
	StopSignalTerm StopSignal = "term"
	StopSignalKill StopSignal = "kill"

	StopStopped     StopResult = "stopped"
	StopAlreadyGone StopResult = "already-gone"
	StopNotStopped  StopResult = "not-stopped"
)

// StopOutcome reports exactly what the proof-run stop ladder observed for one
// recorded identity.
type StopOutcome struct {
	Component string
	Identity  ProcessIdentity
	Signal    StopSignal
	Result    StopResult
	Reason    string
}

// StopOptions supplies the bounded waits and injectable identity-safe process
// operations used by the proof-run stop ladder.
type StopOptions struct {
	TermGrace time.Duration
	KillGrace time.Duration
	Poll      time.Duration
	Prober    identity.Prober
	Signal    func(int, syscall.Signal) error
}

// Stop acts on a durable proof-run record in suite, watchdog, launcher order.
// Every signal is preceded immediately by proof of the corresponding recorded
// kernel identity.
func Stop(record Record, options StopOptions) []StopOutcome {
	options = stopDefaults(options)
	outcomes := make([]StopOutcome, 0, 3)
	suite := stopOne("suite", record.SuiteProcess, -int(record.SuiteProcess.Pgid), true, options)
	outcomes = append(outcomes, suite)

	for _, item := range []struct {
		name     string
		identity ProcessIdentity
	}{
		{name: "watchdog", identity: record.Watchdog},
		{name: "launcher", identity: record.Launcher},
	} {
		state := waitForIdentity(item.identity.Ref(), options.KillGrace, options)
		if state == identity.Dead {
			outcomes = append(outcomes, StopOutcome{Component: item.name, Identity: item.identity, Signal: StopSignalNone, Result: StopAlreadyGone, Reason: "suite ended"})
			continue
		}
		outcomes = append(outcomes, stopOne(item.name, item.identity, int(item.identity.Pid), false, options))
	}
	return outcomes
}

// StopSuite runs the watchdog's identity-safe suite process-group ladder and
// is also used by a launcher whose second fence read requires cleanup.
func StopSuite(process ProcessIdentity, options StopOptions) StopOutcome {
	options = stopDefaults(options)
	return stopOne("suite", process, -int(process.Pgid), true, options)
}

func stopDefaults(options StopOptions) StopOptions {
	if options.Prober == nil {
		options.Prober = identity.KernelProber{}
	}
	if options.Signal == nil {
		options.Signal = syscall.Kill
	}
	if options.Poll <= 0 {
		options.Poll = 20 * time.Millisecond
	}
	if options.TermGrace <= 0 {
		options.TermGrace = 5 * time.Second
	}
	if options.KillGrace <= 0 {
		options.KillGrace = time.Second
	}
	return options
}

func stopOne(component string, process ProcessIdentity, target int, continueFirst bool, options StopOptions) StopOutcome {
	state := identity.AliveRef(options.Prober, process.Ref())
	if state == identity.Dead {
		return StopOutcome{Component: component, Identity: process, Signal: StopSignalNone, Result: StopAlreadyGone, Reason: "already gone"}
	}
	if state != identity.Alive {
		return StopOutcome{Component: component, Identity: process, Signal: StopSignalNone, Result: StopNotStopped, Reason: "identity could not be inspected"}
	}
	if continueFirst {
		if err := signalRecorded(options, process.Ref(), target, syscall.SIGCONT); err != nil {
			return incompleteOutcome(component, process, StopSignalNone, err)
		}
	}
	if err := signalRecorded(options, process.Ref(), target, syscall.SIGTERM); err != nil {
		return incompleteOutcome(component, process, StopSignalNone, err)
	}
	state = waitForIdentity(process.Ref(), options.TermGrace, options)
	if state == identity.Dead {
		return StopOutcome{Component: component, Identity: process, Signal: StopSignalTerm, Result: StopStopped, Reason: "TERM"}
	}
	if state != identity.Alive {
		return StopOutcome{Component: component, Identity: process, Signal: StopSignalTerm, Result: StopNotStopped, Reason: "death after TERM could not be proved"}
	}
	if err := signalRecorded(options, process.Ref(), target, syscall.SIGKILL); err != nil {
		return incompleteOutcome(component, process, StopSignalTerm, err)
	}
	state = waitForIdentity(process.Ref(), options.KillGrace, options)
	if state == identity.Dead {
		return StopOutcome{Component: component, Identity: process, Signal: StopSignalKill, Result: StopStopped, Reason: "TERM ignored"}
	}
	return StopOutcome{Component: component, Identity: process, Signal: StopSignalKill, Result: StopNotStopped, Reason: fmt.Sprintf("death after KILL was %s", state)}
}

func incompleteOutcome(component string, process ProcessIdentity, signal StopSignal, err error) StopOutcome {
	return StopOutcome{Component: component, Identity: process, Signal: signal, Result: StopNotStopped, Reason: fmt.Sprintf("signal failed: %v", err)}
}

func signalRecorded(options StopOptions, ref identity.Ref, target int, signal syscall.Signal) error {
	if state := identity.AliveRef(options.Prober, ref); state != identity.Alive {
		return fmt.Errorf("%s refused because pid %d no longer has its recorded start identity (%s)", signal, ref.Pid, state)
	}
	if err := options.Signal(target, signal); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

func waitForIdentity(ref identity.Ref, duration time.Duration, options StopOptions) identity.Liveness {
	state := identity.AliveRef(options.Prober, ref)
	if duration <= 0 || state != identity.Alive {
		return state
	}
	deadline := time.Now().Add(duration)
	for state == identity.Alive && time.Now().Before(deadline) {
		time.Sleep(options.Poll)
		state = identity.AliveRef(options.Prober, ref)
	}
	return state
}
