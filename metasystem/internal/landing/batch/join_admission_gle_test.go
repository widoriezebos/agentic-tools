package batch

import (
	"errors"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEBatchJoinRedAdmissionReturnsMemberBeforeMembership(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	err := PublishJoinWithAdmission(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil },
		func(string, Unit) (JoinAdmission, error) {
			return JoinAdmission{}, &JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: focused regression"}
		})
	var red *JoinAdmissionRed
	if !errors.As(err, &red) {
		t.Fatalf("red admission result=%v", err)
	}
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected || record.State != StateDissolved {
		t.Fatalf("red admission entered batch membership: state=%s unit=%+v", record.State, record.Units[0])
	}
}

// seedBoardCard writes a goal's card on seat as its stage owners would, at
// the given times, so the claim carries a history of closed spans.
func seedBoardCard(t *testing.T, seat board.Seat, goal string, stages []board.Stage, start time.Time) {
	t.Helper()
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	for index, stage := range stages {
		at := start.Add(time.Duration(index*10) * time.Minute)
		if err := board.WriteAt(home, board.Card{Seat: seat, Goal: goal, Stage: stage, Writer: board.Writer{At: at}}); err != nil {
			t.Fatal(err)
		}
	}
}

func liveBoardCard(t *testing.T, machine, goal string) board.Card {
	t.Helper()
	home, _ := board.Home()
	card, ok := board.LiveCard(home, goal)
	if !ok || card.Seat.Machine != machine {
		t.Fatalf("no live card for %s on %s: %+v", goal, machine, card)
	}
	return card
}

// TestGLEBatchJoinWritesJoinedAndCopiesTheHistory (R24, U10a-3, the join):
// the join writes joined with the batch on the goal's live card and copies
// the closed spans the board holds for the goal into Unit.Stages.
func TestGLEBatchJoinWritesJoinedAndCopiesTheHistory(t *testing.T) {
	t.Parallel()
	goalID := "goal-card-join"
	seat := board.Seat{Machine: "m1-batch-join", Installation: "/checkouts/m1-batch-join/metasystem"}
	start := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	seedBoardCard(t, seat, goalID, []board.Stage{board.StageClaimedIdle, board.StageBuild, board.StageReview, board.StageLandReady}, start)
	bed := newOrdinaryJoinBed(t)
	unit := joiningUnit(goalID, "chain-a")
	bed.expectJoin(unit, testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	must(t, PublishJoinWithAdmission(bed.store, testBatchID, unit, "seat+"+goalID, time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil },
		func(_ string, joining Unit) (JoinAdmission, error) {
			return JoinAdmission{Tree: joining.Admission.Tree, Status: "verified", AttemptID: "attempt-join"}, nil
		}))
	card := liveBoardCard(t, seat.Machine, goalID)
	if card.Stage != board.StageJoined || card.Batch != testBatchID {
		t.Fatalf("joined card = %+v", card)
	}
	joined := load(t, bed.store).Units[0]
	var stages []board.Stage
	for _, span := range joined.Stages {
		stages = append(stages, span.Stage)
	}
	if len(stages) != 3 || stages[0] != board.StageClaimedIdle || stages[1] != board.StageBuild || stages[2] != board.StageReview ||
		joined.Stages[2].Until.Sub(joined.Stages[2].Since) != 10*time.Minute {
		t.Fatalf("Unit.Stages = %+v", joined.Stages)
	}
}
