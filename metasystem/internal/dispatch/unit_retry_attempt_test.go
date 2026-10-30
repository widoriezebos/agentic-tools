package dispatch

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// unitRetryGoal is a claimed goal whose unit round already counted its build
// as an attempt, with room for many more capped minutes.
func unitRetryGoal(t *testing.T, attempts uint64) (string, *goal.GoalFile, time.Time) {
	t.Helper()
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: file.Claimed.Machine, ClaimEpoch: 1}
	file.Budget.AttemptLimit, file.Budget.ReservedJobMinutesLimit, file.Budget.ActiveJobLimit = attempts, 1000, 5
	start := time.Date(2026, 8, 28, 8, 10, 0, 0, time.UTC)
	if err := ReserveUnitLaunch(root, "build", "run", file, 1, 5, start, false); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileUnitLaunch(root, "build", "run", file.Id, file.Claimed.Revision, "completed", "", start.Format(time.RFC3339Nano), start.Add(time.Minute).Format(time.RFC3339Nano), true); err != nil {
		t.Fatal(err)
	}
	return root, file, start.Add(2 * time.Minute)
}

func TestUnitRetryNeedsAnAttemptOfItsOwn(t *testing.T) {
	t.Parallel()
	root, file, now := unitRetryGoal(t, 1)
	err := ReserveUnitLaunch(root, "check-retry", "run", file, 1, 5, now, true)
	if refusal.CodeOf(err) != "BUDGET_REFUSED" || !strings.Contains(refusal.DetailOf(err), "attemptLimit used=1 limit=1") || strings.Contains(refusal.DetailOf(err), "reservedJobMinutesLimit") {
		t.Fatalf("a machine retry at the attempt limit was admitted or refused for minutes: %v", err)
	}
	if err := ReserveUnitLaunch(root, "check-relaunch", "run", file, 1, 5, now, false); err != nil {
		t.Fatalf("a relaunch within the round's counted attempt was refused: %v", err)
	}
}

func TestAdmittedUnitRetryKeepsItsAttempt(t *testing.T) {
	t.Parallel()
	root, file, now := unitRetryGoal(t, 2)
	before := ProjectBudget(root, file, now)
	if err := ReserveUnitLaunch(root, "check-retry", "run", file, 1, 5, now, true); err != nil {
		t.Fatalf("the machine retry was refused with an attempt to spare: %v", err)
	}
	pending := ProjectBudget(root, file, now)
	if pending.Status != BudgetKnown || pending.Attempts != 2 || pending.OpenCapMinutes != 5 || pending.ReservedJobMinutes != before.ReservedJobMinutes+5 {
		t.Fatalf("a pending retry did not count its attempt and cap: %+v", pending)
	}
	end := now.Add(3 * time.Minute)
	if err := ReconcileUnitLaunch(root, "check-retry", "run", file.Id, file.Claimed.Revision, "failed", "environment", now.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), true); err != nil {
		t.Fatal(err)
	}
	settled := ProjectBudget(root, file, end)
	if settled.Status != BudgetKnown || settled.Attempts != 2 || settled.ReservedJobMinutes != before.ReservedJobMinutes || settled.ObservedJobMinutes != before.ObservedJobMinutes {
		t.Fatalf("an environment stop dropped the retry's attempt or charged its minutes: before %+v settled %+v", before, settled)
	}
	// An environment relaunch the machine does not charge stays excluded.
	if err := ReserveUnitLaunch(root, "check-relaunch", "run", file, 1, 5, end, false); err != nil {
		t.Fatal(err)
	}
	later := end.Add(time.Minute)
	if err := ReconcileUnitLaunch(root, "check-relaunch", "run", file.Id, file.Claimed.Revision, "failed", "environment", end.Format(time.RFC3339Nano), later.Format(time.RFC3339Nano), true); err != nil {
		t.Fatal(err)
	}
	if excluded := ProjectBudget(root, file, later); excluded.Attempts != 2 || excluded.ReservedJobMinutes != settled.ReservedJobMinutes {
		t.Fatalf("an uncharged environment relaunch was counted: %+v", excluded)
	}
}

// A reservation a takeover failed before its collection takes the real
// outcome; a collected or started record still refuses a contradiction.
func TestASweptReservationTakesItsRealOutcome(t *testing.T) {
	t.Parallel()
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: file.Claimed.Machine, ClaimEpoch: 1}
	file.Budget.AttemptLimit, file.Budget.ReservedJobMinutesLimit, file.Budget.ActiveJobLimit = 5, 1000, 5
	reserved := time.Date(2026, 8, 28, 8, 10, 0, 0, time.UTC)
	sweep := func(id string, fields map[string]any) string {
		t.Helper()
		if err := ReserveUnitLaunch(root, id, "run", file, 1, 5, reserved, false); err != nil {
			t.Fatal(err)
		}
		_, path, _ := paths(root, id)
		record, err := readObject(path)
		if err != nil {
			t.Fatal(err)
		}
		record["status"], record["phase"], record["error"], record["endedAt"] = "failed", "claim-sweep", "stale-claim-epoch", "2026-08-28T08:11:00Z"
		for key, value := range fields {
			record[key] = value
		}
		if err := writeRecord(path, record); err != nil {
			t.Fatal(err)
		}
		return path
	}
	started, ended := reserved.Add(time.Minute).Format(time.RFC3339Nano), reserved.Add(3*time.Minute).Format(time.RFC3339Nano)
	reconcile := func(id, status string) error {
		return ReconcileUnitLaunch(root, id, "run", file.Id, file.Claimed.Revision, status, "", started, ended, true)
	}

	path := sweep("unit-read", nil)
	if err := reconcile("unit-read", "completed"); err != nil {
		t.Fatalf("the collection refused a swept reservation: %v", err)
	}
	record, _ := readObject(path)
	_, phase := record["phase"]
	_, failure := record["error"]
	if record["status"] != "completed" || record["startedAt"] != started || record["endedAt"] != ended || record["unitExecuted"] != true || phase || failure {
		t.Fatalf("the swept reservation did not take its real outcome: %v", record)
	}
	before, _ := os.ReadFile(path)
	if err := reconcile("unit-read", "completed"); err != nil {
		t.Fatalf("a repeated collection was refused: %v", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("a repeated collection changed the record:\nbefore %s\nafter  %s", before, after)
	}
	if err := reconcile("unit-read", "failed"); err == nil || !strings.Contains(err.Error(), "contradicts its terminal reservation") {
		t.Fatalf("a contradicting outcome on a collected record = %v", err)
	}

	path = sweep("job-started", map[string]any{"startedAt": "2026-08-28T08:10:30Z"})
	before, _ = os.ReadFile(path)
	if err := reconcile("job-started", "completed"); err == nil || !strings.Contains(err.Error(), "contradicts its terminal reservation") {
		t.Fatalf("a started record the sweep failed took a different outcome: %v", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("a refused collection changed the started record:\nbefore %s\nafter  %s", before, after)
	}
}
