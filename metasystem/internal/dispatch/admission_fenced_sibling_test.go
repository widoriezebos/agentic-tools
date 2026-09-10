package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func addFencedAdmissionGoal(t *testing.T, root, id, stopID string, opids [3]string) {
	t.Helper()
	openedAt := "2026-08-28T08:00:00Z"
	claimedAt := "2026-08-28T09:00:00Z"
	closedAt := "2026-08-28T09:30:00Z"
	file := &goal.GoalFile{
		Id: id, State: goal.StateClaimed, Intent: "Hold the stopped work", Origin: goal.OriginMain,
		NextStep: "Wait for a human resume.", OpenedAt: openedAt, Revision: 3,
		Budget: &goal.Budget{
			ElapsedLimit: "1d", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1, ReviewRoundLimit: 3,
		},
		Claimed:        &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: claimedAt, Revision: 2},
		StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: "bed-m1", ClaimEpoch: 7, FenceEpoch: 1},
		StopFence: &goal.StopFence{
			StopID: stopID, Revision: 2, Epoch: 1, CapabilityGeneration: 2,
			ClosedAt: closedAt, Reason: goal.StopReasonElapsedLimit,
		},
		History: []goal.HistoryLine{
			{At: openedAt, Opid: goal.Opid(opids[0], "bed-m1", "coordinator"), Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
			{At: claimedAt, Opid: goal.Opid(opids[1], "bed-m1", "coordinator"), Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
			{At: closedAt, Opid: goal.Opid(opids[2], "bed-m1", "coordinator"), Verb: "breach-stop", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1},
		},
	}
	path := filepath.Join(root, "plans", "goals", id+".md")
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "plans/goals"},
		{"commit", "-q", "-m", "add fenced admission goal"},
		{"update-ref", goal.LocalLedgerBranch, "HEAD"},
		{"update-ref", goal.AcceptedRef, "HEAD"},
	} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
}

func TestGoalAdmissionIgnoresSiblingFencedClaim(t *testing.T) {
	root := revisionBindingBed(t, 2)
	addFencedAdmissionGoal(t, root, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
	})

	verdict, err := EvaluateGoalAdmission(root, "coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil || verdict.Refused() {
		t.Fatalf("the fenced sibling kept admission closed for the live goal: %+v %v", verdict, err)
	}
}

func TestGoalRevisionAdmissionStillRefusesTheFencedGoal(t *testing.T) {
	root := revisionBindingBed(t, 2)
	addFencedAdmissionGoal(t, root, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
	})

	verdict, err := EvaluateGoalRevisionAdmission(root, "stopped-a", 2, 5,
		time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil || !verdict.Refused() || verdict.Refusal == nil ||
		verdict.Refusal.Unknown == nil ||
		verdict.Refusal.Unknown.Reason != "launch fence closed by stop batch stop-stopped-a-r2-f1" ||
		verdict.LiveStopReason != goal.StopReasonElapsedLimit ||
		verdict.Refusal.LiveStopReason != goal.StopReasonElapsedLimit {
		t.Fatalf("the fenced goal did not retain its live-stop refusal: %+v %v", verdict, err)
	}
}

func TestGoalAdmissionIgnoresMultipleSiblingFencedClaims(t *testing.T) {
	root := revisionBindingBed(t, 2)
	addFencedAdmissionGoal(t, root, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
	})
	addFencedAdmissionGoal(t, root, "stopped-c", "stop-stopped-c-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAD", "01ARZ3NDEKTSV4RRFFQ69G5FAE", "01ARZ3NDEKTSV4RRFFQ69G5FAF",
	})

	verdict, err := EvaluateGoalAdmission(root, "coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil || verdict.Refused() {
		t.Fatalf("multiple fenced siblings kept admission closed for the live goal: %+v %v", verdict, err)
	}
}
