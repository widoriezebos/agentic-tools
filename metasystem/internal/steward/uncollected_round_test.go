package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestUncollectedRunsReadOnlyTheGoalsAsked(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	store := launch.Store{Root: filepath.Join(home, "launch")}
	var runs []UncollectedRun
	for _, id := range []string{"held", "foreign"} {
		run := UncollectedRun{Goal: id, Run: "run-" + id, LaunchID: "launch-" + id, Step: "build", EndedAt: now.Format(time.RFC3339)}
		writeStewardRecord(t, filepath.Join(home, "unit", run.Run, "run.json"), map[string]any{
			"id": run.Run, "goal": run.Goal, "state": "running",
			"rounds": []launch.UnitRound{{Steps: []launch.UnitStep{{Name: run.Step, LaunchID: run.LaunchID, State: launch.StepRunning}}}},
		})
		if err := store.Create(launch.Record{ID: run.LaunchID, State: launch.Completed, FinishedAt: run.EndedAt}); err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
	}
	for _, row := range []struct {
		name  string
		goals []string
		want  []UncollectedRun
	}{
		{name: "held only", goals: []string{"held"}, want: runs[:1]},
		{name: "both goals", goals: []string{"held", "foreign"}, want: runs},
		{name: "no goals"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := uncollectedRunsAt(home, row.goals, func(identity.Ref) identity.Liveness { return identity.Dead })
			if err != nil || len(got) != len(row.want) {
				t.Fatalf("uncollected runs for %v = %+v, %v; want %+v", row.goals, got, err, row.want)
			}
			for _, want := range row.want {
				if !slices.Contains(got, want) {
					t.Fatalf("uncollected runs for %v = %+v; missing %+v", row.goals, got, want)
				}
			}
		})
	}
}

func TestADeadSeatsBriefCarriesTheCollection(t *testing.T) {
	t.Parallel()
	b, units, work, _ := seatBusyFixture(t)
	seatBusyRun(t, units, b.now, 99)
	store := launch.Store{Root: filepath.Join(filepath.Dir(units), "launch")}
	if _, err := store.Update("launch-1", func(r *launch.Record) error {
		r.State, r.FinishedAt = launch.Completed, b.now.Format(time.RFC3339)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	deps := b.dependencies()
	deps.ReadClaimableBudgetedWork = func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return work, nil }
	deps.Seat.Home = func() (string, error) { return filepath.Dir(units), nil }
	deps.Seat.Prober = seatBusyDeadProber{}
	deps.Seat.Units = func(string, string) ([]UnitStage, error) { return []UnitStage{{Unit: "U3", Run: "run-1"}}, nil }
	for pass := 0; pass < 2; pass++ {
		result, err := decideTickWithDependencies(b.root, TickConfig{Now: b.now, WorkStateRoot: filepath.Dir(units), Prober: seatBusyDeadProber{}, ProviderHome: testprovider.Home(b.root)}, fakeCensus{workers: deadWorkers}, Evidence{}, Marks{}, deps)
		if err != nil || result.Seat == nil || result.Seat.Goal != "held" {
			t.Fatalf("tick: %+v %v", result, err)
		}
		after, readErr := (&launch.UnitRunner{Root: units}).Status("run-1")
		want := launch.StepRunning
		if pass > 0 {
			want = launch.StepPassed
		}
		if readErr != nil || after.Rounds[0].Steps[0].State != want {
			t.Fatalf("the tick collected the run: %+v %v", after, readErr)
		}
		if len(b.launcher.starts) != pass {
			t.Fatal("the decision started work")
		}
		record, err := startSeatWithDependencies(b.root, *result.Seat, *deps.Seat)
		if err != nil {
			t.Fatal(err)
		}
		brief, err := os.ReadFile(b.launcher.starts[pass].Brief)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(brief), "Collect first:\nmetasystem work wait run:run-1 --timeout 30m") != (pass == 0) {
			t.Fatalf("collection line at pass %d: %s", pass, brief)
		}
		b.end(record.LaunchID, "completed", "RESULT: DONE\n")
		writeStewardRecord(t, filepath.Join(units, "run-1", "run.json"), map[string]any{"id": "run-1", "goal": "held", "state": "running", "rounds": []launch.UnitRound{{Steps: []launch.UnitStep{{Name: "build", State: launch.StepPassed, LaunchID: "launch-1"}}}}})
		b.now = b.now.Add(time.Minute)
	}
}

func TestTheSeatBriefReadsTheHostStoresByDefault(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"default Home", "failing Home", "StartSeat config"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			if row == "default Home" {
				d := defaultSeatDependencies(nil)
				if d.Home == nil {
					t.Fatal("the default seat has no host-store reader")
				}
				got, gotErr := d.Home()
				want, wantErr := HomeStateRoot()
				if got != want || (gotErr == nil) != (wantErr == nil) {
					t.Fatalf("default Home = %q, %v; HomeStateRoot = %q, %v", got, gotErr, want, wantErr)
				}
				return
			}
			b, units, _, _ := seatBusyFixture(t)
			seatBusyRun(t, units, b.now, 99)
			if _, err := (launch.Store{Root: filepath.Join(filepath.Dir(units), "launch")}).Update("launch-1", func(r *launch.Record) error {
				r.State, r.FinishedAt = launch.Completed, b.now.Format(time.RFC3339)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			d := *b.seatDependencies()
			d.Units = func(string, string) ([]UnitStage, error) { return []UnitStage{{Unit: "U3", Run: "run-1"}}, nil }
			d.Home = func() (string, error) { return filepath.Dir(units), errors.New("host stores unavailable") }
			if row == "StartSeat config" {
				cfg := TickConfig{Now: b.now, WorkStateRoot: filepath.Dir(units), Prober: seatBusyDeadProber{}, Seat: b.launcher, ProviderHome: testprovider.Home(b.root), Units: d.Units, GoalProjection: d.Project}
				configured := seatDependenciesForConfig(cfg)
				// Keep the start's config wiring and replace its external readers.
				configured.Tips, configured.Machine, configured.Gate, configured.Fence = d.Tips, d.Machine, d.Gate, d.Fence
				configured.Lane, configured.Main, configured.Contains = d.Lane, d.Main, d.Contains
				configured.Jobs, configured.Refusals, configured.Launches, configured.Threads = d.Jobs, d.Refusals, d.Launches, d.Threads
				configured.Home = func() (string, error) {
					t.Error("an explicit store root must take precedence over Home")
					return "", errors.New("unexpected host-store read")
				}
				if _, ok := configured.Prober.(seatBusyDeadProber); !ok {
					t.Fatal("the seat start lost the tick's prober")
				}
				d = configured
			}
			if _, err := startSeatWithDependencies(b.root, SeatSelection{Goal: "held", Held: true}, d); err != nil {
				t.Fatal(err)
			}
			brief, err := os.ReadFile(b.launcher.starts[0].Brief)
			if err != nil {
				t.Fatal(err)
			}
			if row == "failing Home" {
				if !strings.Contains(string(brief), "Collect first unavailable;") || strings.Contains(string(brief), "metasystem work wait run:") {
					t.Fatalf("the brief must report unavailable stores without a collection command: %s", brief)
				}
			} else if !strings.Contains(string(brief), "Collect first:\nmetasystem work wait run:run-1 --timeout 30m") {
				t.Fatalf("the configured seat brief lost the collection: %s", brief)
			}
		})
	}
}

func TestTheTickAsksForAPersonsRoundFromTheHostStore(t *testing.T) {
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	home, err := HomeStateRoot()
	if err != nil {
		t.Fatal(err)
	}
	b := newPersonWaitBed(t, false)
	step := launch.UnitStep{Name: "build", LaunchID: "launch-1", State: launch.StepRunning, StartedAt: b.now.Add(-time.Minute).Format(time.RFC3339)}
	run := launch.UnitRunRecord{ID: "run-1", Goal: "held-goal", State: "running", Rounds: []launch.UnitRound{{Number: 1, Steps: []launch.UnitStep{step}}}, Revisions: []launch.UnitRevision{{Attempt: 1, Person: "Wido"}}}
	writeStewardRecord(t, filepath.Join(home, "unit", run.ID, "run.json"), map[string]any{
		"id": run.ID, "goal": run.Goal, "state": run.State, "rounds": run.Rounds, "revisions": run.Revisions,
	})
	if err := (launch.Store{Root: filepath.Join(home, "launch")}).Create(launch.Record{ID: step.LaunchID, State: launch.Completed, FinishedAt: b.now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	work, err := goal.ClaimableWorkFromProjection(b.seat.projection(b.now), seatBedMachine, seatBusyDeadProber{})
	if err != nil {
		t.Fatal(err)
	}
	work.InFlight = []string{"claim:" + run.Goal}
	var questionID string
	for pass := 0; pass < 2; pass++ {
		f := newTickContinuationFixture(t, b.attentionPolicyBed, false)
		f.census = fakeCensus{workers: deadWorkers}
		f.cfg.Prober, f.cfg.ProviderHome = seatBusyDeadProber{}, testprovider.Home(b.root)
		f.dependencies.openWork.ReadClaimableBudgetedWork = func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return work, nil }
		f.dependencies.openWork.Seat = b.seat.seatDependencies()
		f.dependencies.openWork.Seat.Prober = seatBusyDeadProber{}
		f.dependencies.resolveMachine = func(string) (string, error) { return "ui", nil }
		if f.cfg.WorkStateRoot != "" || f.dependencies.openWork.Seat.WorkStateRoot != "" {
			t.Fatal("the tick must resolve its stores from the host home")
		}
		result, completed, err := f.run(t)
		if err != nil || !completed || result.Seat != nil {
			t.Fatalf("tick %d: %+v completed=%t err=%v", pass, result, completed, err)
		}
		qs := b.questions()
		if len(qs) != 1 {
			t.Fatalf("tick %d: collection questions = %+v; want one", pass, qs)
		}
		q := qs[0]
		if !q.Contract || q.Machine != "ui" || q.Decision == nil || *q.Decision != (channel.Decision{Kind: "uncollected-round", Subject: run.Goal, State: step.LaunchID}) || len(q.Options) != 2 || q.Options[0].Label != "Collect it" || !slices.Equal(q.Options[0].Act, []string{"metasystem", "work", "wait", "run:" + run.ID, "--timeout", "1m"}) || q.Options[1].Label != "Leave it" || q.State != "open" || !q.RetiredAt.IsZero() {
			t.Fatalf("tick %d: collection question = %+v", pass, q)
		}
		if pass == 0 {
			questionID = q.ID
		} else if q.ID != questionID {
			t.Fatalf("the second tick replaced question %s with %s", questionID, q.ID)
		}
		b.poll()
		if posts := b.channel.posts(t); len(posts) != 1 {
			t.Fatalf("tick %d: posted %d questions; want one", pass, len(posts))
		}
		b.now = b.now.Add(time.Minute)
		b.seat.now = b.now
	}
}

func TestAPersonsRoundAsksOnceAndTheCollectionSettlesIt(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"revision", "pane round one", "pane revision", "seat", "starts seat", "projection"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			b := newPersonWaitBed(t, false)
			home := t.TempDir()
			step := launch.UnitStep{Name: "build", LaunchID: "launch-1", State: launch.StepRunning, StartedAt: b.now.Add(-time.Minute).Format(time.RFC3339)}
			run := launch.UnitRunRecord{ID: "run-1", Goal: "held-goal", State: "running", Rounds: []launch.UnitRound{{Number: 1, Steps: []launch.UnitStep{step}}}}
			if row == "revision" || row == "projection" {
				run.Revisions = []launch.UnitRevision{{Attempt: 1, Person: "Wido"}}
			}
			if strings.HasPrefix(row, "pane") {
				run.Rounds[0].Steps[0].Retained = &launch.StartSpec{Actor: "person"}
			}
			if row == "pane revision" {
				run.Revisions = []launch.UnitRevision{{Attempt: 1}}
			}
			writeRun := func() {
				data, _ := json.Marshal(run)
				var fields map[string]any
				_ = json.Unmarshal(data, &fields)
				writeStewardRecord(t, filepath.Join(home, "unit", run.ID, "run.json"), fields)
			}
			writeRun()
			if err := (launch.Store{Root: filepath.Join(home, "launch")}).Create(launch.Record{ID: "launch-1", State: launch.Completed, FinishedAt: b.now.Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			var prober identity.Prober = seatBusyDeadProber{}
			if row == "projection" {
				writeStewardRecord(t, filepath.Join(b.root, "artifacts", "agents", "mains", "live.json"), map[string]any{"mainId": "live", "ownerLineage": SeatLineage, "pid": 41, "pidStartedAt": 100})
				prober = censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
					return identity.Exact{Pid: pid, StartedAt: time.Unix(100, 0)}, identity.Alive, nil
				})
			}
			work, err := goal.ClaimableWorkFromProjection(b.seat.projection(b.now), seatBedMachine, prober)
			if err != nil {
				t.Fatal(err)
			}
			if row == "projection" {
				if !slices.Contains(work.InFlight, "claim:held-goal") {
					t.Fatalf("announced holder missing from projection: %+v", work)
				}
			} else {
				work.InFlight = []string{"claim:held-goal"}
			}
			if row == "starts seat" {
				work.InFlight = nil
				run.Revisions = []launch.UnitRevision{{Attempt: 1, Person: "Wido"}}
				writeRun()
			}
			pass := func() TickResult {
				f := newTickContinuationFixture(t, b.attentionPolicyBed, false)
				f.census = fakeCensus{workers: deadWorkers}
				f.cfg.WorkStateRoot, f.cfg.Prober, f.cfg.ProviderHome = home, prober, testprovider.Home(b.root)
				f.dependencies.openWork.ReadClaimableBudgetedWork = func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return work, nil }
				f.dependencies.openWork.Seat = b.seat.seatDependencies()
				f.dependencies.openWork.Seat.WorkStateRoot = home
				f.dependencies.resolveMachine = func(string) (string, error) { return "ui", nil }
				result, completed, err := f.run(t)
				if err != nil || !completed {
					t.Fatalf("tick: %+v %t %v", result, completed, err)
				}
				b.now = b.now.Add(time.Minute)
				return result
			}
			result := pass()
			if row == "seat" || row == "starts seat" {
				if len(b.questions()) != 0 {
					t.Fatalf("asked instead of handing to the seat: %+v", b.questions())
				}
				if row == "starts seat" {
					if result.Seat == nil {
						t.Fatal("did not select the replacement seat")
					}
					work.InFlight = []string{"claim:held-goal"}
					orphan := UncollectedRun{Goal: run.Goal, Run: run.ID, LaunchID: "launch-1", Step: "build", EndedAt: b.now.Format(time.RFC3339)}
					if err := askUncollectedRounds(b.root, "ui", TickConfig{Now: b.now, WorkStateRoot: home, Prober: prober}, work, result.Seat, map[UncollectedRun]string{orphan: "Wido"}); err != nil {
						t.Fatal(err)
					}
					if len(b.questions()) != 0 {
						t.Fatal("asked after selecting the replacement seat")
					}
				}
				return
			}
			qs := b.questions()
			if len(qs) != 1 || !qs[0].Contract || qs[0].Decision == nil || *qs[0].Decision != (channel.Decision{Kind: "uncollected-round", Subject: run.Goal, State: "launch-1"}) || !slices.Equal(qs[0].Options[0].Act, []string{"metasystem", "work", "wait", "run:run-1", "--timeout", "1m"}) || qs[0].Options[1].Label != "Leave it" {
				t.Fatalf("collection question: %+v", qs)
			}
			b.poll()
			if err := QueueNotification(b.root, PendingNotification{Nonce: "collect-visibility", Message: "steward: the round waits"}); err != nil {
				t.Fatal(err)
			}
			if _, err := deliverPendingWithDependencies(b.root, pendingDeliveryDependencies{transport: func(string, string) error { t.Fatal("steward notice reached the person"); return nil }, funnel: noticeFunnel{clock: func() time.Time { return b.now }}}); err != nil {
				t.Fatal(err)
			}
			journal, err := os.ReadFile(NotificationJournalPath(b.root))
			if err != nil || !strings.Contains(string(journal), `"withheld":`) {
				t.Fatalf("notice was not journaled withheld: %s %v", journal, err)
			}
			for i := 0; i < 3; i++ {
				pass()
				b.poll()
			}
			if len(b.questions()) != 1 || len(b.channel.posts(t)) != 1 {
				t.Fatal("repeated tick posted another question")
			}
			q := b.questions()[0]
			reply, _ := json.Marshal(map[string]any{"thread_ts": q.Thread.ThreadID, "user": "UWIDO", "text": "leave it"})
			if err := os.WriteFile(filepath.Join(b.channel.dir, "replies.jsonl"), append(reply, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
			b.poll()
			left := b.questions()[0]
			if left.State != "open" || len(left.Attempts) != 1 || left.Attempts[0].Label != "Leave it" || left.Attempts[0].Outcome != "chosen" {
				t.Fatalf("Leave it did not keep the waiting question: %+v", left)
			}
			pass()
			b.poll()
			if len(b.questions()) != 1 {
				t.Fatal("the waiting decision was asked again")
			}
			left = b.questions()[0]
			left.State, left.ClosedAt = "closed", b.now
			data, _ := json.Marshal(left)
			if err := os.WriteFile(filepath.Join(b.root, "artifacts", "agents", "channel", "questions", left.ID+".json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			pass()
			b.poll()
			if len(b.questions()) != 1 {
				t.Fatal("a closed, unretired decision was asked again")
			}
			run.Rounds[0].Steps[0].State = launch.StepPassed
			writeRun()
			pass()
			closed, err := channel.ReadQuestion(b.root, q.ID)
			if err != nil || closed.RetiredAt.IsZero() || closed.State != "closed" {
				t.Fatalf("collection did not retire the question: %+v %v", closed, err)
			}
		})
	}
}
