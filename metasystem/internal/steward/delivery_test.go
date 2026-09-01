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

	t.Run("30-minute slice alarms after 46 minutes", func(t *testing.T) {
		claimAt := now.Add(-2 * time.Hour)
		createdAt := now.Add(-46 * time.Minute)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "small-cap-delegate", fmt.Sprintf(`{"jobId":"small-cap-delegate","goalId":"delivery-goal","status":"running","createdAt":%q,"capMin":30,"capDeadline":%q}`,
			createdAt.Format(time.RFC3339), createdAt.Add(30*time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "delivery-goal") ||
			!strings.Contains(role.Reason, "small-cap-delegate") || !strings.Contains(role.Reason, "30-minute budget") ||
			!strings.Contains(role.Reason, "53.3%") {
			t.Fatalf("a slice more than 50 percent over its own budget was not reported: %+v", role)
		}
	})

	t.Run("200-minute slice does not alarm at its deadline", func(t *testing.T) {
		claimAt := now.Add(-4 * time.Hour)
		createdAt := now.Add(-200 * time.Minute)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "large-cap-delegate", fmt.Sprintf(`{"jobId":"large-cap-delegate","goalId":"delivery-goal","status":"running","createdAt":%q,"capMin":200,"capDeadline":%q}`,
			createdAt.Format(time.RFC3339), createdAt.Add(200*time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthAlive || !strings.Contains(role.Reason, "delivery-goal") {
			t.Fatalf("a slice at its own 200-minute deadline was not kept alive: %+v", role)
		}
	})

	t.Run("slice past cap plus 50 percent with newer receipt stays alive", func(t *testing.T) {
		claimAt := now.Add(-3 * time.Hour)
		createdAt := now.Add(-2 * time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, deliveryReceipt(createdAt.Add(time.Minute)))
		writeHealthJob(t, root, "delivered-slice", fmt.Sprintf(`{"jobId":"delivered-slice","goalId":"delivery-goal","status":"running","createdAt":%q,"capMin":30,"capDeadline":%q}`,
			createdAt.Format(time.RFC3339), createdAt.Add(30*time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthAlive || !strings.Contains(role.Reason, "delivery-goal") {
			t.Fatalf("a landing receipt newer than the slice creation did not keep the goal alive: %+v", role)
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
		createdAt := claimAt.Add(time.Minute)
		writeHealthJob(t, root, "young-delegate", fmt.Sprintf(`{"jobId":"young-delegate","goalId":"delivery-goal","status":"running","createdAt":%q,"capMin":120,"capDeadline":%q}`,
			createdAt.Format(time.RFC3339), createdAt.Add(120*time.Minute).Format(time.RFC3339)))

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

	t.Run("claim backstop uses a tighter goal elapsed limit", func(t *testing.T) {
		claimAt := now.Add(-181 * time.Minute)
		file := deliveryGoal(claimAt)
		file.Budget.ElapsedLimit = "2h"
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": file})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "delivery-goal") ||
			!strings.Contains(role.Reason, "150%") || !strings.Contains(role.Reason, "2h elapsed limit") {
			t.Fatalf("a claim beyond 150 percent of its tighter goal budget was not reported: %+v", role)
		}
	})

	t.Run("claim backstop does not use the slice norm for a longer goal budget", func(t *testing.T) {
		claimAt := now.Add(-20 * time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthAlive || !strings.Contains(role.Reason, "no landing receipt yet") {
			t.Fatalf("the flat slice-norm claim clock still triggered for a longer goal budget: %+v", role)
		}
	})

	t.Run("non-terminal job missing cap deadline raises dead", func(t *testing.T) {
		claimAt := now.Add(-time.Hour)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"delivery-goal": deliveryGoal(claimAt)})
		writeDeliveryReceipts(t, root, "1|1970-01-01T00:00:01Z|RECEIPT|goal=unrelated")
		writeHealthJob(t, root, "missing-cap-deadline", fmt.Sprintf(`{"jobId":"missing-cap-deadline","goalId":"delivery-goal","status":"running","createdAt":%q,"capMin":30}`,
			claimAt.Add(time.Minute).Format(time.RFC3339)))

		role := checkClaimedGoalDelivery(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "delivery-goal") ||
			!strings.Contains(role.Reason, "missing-cap-deadline.json") || !strings.Contains(role.Reason, "capDeadline is missing") {
			t.Fatalf("a non-terminal record without capDeadline did not fail safely: %+v", role)
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
