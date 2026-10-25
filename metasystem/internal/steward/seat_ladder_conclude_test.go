package steward

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestHolderConclusionStartsASuccessorOnTheSeatLadder(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	file := seatLandingGoal("held", 3, now.Add(-time.Hour))
	file.History = append(file.History, goal.HistoryLine{At: now.Format(time.RFC3339),
		Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC4", seatBedMachine, SeatLineage), Verb: goal.LandedVerb,
		Actor: seatBedMachine + "+" + SeatLineage, Targets: []string{file.Id}, Keep: -1, Reason: "landed under the lane"})
	file.Revision++
	bed := newSeatBed(t, file)
	bed.now = now
	result := bed.tick(deadWorkers)
	if result.Decision.Action != ActRevive || result.Seat == nil || !result.Seat.Held || result.Seat.Goal != file.Id {
		t.Fatalf("a recorded landing's completion starts no successor: %+v %+v", result.Decision, result.Seat)
	}
	bed.start(result.Seat)
	if len(bed.launcher.starts) != 1 {
		t.Fatalf("the steward did not start the holder's seat: %+v", bed.launcher.starts)
	}
}
