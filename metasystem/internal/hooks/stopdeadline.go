package hooks

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	StopWorkerCompleted  = "worker-completed"
	StopDeadlineReached  = "deadline-reached"
	StopWorkerTerminated = "term-sent"
	StopWorkerKilled     = "kill-sent"
	StopWorkerGone       = "worker-gone"
	StopWorkerUnverified = "worker-unverifiable"
)

// StopDeadlineDependencies is the per-invocation lifetime boundary for one
// Stop worker. Events report process-state changes; the timer remains the only
// authority that declares the deadline reached.
type StopDeadlineDependencies struct {
	Now       func() time.Time
	After     func(time.Duration) <-chan time.Time
	Events    <-chan struct{}
	Prober    identity.Prober
	Signal    identity.SignalFunc
	BootClock func() (string, time.Duration, error)
}

// StopDeadlineBootBoundary preserves the worker's original monotonic lifetime
// across the shell-to-engine process boundary.
type StopDeadlineBootBoundary struct {
	BootID   string
	Origin   time.Duration
	Deadline time.Duration
}

type StopDeadlineWaitResult struct {
	State      string        `json:"state"`
	ElapsedSec int64         `json:"elapsedSec"`
	Worker     *identity.Ref `json:"worker,omitempty"`
}

type StopDeadlineCleanupResult struct {
	State    string `json:"state"`
	TermSent bool   `json:"termSent"`
	KillSent bool   `json:"killSent"`
}

// WaitForStopDeadline observes one pid until it exits or the deadline event
// fires. It records the first exact identity it sees so later cleanup cannot
// signal a reused pid.
func WaitForStopDeadline(ctx context.Context, pid int64, boundary StopDeadlineBootBoundary, deps StopDeadlineDependencies) (StopDeadlineWaitResult, error) {
	if boundary.BootID == "" || boundary.Origin < 0 || boundary.Deadline < boundary.Origin {
		return StopDeadlineWaitResult{}, fmt.Errorf("invalid boot deadline boundary")
	}
	if deps.BootClock == nil {
		return StopDeadlineWaitResult{}, fmt.Errorf("boot clock is unavailable")
	}
	bootID, current, err := deps.BootClock()
	if err != nil {
		return StopDeadlineWaitResult{}, fmt.Errorf("read boot clock: %w", err)
	}
	if bootID != boundary.BootID {
		return StopDeadlineWaitResult{}, fmt.Errorf("boot identity changed")
	}
	if current < boundary.Origin {
		return StopDeadlineWaitResult{}, fmt.Errorf("boot clock precedes deadline origin")
	}
	elapsedAtEntry := current - boundary.Origin
	started := deps.Now().Add(-elapsedAtEntry)
	remaining := boundary.Deadline - current
	if remaining < 0 {
		remaining = 0
	}
	timer := deps.After(remaining)
	var worker *identity.Ref
	observe := func() bool {
		exact, state, _ := deps.Prober.Probe(pid)
		if state == identity.Dead || (state == identity.Alive && exact.Zombie) {
			return true
		}
		if state != identity.Alive {
			return false
		}
		if worker == nil {
			ref := exact.Ref()
			worker = &ref
			return false
		}
		return !identity.SameIdentity(exact, *worker)
	}
	if observe() {
		return stopDeadlineWaitResult(StopWorkerCompleted, started, deps.Now(), worker), nil
	}
	for {
		select {
		case <-ctx.Done():
			return stopDeadlineWaitResult(StopWorkerUnverified, started, deps.Now(), worker), nil
		case <-deps.Events:
			if observe() {
				return stopDeadlineWaitResult(StopWorkerCompleted, started, deps.Now(), worker), nil
			}
		case <-timer:
			if observe() {
				return stopDeadlineWaitResult(StopWorkerCompleted, started, deps.Now(), worker), nil
			}
			return stopDeadlineWaitResult(StopDeadlineReached, started, deps.Now(), worker), nil
		}
	}
}

func stopDeadlineWaitResult(state string, started, now time.Time, worker *identity.Ref) StopDeadlineWaitResult {
	elapsed := now.Sub(started)
	if elapsed < 0 {
		elapsed = 0
	}
	return StopDeadlineWaitResult{State: state, ElapsedSec: int64(elapsed / time.Second), Worker: worker}
}

// StopDeadlineWorker applies the production TERM/KILL ladder to the exact
// identity captured by WaitForStopDeadline. Unknown identity never authorizes
// either signal.
func StopDeadlineWorker(ctx context.Context, worker *identity.Ref, grace time.Duration, deps StopDeadlineDependencies) StopDeadlineCleanupResult {
	if worker == nil {
		return StopDeadlineCleanupResult{State: StopWorkerUnverified}
	}
	result := StopDeadlineCleanupResult{}
	if err := signalStopDeadlineWorker(deps, *worker, syscall.SIGTERM); err != nil {
		if errors.Is(err, identity.ErrGone) {
			result.State = StopWorkerGone
		} else {
			result.State = StopWorkerUnverified
		}
		return result
	}
	result.TermSent = true
	timer := deps.After(grace)
	for {
		select {
		case <-ctx.Done():
			result.State = StopWorkerUnverified
			return result
		case <-deps.Events:
			if stopDeadlineWorkerEnded(deps.Prober, *worker) {
				result.State = StopWorkerTerminated
				return result
			}
		case <-timer:
			err := signalStopDeadlineWorker(deps, *worker, syscall.SIGKILL)
			switch {
			case err == nil:
				result.State, result.KillSent = StopWorkerKilled, true
			case errors.Is(err, identity.ErrGone):
				result.State = StopWorkerTerminated
			default:
				result.State = StopWorkerUnverified
			}
			return result
		}
	}
}

// signalStopDeadlineWorker treats a proved zombie or reused pid as gone at
// the last boundary before signalling. The generic identity helper only
// compares identity tokens; Stop cleanup also owns the zombie classification.
func signalStopDeadlineWorker(deps StopDeadlineDependencies, worker identity.Ref, signal syscall.Signal) error {
	exact, state, err := deps.Prober.Probe(worker.Pid)
	switch {
	case state == identity.Dead:
		return identity.ErrGone
	case err != nil || state == identity.Unknown:
		return identity.ErrUninspectable
	case exact.Zombie || !identity.SameIdentity(exact, worker):
		return identity.ErrGone
	}
	sender := deps.Signal
	if sender == nil {
		sender = syscall.Kill
	}
	if err := sender(int(worker.Pid), signal); errors.Is(err, syscall.ESRCH) {
		return identity.ErrGone
	} else {
		return err
	}
}

func stopDeadlineWorkerEnded(prober identity.Prober, worker identity.Ref) bool {
	exact, state, _ := prober.Probe(worker.Pid)
	return state == identity.Dead || state == identity.Alive && (exact.Zombie || !identity.SameIdentity(exact, worker))
}
