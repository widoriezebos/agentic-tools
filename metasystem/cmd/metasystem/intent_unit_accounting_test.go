package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestIntentUnitLaunchAccountingStartRefusal(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.manager.Supervisor = nil
	brief := bed.brief("brief.md", "Build the unit.\n")
	args := append([]string{"work", "build", bed.id, "no-supervisor", "--brief", brief, "--lines", "20"}, workCheck...)
	for attempt := 0; attempt < 2; attempt++ {
		code, result, _ := bed.work(args...)
		if code != 1 {
			t.Fatalf("start refusal: exit=%d result=%+v", code, result)
		}
		data := resultData(t, result)
		run, _ := data["run"].(string)
		unit, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
		if err != nil {
			t.Fatal(err)
		}
		step := unit.Rounds[0].Steps[0]
		if step.State != launch.StepFailed || step.FinishedAt == "" || !strings.Contains(step.Reason, "supervisor is unavailable") {
			t.Fatalf("failure was not saved: %+v", step)
		}
		if _, err := bed.manager.Store.Read(step.LaunchID); !os.IsNotExist(err) {
			t.Fatalf("refusal invented a launch: %v", err)
		}
		projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
		if projection.Status != dispatchcore.BudgetKnown || projection.Attempts != 0 || projection.ReservedJobMinutes != 0 || projection.ActiveJobs != 0 {
			t.Fatalf("unexecuted reservation remains charged: %+v", projection)
		}
	}
	bed.manager.Supervisor = bed.starter
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "after-refusal", "--brief", brief, "--lines", "20"}, workCheck...)...)
	if code != 0 || resultData(t, result)["outcome"] != "green" {
		t.Fatalf("later build did not proceed: exit=%d result=%+v", code, result)
	}
}

func TestIntentUnitLaunchAccountingBudgetAdmission(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, prior string
		limit       uint64
	}{
		{name: "breached", prior: "completed", limit: 1},
		{name: "remaining-cap", limit: 119},
		{name: "attempt-limit", prior: "completed", limit: 240},
		{name: "active-limit", prior: "running", limit: 240},
		{name: "unknown", prior: "unreadable", limit: 240},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBedWith(t, func(file *goal.GoalFile) {
				workApprovedBox(file)
				file.StopCapability = &goal.StopCapability{Generation: 2, Revision: file.Claimed.Revision, Machine: file.Claimed.Machine, ClaimEpoch: 1}
				file.Budget.ReservedJobMinutesLimit = row.limit
				file.Budget.AttemptLimit = 1
				if row.name == "active-limit" {
					file.Budget.AttemptLimit, file.Budget.ActiveJobLimit = 4, 1
				}
				file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
			})
			path := filepath.Join(bed.root(), "artifacts", "agents", "jobs", "prior.json")
			var prior []byte
			if row.prior != "" {
				file := bed.goalFile(bed.id)
				status := row.prior
				if status == "unreadable" {
					status = "completed"
				}
				prior, _ = json.Marshal(map[string]any{"jobId": "prior", "operationId": "prior", "goalId": bed.id, "goalRevision": file.Claimed.Revision,
					"status": status, "capMin": 2, "pid": 20, "startedAt": bed.manager.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano), "endedAt": bed.manager.Now().UTC().Format(time.RFC3339Nano)})
				data := prior
				if row.prior == "unreadable" {
					data = []byte("{")
				}
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			brief := bed.brief("brief.md", "Build the unit.\n")
			args := append([]string{"work", "build", bed.id, "budgeted", "--brief", brief, "--lines", "20"}, workCheck...)
			code, refused, _ := bed.work(args...)
			if code != 1 || !strings.Contains(resultWords(refused), "metasystem goal budget "+bed.id) || refused.Next == nil || !slices.Equal(refused.Next.Argv, []string{"metasystem", "goal", "budget", bed.id, "BOX"}) || len(bed.starter.launched()) != 0 {
				t.Fatalf("budget refusal launched work or has no remedy: exit=%d result=%+v", code, refused)
			}
			remedy := append(append([]string(nil), refused.Next.Argv[1:4]...), "4h/2/240m/2/2", "--by", "Wido", "--fixture-human-authority", "--lineage", "m1")
			code, raised := bed.runJSON(bed.owners(), remedy...)
			if code != 0 {
				t.Fatalf("person's budget act: exit=%d result=%+v", code, raised)
			}
			if row.prior == "unreadable" {
				if err := os.WriteFile(path, prior, 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, result, _ := bed.work(args...)
			if code != 0 || resultData(t, result)["outcome"] != "green" || len(bed.starter.launched()) != 2 {
				t.Fatalf("raised budget did not admit retained work: exit=%d result=%+v", code, result)
			}
		})
	}
}

func TestIntentUnitLaunchAccountingUnaccountedMigration(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	starter := &accountingStarter{bed: bed}
	bed.manager.Supervisor = starter
	brief := bed.brief("brief.md", "Build the unit.\n")
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "legacy", "--brief", brief, "--lines", "20"}, workCheck...)...)
	if code != 3 {
		t.Fatalf("retained run: exit=%d result=%+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	reservation := filepath.Join(bed.root(), "artifacts", "agents", "jobs", starter.held+".json")
	if err := os.Remove(reservation); err != nil {
		t.Fatal(err)
	}
	if _, err := bed.manager.Store.Update(starter.held, func(record *launch.Record) error {
		exit := 0
		record.State, record.ExitCode, record.FinishedAt = launch.Completed, &exit, bed.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for replay := 0; replay < 2; replay++ {
		code, result, _ = bed.work("work", "build", "run:"+run)
		if code != 0 || resultData(t, result)["outcome"] != "green" {
			t.Fatalf("legacy advance: exit=%d result=%+v", code, result)
		}
		data, err := os.ReadFile(reservation)
		if err != nil {
			t.Fatal(err)
		}
		var recorded map[string]any
		if err := json.Unmarshal(data, &recorded); err != nil {
			t.Fatal(err)
		}
		if recorded["unitUnaccounted"] != true || recorded["status"] != "completed" {
			t.Fatalf("legacy outcome not recorded as unaccounted: %s", data)
		}
		projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
		if projection.Status != dispatchcore.BudgetKnown || projection.ReservedJobMinutes != 1 || projection.ActiveJobs != 0 || len(starter.ids) != 2 {
			t.Fatalf("legacy execution charged or collection replay launched work: %+v ids=%v", projection, starter.ids)
		}
	}
}

type accountingStarter struct {
	bed  *workBed
	deny string
	ids  []string
	held string
}

func (s *accountingStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	s.ids = append(s.ids, id)
	denied := record.Kind == s.deny
	if denied {
		s.deny = ""
	} else if s.deny == "" && s.held == "" {
		s.held = id
	}
	_, err = s.bed.manager.Store.Update(id, func(current *launch.Record) error {
		supervisor := workProcessRef(99)
		current.Supervisor = &supervisor
		current.FinishedAt = s.bed.manager.Now().UTC().Format(time.RFC3339Nano)
		if denied {
			exit, child := 1, workProcessRef(20)
			current.ExitCode, current.Child = &exit, &child
			current.State, current.Cause, current.Reason = launch.Failed, "sandbox-denied", "sandbox denied command execution"
			return nil
		}
		child := workProcessRef(20)
		current.Child = &child
		if id == s.held {
			current.State = launch.Running
			current.FinishedAt = ""
			return nil
		}
		exit := 0
		current.State, current.ExitCode = launch.Completed, &exit
		if record.Kind == "read" {
			yes := true
			current.VerdictCounts, current.Measurement.Verdict = &yes, "pass"
		}
		return nil
	})
	return workProcessRef(99), err
}

func TestIntentUnitLaunchAccountingEnvironmentRetry(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"build", "proof", "read"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			starter := &accountingStarter{bed: bed, deny: kind}
			bed.manager.Supervisor = starter
			brief := bed.brief("brief.md", "Read each round: yes\nBuild the unit.\n")
			file := bed.goalFile(bed.id)
			now := bed.manager.Now()
			before := dispatchcore.ProjectBudget(bed.root(), file, now)
			if before.Status != dispatchcore.BudgetKnown || before.Attempts != 0 || before.ReservedJobMinutes != 0 {
				t.Fatalf("before: %+v", before)
			}
			args := append([]string{"work", "build", bed.id, "accounted", "--brief", brief, "--lines", "20"}, workCheck...)
			code, result, _ := bed.work(args...)
			if code != 3 {
				t.Fatalf("pending retry: exit=%d result=%+v", code, result)
			}
			run := resultData(t, result)["run"].(string)
			file = bed.goalFile(bed.id)
			pending := dispatchcore.ProjectBudget(bed.root(), file, bed.manager.Now())
			// One physical execution is open; completed successes cost one minute each.
			successes := uint64(0)
			denied := ""
			for _, id := range starter.ids {
				execution, err := bed.manager.Store.Read(id)
				if err != nil {
					t.Fatal(err)
				}
				if execution.Cause == "sandbox-denied" {
					denied = id
					continue
				}
				if execution.State == launch.Completed {
					successes++
				}
			}
			if pending.Status != dispatchcore.BudgetKnown || pending.Attempts != 1 || pending.ActiveJobs != 1 || pending.ObservedJobMinutes != successes || pending.OpenCapMinutes != 120 || pending.ReservedJobMinutes != 120+successes {
				t.Fatalf("denied execution charged or retry not reserved: %+v unknown=%+v", pending, pending.Unknown)
			}
			if starter.held == "" {
				t.Fatal("no physical execution held")
			}
			// Complete the exact retained execution, as the supervisor's terminal writer does.
			if _, err := bed.manager.Store.Update(starter.held, func(record *launch.Record) error {
				exit := 0
				record.State, record.ExitCode, record.FinishedAt = launch.Completed, &exit, bed.manager.Now().UTC().Format(time.RFC3339Nano)
				if record.Kind == "read" {
					yes := true
					record.VerdictCounts, record.Measurement.Verdict = &yes, "pass"
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			code, result, _ = bed.work("work", "wait", "run:"+run)
			if code != 0 || resultData(t, result)["outcome"] != "green" {
				t.Fatalf("successful retry: exit=%d result=%+v", code, result)
			}
			after := dispatchcore.ProjectBudget(bed.root(), file, bed.manager.Now())
			if after.Status != dispatchcore.BudgetKnown || after.Attempts != 1 || after.ReservedJobMinutes != 3 || after.ObservedJobMinutes != 3 || after.OpenCapMinutes != 0 || after.ActiveJobs != 0 {
				t.Fatalf("after: %+v", after)
			}
			raw, err := os.ReadFile(filepath.Join(bed.unitRoot, run, "run.json"))
			if err != nil {
				t.Fatal(err)
			}
			var unit launch.UnitRunRecord
			if err := json.Unmarshal(raw, &unit); err != nil {
				t.Fatal(err)
			}
			if len(unit.Rounds) != 1 || len(starter.ids) != 4 {
				t.Fatalf("physical history lost: rounds=%d executions=%v", len(unit.Rounds), starter.ids)
			}
			executions := 0
			for _, step := range unit.Rounds[0].Steps {
				executions += len(step.LaunchIDs)
			}
			if executions != 4 || denied == "" {
				t.Fatalf("physical history=%+v", unit.Rounds[0].Steps)
			}
			// Waiting again replays collection of the retained physical executions.
			code, result, _ = bed.work("work", "wait", "run:"+run)
			if code != 0 {
				t.Fatalf("replayed collection: exit=%d result=%+v", code, result)
			}
			replay := dispatchcore.ProjectBudget(bed.root(), file, bed.manager.Now())
			// Elapsed observation time can advance; all spending evidence stays identical.
			after.Elapsed, replay.Elapsed = 0, 0
			if !reflect.DeepEqual(after, replay) || len(starter.ids) != 4 {
				t.Fatalf("collection refunded twice: before=%+v after=%+v ids=%v", after, replay, starter.ids)
			}
		})
	}
}

func TestIntentUnitLaunchAccountingBaseline(t *testing.T) {
	t.Parallel()
	bed := unitFlakeBuild(t, true, false)
	projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if projection.Status != dispatchcore.BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 3 || projection.ObservedJobMinutes != 3 || projection.ActiveJobs != 0 {
		t.Fatalf("the branch and baseline executions were not settled: %+v unknown=%+v", projection, projection.Unknown)
	}
}

func TestIntentUnitLaunchAccountingUnreadableCollection(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	starter := &accountingStarter{bed: bed, deny: "build"}
	bed.manager.Supervisor = starter
	brief := bed.brief("brief.md", "Build the unit.\n")
	args := append([]string{"work", "build", bed.id, "recovery", "--brief", brief, "--lines", "20"}, workCheck...)
	code, result, _ := bed.work(args...)
	if code != 3 {
		t.Fatalf("pending execution: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	if _, err := bed.manager.Store.Update(starter.held, func(record *launch.Record) error {
		exit := 0
		record.State, record.ExitCode, record.FinishedAt = launch.Completed, &exit, "unreadable"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	code, result, _ = bed.work("work", "wait", "run:"+run)
	if code != 1 {
		t.Fatalf("unreadable execution was treated as collected: %d %+v", code, result)
	}
	if len(starter.ids) != 2 {
		t.Fatal("collection failure started another execution")
	}
	projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if projection.Status != dispatchcore.BudgetKnown || projection.OpenCapMinutes != 120 || projection.ObservedJobMinutes != 0 {
		t.Fatalf("unreadable collection invented a settlement: %+v", projection)
	}
	if _, err := bed.manager.Store.Update(starter.held, func(record *launch.Record) error {
		record.FinishedAt = bed.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	code, result, _ = bed.work("work", "wait", "run:"+run)
	if code != 0 || resultData(t, result)["outcome"] != "green" || len(starter.ids) != 3 {
		t.Fatalf("retained collection did not recover: %d %+v ids=%v", code, result, starter.ids)
	}
	projection = dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if projection.Status != dispatchcore.BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 2 || projection.ActiveJobs != 0 {
		t.Fatalf("recovered spending: %+v", projection)
	}
	state, err := bed.manager.Store.StateDir(starter.ids[0])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "record.json")
	retained, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	code, result, _ = bed.work("work", "wait", "run:"+run)
	if code != 1 || len(starter.ids) != 3 {
		t.Fatalf("missing history was silently collected: %d %+v", code, result)
	}
	if err := os.WriteFile(path, retained, 0600); err != nil {
		t.Fatal(err)
	}
	code, result, _ = bed.work("work", "wait", "run:"+run)
	if code != 0 || len(starter.ids) != 3 {
		t.Fatalf("restored history did not recover: %d %+v", code, result)
	}
}
