package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

func callerErrorRun(t *testing.T, bed *workBed, run string) launch.UnitRunRecord {
	t.Helper()
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func noteDuringWait(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record launch.UnitRunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	record.Notes = append(record.Notes, "another command kept this note")
	data, err = json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return data
}

func assertCallerErrorSummary(t *testing.T, result intentResult, summary string) {
	t.Helper()
	if result.Summary != summary || utf8.RuneCountInString(strings.SplitN(result.Summary, "\n", 2)[0]) > 100 {
		t.Fatalf("caller error summary: %q", result.Summary)
	}
}

func assertComparisonProgress(t *testing.T, bed *workBed, run string, active uint64) launch.UnitRunRecord {
	t.Helper()
	record := callerErrorRun(t, bed, run)
	round := record.Rounds[0]
	if record.State != "running" || round.Stop != nil || round.Outcome != "" || round.Steps[1].RedCause != nil || round.Steps[1].Comparison.Verified {
		t.Fatalf("classification recorded a failure: state=%s outcome=%s stop=%v", record.State, round.Outcome, round.Stop)
	}
	budget := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if budget.Status != dispatchcore.BudgetKnown || budget.ActiveJobs != active {
		t.Fatalf("comparison budget: %+v", budget)
	}
	return record
}

func assertOwnComparison(t *testing.T, bed *workBed, run string) {
	t.Helper()
	code, result, _ := bed.work("work", "wait", "run:"+run)
	record := callerErrorRun(t, bed, run)
	if code != 1 || resultData(t, result)["outcome"] != "proof-red" || record.Rounds[0].Cause != "own" || !record.Rounds[0].Steps[1].Comparison.Verified || len(bed.starter.launched()) != 3 {
		t.Fatalf("comparison did not resume once: code=%d summary=%q cause=%s launches=%v", code, result.Summary, record.Rounds[0].Cause, bed.starter.launched())
	}
	budget := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if budget.Status != dispatchcore.BudgetKnown || budget.ActiveJobs != 0 {
		t.Fatalf("ended comparison remains active: %+v", budget)
	}
}

func TestARefusedWaitRecordsNoAttribution(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"claim", "checkout", "cap", "repair"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			bed, args := comparisonAccountingBed(t, 121)
			code, held, _ := bed.work(args...)
			if code != 1 || !strings.Contains(resultWords(held), "BUDGET_REFUSED") {
				t.Fatalf("comparison did not reach its budget hold: %d %s", code, held.Summary)
			}
			run := resultData(t, held)["run"].(string)
			if code, result := bed.runJSON(bed.owners(), "goal", "budget", bed.id, "4h/4/240m/1/2", "--by", "Wido", "--fixture-human-authority", "--lineage", "m1"); code != 0 {
				t.Fatalf("budget remedy: %d %s", code, result.Summary)
			}
			owners := bed.workOwners()
			const holder = "main-1792389436-6018-51cc28"
			next := []string{"metasystem", "work", "wait", "run:" + run}
			reason, cause := "after the cause in the details is repaired", "dispatch cap must be a positive integer"
			var claimErr error
			switch kind {
			case "claim":
				claimErr = goalHeldElsewhere{goal: bed.id, holder: "other-machine", holderLineage: holder, machine: "this-machine", lineage: "this-session"}
				next = []string{"metasystem", "goal", "claim", bed.id, "--take-over", "--reason", "TEXT"}
				reason, cause = "a person takes the goal over; or the session holding it builds", holder
			case "checkout":
				claimErr = refusal.New("OWNED-ELSEWHERE", "holder="+holder+" class=MAIN main=this-session", fmt.Errorf("this checkout is held by %s, not by this session\nfor a separate copy to write in, run: metasystem session isolate", holder))
				next = []string{"metasystem", "session", "isolate"}
				reason, cause = "then repeat the claim in the resulting writer session", holder
			case "cap":
				if err := os.WriteFile(filepath.Join(bed.root(), "metasystem.conf"), []byte("dispatch.cap-min=broken\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "repair":
				claimErr = &refusal.Coded{Code: "REPAIR_NEEDED", Reason: fmt.Errorf("the launch configuration needs repair"), Run: "metasystem settings check"}
				reason, cause = "metasystem settings check", "the launch configuration needs repair"
			}
			if claimErr != nil {
				owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error { return func() error { return claimErr } }
			}
			code, result := bed.runJSON(owners, "work", "wait", "run:"+run)
			assertCallerErrorSummary(t, result, "The next step could not start; no failure was recorded")
			if code != 1 || result.Outcome != intentRefused || resultData(t, result)["state"] != "running" || result.Next == nil || !slices.Equal(result.Next.Argv, next) || result.Next.Reason != reason || !strings.Contains(strings.Join(result.Details, "\n"), cause) {
				t.Fatalf("refusal lost its cause or remedy: code=%d outcome=%s state=%v next=%+v details=%v", code, result.Outcome, resultData(t, result)["state"], result.Next, result.Details)
			}
			record := assertComparisonProgress(t, bed, run, 0)
			if len(bed.starter.launched()) != 2 {
				t.Fatal("a refused observer launched the comparison")
			}
			id := record.Rounds[0].Steps[1].Comparison.LaunchID
			if _, err := os.Stat(filepath.Join(bed.root(), "artifacts", "agents", "jobs", id+".json")); !os.IsNotExist(err) {
				t.Fatalf("refused comparison has a reservation: %v", err)
			}
			if kind == "cap" {
				if err := os.WriteFile(filepath.Join(bed.root(), "metasystem.conf"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			assertOwnComparison(t, bed, run)
		})
	}
}

type callerComparisonStarter struct {
	unitProofStarter
	hold bool
}

func (s callerComparisonStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	if !s.hold || !strings.HasSuffix(id, "-base") {
		return s.unitProofStarter.StartSupervisor(id, state)
	}
	s.bed.starter.mu.Lock()
	s.bed.starter.kinds = append(s.bed.starter.kinds, "proof")
	s.bed.starter.mu.Unlock()
	_, err := s.bed.manager.Store.Update(id, func(r *launch.Record) error {
		supervisor, child := workProcessRef(10), workProcessRef(20)
		r.Supervisor, r.Child, r.State = &supervisor, &child, launch.Running
		return nil
	})
	return workProcessRef(10), err
}

func TestAttributionCompletesAfterWaiterExits(t *testing.T) {
	t.Parallel()
	bed, args := comparisonAccountingBed(t, 240)
	bed.manager.Supervisor = callerComparisonStarter{unitProofStarter: unitProofStarter{bed: bed, t: t}, hold: true}
	code, held, _ := bed.work(args...)
	if code != 3 || !strings.Contains(held.Summary, "comparing the red with the base") {
		t.Fatalf("comparison did not remain in progress: %d %s", code, held.Summary)
	}
	run := resultData(t, held)["run"].(string)
	record := assertComparisonProgress(t, bed, run, 1)
	id := record.Rounds[0].Steps[1].Comparison.LaunchID
	path := filepath.Join(bed.unitRoot, run, "run.json")
	sleep := bed.manager.Sleep
	var written []byte
	bed.manager.Sleep = func(d time.Duration) {
		if written == nil {
			written = noteDuringWait(t, path)
		}
		sleep(d)
	}
	code, retry, _ := bed.work("work", "wait", "run:"+run)
	assertCallerErrorSummary(t, retry, "This wait needs repeating; its error was not recorded as a work failure")
	after, err := os.ReadFile(path)
	if err != nil || written == nil || !bytes.Equal(after, written) || code != 3 || retry.Outcome != intentInProgress || retry.Next == nil || !slices.Equal(retry.Next.Argv, []string{"metasystem", "work", "wait", "run:" + run}) {
		t.Fatalf("wait overwrote another command: %d %+v err=%v equal=%v", code, retry.Summary, err, bytes.Equal(after, written))
	}
	assertComparisonProgress(t, bed, run, 1)
	base, err := bed.manager.Store.Read(id)
	if err != nil || base.State != launch.Running {
		t.Fatalf("comparison stopped during retry: state=%s err=%v", base.State, err)
	}
	state, _ := bed.manager.Store.StateDir(id)
	if err := os.WriteFile(filepath.Join(state, "exec.log"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bed.manager.Store.Update(id, func(r *launch.Record) error {
		code := 0
		r.State, r.ExitCode, r.FinishedAt = launch.Completed, &code, bed.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	bed.manager.Sleep = sleep
	assertOwnComparison(t, bed, run)
	if !slices.Contains(callerErrorRun(t, bed, run).Notes, "another command kept this note") {
		t.Fatal("resumption lost the other command's note")
	}
}

func TestAComparisonStartRetryReturnsToItsCaller(t *testing.T) {
	t.Parallel()
	bed, args := comparisonAccountingBed(t, 240)
	var written []byte
	var path string
	bed.manager.Supervisor = unitProofStarter{bed: bed, t: t, before: func(r launch.Record) {
		if strings.HasSuffix(r.ID, "-base") {
			run := strings.Split(r.ID, "-r1-")[0]
			path = filepath.Join(bed.unitRoot, run, "run.json")
			written = noteDuringWait(t, path)
		}
	}}
	code, result, _ := bed.work(args...)
	assertCallerErrorSummary(t, result, "This wait needs repeating; its error was not recorded as a work failure")
	after, err := os.ReadFile(path)
	if code != 3 || err != nil || written == nil || !bytes.Equal(after, written) {
		t.Fatalf("start retry overwrote another command: %d %+v err=%v equal=%v", code, result.Summary, err, bytes.Equal(after, written))
	}
	run := resultData(t, result)["run"].(string)
	record := callerErrorRun(t, bed, run)
	if record.Rounds[0].Steps[1].Comparison.Verified {
		t.Fatal("start retry verified the comparison")
	}
	assertOwnComparison(t, bed, run)
	if !slices.Contains(callerErrorRun(t, bed, run).Notes, "another command kept this note") {
		t.Fatal("resumption lost the other command's note")
	}
}
