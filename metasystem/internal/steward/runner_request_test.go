package steward

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

func TestRunnerRequestsKeepTheScheduledClock(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		requests []time.Duration
		during   bool
		until    time.Duration
		want     []time.Duration
	}{
		{name: "wake", requests: []time.Duration{100*time.Second + 100*time.Millisecond}, until: 601 * time.Second, want: []time.Duration{0, 100*time.Second + 200*time.Millisecond, 600 * time.Second}},
		{name: "fallback", until: 1800 * time.Second, want: []time.Duration{0, 600 * time.Second, 1200 * time.Second}},
		{name: "gap", requests: []time.Duration{time.Second, 20 * time.Second}, until: 40 * time.Second, want: []time.Duration{0, 15 * time.Second, 30 * time.Second}},
		{name: "near scheduled", requests: []time.Duration{597 * time.Second}, until: 601 * time.Second, want: []time.Duration{0, 600 * time.Second}},
		{name: "during cycle", during: true, until: 40 * time.Second, want: []time.Duration{0, 15 * time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			start := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
			now := start
			var got []time.Duration
			write := func(reason string) {
				t.Helper()
				if err := tickrequest.Write(loop.root, reason, now); err != nil {
					t.Fatal(err)
				}
			}
			deps := runnerLoopDependencies{
				Tick: func(_ string, cfg TickConfig, _ WorkerCensus) (TickResult, error) {
					got = append(got, now.Sub(start))
					if len(got) == 1 && tc.during {
						for i := 0; i < 50; i++ {
							write(fmt.Sprintf("job-ended:%d", i))
						}
					}
					if len(got) > 10 {
						t.Fatal("runner spun")
					}
					return TickResult{}, nil
				},
				Resumable:      func(string) (string, bool, error) { return "", false, nil },
				DeliverPending: func(string) (int, error) { return 0, nil },
				Channel:        func(context.Context, string) (int, error) { return 0, nil },
				Now:            func() time.Time { return now },
				Sleep: func(d time.Duration) {
					before := now.Sub(start)
					now = now.Add(d)
					for _, at := range tc.requests {
						if before < at && now.Sub(start) >= at {
							write("launch-ended:seat")
						}
					}
					if now.Sub(start) >= tc.until {
						loop.stop(t)
					}
				},
			}
			deps.Light = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
				if now.Sub(start)%(600*time.Second) == 0 {
					t.Fatal("light cycle replaced a scheduled tick")
				}
				return deps.Tick(root, cfg, census)
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, TickConfig{Now: start}, deps); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("cycles=%v want %v", got, tc.want)
			}
			if _, err := tickrequest.Read(loop.root); !os.IsNotExist(err) {
				t.Fatalf("request not acknowledged: %v", err)
			}
			log, err := os.ReadFile(runnerLogPath(loop.root))
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "wake" && (!strings.Contains(string(log), "request-driven") || !strings.Contains(string(log), "launch-ended:seat")) {
				t.Fatalf("missing cause: %s", log)
			}
		})
	}
}

func TestRunnerRequestCeilingAndRestart(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	start := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	now := start
	if err := tickrequest.Write(loop.root, "before restart", now.Add(-30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	requests, ticks := 0, 0
	deps := runnerLoopDependencies{
		Tick: func(_ string, cfg TickConfig, _ WorkerCensus) (TickResult, error) {
			ticks++
			if requests > 8 {
				t.Fatal("request ceiling exceeded")
			}
			return TickResult{}, nil
		},
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		DeliverPending: func(string) (int, error) { return 0, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		Now:            func() time.Time { return now }, Sleep: func(d time.Duration) {
			before := now
			now = now.Add(d)
			if before.Unix() != now.Unix() && now.Sub(start) < 600*time.Second {
				if err := tickrequest.Write(loop.root, "steady completion", now); err != nil {
					t.Fatal(err)
				}
			}
			if now.Sub(start) >= 601*time.Second {
				loop.stop(t)
			}
		},
	}
	deps.Light = func(string, TickConfig, WorkerCensus) (TickResult, error) {
		requests++
		if requests > 8 {
			t.Fatal("request ceiling exceeded")
		}
		return TickResult{}, nil
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, TickConfig{Now: start}, deps); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(runnerLogPath(loop.root))
	if err != nil || !strings.Contains(string(log), "scheduled tick: before restart (1 requests)") {
		t.Fatalf("old pending request was not served on restart: %s, %v", log, err)
	}
	if requests != 8 || ticks != 2 {
		t.Fatalf("requests=%d scheduled=%d", requests, ticks)
	}
	if _, err := tickrequest.Read(loop.root); !os.IsNotExist(err) {
		t.Fatalf("scheduled tick did not serve pending request: %v", err)
	}
}

func TestRunnerRequestHoldsAndDrain(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"helm", "stop", "drain", "drain-during", "replacement", "boundary"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			start := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
			now := start
			deps := idleRunnerDependencies(&now)
			ticks, starts, revives, boundaries, rearms := 0, 0, 0, 0, 0
			var signal func()
			if strings.HasPrefix(mode, "drain") {
				fake := newFakeStopSignals()
				deps.StopSignals = fake.source(t)
				signal = func() {
					relay := receiveWithin(t, fake.signals, "relay")
					relay <- syscall.SIGTERM
					receiveWithin(t, fake.reported, "drain")
				}
			}
			cfg := TickConfig{Now: start}
			deps.Tick = func(_ string, c TickConfig, _ WorkerCensus) (TickResult, error) {
				ticks++
				if ticks == 2 && mode == "drain-during" {
					signal()
				}
				return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
			}
			deps.Light = deps.Tick
			deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
				starts++
				return SeatRecord{}, nil
			}
			cfg.CompletedBoundary = func(string, time.Time) error { boundaries++; return nil }
			cfg.RearmAtBoundary = func() (bool, error) { rearms++; return false, nil }
			cfg.FollowDeployedEngine = func() (bool, bool, error) { return mode == "replacement" && ticks == 2, false, nil }
			if mode == "helm" {
				takeHelmFixture(t, loop.root)
			}
			deps.Sleep = func(d time.Duration) {
				before := now
				now = now.Add(d)
				if before.Sub(start) < 20*time.Second && now.Sub(start) >= 20*time.Second {
					if err := tickrequest.Write(loop.root, "person approved", now); err != nil {
						t.Fatal(err)
					}
					if mode == "stop" {
						loop.stop(t)
					}
					if mode == "drain" {
						signal()
					}
				}
				if now.Sub(start) >= 40*time.Second {
					loop.stop(t)
				}
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { revives++; return nil }, 600*time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "helm":
				if ticks != 2 || starts != 0 || revives != 0 || boundaries != 0 || rearms != 0 {
					t.Fatalf("helm lost: ticks=%d starts=%d revives=%d boundaries=%d rearms=%d", ticks, starts, revives, boundaries, rearms)
				}
			case "stop", "drain":
				if ticks != 1 {
					t.Fatalf("stop lost: %d ticks", ticks)
				}
				if _, err := tickrequest.Read(loop.root); err != nil {
					t.Fatal("stop consumed pending request")
				}
			case "drain-during":
				if ticks != 2 || starts != 1 {
					t.Fatalf("replacement continued: ticks=%d starts=%d", ticks, starts)
				}
				if _, err := tickrequest.Read(loop.root); err != nil {
					t.Fatal("replacement lost unfinished request")
				}
			case "boundary", "replacement":
				if ticks != 2 || boundaries != 1 || rearms != 1 || starts != 2 {
					t.Fatalf("light cycle ran a boundary or follow: %d/%d/%d starts=%d", ticks, boundaries, rearms, starts)
				}
			}
		})
	}
}

func TestRunnerRequestSettingsAndStatus(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"0", "-1", "oops", "900"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("steward.request-gap-sec="+value+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			s, err := readCycleSchedule(root, now)
			if value != "900" {
				if err == nil {
					t.Fatal("invalid gap admitted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			s.NextTick = now.Add(600 * time.Second)
			s.LastEnd = now
			if err := writeJSONAtomic(cycleSchedulePath(root), s); err != nil {
				t.Fatal(err)
			}
			status := runnerCycleStatus(root, now, true)
			if status.NextDue == nil || !status.NextDue.Equal(s.NextTick) || status.Pending != "" {
				t.Fatalf("status=%+v", status)
			}
			if err := tickrequest.Write(root, "job-ended:one", now); err != nil {
				t.Fatal(err)
			}
			status = runnerCycleStatus(root, now, true)
			if status.NextDue == nil || !status.NextDue.Equal(s.NextTick) || status.Pending != "job-ended:one" {
				t.Fatalf("gap moved scheduled tick: %+v", status)
			}
			status = runnerCycleStatus(root, now, false)
			if status.NextDue != nil || status.Pending == "" || !strings.Contains(status.Line, "no steward runner") {
				t.Fatalf("dormant status=%+v", status)
			}
			s.Gap = 15 * time.Second
			if err := writeJSONAtomic(cycleSchedulePath(root), s); err != nil {
				t.Fatal(err)
			}
			status = runnerCycleStatus(root, now.Add(5*time.Second), true)
			if status.NextDue == nil || !status.NextDue.Equal(now.Add(15*time.Second)) {
				t.Fatalf("pending gap due=%+v", status)
			}
		})
	}
}

func TestRunnerDiscardsBadRequestsAndKeepsTicking(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"empty", "json", "future", "foreign", "directory", "symlink", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			other := t.TempDir()
			start := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
			now := start
			if err := os.MkdirAll(filepath.Dir(tickrequest.Path(loop.root)), 0700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "empty", "json":
				data := ""
				if kind == "json" {
					data = "{"
				}
				if err := os.WriteFile(tickrequest.Path(loop.root), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := tickrequest.Write(loop.root, "unreadable", now); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(tickrequest.Path(loop.root), 0); err != nil {
					t.Fatal(err)
				}
			case "future":
				if err := tickrequest.Write(loop.root, "tomorrow", now.Add(24*time.Hour)); err != nil {
					t.Fatal(err)
				}
			case "foreign":
				if err := tickrequest.Write(other, "foreign", now); err != nil {
					t.Fatal(err)
				}
				data, _ := os.ReadFile(tickrequest.Path(other))
				if err := os.WriteFile(tickrequest.Path(loop.root), data, 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(tickrequest.Path(loop.root), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(tickrequest.Path(loop.root), "keep"), []byte("owned"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := tickrequest.Write(other, "other", now); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(tickrequest.Path(other), tickrequest.Path(loop.root)); err != nil {
					t.Fatal(err)
				}
			}
			deps := idleRunnerDependencies(&now)
			ticks := 0
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { ticks++; return TickResult{}, nil }
			deps.Sleep = func(d time.Duration) {
				now = now.Add(d)
				if now.Sub(start) >= 601*time.Second {
					loop.stop(t)
				}
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, TickConfig{Now: start}, deps); err != nil {
				t.Fatal(err)
			}
			if ticks != 2 {
				t.Fatalf("invalid request changed schedule: %d", ticks)
			}
			log, _ := os.ReadFile(runnerLogPath(loop.root))
			if strings.Count(string(log), "ignored tick request") != 1 {
				t.Fatalf("diagnostics=%s", log)
			}
			if kind == "symlink" {
				if _, err := tickrequest.Read(other); err != nil {
					t.Fatal("followed request symlink")
				}
			}
		})
	}
}

func TestLaneKeeperRequestsOnlyChangedSlowOutcomes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	outcome := lane.AgentRunning
	keeper := laneKeeping{root: root, now: func() time.Time { return now }, step: func() lane.AgentRun { return lane.AgentRun{Outcome: outcome} }}
	keeper.run()
	for _, next := range []lane.AgentOutcome{lane.AgentHeld, lane.AgentIdle, lane.AgentStarted, lane.AgentRunning} {
		outcome = next
		keeper.run()
	}
	if _, err := tickrequest.Read(root); !os.IsNotExist(err) {
		t.Fatal("fast keeper outcome requested a tick")
	}
	for _, next := range []lane.AgentOutcome{lane.AgentPaused, lane.AgentFailed, lane.AgentNotKept} {
		outcome = next
		keeper.run()
		r, err := tickrequest.Read(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := tickrequest.Acknowledge(root, r); err != nil {
			t.Fatal(err)
		}
		keeper.run()
		if _, err := tickrequest.Read(root); !os.IsNotExist(err) {
			t.Fatal("unchanged keeper requested another cycle")
		}
	}
}

func TestRunnerRequestsDoNotPostponeProviderProbe(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	start := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	now := start
	if _, err := testprovider.Record(loop.root, outage.ProviderLimit, "limited", "fixture", now); err != nil {
		t.Fatal(err)
	}
	deps := idleRunnerDependencies(&now)
	deps.ProviderHome = testprovider.Home(loop.root)
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil }
	deps.Sleep = func(d time.Duration) {
		before := now
		now = now.Add(d)
		if before.Unix() != now.Unix() && now.Sub(start) < 120*time.Second {
			if err := tickrequest.Write(loop.root, "completion", now); err != nil {
				t.Fatal(err)
			}
		}
		if now.Sub(start) >= 125*time.Second {
			loop.stop(t)
		}
	}
	probes := 0
	cfg := TickConfig{Now: start, ProviderHome: deps.ProviderHome, ProbeProvider: func(string) (bool, error) {
		probes++
		if now.Sub(start) != 2*time.Minute {
			t.Fatalf("probe moved to %s", now.Sub(start))
		}
		return false, nil
	}}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, cfg, deps); err != nil {
		t.Fatal(err)
	}
	if probes != 1 {
		t.Fatalf("probes=%d", probes)
	}
}

func TestInterruptedRequestIsServedByRestart(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	if err := tickrequest.Write(loop.root, "launch-ended:restart", now); err != nil {
		t.Fatal(err)
	}
	deps := idleRunnerDependencies(&now)
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { panic("runner interrupted") }
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("runner was not interrupted")
			}
		}()
		_ = runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, TickConfig{Now: now}, deps)
	}()
	if _, err := tickrequest.Read(loop.root); err != nil {
		t.Fatal("interruption lost pending request")
	}
	ticks := 0
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { ticks++; return TickResult{}, nil }
	deps.Sleep = func(d time.Duration) { now = now.Add(d); loop.stop(t) }
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, TickConfig{Now: now}, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != 1 {
		t.Fatalf("restart served %d cycles", ticks)
	}
	if _, err := tickrequest.Read(loop.root); !os.IsNotExist(err) {
		t.Fatal("restart did not acknowledge request")
	}
}

func TestRunnerWithoutRequestsKeepsProviderProbeTiming(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                   string
		limitTick              int
		work, probeWork, until time.Duration
		want                   []time.Duration
	}{
		{"late limit", 2, 0, 0, 730 * time.Second, []time.Duration{720 * time.Second}},
		{"cycle work", 1, 30 * time.Second, 0, 160 * time.Second, []time.Duration{150 * time.Second}},
		{"slow probe", 1, 0, 10 * time.Second, 370 * time.Second, []time.Duration{120 * time.Second, 240 * time.Second, 360 * time.Second}},
		{"scheduled reset", 1, 0, 0, 730 * time.Second, []time.Duration{120 * time.Second, 240 * time.Second, 360 * time.Second, 480 * time.Second, 720 * time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			start := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
			now := start
			deps := idleRunnerDependencies(&now)
			deps.ProviderHome = testprovider.Home(loop.root)
			ticks := 0
			deps.Tick = func(_ string, cfg TickConfig, _ WorkerCensus) (TickResult, error) {
				ticks++
				if ticks == tc.limitTick {
					if _, err := testprovider.Record(loop.root, outage.ProviderLimit, "limited", "fixture", now); err != nil {
						t.Fatal(err)
					}
				}
				now = now.Add(tc.work)
				return TickResult{}, nil
			}
			deps.Sleep = func(d time.Duration) {
				now = now.Add(d)
				if now.Sub(start) >= tc.until {
					loop.stop(t)
				}
			}
			var got []time.Duration
			cfg := TickConfig{Now: start, ProviderHome: deps.ProviderHome, ProbeProvider: func(string) (bool, error) {
				got = append(got, now.Sub(start))
				now = now.Add(tc.probeWork)
				return false, nil
			}}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("probe times=%v want %v", got, tc.want)
			}
		})
	}
}

func TestQuestionWithdrawalWakesTheRunnerAndReleasesItsGoal(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("asked", "Build it."))
	alias := filepath.Join(t.TempDir(), "checkout")
	if err := os.Symlink(bed.root, alias); err != nil {
		t.Fatal(err)
	}
	question, err := channel.Ask(channel.AskRequest{RepoRoot: bed.root, Goal: "asked", Kind: "other", Machine: seatBedMachine, Facts: []string{"A choice is needed"}, Now: bed.now})
	if err != nil {
		t.Fatal(err)
	}
	start := bed.now
	now := start
	deps := idleRunnerDependencies(&now)
	ticks := 0
	deps.Tick = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
		ticks++
		bed.now = now
		cfg.Now = now
		cfg.ProviderHome = testprovider.Home(root)
		work := bed.dependencies()
		work.Seat.OpenQuestions = channel.GoalOpenQuestions
		previous, err := LoadEvidence(EvidencePath(root))
		if err != nil {
			return TickResult{}, err
		}
		result, err := decideTickWithDependencies(root, cfg, census, previous, Marks{HeadOid: "h", OpidDigest: "d"}, work)
		if err == nil {
			err = SaveEvidence(root, EvidencePath(root), result.Evidence)
		}
		if ticks == 1 && result.Seat != nil {
			t.Fatal("open question did not hold the goal")
		}
		return result, err
	}
	deps.Light = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
		ticks++
		bed.now, cfg.Now = now, now
		cfg.ProviderHome = testprovider.Home(root)
		work := bed.dependencies()
		work.Seat.OpenQuestions = channel.GoalOpenQuestions
		return runLightDecisionsWith(root, cfg, census, work, ReapContinuations)
	}

	deps.StartSeat = func(_ string, _ TickConfig, _ WorkerCensus, selection SeatSelection) (SeatRecord, error) {
		if ticks != 2 || now.Sub(start) > 20*time.Second+200*time.Millisecond {
			t.Fatalf("goal released at tick %d, %s", ticks, now.Sub(start))
		}
		if selection.Goal != "asked" {
			t.Fatalf("selected %+v", selection)
		}
		return startSeatWithDependencies(bed.root, selection, *bed.seatDependencies())
	}
	withdrawn := false
	deps.Sleep = func(d time.Duration) {
		now = now.Add(d)
		if !withdrawn && now.Sub(start) >= 20*time.Second {
			withdrawn = true
			if _, err := channel.Withdraw(alias, question.ID, "choice no longer needed", nil, channel.DestinationConfig{}, now); err != nil {
				t.Fatal(err)
			}
		}
		if now.Sub(start) >= 30*time.Second {
			if err := stopRunnerLoop(bed.root); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := runLoopWithDependencies(bed.root, fakeCensus{workers: deadWorkers}, nil, 600*time.Second, TickConfig{Now: start}, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != 2 || len(bed.launcher.starts) != 1 {
		t.Fatalf("ticks=%d starts=%d", ticks, len(bed.launcher.starts))
	}
}

func TestRequestedAndScheduledCyclesUseTheSameSeatDecision(t *testing.T) {
	t.Parallel()
	for _, held := range []bool{false, true} {
		t.Run(fmt.Sprint(held), func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, seatReadyGoal("ready", "Build it."))
			if held {
				bed.fence = "the seat is paused"
			}
			previous := Evidence{Marks: Marks{HeadOid: "h", OpidDigest: "d"}}
			var ordinary TickResult
			for _, requested := range []bool{false, true} {
				cfg := TickConfig{Now: bed.now, ProviderHome: testprovider.Home(bed.root)}
				var result TickResult
				var err error
				if requested {
					result, err = runLightDecisionsWith(bed.root, cfg, fakeCensus{workers: deadWorkers}, bed.dependencies(), ReapContinuations)
				} else {
					result, err = decideTickWithDependencies(bed.root, cfg, fakeCensus{workers: deadWorkers}, previous, previous.Marks, bed.dependencies())
				}
				if err != nil {
					t.Fatal(err)
				}
				if !requested {
					ordinary = result
					continue
				}
				if !reflect.DeepEqual(result.Decision, ordinary.Decision) || !reflect.DeepEqual(result.Seat, ordinary.Seat) {
					t.Fatalf("request changed decisions: ordinary=%+v/%+v requested=%+v/%+v", ordinary.Decision, ordinary.Seat, result.Decision, result.Seat)
				}
				if held && result.Seat != nil {
					t.Fatal("request lifted seat pause")
				}
			}
		})
	}
}

func TestSeatLaunchCompletionWakesItsSuccessorDecision(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
	store := launch.Store{Root: t.TempDir()}
	alias := filepath.Join(t.TempDir(), "checkout")
	if err := os.Symlink(bed.root, alias); err != nil {
		t.Fatal(err)
	}
	fence, _ := json.Marshal(alias)
	if err := store.Create(launch.Record{ID: "current-seat", Kind: "seat", State: launch.Running, AdapterData: map[string]json.RawMessage{"fenceRoot": fence}}); err != nil {
		t.Fatal(err)
	}
	start := bed.now
	now := start
	census := idleCensus(func(string) (Workers, error) {
		record, err := store.Read("current-seat")
		if err != nil {
			return Workers{}, err
		}
		if record.State.Terminal() {
			return deadWorkers, nil
		}
		return Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true}, nil
	})
	deps := idleRunnerDependencies(&now)
	ticks := 0
	deps.Tick = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
		ticks++
		bed.now = now
		cfg.Now = now
		cfg.ProviderHome = testprovider.Home(root)
		previous, err := LoadEvidence(EvidencePath(root))
		if err != nil {
			return TickResult{}, err
		}
		result, err := decideTickWithDependencies(root, cfg, census, previous, Marks{HeadOid: "h", OpidDigest: "d"}, bed.dependencies())
		if err == nil {
			err = SaveEvidence(root, EvidencePath(root), result.Evidence)
		}
		if ticks == 1 && result.Seat != nil {
			t.Fatal("live seat displaced")
		}
		return result, err
	}
	deps.Light = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
		ticks++
		bed.now, cfg.Now = now, now
		cfg.ProviderHome = testprovider.Home(root)
		work := bed.dependencies()
		work.Seat.OpenQuestions = channel.GoalOpenQuestions
		return runLightDecisionsWith(root, cfg, census, work, ReapContinuations)
	}

	deps.StartSeat = func(_ string, _ TickConfig, _ WorkerCensus, selection SeatSelection) (SeatRecord, error) {
		if ticks != 2 || now.Sub(start) > 20*time.Second+200*time.Millisecond || !selection.Held || selection.Goal != "held" {
			t.Fatalf("successor selection at %s: %+v", now.Sub(start), selection)
		}
		return startSeatWithDependencies(bed.root, selection, *bed.seatDependencies())
	}
	ended := false
	deps.Sleep = func(d time.Duration) {
		now = now.Add(d)
		if !ended && now.Sub(start) >= 20*time.Second {
			ended = true
			if _, err := store.Update("current-seat", func(record *launch.Record) error { record.State = launch.Completed; return nil }); err != nil {
				t.Fatal(err)
			}
		}
		if now.Sub(start) >= 30*time.Second {
			if err := stopRunnerLoop(bed.root); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := runLoopWithDependencies(bed.root, census, nil, 600*time.Second, TickConfig{Now: start}, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != 2 || len(bed.launcher.starts) != 1 {
		t.Fatalf("ticks=%d starts=%d", ticks, len(bed.launcher.starts))
	}
}

func TestRunnerRequestDefaultsWithoutConfiguration(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	schedule, err := readCycleSchedule(t.TempDir(), now)
	if err != nil || schedule.Gap != 15*time.Second || schedule.Limit != 8 || !schedule.NextTick.Equal(now) {
		t.Fatalf("missing configuration must use request defaults: %+v, %v", schedule, err)
	}
}
