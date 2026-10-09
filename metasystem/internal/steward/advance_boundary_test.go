package steward

import (
	"errors"
	"testing"
	"time"
)

func TestBoundaryPreparationRequiresThisPassRearm(t *testing.T) {
	t.Parallel()
	for _, result := range []string{"unknown", "failed", "replacement", "ready"} {
		t.Run(result, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
			deps := idleRunnerDependencies(&now)
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil }
			deps.Sleep = func(d time.Duration) { now = now.Add(d); loop.stop(t) }
			calls := 0
			cfg := TickConfig{Now: now, AdvanceBoundary: func(string) error { calls++; return nil }}
			if result != "unknown" {
				cfg.RearmAtBoundary = func() (bool, error) {
					if result == "failed" {
						return false, errors.New("engine unavailable")
					}
					return result == "replacement", nil
				}
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { return nil }, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			want := 0
			if result == "ready" {
				want = 1
			}
			if calls != want {
				t.Fatalf("%s engine: preparation calls=%d want=%d", result, calls, want)
			}
			// A replacement starts without inheriting its predecessor's observation.
			if result == "replacement" {
				cfg.RearmAtBoundary = func() (bool, error) { return false, nil }
				if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { return nil }, time.Second, cfg, deps); err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("replacement did not resume preparation: %d", calls)
				}
			}
		})
	}
}
