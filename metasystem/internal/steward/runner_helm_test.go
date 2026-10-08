package steward

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestRunnerRefreshOutcomePreservesPass(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"unchanged", "refused", "replaced"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
			var keeps, starts, revives, delivers, channels, trims int
			deps := idleRunnerDependencies(&now)
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
			}
			deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
				starts++
				return SeatRecord{}, nil
			}
			deps.Resumable = func(string) (string, bool, error) { return "prepared", true, nil }
			deps.DeliverPending = func(string) (int, error) { delivers++; return 0, nil }
			deps.Channel = func(context.Context, string) (int, error) { channels++; return 0, nil }
			deps.TrimCaches = func(context.Context, string, TickConfig) error { trims++; return nil }
			cfg := TickConfig{Now: now,
				KeepLandingLane: func() lane.AgentRun { keeps++; return lane.AgentRun{} },
				RearmAtBoundary: func() (bool, error) {
					loop.stop(t)
					if outcome == "refused" {
						return false, errors.New("engine source has local edits")
					}
					return outcome == "replaced", nil
				},
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { revives++; return nil }, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			want := [6]int{1, 1, 1, 1, 1, 1}
			if outcome == "replaced" {
				want = [6]int{}
			}
			if got := [6]int{keeps, starts, revives, delivers, channels, trims}; got != want {
				t.Fatalf("post-refresh pass (keeper, seat start, revival, delivery, channel, trim) = %v; want %v", got, want)
			}
		})
	}
}

// helmLoop drives runLoopWithDependencies with every post-tick act injected
// and counted, on an artificial clock: each iteration sleeps once, and
// afterSleep(iteration) runs inside that sleep.
type helmLoop struct {
	root                                       string
	ticks, resumable, revives, delivers, chans int
	calls                                      [][4]int // post-tick calls seen per iteration, in order Resumable, revive, DeliverPending, Channel
}

func newHelmLoop(t *testing.T) *helmLoop {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &helmLoop{root: canonicalPath(root)}
}

func (l *helmLoop) run(t *testing.T, tick func(int) (TickResult, error), afterSleep func(int)) {
	t.Helper()
	now := time.Date(2026, 9, 28, 19, 0, 0, 0, time.UTC)
	snapshot := func() [4]int { return [4]int{l.resumable, l.revives, l.delivers, l.chans} }
	var before [4]int
	deps := runnerLoopDependencies{
		Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
			l.ticks++
			before = snapshot()
			return tick(l.ticks)
		},
		Resumable:      func(string) (string, bool, error) { l.resumable++; return "prepared", true, nil },
		DeliverPending: func(string) (int, error) { l.delivers++; return 0, nil },
		Channel:        func(context.Context, string) (int, error) { l.chans++; return 0, nil },
		Now:            func() time.Time { return now },
		Sleep: func(d time.Duration) {
			now = now.Add(d)
			after := snapshot()
			l.calls = append(l.calls, [4]int{after[0] - before[0], after[1] - before[1], after[2] - before[2], after[3] - before[3]})
			afterSleep(l.ticks)
		},
	}
	revive := func() error { l.revives++; return nil }
	if err := runLoopWithDependencies(l.root, fakeCensus{}, revive, 200*time.Millisecond, TickConfig{Now: now}, deps); err != nil {
		t.Fatal(err)
	}
}

func (l *helmLoop) stop(t *testing.T) {
	if err := stopRunnerLoop(l.root); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerLoopUnderHelmSkipsPostTick(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	takeHelmFixture(t, loop.root)
	loop.run(t, func(int) (TickResult, error) {
		return TickResult{Decision: Decision{VerdictHelm, ActNone, "human at the helm"}}, nil
	}, func(iteration int) {
		if iteration == 1 {
			if _, err := helm.Remove(loop.root); err != nil {
				t.Fatal(err)
			}
			return
		}
		loop.stop(t)
	})
	if len(loop.calls) != 2 || loop.calls[0] != [4]int{} || loop.calls[1] != [4]int{1, 1, 1, 1} {
		t.Fatalf("HM-7: post-tick calls per iteration (Resumable, revive, DeliverPending, Channel) = %v; want none under the helm, all four after return", loop.calls)
	}
}

func TestRunnerLoopUnderHelmSkipsPostTickWhenTickFails(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	takeHelmFixture(t, loop.root)
	loop.run(t, func(int) (TickResult, error) {
		return TickResult{}, errors.New("the tick failed before any decision")
	}, func(int) { loop.stop(t) })
	if len(loop.calls) != 1 || loop.calls[0] != [4]int{} {
		t.Fatalf("HM-13: a failed tick under the helm let post-tick acts run: %v", loop.calls)
	}
}

func TestRunnerLoopHelmTakenDuringTickSkipsPostTick(t *testing.T) {
	t.Parallel()
	for name, answer := range map[string]struct {
		result TickResult
		err    error
	}{
		"HM-13 tick error": {err: errors.New("the tick failed after the take")},
		"HM-13 ActRevive":  {result: TickResult{Decision: Decision{VerdictStalledDead, ActRevive, "dead"}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			intent := testIntent("taken-during-tick")
			loop.run(t, func(int) (TickResult, error) {
				// The tick prepares an intent, and the person takes the helm
				// before the tick returns.
				if err := PrepareIntent(loop.root, filepath.Join(loop.root, "memory", "receipts.log"), intent); err != nil {
					t.Fatal(err)
				}
				takeHelmFixture(t, loop.root)
				return answer.result, answer.err
			}, func(int) { loop.stop(t) })
			if len(loop.calls) != 1 || loop.calls[0] != [4]int{} {
				t.Fatalf("HM-7: a take during the tick let post-tick acts run: %v", loop.calls)
			}
			if live, err := LiveIntents(loop.root); err != nil || len(live) != 1 || live[0].Nonce != intent.Nonce {
				t.Fatalf("HM-7: the prepared intent is no longer on disk: %+v %v", live, err)
			}
		})
	}
}

func TestRunnerLoopUnderHelmKeepsLaneWithoutOtherPostTickActs(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	takeHelmFixture(t, loop.root)
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	keeps, rearms, starts, revives, delivers, channels, trims, resumes := 0, 0, 0, 0, 0, 0, 0, 0
	deps := idleRunnerDependencies(&now)
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
		return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
	}
	deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
		starts++
		return SeatRecord{}, nil
	}
	deps.Resumable = func(string) (string, bool, error) { resumes++; return "prepared", true, nil }
	deps.DeliverPending = func(string) (int, error) { delivers++; return 0, nil }
	deps.Channel = func(context.Context, string) (int, error) { channels++; return 0, nil }
	deps.TrimCaches = func(context.Context, string, TickConfig) error { trims++; return nil }
	cfg := TickConfig{Now: now, KeepLandingLane: func() lane.AgentRun { keeps++; loop.stop(t); return lane.AgentRun{Outcome: lane.AgentIdle} }, RearmAtBoundary: func() (bool, error) { rearms++; return false, nil }}
	// A skipped keeper still terminates deterministically on the injected clock.
	sleep := deps.Sleep
	deps.Sleep = func(d time.Duration) { sleep(d); loop.stop(t) }
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { revives++; return nil }, time.Second, cfg, deps); err != nil {
		t.Fatal(err)
	}
	if keeps != 1 || [7]int{rearms, starts, revives, delivers, channels, trims, resumes} != [7]int{} {
		t.Fatalf("helm post-tick: keeper=%d others=%v", keeps, [7]int{rearms, starts, revives, delivers, channels, trims, resumes})
	}
	if !helm.Active(loop.root).Active {
		t.Fatal("keeper returned helm")
	}
}
