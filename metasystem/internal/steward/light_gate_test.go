package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

// Raw ledger and launch adapters are fixtures; neither the light dispatcher nor
// its start/recheck is replaced. Completion travels through Store.Update and
// the runner's request wait before the production start writes its seat record.
func TestLightSeatProductionPathHonorsScheduledProgress(t *testing.T) {
	for _, name := range []string{"allowed", "governed-obligations", "direct-validation", "ruling-reviews", "counselor-brief", "continuation-reap", "evidence-store", "progress-marks", "spent", "new-breach", "new-read-failure", "latest-degraded"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
			file := bed.goals["held"]
			file.Claimed.At = bed.now.Add(-50 * time.Second).Format(time.RFC3339)
			file.History[len(file.History)-1].At = file.Claimed.At
			if name == "spent" {
				file.Budget.ElapsedLimit = "1m"
				file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget)
			}
			bed.now = bed.now.Add(-50 * time.Second)
			current := bed.start(&SeatSelection{Goal: "held", Held: true})
			bed.now = bed.now.Add(50 * time.Second)
			store := launch.Store{Root: t.TempDir()}
			fence, _ := json.Marshal(bed.root)
			if err := store.Create(launch.Record{ID: current.LaunchID, Kind: "seat", State: launch.Running, AdapterData: map[string]json.RawMessage{"fenceRoot": fence}}); err != nil {
				t.Fatal(err)
			}
			now, start := bed.now, bed.now
			work := bed.dependencies()
			cfg := TickConfig{Now: now, Seat: bed.launcher, ProviderHome: testprovider.Home(bed.root), WorkStateRoot: t.TempDir(), workReads: &work}
			census := idleCensus(func(string) (Workers, error) {
				record, err := store.Read(current.LaunchID)
				if err != nil {
					return Workers{}, err
				}
				if record.State.Terminal() {
					return deadWorkers, nil
				}
				return Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true}, nil
			})
			deps := idleRunnerDependencies(&now)
			deps.ProviderHome = cfg.ProviderHome
			ticks := 0
			deps.Tick = func(root string, tickCfg TickConfig, census WorkerCensus) (TickResult, error) {
				ticks++
				tickCfg.Seat = nil
				tickName := name
				if name == "latest-degraded" && ticks == 2 {
					tickName = "counselor-brief"
				}
				switch tickName {
				case "governed-obligations", "direct-validation", "ruling-reviews", "counselor-brief":
					paths := map[string]string{
						"governed-obligations": run.RecordPath(root, "broken"), "direct-validation": validationWindowPath(root),
						"ruling-reviews": rulingSweepPath(root), "counselor-brief": counselorBriefCursorPath(root),
					}
					path := paths[tickName]
					writeLightPoison(t, path)
					result, err := RunTick(root, tickCfg, census)
					if result.Decision.Verdict != VerdictDegraded || tickCfg.scheduledProgress.collection || tickCfg.scheduledProgress.seat {
						t.Fatalf("early exit: %+v progress=%+v err=%v", result.Decision, tickCfg.scheduledProgress, err)
					}
					pending, _ := PendingNotifications(root)
					found := false
					for _, notice := range pending {
						found = found || notice.Cause == degradedCause(tickName)
					}
					if !found {
						t.Fatalf("scheduled tick did not reach %s", name)
					}
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					return result, err
				}
				attention := newAttentionPolicyBed(t)
				attention.root, attention.now = root, now
				fixture := newTickContinuationFixture(t, attention, false)
				fixture.cfg, fixture.census = tickCfg, census
				fixture.dependencies.openWork = work
				var poison string
				switch name {
				case "continuation-reap":
					poison = filepath.Join(consumedDir(root), "broken.json")
				case "evidence-store":
					poison = EvidencePath(root)
				case "progress-marks":
					fixture.dependencies.readMarks = func(string, ...string) ([]byte, error) { return nil, errors.New("marks unreadable") }
					if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				if poison != "" {
					writeLightPoison(t, poison)
				}
				result, _, err := fixture.run(t)
				if poison != "" {
					if err := os.Remove(poison); err != nil {
						t.Fatal(err)
					}
				}
				if name == "progress-marks" {
					if err := os.Remove(filepath.Join(root, ".git")); err != nil {
						t.Fatal(err)
					}
				}
				wantSeat := name != "continuation-reap" && name != "evidence-store" && name != "progress-marks"
				if err != nil || !tickCfg.scheduledProgress.collection || tickCfg.scheduledProgress.seat != wantSeat {
					t.Fatalf("scheduled progress=%+v result=%+v error=%v", tickCfg.scheduledProgress, result.Decision, err)
				}
				if !wantSeat && result.Decision.Verdict != VerdictDegraded {
					t.Fatalf("missing degraded result: %+v", result.Decision)
				}
				return result, err
			}
			ended := false
			endAt, stopAt, wantTicks := 20*time.Second, 30*time.Second, 1
			if name == "latest-degraded" {
				endAt += 600 * time.Second
				stopAt += 600 * time.Second
				wantTicks = 2
			}
			deps.Sleep = func(d time.Duration) {
				now = now.Add(d)
				bed.now = now
				if !ended && now.Sub(start) >= endAt {
					ended = true
					if name != "allowed" && name != "spent" && name != "new-breach" {
						intent := testIntent("light-collection")
						intent.Notified, intent.LaunchStamped = true, true
						consumedIntentOnDisk(t, bed.root, intent)
						jobRecordOnDisk(t, bed.root, intent.JobId, "completed", now.Format(time.RFC3339))
					}
					if _, err := store.Update(current.LaunchID, func(r *launch.Record) error {
						r.State = launch.Completed
						r.FinishedAt = now.Format(time.RFC3339)
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					bed.launcher.states[current.LaunchID] = SeatLaunchState{Found: true, Terminal: true, State: "completed"}
					if name == "new-breach" {
						file.StopCapability.FenceEpoch = 1
						file.StopFence = &goal.StopFence{StopID: "stop-held-r3-f1", Revision: 3, Epoch: 1, CapabilityGeneration: 3, ClosedAt: now.Format(time.RFC3339), Reason: goal.StopReasonElapsedLimit}
					}
					if name == "new-read-failure" {
						writeLightPoison(t, validationWindowPath(bed.root))
					}
				}
				if now.Sub(start) >= stopAt {
					if err := stopRunnerLoop(bed.root); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := runLoopWithDependencies(bed.root, census, nil, 600*time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			want := 1
			if name == "allowed" {
				want = 2
			}
			if ticks != wantTicks || !ended || len(bed.launcher.starts) != want {
				t.Fatalf("ticks=%d ended=%v starts=%d want=%d", ticks, ended, len(bed.launcher.starts), want)
			}
			records, err := readSeatRecords(bed.root)
			if err != nil || len(records) != want {
				t.Fatalf("durable starts=%d error=%v", len(records), err)
			}
			if _, err := tickrequest.Read(bed.root); !os.IsNotExist(err) {
				t.Fatalf("completion request was not served: %v", err)
			}
			active, err := ConsumedActive(bed.root)
			wantActive := 0
			switch name {
			case "governed-obligations", "direct-validation", "ruling-reviews", "counselor-brief", "new-read-failure", "latest-degraded":
				wantActive = 1
			}
			if err != nil || len(active) != wantActive {
				t.Fatalf("collection crossed its scheduled boundary: active=%d want=%d error=%v", len(active), wantActive, err)
			}
		})
	}
}

func writeLightPoison(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{torn"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLightSeatBeforeFirstTickAndStartRecheck(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
	file := bed.goals["held"]
	file.Claimed.At = bed.now.Add(-50 * time.Second).Format(time.RFC3339)
	file.History[len(file.History)-1].At = file.Claimed.At
	file.Budget.ElapsedLimit = "1m"
	file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget)
	work := bed.dependencies()
	cfg := TickConfig{Now: bed.now, Seat: bed.launcher, ProviderHome: testprovider.Home(bed.root), WorkStateRoot: t.TempDir(), workReads: &work}
	selection := SeatSelection{Goal: "held", Held: true}
	before := lightStateFiles(t, bed.root)
	result, err := runLightDecisions(bed.root, cfg, deadCensus())
	if err != nil || result.Seat != nil || !strings.Contains(result.Decision.Reason, "first scheduled tick") {
		t.Fatalf("before first tick: %+v %v", result.Decision, err)
	}
	if _, err := startLightSeat(bed.root, cfg, deadCensus(), selection); err == nil {
		t.Fatal("start bypassed missing scheduled tick")
	}
	if !reflect.DeepEqual(before, lightStateFiles(t, bed.root)) {
		t.Fatal("missing tick gate changed retained state")
	}
	cfg.scheduledProgress = &scheduledProgress{collection: true, seat: true}
	result, err = runLightDecisions(bed.root, cfg, deadCensus())
	if err != nil || result.Seat == nil {
		t.Fatalf("unspent selection: %+v %v", result.Decision, err)
	}
	bed.now = bed.now.Add(-50 * time.Second)
	current := bed.start(result.Seat)
	cfg.Now = cfg.Now.Add(20 * time.Second)
	bed.now = cfg.Now
	bed.launcher.states[current.LaunchID] = SeatLaunchState{Found: true, Terminal: true, State: "completed"}
	if state := reapSeatLaunches(bed.root, lightSeatDependencies(cfg), cfg.Now); state.Err != nil {
		t.Fatal(state.Err)
	}
	if reason := lightSeatBudgetHold(bed.root, cfg, SeatSelection{Kind: "steward", SeatHeld: []string{"held"}}, work.Seat.Project); !strings.Contains(reason, "budget is spent") {
		t.Fatalf("stewardship bypassed held budget: %s", reason)
	}
	if _, err := startLightSeat(bed.root, cfg, deadCensus(), *result.Seat); err == nil || !strings.Contains(err.Error(), "budget is spent") || len(bed.launcher.starts) != 1 {
		t.Fatalf("locked start bypassed spent budget: %v starts=%d", err, len(bed.launcher.starts))
	}
}
