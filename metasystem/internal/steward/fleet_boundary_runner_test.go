package steward

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestFleetBoundaryRunnerOrdersConsumers(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		name := "durable"
		if fail {
			name = "write-failed"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
			deps := idleRunnerDependencies(&now)
			deadline := now.Add(2 * time.Millisecond)
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				now = now.Add(time.Millisecond)
				if !now.Before(deadline) {
					loop.stop(t)
				}
				return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
			}
			var calls []string
			deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
				calls = append(calls, "seat")
				return SeatRecord{}, nil
			}
			deps.Resumable = func(string) (string, bool, error) { return "prepared", true, nil }
			deps.DeliverPending = func(string) (int, error) { calls = append(calls, "delivery"); return 0, nil }
			deps.Channel = func(context.Context, string) (int, error) { return 0, nil }
			cfg := TickConfig{Now: now,
				CompletedBoundary: func(string, time.Time) error {
					calls = append(calls, "boundary")
					if fail {
						return errors.New("boundary write failed")
					}
					return nil
				},
				RearmAtBoundary: func() (bool, error) { calls = append(calls, "engine"); return false, nil },
				KeepLandingLane: func() lane.AgentRun { calls = append(calls, "keeper"); return lane.AgentRun{} },
			}
			deps.Sleep = func(d time.Duration) { now = now.Add(d); loop.stop(t) }
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { calls = append(calls, "revival"); return nil }, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			want := []string{"boundary", "engine", "keeper", "seat", "revival", "delivery"}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("boundary consumer order: %v; want %v", calls, want)
			}
		})
	}
}
