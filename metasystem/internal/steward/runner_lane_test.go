package steward

import (
	"context"
	"testing"
	"time"
)

// TestRunnerKeepsTheLandingLaneOutsideTheHelm (U12): every cycle outside the
// helm the runner gives the landing lane's keeper one step; at the helm the
// machinery acts on nothing, so the keeper does not run.
func TestRunnerKeepsTheLandingLaneOutsideTheHelm(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	sleeps, steps := 0, 0
	deps := runnerLoopDependencies{
		Tick:           func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil },
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		DeliverPending: func(string) (int, error) { return 0, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		Now:            func() time.Time { return now },
		Sleep: func(d time.Duration) {
			now = now.Add(d)
			sleeps++
			switch sleeps {
			case 1, 2:
				return
			case 3:
				takeHelmFixture(t, loop.root)
				return
			}
			loop.stop(t)
		},
	}
	config := TickConfig{Now: now, KeepLandingLane: func() string { steps++; return "the landing lane at /lane is idle; no landing agent runs" }}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 200*time.Millisecond, config, deps); err != nil {
		t.Fatal(err)
	}
	if steps != 3 {
		t.Fatalf("keeper steps = %d; want one per cycle outside the helm (3) and none at it", steps)
	}
}
