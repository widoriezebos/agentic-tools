package steward

import (
	"errors"
	"testing"
	"time"
)

func TestRunnerDrivesWorkBeforeHelmHold(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	takeHelmFixture(t, loop.root)
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	deps := idleRunnerDependencies(&now)
	ticks, drives := 0, 0
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
		ticks++
		return TickResult{}, errors.New("a failed tick still permits collection")
	}
	deps.Sleep = func(d time.Duration) { now = now.Add(d); loop.stop(t) }
	cfg := TickConfig{Now: now, DriveWork: func(root string) error {
		if root != loop.root || ticks != 1 {
			t.Fatalf("drive root=%s ticks=%d", root, ticks)
		}
		drives++
		return nil
	}}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Second, cfg, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != 1 || drives != 1 {
		t.Fatalf("helm or failed tick suppressed collection: ticks=%d drives=%d", ticks, drives)
	}
}
