package steward

// The runner's orderly stop on a signal. `steward disarm` and a replacement
// end the runner with SIGTERM, and a terminal's interrupt sends SIGINT. A
// signal's default action ended the process wherever it stood, and a goal
// transaction killed after MarkPushed left its journal entry pushed with its
// owner dead: every later publish on that clone was refused until something
// ran recovery (fencedflake, 2026-09-30). So the runner catches both: the
// first starts no new work and lets the transaction in progress finish, and
// the loop ends after the tick that holds it. The drain is bounded — a
// second signal or the bound ends the runner at once.

import (
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// StopDrainBound is how long a signalled runner may take to finish the goal
// transaction in progress before it exits regardless. It matches the goal
// transaction's own retry deadline (goal.DefaultPublishDeadline).
const StopDrainBound = 60 * time.Second

// stopSignalSource is every edge of the drain: where signals come from, the
// clock that bounds the drain, the hard exit and the line a person reads.
// Its zero value installs nothing.
type stopSignalSource struct {
	// Notify relays the stop signals to the channel and returns the
	// function that stops relaying.
	Notify func(chan<- os.Signal) func()
	After  func(time.Duration) <-chan time.Time
	Exit   func(int)
	Report func(string)
	Bound  time.Duration
}

// productionStopSignals relays SIGTERM and SIGINT unless the process was
// told to ignore one (the fixture-only METASYSTEM_STEWARD_RUNNER_IGNORE_TERM
// keeps its meaning: an ignored signal stays ignored).
func productionStopSignals() stopSignalSource {
	return stopSignalSource{
		Notify: func(relay chan<- os.Signal) func() {
			var watched []os.Signal
			for _, sig := range []os.Signal{syscall.SIGTERM, syscall.SIGINT} {
				if !signal.Ignored(sig) {
					watched = append(watched, sig)
				}
			}
			if len(watched) == 0 {
				return func() {}
			}
			signal.Notify(relay, watched...)
			return func() { signal.Stop(relay) }
		},
		After:  time.After,
		Exit:   os.Exit,
		Report: func(line string) { fmt.Fprintln(os.Stderr, line) },
		Bound:  StopDrainBound,
	}
}

// runnerDrain is the one bit the loop and the tick read: a stop signal has
// arrived, so nothing new starts.
type runnerDrain struct{ requested atomic.Bool }

func (d *runnerDrain) Requested() bool { return d != nil && d.requested.Load() }

// signalExitCode is the shell's convention for a death by signal.
func signalExitCode(sig os.Signal) int {
	if number, ok := sig.(syscall.Signal); ok {
		return 128 + int(number)
	}
	return 1
}

// watch starts relaying stop signals into drain and returns the function
// that stops relaying; the loop calls it when it ends in order.
func (s stopSignalSource) watch(drain *runnerDrain) func() {
	if s.Notify == nil {
		return func() {}
	}
	bound := s.Bound
	if bound <= 0 {
		bound = StopDrainBound
	}
	report := s.Report
	if report == nil {
		report = func(string) {}
	}
	signals := make(chan os.Signal, 2)
	stopRelay := s.Notify(signals)
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		var first os.Signal
		select {
		case first = <-signals:
		case <-done:
			return
		}
		drain.requested.Store(true)
		report(fmt.Sprintf("steward run: %s received; starting no new work and finishing the goal transaction in progress (at most %s); a second signal ends the runner now", first, bound))
		expired := s.After(bound)
		select {
		case second := <-signals:
			report(fmt.Sprintf("steward run: a second signal (%s); ending now", second))
			s.Exit(signalExitCode(second))
		case <-expired:
			report(fmt.Sprintf("steward run: the work in progress did not finish within %s; ending now", bound))
			s.Exit(signalExitCode(first))
		case <-done:
		}
	}()
	var once atomic.Bool
	return func() {
		if once.Swap(true) {
			return
		}
		stopRelay()
		close(done)
		<-finished
	}
}
