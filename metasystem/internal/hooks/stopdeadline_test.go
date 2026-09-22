package hooks

import (
	"context"
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type stopDeadlineProbe struct {
	mu       sync.Mutex
	exact    identity.Exact
	state    identity.Liveness
	err      error
	observed chan struct{}
	once     sync.Once
}

func (p *stopDeadlineProbe) Probe(int64) (identity.Exact, identity.Liveness, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.observed != nil {
		p.once.Do(func() { close(p.observed) })
	}
	return p.exact, p.state, p.err
}

func (p *stopDeadlineProbe) set(state identity.Liveness, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state, p.err = state, err
}

func (p *stopDeadlineProbe) setObservation(exact identity.Exact, state identity.Liveness, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.exact, p.state, p.err = exact, state, err
}

func (p *stopDeadlineProbe) acknowledgeNextProbe() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.observed = make(chan struct{})
	p.once = sync.Once{}
	return p.observed
}

func stopDeadlineExact() identity.Exact {
	return identity.Exact{Pid: 42, StartedAt: time.Unix(100, 123000), ArgvKnown: true, Argv: []string{"bash", "supervision-hook.sh"}}
}

func TestStopDeadlineWaitUsesEventsAndInjectedBoundary(t *testing.T) {
	t.Parallel()

	start := time.Unix(1000, 0)
	now := start.Add(10 * time.Second)
	bootNow := 110 * time.Second
	boundary := StopDeadlineBootBoundary{BootID: "boot-a", Origin: 100 * time.Second, Deadline: 117 * time.Second}
	var requested time.Duration
	expiry := make(chan time.Time, 1)
	events := make(chan struct{}, 1)
	timerReady := make(chan struct{})
	probe := &stopDeadlineProbe{exact: stopDeadlineExact(), state: identity.Alive}
	initialObserved := probe.acknowledgeNextProbe()
	deps := StopDeadlineDependencies{
		Now: func() time.Time { return now }, After: func(duration time.Duration) <-chan time.Time {
			requested = duration
			close(timerReady)
			return expiry
		},
		Events: events, Prober: probe,
		BootClock: func() (string, time.Duration, error) { return "boot-a", bootNow, nil },
	}
	type waitOutcome struct {
		result StopDeadlineWaitResult
		err    error
	}
	done := make(chan waitOutcome, 1)
	go func() {
		result, err := WaitForStopDeadline(context.Background(), 42, boundary, deps)
		done <- waitOutcome{result: result, err: err}
	}()
	<-timerReady
	<-initialObserved
	now = start.Add(17 * time.Second)
	expiry <- now
	outcome := <-done
	result := outcome.result
	if outcome.err != nil {
		t.Fatalf("deadline wait error = %v", outcome.err)
	}
	if result.State != StopDeadlineReached || result.ElapsedSec != 17 || result.Worker == nil || result.Worker.Pid != 42 {
		t.Fatalf("deadline result = %+v", result)
	}
	if requested != 7*time.Second {
		t.Fatalf("requested timer = %v, want 7s", requested)
	}

	expiry = make(chan time.Time, 1)
	timerReady = make(chan struct{})
	deps.After = func(time.Duration) <-chan time.Time { close(timerReady); return expiry }
	bootNow = 200 * time.Second
	boundary = StopDeadlineBootBoundary{BootID: "boot-a", Origin: bootNow, Deadline: bootNow + time.Hour}
	now = start
	probe.set(identity.Alive, nil)
	initialObserved = probe.acknowledgeNextProbe()
	done = make(chan waitOutcome, 1)
	go func() {
		result, err := WaitForStopDeadline(context.Background(), 42, boundary, deps)
		done <- waitOutcome{result: result, err: err}
	}()
	<-timerReady
	<-initialObserved
	probe.set(identity.Dead, nil)
	events <- struct{}{}
	if outcome = <-done; outcome.err != nil || outcome.result.State != StopWorkerCompleted {
		result = outcome.result
		t.Fatalf("completion event result = %+v", result)
	}
}

func TestStopDeadlineWaitRejectsUnusableBootBoundary(t *testing.T) {
	t.Parallel()

	readFailure := errors.New("boot clock denied")
	tests := []struct {
		name      string
		boundary  StopDeadlineBootBoundary
		bootID    string
		bootNow   time.Duration
		bootError error
	}{
		{name: "boot mismatch", boundary: StopDeadlineBootBoundary{BootID: "boot-a", Origin: time.Second, Deadline: 2 * time.Second}, bootID: "boot-b", bootNow: time.Second},
		{name: "boot read failure", boundary: StopDeadlineBootBoundary{BootID: "boot-a", Origin: time.Second, Deadline: 2 * time.Second}, bootError: readFailure},
		{name: "clock before origin", boundary: StopDeadlineBootBoundary{BootID: "boot-a", Origin: 2 * time.Second, Deadline: 3 * time.Second}, bootID: "boot-a", bootNow: time.Second},
		{name: "deadline before origin", boundary: StopDeadlineBootBoundary{BootID: "boot-a", Origin: 3 * time.Second, Deadline: 2 * time.Second}, bootID: "boot-a", bootNow: 3 * time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			afterCalled := false
			_, err := WaitForStopDeadline(context.Background(), 42, test.boundary, StopDeadlineDependencies{
				Now: time.Now,
				After: func(time.Duration) <-chan time.Time {
					afterCalled = true
					return make(chan time.Time)
				},
				BootClock: func() (string, time.Duration, error) { return test.bootID, test.bootNow, test.bootError },
			})
			if err == nil || afterCalled {
				t.Fatalf("WaitForStopDeadline error=%v afterCalled=%v", err, afterCalled)
			}
		})
	}
}

func TestStopDeadlineCleanupUsesExactTermKillAndRefusesUnknown(t *testing.T) {
	t.Parallel()

	probe := &stopDeadlineProbe{exact: stopDeadlineExact(), state: identity.Alive}
	worker := probe.exact.Ref()
	grace := make(chan time.Time, 1)
	events := make(chan struct{}, 1)
	var signals []syscall.Signal
	deps := StopDeadlineDependencies{
		Now: time.Now, After: func(time.Duration) <-chan time.Time { return grace }, Events: events, Prober: probe,
	}
	// Keep the sender explicit without coupling the assertion to syscall.Kill.
	deps.Signal = func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil }
	done := make(chan StopDeadlineCleanupResult, 1)
	go func() { done <- StopDeadlineWorker(context.Background(), &worker, 200*time.Millisecond, deps) }()
	grace <- time.Unix(1, 0)
	result := <-done
	if result.State != StopWorkerKilled || !result.TermSent || !result.KillSent || len(signals) != 2 || signals[0] != syscall.SIGTERM || signals[1] != syscall.SIGKILL {
		t.Fatalf("cleanup result=%+v signals=%v", result, signals)
	}

	probe.set(identity.Alive, nil)
	events = make(chan struct{}, 1)
	signalSent := make(chan struct{}, 1)
	signals = nil
	deps.Events = events
	deps.Signal = func(_ int, signal syscall.Signal) error {
		signals = append(signals, signal)
		signalSent <- struct{}{}
		return nil
	}
	done = make(chan StopDeadlineCleanupResult, 1)
	go func() { done <- StopDeadlineWorker(context.Background(), &worker, time.Hour, deps) }()
	<-signalSent
	probe.set(identity.Dead, nil)
	events <- struct{}{}
	result = <-done
	if result.State != StopWorkerTerminated || !result.TermSent || result.KillSent || len(signals) != 1 || signals[0] != syscall.SIGTERM {
		t.Fatalf("TERM cleanup result=%+v signals=%v", result, signals)
	}

	probe.set(identity.Unknown, errors.New("uninspectable"))
	signals = nil
	result = StopDeadlineWorker(context.Background(), &worker, time.Second, deps)
	if result.State != StopWorkerUnverified || len(signals) != 0 {
		t.Fatalf("unverified cleanup result=%+v signals=%v", result, signals)
	}

	probe.setObservation(identity.Exact{Pid: worker.Pid, StartedAt: stopDeadlineExact().StartedAt, Zombie: true}, identity.Alive, nil)
	result = StopDeadlineWorker(context.Background(), &worker, time.Second, deps)
	if result.State != StopWorkerGone || len(signals) != 0 {
		t.Fatalf("zombie cleanup result=%+v signals=%v", result, signals)
	}

	probe.setObservation(identity.Exact{Pid: worker.Pid, StartedAt: stopDeadlineExact().StartedAt.Add(time.Second)}, identity.Alive, nil)
	result = StopDeadlineWorker(context.Background(), &worker, time.Second, deps)
	if result.State != StopWorkerGone || len(signals) != 0 {
		t.Fatalf("replaced cleanup result=%+v signals=%v", result, signals)
	}
}
