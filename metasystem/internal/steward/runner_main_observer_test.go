package steward

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

func TestRunnerWatchesMainDuringLongAndInterruptedCycles(t *testing.T) {
	t.Parallel()
	for _, end := range []string{"cycle", "signal", "follow", "rearm", "failed tick", "stop file"} {
		t.Run(end, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				loop := newHelmLoop(t)
				start := runnerNow()
				moved := start.Add(time.Minute + time.Second)
				var seen time.Time
				steps := 0
				cycle := 0
				fake := newFakeStopSignals()
				cfg := TickConfig{Now: start, ObserveMovedMain: func(stopping func() bool) error {
					if stopping() {
						t.Error("observed after shutdown")
						return nil
					}
					steps++
					if runnerNow().After(moved) && seen.IsZero() {
						seen = runnerNow()
					}
					return errors.New("origin unreachable")
				}, KeepLandingLane: func() lane.AgentRun { cycle++; loop.stop(t); return lane.AgentRun{} }}
				deps := runnerLoopDependencies{Now: runnerNow, Sleep: runnerSleep, Resumable: func(string) (string, bool, error) { return "", false, nil }, DeliverPending: func(string) (int, error) { return 0, nil }, Channel: func(context.Context, string) (int, error) { return 0, nil }}
				if end == "signal" {
					deps.StopSignals = fake.source(t)
				}
				if end == "follow" {
					cfg.FollowDeployedEngine = func() (bool, bool, error) { return true, false, nil }
				}
				if end == "rearm" {
					cfg.RearmAtBoundary = func() (bool, error) { return true, nil }
				}
				deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
					runnerSleep(20 * time.Minute)
					synctest.Wait()
					if seen.Before(moved) || seen.Sub(moved) > 15*time.Second || steps > 81 {
						t.Fatalf("seen=%v, steps=%d", seen, steps)
					}
					before := steps
					switch end {
					case "signal":
						relay := <-fake.signals
						relay <- syscall.SIGTERM
						<-fake.reported
					case "stop file":
						loop.stop(t)
					case "failed tick":
						return TickResult{}, errors.New("tick failed")
					}
					if end == "signal" || end == "stop file" {
						runnerSleep(30 * time.Second)
						synctest.Wait()
						if steps != before {
							t.Fatal("observer continued after stop")
						}
					}
					return TickResult{}, nil
				}
				if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Hour, cfg, deps); err != nil {
					t.Fatal(err)
				}
				log, err := os.ReadFile(runnerLogPath(loop.root))
				if err != nil || strings.Count(string(log), "origin unreachable") != 1 {
					t.Fatalf("runner log %q: %v", log, err)
				}
				before := steps
				runnerSleep(time.Minute)
				if steps != before {
					t.Fatal("observer survived runner")
				}
				if (end == "cycle" || end == "failed tick") != (cycle == 1) && end != "stop file" {
					t.Fatalf("keeper steps %d", cycle)
				}
			})
		})
	}
}

func TestRunnerPendingFollowGetsNextFreeProofSlot(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		loop := newHelmLoop(t)
		proofEnds := runnerNow().Add(30 * time.Second)
		starts, tries := 0, 0
		cfg := TickConfig{Now: runnerNow(), ObserveMovedMain: func(held func() bool) error {
			if !held() && !runnerNow().Before(proofEnds) {
				starts++
				proofEnds = runnerNow().Add(time.Minute)
			}
			return nil
		}}
		var waiting atomic.Bool
		cfg.FollowWaitingForProof = waiting.Load
		cfg.FollowDeployedEngine = func() (bool, bool, error) {
			tries++
			blocked := runnerNow().Before(proofEnds)
			waiting.Store(true)
			if tries > 2 {
				t.Fatal("engine starved")
			}
			return !blocked, blocked, nil
		}
		deps := runnerLoopDependencies{Now: runnerNow, Sleep: runnerSleep, Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil }, DeliverPending: func(string) (int, error) { return 0, nil }, Channel: func(context.Context, string) (int, error) { return 0, nil }}
		if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Minute, cfg, deps); err != nil || starts != 0 || tries != 2 {
			t.Fatalf("starts=%d follows=%d err=%v", starts, tries, err)
		}
	})
}

func TestRunnerLightCycleAndMainTimerSerializeLaneWork(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		loop := newHelmLoop(t)
		if err := os.WriteFile(filepath.Join(loop.root, "metasystem.conf"), []byte("steward.request-gap-sec=15\n"), 0600); err != nil {
			t.Fatal(err)
		}
		var active atomic.Int32
		laneWork := func() {
			if active.Add(1) != 1 {
				t.Error("simultaneous lane work")
			}
			runnerSleep(time.Second)
			active.Add(-1)
		}
		lights, keeps, passes := 0, 0, 0
		cfg := TickConfig{Now: runnerNow(), KeepLandingLane: func() lane.AgentRun {
			keeps++
			laneWork()
			if lights == 1 {
				loop.stop(t)
			}
			return lane.AgentRun{Outcome: lane.AgentIdle}
		}, ObserveMovedMain: func(func() bool) error { passes++; laneWork(); return errors.New("unreachable") }}
		deps := runnerLoopDependencies{Now: runnerNow, Sleep: runnerSleep, Tick: func(root string, _ TickConfig, _ WorkerCensus) (TickResult, error) {
			if err := tickrequest.Write(root, "completed proof", runnerNow()); err != nil {
				t.Fatal(err)
			}
			return TickResult{}, nil
		}, Light: func(string, TickConfig, WorkerCensus) (TickResult, error) {
			lights++
			runnerSleep(15 * time.Second)
			return TickResult{}, nil
		},
			DeliverPending: func(string) (int, error) { return 0, nil }, Channel: func(context.Context, string) (int, error) { return 0, nil }}
		if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Hour, cfg, deps); err != nil {
			t.Fatal(err)
		}
		log, err := os.ReadFile(runnerLogPath(loop.root))
		if err != nil || lights != 1 || keeps < 2 || passes < 3 || strings.Count(string(log), "unreachable") != 1 {
			t.Fatalf("light cycles=%d keeper=%d observations=%d log=%q err=%v", lights, keeps, passes, log, err)
		}
	})
}
