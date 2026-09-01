package steward

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func deliveryGoal(claimAt time.Time) *goal.GoalFile {
	history := []goal.HistoryLine{
		{
			At: claimAt.Add(-time.Hour).Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-m1-00000000",
			Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"delivery-goal"}, Keep: -1,
		},
		{
			At: claimAt.Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-bed-m1-00000001",
			Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{"delivery-goal"}, Keep: -1,
		},
	}
	return &goal.GoalFile{
		Id: "delivery-goal", State: goal.StateClaimed, Intent: "Deliver the claimed goal", Origin: goal.OriginMain,
		NextStep: "Land working evidence.", OpenedAt: claimAt.Add(-time.Hour).Format(time.RFC3339), Revision: 2,
		Budget:  &goal.Budget{ElapsedLimit: "12h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 2},
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: claimAt.Format(time.RFC3339), Revision: 2},
		History: history,
	}
}

func writeDeliveryReceipts(t *testing.T, root string, lines ...string) {
	t.Helper()
	path := filepath.Join(root, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func deliveryReceipt(at time.Time) string {
	return fmt.Sprintf("%d|%s|RECEIPT|goal=delivery-goal|outcome=shipped", at.Unix(), at.Format(time.RFC3339))
}

func TestClaimedGoalDeliveryVerdicts(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	t.Run("failed job after claim raises dead", func(t *testing.T) {
		claimAt := now.Add(-2 * time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "failed-delegate", fmt.Sprintf(`{"jobId":"failed-delegate","goalId":"delivery-goal","status":"failed","startedAt":%q,"endedAt":%q,"error":"delegate process died"}`,
			claimAt.Add(time.Minute).Format(time.RFC3339), now.Add(-30*time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "delivery-goal") ||
			!strings.Contains(role.Reason, "failed-delegate") || !strings.Contains(role.Reason, "delegate process died") ||
			!strings.Contains(role.Reason, "30m0s ago") {
			t.Fatalf("failed work after the claim was not reported: %+v", role)
		}
	})

	t.Run("failed job with newer receipt stays alive", func(t *testing.T) {
		claimAt := now.Add(-2 * time.Hour)
		failedAt := now.Add(-time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, deliveryReceipt(now.Add(-30*time.Minute)))
		writeHealthJob(t, root, "recovered-delegate", fmt.Sprintf(`{"jobId":"recovered-delegate","goalId":"delivery-goal","status":"failed","createdAt":%q,"endedAt":%q,"error":"temporary failure"}`,
			claimAt.Add(time.Minute).Format(time.RFC3339), failedAt.Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthAlive || !strings.Contains(role.Reason, "delivery-goal") || !strings.Contains(role.Reason, "30m0s old") {
			t.Fatalf("newer delivery evidence did not recover the failed job: %+v", role)
		}
	})

	t.Run("old claim with reserved jobs and no receipt raises dead", func(t *testing.T) {
		claimAt := now.Add(-7 * time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "burning-delegate", fmt.Sprintf(`{"jobId":"burning-delegate","goalId":"delivery-goal","status":"running","createdAt":%q}`,
			claimAt.Add(time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "delivery-goal") ||
			!strings.Contains(role.Reason, "7.0 hours") || !strings.Contains(role.Reason, "6.0-hour threshold") ||
			!strings.Contains(role.Reason, "1 job reserved since claim") {
			t.Fatalf("burn without delivery was not reported: %+v", role)
		}
	})

	t.Run("old claim with newer receipt stays alive", func(t *testing.T) {
		claimAt := now.Add(-7 * time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, deliveryReceipt(now.Add(-time.Hour)))
		writeHealthJob(t, root, "delivered-delegate", fmt.Sprintf(`{"jobId":"delivered-delegate","goalId":"delivery-goal","status":"completed","startedAt":%q}`,
			claimAt.Add(time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthAlive || !strings.Contains(role.Reason, "delivery-goal") || !strings.Contains(role.Reason, "1h0m0s old") {
			t.Fatalf("newer delivery evidence did not keep an old claim alive: %+v", role)
		}
	})

	t.Run("young claim stays alive", func(t *testing.T) {
		claimAt := now.Add(-time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "young-delegate", fmt.Sprintf(`{"jobId":"young-delegate","goalId":"delivery-goal","status":"running","startedAt":%q}`,
			claimAt.Add(time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthAlive || !strings.Contains(role.Reason, "delivery-goal") || !strings.Contains(role.Reason, "no landing receipt yet") {
			t.Fatalf("a young claim was not kept alive: %+v", role)
		}
	})

	t.Run("unreadable receipts log raises dead", func(t *testing.T) {
		claimAt := now.Add(-time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "receipts.log") || !strings.Contains(role.Reason, "unreadable") {
			t.Fatalf("a missing receipt log did not fail safely: %+v", role)
		}
	})

	t.Run("configured slice norm sets the burn threshold", func(t *testing.T) {
		claimAt := now.Add(-7 * time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.slice-norm-hours=8\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "configured-norm-delegate", fmt.Sprintf(`{"jobId":"configured-norm-delegate","goalId":"delivery-goal","status":"running","createdAt":%q}`,
			claimAt.Add(time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthAlive {
			t.Fatalf("the configured 12-hour threshold was not used for a seven-hour claim: %+v", role)
		}
	})

	t.Run("unreadable job record raises dead", func(t *testing.T) {
		claimAt := now.Add(-time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "broken-job", `{not-json`)

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "delivery-goal") || !strings.Contains(role.Reason, "broken-job.json") {
			t.Fatalf("an unreadable job record did not fail safely: %+v", role)
		}
	})
}

func TestClaimedGoalDeliveryRoleIsInHealthOrder(t *testing.T) {
	for _, role := range healthRoleOrder {
		if role == RoleClaimedGoalDelivery {
			return
		}
	}
	t.Fatal("claimed-goal-delivery is absent from the published health role order")
}
