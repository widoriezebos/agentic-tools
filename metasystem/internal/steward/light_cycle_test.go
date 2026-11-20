package steward

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

type lightTraceBridge struct{ record func(string) }

func (b lightTraceBridge) Step() string { b.record("bridge"); return "fixture" }
func (b lightTraceBridge) Close()       {}

func TestLightCycleRunsOnlyAllowedOwners(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("ready", "Build it."))
	now, start := bed.now, bed.now
	var trace []string
	record := func(step string) { trace = append(trace, step) }
	deps := idleRunnerDependencies(&now)
	deps.ProviderHome = testprovider.Home(bed.root)
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { record("tick"); return TickResult{}, nil }
	deps.SweepDisk = func(string, time.Time, bool) error { record("disk"); return nil }
	deps.Bridge = func(string) bridgeStepper { return lightTraceBridge{record} }
	deps.SettleReplies = func(context.Context, string) (int, error) { record("replies"); return 0, nil }
	deps.Resumable = func(string) (string, bool, error) { record("resumable"); return "", false, nil }
	deps.Channel = func(context.Context, string) (int, error) { record("channel"); return 0, nil }
	deps.TrimCaches = func(context.Context, string, TickConfig) error { record("cache"); return nil }
	deps.Light = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
		bed.now, cfg.Now = now, now
		cfg.ProviderHome = testprovider.Home(root)
		work := bed.dependencies()
		read := work.ReadClaimableBudgetedWork
		work.ReadClaimableBudgetedWork = func(root string, now time.Time) (goal.ClaimableBudgetedWork, error) {
			record("seat decision")
			return read(root, now)
		}
		return runLightDecisionsWith(root, cfg, census, work, func(root string) ([]ReapReport, error) { record("collect"); return ReapContinuations(root) })
	}
	deps.StartSeat = func(root string, cfg TickConfig, census WorkerCensus, selected SeatSelection) (SeatRecord, error) {
		record("seat start")
		return startSeatWithDependencies(root, selected, *bed.seatDependencies())
	}
	deps.DeliverPending = func(root string) (int, error) {
		record("deliver")
		return deliverPendingWith(root, func(string, string) error { return nil })
	}
	cfg := TickConfig{Now: now,
		KeepLandingLane:      func() lane.AgentRun { record("keeper"); return lane.AgentRun{Outcome: lane.AgentNotKept} },
		FollowDeployedEngine: func() (bool, bool, error) { record("follow"); return false, false, nil },
		CompletedBoundary:    func(string, time.Time) error { record("boundary"); return nil },
		RearmAtBoundary:      func() (bool, error) { record("rearm"); return false, nil },
		Situation:            func(string, time.Time) Situation { record("situation"); return Situation{} },
	}
	requested := false
	deps.Sleep = func(d time.Duration) {
		now = now.Add(d)
		if !requested && now.Sub(start) >= 20*time.Second {
			requested = true
			want := []string{"tick", "disk", "bridge", "replies", "situation", "follow", "boundary", "rearm", "keeper", "resumable", "deliver", "channel", "cache"}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("scheduled steps=%v want=%v", trace, want)
			}
			trace = nil
			it := testIntent("light-ended")
			it.Notified, it.LaunchStamped = true, true
			consumedIntentOnDisk(t, bed.root, it)
			jobRecordOnDisk(t, bed.root, it.JobId, "completed", now.Format(time.RFC3339))
			if err := QueueNotification(bed.root, PendingNotification{Nonce: "queued-before-request", Message: "already queued"}); err != nil {
				t.Fatal(err)
			}
			if err := tickrequest.Write(bed.root, "job ended", now); err != nil {
				t.Fatal(err)
			}
		}
		if now.Sub(start) >= 30*time.Second {
			if err := stopRunnerLoop(bed.root); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := runLoopWithDependencies(bed.root, deadCensus(), func() error { record("revive"); return nil }, 600*time.Second, cfg, deps); err != nil {
		t.Fatal(err)
	}
	want := []string{"collect", "seat decision", "keeper", "seat start", "deliver"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("light steps=%v want=%v", trace, want)
	}
	if len(bed.launcher.starts) != 1 {
		t.Fatalf("seat starts=%d", len(bed.launcher.starts))
	}
	if active, err := ConsumedActive(bed.root); err != nil || len(active) != 0 {
		t.Fatalf("collection=%v %v", active, err)
	}
	if pending, err := PendingNotifications(bed.root); err != nil || len(pending) != 0 {
		t.Fatalf("delivery=%v %v", pending, err)
	}
}

// Every retained byte is compared around each light decision. This includes
// health sequences, failures, pattern quiet counts, duties and component attempts.
func lightStateFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasSuffix(path, ".lock") || strings.HasSuffix(path, ".flock") || path == runnerLogPath(root) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err == nil {
			files[rel] = string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestLightCyclesPreserveScheduledState(t *testing.T) {
	t.Parallel()
	var baseline Evidence
	var baselineHealth HealthObservationState
	var baselineAudits [3]int
	for _, busy := range []bool{false, true} {
		loop := newHelmLoop(t)
		start := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
		now := start
		deps := idleRunnerDependencies(&now)
		deps.ProviderHome = testprovider.Home(loop.root)
		var audits *runnerReadState
		requests, scheduled := 0, 0
		evidence := Evidence{TicksSinceAdvance: 4, Degraded: 1, DryRevivals: 3}
		deps.Tick = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
			scheduled++
			evidence = Observe(evidence, evidence.Marks)
			if err := SaveEvidence(root, EvidencePath(root), evidence); err != nil {
				t.Fatal(err)
			}
			_, err := observeHealthWithEvaluation(root, now, nil, func(string, string, time.Time, identity.Prober, bool) ([]RoleVerdict, SpendObservation) {
				return []RoleVerdict{{Role: RoleRepoWatcher, Status: HealthDead}, {Role: RoleStewardRunner, Status: HealthUnknown}}, SpendObservation{}
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := writeJSONAtomic(ComponentEvidencePath(root, "capability-probe-claude"), map[string]int{"attemptSeq": scheduled}); err != nil {
				t.Fatal(err)
			}
			if _, err := UpdatePatterns(root, PatternCycle{Now: now, ClearTicks: 5, Step: func([]byte) ([]byte, []PatternRun, error) {
				return nil, []PatternRun{{Pattern: "quiet", Observations: []PatternObservation{{Kind: ObsClear, Work: "goal"}}}}, nil
			}}); err != nil {
				t.Fatal(err)
			}
			state, _ := runnerReads.Load(root)
			audits = state.(*runnerReadState)
			return TickResult{}, nil
		}
		deps.Light = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
			requests++
			before := lightStateFiles(t, root)
			counts := [3]int{audits.tick, audits.scans, audits.reads}
			result, err := runLightDecisions(root, cfg, census)
			if err != nil {
				t.Fatal(err)
			}
			if after := lightStateFiles(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("light decisions changed retained state")
			}
			if counts != [3]int{audits.tick, audits.scans, audits.reads} {
				t.Fatal("light decisions advanced read audit counters")
			}
			return result, nil
		}
		deps.Sleep = func(d time.Duration) {
			before := now
			now = now.Add(d)
			if busy && before.Unix() != now.Unix() {
				if err := tickrequest.Write(loop.root, "completion", now); err != nil {
					t.Fatal(err)
				}
			}
			if now.Sub(start) >= 1201*time.Second {
				loop.stop(t)
			}
		}
		if err := runLoopWithDependencies(loop.root, deadCensus(), nil, 600*time.Second, TickConfig{Now: start}, deps); err != nil {
			t.Fatal(err)
		}
		health, err := loadHealthRecord(HealthRecordPath(loop.root))
		if err != nil {
			t.Fatal(err)
		}
		if scheduled != 3 || busy && requests != 16 || !busy && requests != 0 {
			t.Fatalf("scheduled=%d requests=%d", scheduled, requests)
		}
		counts := [3]int{audits.tick, audits.scans, audits.reads}
		if !busy {
			baseline, baselineHealth, baselineAudits = evidence, health.State, counts
		} else if evidence != baseline || !reflect.DeepEqual(health.State, baselineHealth) || counts != baselineAudits {
			t.Fatalf("requests changed scheduled counters: evidence=%+v health=%+v audits=%v", evidence, health.State, counts)
		}
	}
}

func TestLightSeatKeepsProviderAndCollectionHolds(t *testing.T) {
	t.Parallel()
	for _, age := range []time.Duration{4 * time.Minute, 6 * time.Minute} {
		t.Run(fmt.Sprint(age), func(t *testing.T) {
			bed, cfg, work := seatBusyReadBed(t, launch.StepStarting, age)
			cfg.Now = bed.now
			cfg.ProviderHome = testprovider.Home(bed.root)
			result, err := runLightDecisionsWith(bed.root, cfg, deadCensus(), work, ReapContinuations)
			if err != nil || (result.Seat == nil) != (age < 5*time.Minute) {
				t.Fatalf("collection at %s: %+v %v", age, result, err)
			}
		})
	}
	bed := newSeatBed(t, seatReadyGoal("ready", "Build it."))
	if _, err := outage.Observe(testprovider.Home(bed.root), "claude", "fixture-model", outage.ProviderLimit, "limited", "light", bed.now); err != nil {
		t.Fatal(err)
	}
	result, err := runLightDecisionsWith(bed.root, TickConfig{Now: bed.now, ProviderHome: testprovider.Home(bed.root)}, deadCensus(), bed.dependencies(), ReapContinuations)
	if err != nil || result.Seat != nil || result.Decision.Action == ActRevive {
		t.Fatalf("provider hold lifted: %+v %v", result, err)
	}
}

func TestLightSeatReadsSituationWithoutPublishing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 11, 20, 5, 0, 0, 0, time.UTC)
	reads := 0
	cfg := TickConfig{Situation: func(string, time.Time) Situation { reads++; return Situation{At: now} }}
	before := lightStateFiles(t, root)
	dependencies := lightSeatDependencies(cfg)
	if _, err := dependencies.Situation(root, now); err != nil {
		t.Fatal(err)
	}
	if reads != 1 || !reflect.DeepEqual(before, lightStateFiles(t, root)) {
		t.Fatal("seat situation read published state")
	}
}
