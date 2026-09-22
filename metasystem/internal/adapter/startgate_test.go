package adapter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStartGateUsesInjectedOpenAndExpiryEvents(t *testing.T) {
	t.Parallel()

	start := time.Unix(100, 0)
	for _, test := range []struct {
		name      string
		openGate  bool
		wantError error
	}{
		{name: "gate opens", openGate: true},
		{name: "timer expires", wantError: ErrStartGateExpired},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := start
			opened := false
			var requested time.Duration
			expiry := make(chan time.Time, 1)
			events := make(chan struct{}, 1)
			timerReady := make(chan struct{})
			deps := StartGateDependencies{
				Now: func() time.Time { return now },
				After: func(duration time.Duration) <-chan time.Time {
					requested = duration
					close(timerReady)
					return expiry
				},
				Events: events, Exists: func() bool { return opened },
			}
			done := make(chan error, 1)
			go func() { _, err := WaitStartGate(context.Background(), time.Second, deps); done <- err }()
			<-timerReady
			now = start.Add(time.Second)
			if test.openGate {
				opened = true
				events <- struct{}{}
			} else {
				expiry <- now
			}
			if err := <-done; !errors.Is(err, test.wantError) {
				t.Fatalf("WaitStartGate error = %v, want %v", err, test.wantError)
			}
			if requested != time.Second {
				t.Fatalf("requested timer = %v, want 1s", requested)
			}
		})
	}
}

func TestStartGatePublishedAtExpiryBoundaryWins(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	expiry := make(chan time.Time, 1)
	timerReady := make(chan struct{})
	type gateOutcome struct {
		result StartGateResult
		err    error
	}
	done := make(chan gateOutcome, 1)
	go func() {
		result, err := WaitStartGate(context.Background(), time.Second, StartGateDependencies{
			Now: time.Now,
			After: func(time.Duration) <-chan time.Time {
				close(timerReady)
				return expiry
			},
			Events: make(chan struct{}),
			Exists: func() bool {
				select {
				case <-gate:
					return true
				default:
					return false
				}
			},
		})
		done <- gateOutcome{result: result, err: err}
	}()
	<-timerReady
	close(gate)
	expiry <- time.Unix(1, 0)
	outcome := <-done
	result, err := outcome.result, outcome.err
	if err != nil || !result.Opened {
		t.Fatalf("published boundary gate result=%+v error=%v", result, err)
	}
}
