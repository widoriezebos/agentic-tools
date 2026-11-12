package steward

import (
	"testing"
	"time"
)

func TestRunnerFollowReplacementEndsNormalAndHelmPass(t *testing.T) {
	t.Parallel()
	for _, held := range []bool{false, true} {
		name := "normal"
		if held {
			name = "helm"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			if held {
				takeHelmFixture(t, loop.root)
			}
			now := time.Date(2026, 11, 11, 11, 0, 0, 0, time.UTC)
			deps := idleRunnerDependencies(&now)
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil }
			follows, rebuilds := 0, 0
			cfg := TickConfig{Prober: seatBusyDeadProber{},
				FollowDeployedEngine: func() (bool, bool, error) { follows++; loop.stop(t); return true, false, nil },
				RearmAtBoundary:      func() (bool, error) { rebuilds++; loop.stop(t); return false, nil },
			}
			deps.Sleep = func(time.Duration) { loop.stop(t) }
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			if follows != 1 || rebuilds != 0 {
				t.Fatalf("follow=%d rebuild=%d", follows, rebuilds)
			}
		})
	}
}
