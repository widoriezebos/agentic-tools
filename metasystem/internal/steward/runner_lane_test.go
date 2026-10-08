package steward

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func TestRunnerWaitProviderLimit(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, class string
		wait        time.Duration
		calls       int
	}{
		{"recovers", outage.ProviderLimit, 4 * time.Minute, 2},
		{"removed", outage.ProviderLimit, 165 * time.Second, 1},
		{"lapsed", outage.ProviderLimit, 165 * time.Second, 1},
		{"fails", outage.ProviderLimit, 10 * time.Minute, 4},
		{"helm", outage.ProviderLimit, 10 * time.Minute, 0},
		{"overloaded", "overloaded", 10 * time.Minute, 0},
		{"5xx", "http-503", 10 * time.Minute, 0},
		{"nil", outage.ProviderLimit, 10 * time.Minute, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			start := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
			now, calls := start, 0
			last := start
			if test.name == "lapsed" {
				last = start.Add(-outage.Horizon + 155*time.Second)
			}
			mark, err := outage.Record(loop.root, test.class, "limited", "fixture", last)
			if err != nil {
				t.Fatal(err)
			}
			if test.name == "helm" {
				takeHelmFixture(t, loop.root)
			}
			probe := func(top string) (bool, error) {
				calls++
				if got, _ := outage.Read(top); got != mark {
					t.Fatalf("failed probe changed the mark: %#v", got)
				}
				if test.name == "fails" {
					return false, errors.New("provider unavailable")
				}
				return test.name == "recovers" && calls == 2, nil
			}
			if test.name == "nil" {
				probe = nil
			}
			deps := runnerLoopDependencies{Now: func() time.Time { return now }, Sleep: func(d time.Duration) {
				now = now.Add(d)
				if test.name == "removed" && now.Sub(start) == 155*time.Second {
					if err := outage.Clear(loop.root); err != nil {
						t.Fatal(err)
					}
				}
			}}
			if stopped := runnerWait(loop.root, 10*time.Minute, deps, nil, nil, probe); stopped || now.Sub(start) != test.wait || calls != test.calls {
				t.Fatalf("stopped=%v, waited=%v, calls=%d; want false, %v, %d", stopped, now.Sub(start), calls, test.wait, test.calls)
			}
			_, err = os.Stat(outage.Path(loop.root))
			if (test.name == "recovers" || test.name == "removed") && !os.IsNotExist(err) {
				t.Fatalf("mark remains: %v", err)
			}
		})
	}
}

func TestRunnerProviderRecoveryStartsTheNextCycle(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	start := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	if _, err := outage.Record(loop.root, outage.ProviderLimit, "limited", "fixture", start); err != nil {
		t.Fatal(err)
	}
	now, ticks := start, 0
	deps := runnerLoopDependencies{
		Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
			ticks++
			if ticks == 2 {
				loop.stop(t)
			}
			return TickResult{}, nil
		},
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		DeliverPending: func(string) (int, error) { return 0, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		Now:            func() time.Time { return now }, Sleep: func(d time.Duration) { now = now.Add(d) },
	}
	config := TickConfig{Now: start, ProbeProvider: func(string) (bool, error) { return true, nil }}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 10*time.Minute, config, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != 2 || now.Sub(start) != 2*time.Minute {
		t.Fatalf("ticks=%d, waited=%v; want 2, 2m", ticks, now.Sub(start))
	}
}

// The runner observes the landing lane each cycle, including at the helm.
// The keeper admits only a recorded person selection through that fence.
func TestRunnerKeepsTheLandingLaneAcrossTheHelm(t *testing.T) {
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
	if steps != 4 {
		t.Fatalf("keeper steps = %d; want one per cycle, including the cycle at the helm (4)", steps)
	}
}

// TestRunnerStepsAWaitingLaneBetweenCycles: while the lane waits on a proof
// or, idle, on a hand-in the runner steps its keeper every laneRecheck, so
// the landing agent wakes within it of the proof's end or of the hand-in,
// not at the next ten-minute cycle.
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

	handIn := start.Add(2 * time.Minute)
	woke = time.Time{}
	run(func(now time.Time) lane.AgentRun {
		if now.Before(handIn) {
			return lane.AgentRun{Outcome: lane.AgentIdle, Line: "the landing lane at /lane is idle; no landing agent runs"}
		}
		if woke.IsZero() {
			woke = now
			return lane.AgentRun{Outcome: lane.AgentStarted, Line: "woke the landing agent at /lane: queued"}
		}
		return lane.AgentRun{Outcome: lane.AgentRunning, Line: "the landing agent is running at /lane"}
	}, 5*time.Minute)
	if woke.IsZero() || woke.Sub(handIn) > laneRecheck {
		t.Fatalf("the agent woke at %v; want within %v of the hand-in at %v", woke, laneRecheck, handIn)
	}
}
