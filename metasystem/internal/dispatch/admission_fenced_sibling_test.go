package dispatch

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestGoalAdmissionIgnoresSiblingFencedClaim(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	bed.addFenced(t, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
	})

	verdict, err := bed.admission("coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil || verdict.Refused() {
		t.Fatalf("the fenced sibling kept admission closed for the live goal: %+v %v", verdict, err)
	}
}

func TestGoalRevisionAdmissionStillRefusesTheFencedGoal(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	bed.addFenced(t, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
	})

	verdict, err := bed.revisionAdmission("stopped-a", 2, 5,
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
	bed := newGoalAdmissionBed(t, 2)
	bed.addFenced(t, "stopped-a", "stop-stopped-a-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAA", "01ARZ3NDEKTSV4RRFFQ69G5FAB", "01ARZ3NDEKTSV4RRFFQ69G5FAC",
	})
	bed.addFenced(t, "stopped-c", "stop-stopped-c-r2-f1", [3]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAD", "01ARZ3NDEKTSV4RRFFQ69G5FAE", "01ARZ3NDEKTSV4RRFFQ69G5FAF",
	})

	verdict, err := bed.admission("coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil || verdict.Refused() {
		t.Fatalf("multiple fenced siblings kept admission closed for the live goal: %+v %v", verdict, err)
	}
}
