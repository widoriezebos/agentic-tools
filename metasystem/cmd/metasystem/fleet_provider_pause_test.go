package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

type fleetPauseCensus struct{}

func (fleetPauseCensus) Workers(string) (steward.Workers, error) {
	return steward.Workers{Live: 1, CensusComplete: true}, nil
}

func fleetWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func fleetAssertBudget(t *testing.T, bed *workBed, now time.Time, elapsed time.Duration) {
	t.Helper()
	owners := bed.workOwners()
	owners.commandNow = func(string) (time.Time, error) { return now, nil }
	owners.lookupEnv = func(key string) (string, bool) {
		return bed.root(), key == "METASYSTEM_SUPERVISION_REGISTRY_HOME"
	}
	for _, argv := range [][]string{{"goal", "show", bed.id, "--json"}, {"goal", "budget", bed.id, "--json"}, {"work", "status", bed.id, "--json"}} {
		code, result := bed.runJSON(owners, argv...)
		if code != 0 {
			t.Fatalf("%v exit %d: %+v", argv, code, result)
		}
		data, err := json.Marshal(result.Data)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Budget struct{ Projection dispatchcore.BudgetProjection }
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		if p := parsed.Budget.Projection; p.Status != dispatchcore.BudgetKnown || p.Elapsed != elapsed {
			t.Fatalf("%v: want known budget, elapsed %v; got %+v", argv, elapsed, p)
		}
	}
}

func TestFleetCompletedWorkDoesNotPause(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"job", "job-result", "launch"} {
		for _, outcome := range []string{"completed", "cancelled", "failed", "provider-failed"} {
			t.Run(source+"/"+outcome, func(t *testing.T) {
				t.Parallel()
				var file *goal.GoalFile
				bed := newWorkBedWith(t, func(f *goal.GoalFile) { f.Obligation = nil; file = f })
				start, err := time.Parse(time.RFC3339, file.Claimed.At)
				if err != nil {
					t.Fatal(err)
				}
				home := testprovider.Register(t, bed.root())
				status, cause := outcome, ""
				want := 12 * time.Minute
				if outcome == "provider-failed" {
					status, cause, want = "failed", outage.ProviderLimit, 4*time.Minute
				}
				if source == "launch" {
					if err := (launch.Store{Root: filepath.Join(home, "launch")}).Create(launch.Record{
						ID: "finished", Goal: bed.id, Adapter: "claude-headless", State: launch.State(status), Cause: cause,
						StartedAt: start.Format(time.RFC3339Nano), FinishedAt: start.Add(time.Minute).Format(time.RFC3339Nano),
					}); err != nil {
						t.Fatal(err)
					}
				} else {
					fleetWriteJSON(t, filepath.Join(bed.root(), "artifacts", "agents", "jobs", "finished.json"), map[string]any{
						"jobId": "finished", "operationId": "finished", "goalId": bed.id, "goalRevision": file.Claimed.Revision,
						"capMin": 20, "status": status, "runtime": "claude", "pid": 41, "error": "runtime_error",
						"round":     "1",
						"startedAt": start.Format(time.RFC3339Nano), "endedAt": start.Add(time.Minute).Format(time.RFC3339Nano),
					})
					if cause != "" && source == "job-result" {
						fleetWriteJSON(t, filepath.Join(bed.root(), "artifacts", "agents", "finished", "rounds", "1", "claude-result.json"),
							map[string]any{"is_error": true, "result": "HTTP 429 Too Many Requests"})
					} else if cause != "" {
						if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "jobs", "finished.log"), []byte("HTTP 429 Too Many Requests\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				if _, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit,
					fmt.Sprintf("Claude AI usage limit reached|%d", start.Add(8*time.Minute).Unix()), "other-work", start.Add(2*time.Minute)); err != nil {
					t.Fatal(err)
				}
				fleetAssertBudget(t, bed, start.Add(12*time.Minute), want)
			})
		}
	}
}

func TestFleetProviderWaitWithPastIdleIsKnown(t *testing.T) {
	t.Parallel()
	for _, landing := range []bool{false, true} {
		t.Run(fmt.Sprint(landing), func(t *testing.T) {
			t.Parallel()
			var file *goal.GoalFile
			var start time.Time
			bed := newWorkBedWith(t, func(f *goal.GoalFile) {
				f.Obligation = nil
				file = f
				var err error
				start, err = time.Parse(time.RFC3339, f.Claimed.At)
				if err != nil {
					t.Fatal(err)
				}
				f.Claimed.EpisodeAt = f.Claimed.At
				f.Claimed.EpisodeRevision = f.Claimed.Revision
				f.Claimed.AccountingRevision = f.Claimed.Revision
				f.Claimed.At = start.Add(6 * time.Minute).Format(time.RFC3339)
				f.Claimed.IdleSeconds = 120
				f.History = append(f.History,
					goal.HistoryLine{At: start.Add(4 * time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAX", f.Claimed.Machine, f.Claimed.Lineage), Verb: "release", Actor: f.History[1].Actor, Targets: []string{f.Id}, Keep: -1},
					goal.HistoryLine{At: f.Claimed.At, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAY", f.Claimed.Machine, f.Claimed.Lineage), Verb: "claim", Actor: f.History[1].Actor, Targets: []string{f.Id}, Keep: -1})
				f.Revision, f.Claimed.Revision = uint64(len(f.History)), uint64(len(f.History))
				f.StopCapability.Generation, f.StopCapability.Revision = f.Revision, f.Revision
				if landing {
					f.Landing = &goal.LandingRecord{At: start.Add(7 * time.Minute).Format(time.RFC3339), Opid: f.History[len(f.History)-1].Opid}
				}
			})
			home := testprovider.Register(t, bed.root())
			fleetWriteJSON(t, filepath.Join(bed.root(), "artifacts", "agents", "jobs", "running.json"), map[string]any{
				"jobId": "running", "operationId": "running", "goalId": bed.id, "goalRevision": file.Claimed.Revision,
				"capMin": 20, "status": "failed", "runtime": "claude", "pid": 41, "startedAt": start.Format(time.RFC3339Nano),
				"endedAt": start.Add(time.Minute).Format(time.RFC3339Nano), "error": "runtime_error",
			})
			if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "jobs", "running.log"), []byte("HTTP 429 Too Many Requests\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit,
				fmt.Sprintf("Claude AI usage limit reached|%d", start.Add(8*time.Minute).Unix()), "running", start.Add(2*time.Minute)); err != nil {
				t.Fatal(err)
			}
			want := 6 * time.Minute // Twelve elapsed, less two past idle and four current provider wait.
			if landing {
				want = 4 * time.Minute // The six-minute union excludes the overlap once.
			}
			fleetAssertBudget(t, bed, start.Add(12*time.Minute), want)
		})
	}
}

func TestFleetPartialOutageSamplesUsePausedAge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := testprovider.Register(t, root)
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte("# Goals\n\n## Current goal: fixture — Build it\n- Origin: main\n- Next step: Build it.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\nrole.steward-continuation.runtime=claude\nrole.steward-continuation.model.claude=fixture-model\n"), 0600); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	stateRoot := t.TempDir()
	tick := func(minutes int, age time.Duration, verdict steward.Verdict) {
		t.Helper()
		result, err := steward.RunTick(root, steward.TickConfig{Now: start.Add(time.Duration(minutes) * time.Minute),
			ProviderHome: home, WorkStateRoot: stateRoot, StaleTicks: 2, Runner: &seat.RunnerContext{TickSeconds: 600}}, fleetPauseCensus{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Evidence.Age != age || result.Decision.Verdict != verdict {
			t.Fatalf("minute %d: want age %v, verdict %s; got evidence %+v, decision %+v", minutes, age, verdict, result.Evidence, result.Decision)
		}
		retained, err := steward.LoadEvidence(steward.EvidencePath(root))
		if err != nil || retained != result.Evidence {
			t.Fatalf("tick did not retain paused age: %+v %v", retained, err)
		}
	}
	tick(0, 0, steward.VerdictHealthy)
	for _, minutes := range []int{1, 11} {
		if _, err := outage.Observe(home, "claude", "fixture-model", "overloaded", "API Error: 529", "fixture", start.Add(time.Duration(minutes)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := outage.Observe(home, "claude", "fixture-model", "", "", "fixture", start.Add(time.Duration(minutes+8)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		tick(minutes+9, time.Duration((minutes/10+1)*2)*time.Minute, steward.VerdictHealthy)
	}
	tick(20, 4*time.Minute, steward.VerdictHealthy)
	tick(35, 19*time.Minute, steward.VerdictHealthy)
	tick(36, 20*time.Minute, steward.VerdictStalledIdle)
}

func TestFleetProviderPausePublicStatus(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, runtime                                                                                                                                string
		landing, recovery, switchProvider, corrupt, missing, horizon, lateRecovery, concurrentProvider, runningProvider, billable, noLaunch, noOwner bool
		elapsed, paused                                                                                                                              time.Duration
	}{
		{name: "billable during outage", runtime: "claude-headless", billable: true, elapsed: 4 * time.Minute, paused: 8 * time.Minute},
		{name: "no provider work", runtime: "claude-headless", noLaunch: true, elapsed: 12 * time.Minute},
		{name: "dependent", runtime: "claude-headless", elapsed: 4 * time.Minute, paused: 8 * time.Minute},
		{name: "overlap landing", runtime: "claude-headless", landing: true, elapsed: 2 * time.Minute, paused: 10 * time.Minute},
		{name: "another provider", runtime: "codex-exec", elapsed: 12 * time.Minute},
		{name: "local without provider owner", runtime: "plain-exec", noOwner: true, elapsed: 12 * time.Minute},
		{name: "local work", runtime: "plain-exec", elapsed: 12 * time.Minute},
		{name: "concurrent provider", runtime: "claude-headless", concurrentProvider: true, elapsed: 8 * time.Minute, paused: 4 * time.Minute},
		{name: "running provider", runtime: "claude-headless", concurrentProvider: true, runningProvider: true, elapsed: 12 * time.Minute},
		{name: "switch provider", runtime: "claude-headless", switchProvider: true, elapsed: 8 * time.Minute, paused: 4 * time.Minute},
		{name: "late recovery", runtime: "claude-headless", lateRecovery: true, elapsed: 22 * time.Minute, paused: 8 * time.Minute},
		{name: "recovery", runtime: "claude-headless", recovery: true, elapsed: 8 * time.Minute, paused: 4 * time.Minute},
		{name: "late no reset", runtime: "claude-headless", horizon: true, elapsed: 15 * time.Minute, paused: 30 * time.Minute},
		{name: "corrupt history", runtime: "claude-headless", corrupt: true},
		{name: "missing dependency", missing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var file *goal.GoalFile
			bed := newWorkBedWith(t, func(f *goal.GoalFile) {
				f.Obligation = nil
				file = f
				if test.landing {
					at, err := time.Parse(time.RFC3339, f.Claimed.At)
					if err != nil {
						t.Fatal(err)
					}
					f.Landing = &goal.LandingRecord{At: at.Add(4 * time.Minute).Format(time.RFC3339), Opid: f.History[1].Opid}
				}
			})
			start, err := time.Parse(time.RFC3339, file.Claimed.At)
			if err != nil {
				t.Fatal(err)
			}
			home := testprovider.Home(bed.root())
			if !test.noOwner {
				home = testprovider.Register(t, bed.root())
			}
			now := start.Add(12 * time.Minute)
			if test.horizon {
				now = start.Add(45 * time.Minute)
			}
			if test.lateRecovery {
				now = start.Add(30 * time.Minute)
			}
			owners := bed.workOwners()
			owners.commandNow = func(string) (time.Time, error) { return now, nil }
			owners.lookupEnv = func(key string) (string, bool) {
				if key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" {
					return bed.root(), true
				}
				return "", false
			}
			store := launch.Store{Root: filepath.Join(home, "launch")}
			record := launch.Record{ID: "dependent-build", Goal: bed.id, Kind: "build", Adapter: test.runtime,
				State: launch.Failed, Cause: outage.ProviderLimit, StartedAt: start.Format(time.RFC3339Nano), FinishedAt: start.Add(2 * time.Minute).Format(time.RFC3339Nano)}
			if !test.noLaunch {
				if err := store.Create(record); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path string, value any) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			jobEnd, minutes := start.Add(2*time.Minute), uint64(2)
			if test.billable {
				jobEnd, minutes = now, 12
			}
			if test.noLaunch {
				minutes = 0
			}
			if !test.missing && !test.noLaunch {
				write(filepath.Join(bed.root(), "artifacts", "agents", "jobs", "executed.json"), map[string]any{
					"jobId": "executed", "operationId": "executed", "goalId": bed.id, "goalRevision": file.Claimed.Revision,
					"capMin": 20, "status": "completed", "runtime": strings.TrimSuffix(test.runtime, "-headless"), "pid": 41,
					"startedAt": start.Format(time.RFC3339Nano), "endedAt": jobEnd.Format(time.RFC3339Nano),
				})
			}
			detail := fmt.Sprintf("Claude AI usage limit reached|%d", start.Add(8*time.Minute).Unix())
			if test.horizon {
				detail = "HTTP 429 Too Many Requests"
			}
			if !test.noOwner {
				if _, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit, detail, "build-result", start.Add(2*time.Minute)); err != nil {
					t.Fatal(err)
				}
			}
			if test.recovery || test.lateRecovery {
				successAt := start.Add(6 * time.Minute)
				if test.lateRecovery {
					successAt = start.Add(20 * time.Minute)
				}
				if _, err := outage.Observe(home, "claude", "fixture-model", "", "", "provider-success", successAt); err != nil {
					t.Fatal(err)
				}
			}
			if test.concurrentProvider {
				concurrent := record
				concurrent.ID, concurrent.Adapter, concurrent.StartedAt, concurrent.FinishedAt = "parallel-build", "codex-exec", start.Add(-time.Minute).Format(time.RFC3339Nano), start.Add(6*time.Minute).Format(time.RFC3339Nano)
				concurrent.State = launch.Completed
				if test.runningProvider {
					concurrent.State = launch.Running
					concurrent.FinishedAt = ""
				}
				if err := store.Create(concurrent); err != nil {
					t.Fatal(err)
				}
			}
			if test.switchProvider {
				record.ID, record.Adapter, record.StartedAt, record.FinishedAt = "next-build", "codex-exec", start.Add(6*time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)
				if err := store.Create(record); err != nil {
					t.Fatal(err)
				}
			}
			if test.corrupt {
				state, err := outage.ReadProviders(home)
				if err != nil {
					t.Fatal(err)
				}
				c := state.Current["anthropic"]
				c.Intervals = []outage.Interval{{Since: "broken", Until: now.Format(time.RFC3339Nano)}}
				state.Current["anthropic"] = c
				write(testprovider.Path(bed.root()), state)
			}
			for _, argv := range [][]string{{"goal", "show", bed.id, "--json"}, {"goal", "budget", bed.id, "--json"}, {"work", "status", bed.id, "--json"}} {
				code, result := bed.runJSON(owners, argv...)
				if code != 0 {
					t.Fatalf("%v exit %d: %+v", argv, code, result)
				}
				data, err := json.Marshal(result.Data)
				if err != nil {
					t.Fatal(err)
				}
				var parsed struct {
					Budget struct{ Projection dispatchcore.BudgetProjection }
				}
				if err := json.Unmarshal(data, &parsed); err != nil {
					t.Fatal(err)
				}
				p := parsed.Budget.Projection
				if test.corrupt || test.missing {
					if p.Status != dispatchcore.BudgetUnknown || p.Unknown == nil || !strings.Contains(p.Unknown.Reason, "unreadable") && !strings.Contains(p.Unknown.Reason, "dependency is unknown") {
						t.Fatalf("%v must show unknown source accounting: %+v", argv, p)
					}
					continue
				}
				if p.Status != dispatchcore.BudgetKnown || p.Elapsed != test.elapsed || p.ObservedJobMinutes != minutes || p.ReservedJobMinutes != minutes {
					t.Fatalf("%v: elapsed %v paused %v minutes %d/%d, want %v/%v and %d/%d: %+v", argv, p.Elapsed, p.Wait, p.ObservedJobMinutes, p.ReservedJobMinutes, test.elapsed, test.paused, minutes, minutes, p)
				}
			}
			if test.corrupt || test.missing {
				return
			}
			if span := dispatchcore.WaitSpan(bed.root(), file, now, home); span != test.paused {
				t.Fatalf("wait projection = %v, want %v", span, test.paused)
			}
			consumption := dispatchcore.ProjectConsumption(bed.root(), file, now, home)
			if consumption.Status != dispatchcore.BudgetKnown || consumption.ObservedJobMinutes != minutes {
				t.Fatalf("consumption changed billable job minutes: %+v", consumption)
			}
			if test.noOwner {
				return
			}
			// A public tick uses the same host condition with its selected continuation.
			tickRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(tickRoot, "plans"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(tickRoot, "plans", "goals.md"), []byte("# Goals\n\n## Current goal: fixture — Build it\n- Origin: main\n- Next step: Build it.\n"), 0600); err != nil {
				t.Fatal(err)
			}
			runtime := "claude"
			if test.runtime == "codex-exec" || test.runtime == "plain-exec" {
				runtime = "codex"
			}
			config := "metasystem.runtimes=claude,codex\nrole.steward-continuation.runtime=" + runtime + "\nrole.steward-continuation.model." + runtime + "=fixture-model\n"
			if err := os.WriteFile(filepath.Join(tickRoot, "metasystem.conf"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			tick := func(at time.Time) steward.TickResult {
				t.Helper()
				result, err := steward.RunTick(tickRoot, steward.TickConfig{Now: at, ProviderHome: home, WorkStateRoot: t.TempDir()}, fleetPauseCensus{})
				if err != nil {
					t.Fatal(err)
				}
				retained, err := steward.LoadEvidence(steward.EvidencePath(tickRoot))
				if err != nil || retained != result.Evidence {
					t.Fatalf("public tick lost its clock sample: %+v %v", retained, err)
				}
				return result
			}
			tick(start)
			first := tick(start.Add(2 * time.Minute))
			middle := tick(start.Add(4 * time.Minute))
			if runtime == "claude" && (middle.Evidence.Age != 2*time.Minute || middle.Evidence.TicksSinceAdvance != first.Evidence.TicksSinceAdvance) {
				t.Fatalf("dependent tick must pause elapsed age and decision ticks: before %+v, during %+v", first.Evidence, middle.Evidence)
			}
			observed := tick(now)
			wantAge := test.elapsed
			if test.landing {
				wantAge = 4 * time.Minute
			} // This steward's clock has no landing wait.
			if test.switchProvider || test.concurrentProvider || test.noLaunch {
				wantAge = 4 * time.Minute
			} // Its continuation still depends on Claude.
			if observed.Evidence.Age != wantAge {
				t.Fatalf("public tick age = %v, want %v: %+v", observed.Evidence.Age, wantAge, observed)
			}
			// A restart reads the prior sample; replay at the same instant cannot add age.
			if replay := tick(now); replay.Evidence.Age != wantAge {
				t.Fatalf("replayed sample added age: %+v", replay.Evidence)
			}
			previous := observed.Evidence
			previous.Marks.HeadOid = "older-head"
			previous.DryRevivals = 2
			if err := steward.SaveEvidence(tickRoot, steward.EvidencePath(tickRoot), previous); err != nil {
				t.Fatal(err)
			}
			if advanced := tick(now); advanced.Evidence.Age != 0 || advanced.Evidence.TicksSinceAdvance != 0 || advanced.Evidence.DryRevivals != 0 {
				t.Fatalf("progress must reset clocks while a mark is retained: %+v", advanced.Evidence)
			}
		})
	}
}
