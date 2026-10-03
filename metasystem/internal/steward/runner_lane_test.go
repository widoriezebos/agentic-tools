package steward

import (
	"context"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
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
	config := TickConfig{Now: now, KeepLandingLane: func() lane.AgentRun {
		steps++
		return lane.AgentRun{Outcome: lane.AgentIdle, Line: "the landing lane at /lane is idle; no landing agent runs"}
	}}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 200*time.Millisecond, config, deps); err != nil {
		t.Fatal(err)
	}
	if steps != 3 {
		t.Fatalf("keeper steps = %d; want one per cycle outside the helm (3) and none at it", steps)
	}
}

// TestRunnerStepsAWaitingLaneBetweenCycles: while the lane waits on a proof
// the runner steps its keeper every laneRecheck, so the landing agent wakes
// within it of the proof's end, not at the next ten-minute cycle; an idle
// lane is stepped once per cycle.
func TestRunnerStepsAWaitingLaneBetweenCycles(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	run := func(step func(now time.Time) lane.AgentRun, until time.Duration) (steps int) {
		loop := newHelmLoop(t)
		now := start
		deps := runnerLoopDependencies{
			Tick:           func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil },
			Resumable:      func(string) (string, bool, error) { return "", false, nil },
			DeliverPending: func(string) (int, error) { return 0, nil },
			Channel:        func(context.Context, string) (int, error) { return 0, nil },
			Now:            func() time.Time { return now },
			Sleep: func(d time.Duration) {
				now = now.Add(d)
				if now.Sub(start) >= until {
					loop.stop(t)
				}
			},
		}
		config := TickConfig{Now: start, KeepLandingLane: func() lane.AgentRun { steps++; return step(now) }}
		if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 10*time.Minute, config, deps); err != nil {
			t.Fatal(err)
		}
		return steps
	}

	proofEnds := start.Add(2 * time.Minute)
	var woke time.Time
	run(func(now time.Time) lane.AgentRun {
		if now.Before(proofEnds) {
			return lane.AgentRun{Outcome: lane.AgentHeld, Line: "the landing agent at /lane is not started: tree abc is being proven"}
		}
		if woke.IsZero() {
			woke = now
			return lane.AgentRun{Outcome: lane.AgentStarted, Line: "woke the landing agent at /lane: queued, proof-finished"}
		}
		return lane.AgentRun{Outcome: lane.AgentRunning, Line: "the landing agent is running at /lane"}
	}, 5*time.Minute)
	if woke.IsZero() || woke.Sub(proofEnds) > laneRecheck {
		t.Fatalf("the agent woke at %v; want within %v of the proof's end at %v", woke, laneRecheck, proofEnds)
	}

	idle := run(func(time.Time) lane.AgentRun {
		return lane.AgentRun{Outcome: lane.AgentIdle, Line: "the landing lane at /lane is idle; no landing agent runs"}
	}, 15*time.Minute)
	if idle != 2 {
		t.Fatalf("idle lane keeper steps = %d over a cycle and a half; want one per cycle (2)", idle)
	}
}
