package steward

import (
	"context"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
)

func TestRunnerReviewWorkBeforeHelm(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	now := helmFixtureClock
	observed := []bool{}
	sleeps := 0
	cfg := TickConfig{Now: now, ReviewWork: func(root string) error {
		observed = append(observed, helm.Active(root).Active)
		return nil
	}}
	deps := runnerLoopDependencies{
		Tick:           func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil },
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		DeliverPending: func(string) (int, error) { return 0, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		Now:            func() time.Time { return now },
		Sleep: func(d time.Duration) {
			now = now.Add(d)
			sleeps++
			if sleeps == 1 {
				takeHelmFixture(t, loop.root)
			} else {
				loop.stop(t)
			}
		},
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 200*time.Millisecond, cfg, deps); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 2 || observed[0] || !observed[1] {
		t.Fatalf("review observation at helm = %v; want one free pass and one held pass", observed)
	}
}
