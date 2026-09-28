package adapter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var ErrStartGateExpired = errors.New("start gate deadline expired")

type StartGateDependencies struct {
	Now    func() time.Time
	After  func(time.Duration) <-chan time.Time
	Events <-chan struct{}
	Exists func() bool
}

type StartGateResult struct {
	Opened    bool
	ElapsedMS int64
}

// WaitStartGate waits for the runner-owned gate event. The timer decides only
// after one last existence check, so an already-published gate wins at the
// boundary even if its notification and the timer become ready together.
func WaitStartGate(ctx context.Context, timeout time.Duration, deps StartGateDependencies) (StartGateResult, error) {
	started := deps.Now()
	result := func(open bool) StartGateResult {
		elapsed := deps.Now().Sub(started)
		if elapsed < 0 {
			elapsed = 0
		}
		return StartGateResult{Opened: open, ElapsedMS: elapsed.Milliseconds()}
	}
	if deps.Exists() {
		return result(true), nil
	}
	timer := deps.After(timeout)
	for {
		select {
		case <-ctx.Done():
			return result(false), ctx.Err()
		case <-deps.Events:
			if deps.Exists() {
				return result(true), nil
			}
		case <-timer:
			if deps.Exists() {
				return result(true), nil
			}
			return result(false), ErrStartGateExpired
		}
	}
}

// StartGateFixtureExpiryEventEnvironment names a fixture-only file whose
// appearance stands in for the gate timer, so a fixture can expire a gate
// without waiting for wall time.
const StartGateFixtureExpiryEventEnvironment = "METASYSTEM_START_GATE_EXPIRY_EVENT"

// WaitStartGateFile waits for the runner-owned gate file at path, polling
// every poll, for at most timeout. It returns ErrStartGateExpired when the
// deadline passes first. getenv supplies the fixture expiry event, honored
// only in a fake-runtime root with its exact fixture owner live.
func WaitStartGateFile(root, path string, timeout, poll time.Duration, getenv func(string) string) error {
	if path == "" || timeout <= 0 || poll <= 0 {
		return errors.New("start gate wait requires a path, a positive timeout and a positive poll interval")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	after := time.After
	if eventPath := getenv(StartGateFixtureExpiryEventEnvironment); eventPath != "" {
		if root == "" || !fixtureauth.FixtureModeRoot(root) {
			return errors.New("fixture expiry event requires a fake-runtime root")
		}
		if err := liveFixtureOwner(getenv(identity.FixtureOwnerEnv)); err != nil {
			return errors.New("fixture expiry event requires its exact owner")
		}
		after = func(time.Duration) <-chan time.Time {
			fired := make(chan time.Time, 1)
			go func() {
				ticker := time.NewTicker(poll)
				defer ticker.Stop()
				for {
					if _, err := os.Stat(eventPath); err == nil {
						fired <- time.Now()
						return
					}
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
			}()
			return fired
		}
	}
	_, err := WaitStartGate(ctx, timeout, StartGateDependencies{
		Now: time.Now, After: after, Events: startGateTicks(ctx, poll),
		Exists: func() bool { _, err := os.Stat(path); return err == nil },
	})
	return err
}

func startGateTicks(ctx context.Context, interval time.Duration) <-chan struct{} {
	events := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(events)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case events <- struct{}{}:
				default:
				}
			}
		}
	}()
	return events
}

// liveFixtureOwner accepts only an encoded fixture owner that is the live
// exact process it names.
func liveFixtureOwner(value string) error {
	key, err := identity.ParseKey(value)
	if err != nil {
		return fmt.Errorf("fixture lifetime owner is malformed: %w", err)
	}
	exact, state, err := identity.KernelProber{}.Probe(key.Owner.Pid)
	if err != nil || state != identity.Alive || !identity.SameIdentity(exact, key.Owner) {
		return errors.New("fixture lifetime owner does not match the live exact process")
	}
	return nil
}
