package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

func TestReconcileRepairsAForeignTerminalRecord(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"cancelled", "failed", "owned"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			root, file := budgetProjectionRoot(t), budgetGoal()
			file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: file.Claimed.Machine, ClaimEpoch: 1}
			start := time.Date(2026, 8, 28, 8, 10, 0, 0, time.UTC)
			end := start.Add(time.Minute)
			if err := ReserveUnitLaunch(root, "execution", "unit", file, 1, 5, start, false); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "artifacts/agents/jobs/execution.json")
			record, err := readObject(path)
			if err != nil {
				t.Fatal(err)
			}
			record["status"], record["endedAt"] = status, end.Format(time.RFC3339Nano)
			if status == "failed" {
				record["phase"], record["error"] = "launch", "launch-interrupted"
			} else {
				record["phase"] = "cancelled"
			}
			if status == "owned" {
				record["status"], record["unitExecuted"], record["startedAt"] = "failed", false, start.Format(time.RFC3339Nano)
			}
			if err := writeRecord(path, record); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			for repeat := 0; repeat < 2; repeat++ {
				err := ReconcileUnitLaunch(root, "execution", "unit", file.Id, 3, "completed", "", start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), true)
				if status == "owned" {
					if err == nil || !strings.Contains(err.Error(), "contradicts its terminal reservation") {
						t.Fatalf("a conflicting owner outcome was accepted: %v", err)
					}
					after, _ := os.ReadFile(path)
					if string(after) != string(before) {
						t.Fatal("a conflicting collection changed the reservation")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				settled, err := readObject(path)
				if err != nil || settled["status"] != "completed" || settled["startedAt"] != start.Format(time.RFC3339Nano) || settled["endedAt"] != end.Format(time.RFC3339Nano) || settled["unitExecuted"] != true || settled["phase"] != record["phase"] || settled["error"] != record["error"] {
					t.Fatalf("collection lost outcome or history: %v, %v", settled, err)
				}
				projection := ProjectBudget(root, file, end)
				if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 1 {
					t.Fatalf("repaired outcome did not charge the attempt: %+v", projection)
				}
			}
		})
	}
}

func TestANeverExecutedSettlementFreesItsSlot(t *testing.T) {
	t.Parallel()
	root, file := budgetProjectionRoot(t), budgetGoal()
	file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: file.Claimed.Machine, ClaimEpoch: 1}
	at := time.Date(2026, 8, 28, 8, 10, 0, 0, time.UTC)
	reserve := func(id string) error { return ReserveUnitLaunch(root, id, id, file, 1, 5, at, false) }
	settle := func(id, status string, executed bool, end time.Time) error {
		return ReconcileUnitLaunch(root, id, id, file.Id, 3, status, "", at.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), executed)
	}
	if err := reserve("a"); err != nil {
		t.Fatal(err)
	}
	if err := reserve("b"); refusal.CodeOf(err) != "BUDGET_REFUSED" || !strings.Contains(refusal.DetailOf(err), "activeJobLimit") {
		t.Fatalf("a second job was not refused at the slot limit: %v", err)
	}
	if err := settle("a", "cancelled", false, at); err != nil {
		t.Fatal(err)
	}
	projection := ProjectBudget(root, file, at)
	if projection.Status != BudgetKnown || projection.ActiveJobs != 0 || projection.ReservedJobMinutes != 0 || projection.ObservedJobMinutes != 0 || projection.Attempts != 0 {
		t.Fatalf("unexecuted cancellation still spends: %+v", projection)
	}
	if err := reserve("b"); err != nil {
		t.Fatalf("the freed slot did not admit the next job: %v", err)
	}
	path := filepath.Join(root, "artifacts/agents/jobs/a.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	at = at.Add(time.Minute)
	if err := settle("a", "cancelled", false, at); err != nil {
		t.Fatalf("a later collection was not the same settlement: %v", err)
	}
	for _, outcome := range []struct {
		status   string
		executed bool
	}{{"failed", false}, {"cancelled", true}} {
		if err := settle("a", outcome.status, outcome.executed, at); err == nil {
			t.Fatal("contradictory status or execution was accepted")
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatalf("repeat or contradiction changed the reservation: %v", err)
	}
	if err := settle("b", "cancelled", false, at); err != nil {
		t.Fatal(err)
	}
	if err := reserve("c"); err != nil {
		t.Fatal(err)
	}
	if err := settle("c", "completed", true, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(root, "artifacts/agents/jobs/c.json")
	before, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := settle("c", "completed", true, at.Add(2*time.Minute)); err == nil || !strings.Contains(err.Error(), "contradicts its terminal reservation") {
		t.Fatalf("changed executed times were accepted: %v", err)
	}
	after, err = os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatalf("contradictory executed times changed the reservation: %v", err)
	}
}

func TestStewardBookingAndSessionCollectionShareOneTerminalWrite(t *testing.T) {
	t.Parallel()
	root, file := budgetProjectionRoot(t), budgetGoal()
	file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: file.Claimed.Machine, ClaimEpoch: 1}
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	if err := ReserveUnitLaunch(root, "execution", "unit", file, 1, 5, now, false); err != nil {
		t.Fatal(err)
	}
	exit := 0
	execution := launch.Record{ID: "execution", State: launch.Completed, ExitCode: &exit, StartedAt: now.Format(time.RFC3339Nano), FinishedAt: now.Add(time.Minute).Format(time.RFC3339Nano)}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := BookEndedUnitLaunch(root, execution, "unit", file.Id, "", now.Add(10*time.Minute))
		results <- err
	}()
	go func() {
		<-start
		results <- ReconcileUnitLaunch(root, execution.ID, "unit", file.Id, 3, string(execution.State), "", execution.StartedAt, execution.FinishedAt, true)
	}()
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "artifacts/agents/jobs/execution.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if changed, err := BookEndedUnitLaunch(root, execution, "unit", file.Id, "", now.Add(time.Hour)); err != nil || changed {
		t.Fatalf("repeat booking changed terminal: %v %v", changed, err)
	}
	if err := ReconcileUnitLaunch(root, execution.ID, "unit", file.Id, 3, string(execution.State), "", execution.StartedAt, execution.FinishedAt, true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	afterInfo, _ := os.Stat(path)
	if err != nil || string(before) != string(after) || !info.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("joined collection rewrote terminal evidence", err)
	}
	budget := ProjectBudget(root, file, now.Add(time.Hour))
	if budget.Status != BudgetKnown || budget.ActiveJobs != 0 || budget.OpenCapMinutes != 0 || budget.ObservedJobMinutes != 1 || budget.Attempts != 1 {
		t.Fatalf("collection counted twice: %+v", budget)
	}
}

func TestStewardBookingNeverOverwritesAPersonsTerminalReservation(t *testing.T) {
	t.Parallel()
	root, file := budgetProjectionRoot(t), budgetGoal()
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(root, "artifacts/agents/jobs/execution.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeRecord(path, map[string]any{"jobId": "execution", "operationId": "unit-launch:execution", "unitRun": "unit", "goalId": file.Id, "status": "cancelled", "phase": "cancelled", "endedAt": now.Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	execution := launch.Record{ID: "execution", State: launch.Completed, StartedAt: now.Format(time.RFC3339Nano), FinishedAt: now.Add(time.Minute).Format(time.RFC3339Nano)}
	if changed, err := BookEndedUnitLaunch(root, execution, "unit", file.Id, "", now.Add(time.Hour)); err != nil || changed {
		t.Fatalf("person's stop was overwritten: %v %v", changed, err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("person's cancellation was rewritten")
	}
}
