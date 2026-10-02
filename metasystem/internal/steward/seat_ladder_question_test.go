package steward

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A goal an open channel question names is the person's to move, like one
// whose next step waits on a human word: the seat ladder starts no seat for
// it and names the next free goal instead; once the question is answered or
// withdrawn the goal is free again.
func TestAGoalWithAnOpenQuestionStartsNoSeat(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("asked", "Build it."))
	bed.questions = []goal.OpenQuestion{{ID: "q-asked", Goal: "asked", Machine: "m1", Lineage: "seat"}}
	result := bed.tick(deadWorkers)
	if result.Decision.Action != ActNotify || result.Seat != nil || !strings.Contains(result.Decision.Reason, "asked") {
		t.Fatalf("a goal with an open question starts no seat and is named: %+v %+v", result.Decision, result.Seat)
	}
	bed.put(seatReadyGoal("free", "Build it."))
	result = bed.tick(deadWorkers)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "free" {
		t.Fatalf("the free goal is named, not the asked one: %+v %+v", result.Decision, result.Seat)
	}
	bed.questions = nil
	bed.drop("free")
	result = bed.tick(deadWorkers)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "asked" {
		t.Fatalf("answered or withdrawn, the goal is free again: %+v %+v", result.Decision, result.Seat)
	}
}

// A held goal of the seat lineage that an open question names is not due:
// no successor starts for it.
func TestAHeldGoalWithAnOpenQuestionStartsNoSuccessor(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
	bed.questions = []goal.OpenQuestion{{ID: "q-held", Goal: "held", Machine: seatBedMachine, Lineage: SeatLineage}}
	result := bed.tick(deadWorkers)
	if result.Seat != nil || len(bed.launcher.starts) != 0 {
		t.Fatalf("a held goal with an open question starts no successor: %+v %+v", result.Decision, result.Seat)
	}
}
