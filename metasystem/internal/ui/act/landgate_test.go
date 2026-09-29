package act

// The landing gate's acts from a signed-in session (g1-s70 D2, D4): the hold
// and its release on the goal's history, a repeat applied rather than refused,
// and land without a sitting refused without its reason.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestASittingHoldsTheGoalAndItsEndReleasesIt(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-held", actReviewedTip)
	if err := sessionFor(t, bed).Sitting("ui-held", record, true); err != nil {
		t.Fatalf("hold: %v", err)
	}
	holds := goal.HoldsOf(readGoal(t, bed, "ui-held"))
	if len(holds) != 1 || holds[0].By != "Wido" || holds[0].Record != "plans/reviews/review-of-ui-held.md" {
		t.Fatalf("the hold = %+v", holds)
	}
	lines := len(readGoal(t, bed, "ui-held").History)
	if err := sessionFor(t, bed).Sitting("ui-held", record, true); err != nil {
		t.Fatalf("the repeat was refused: %v", err)
	}
	testutil.Expect(t, "one hold line", len(readGoal(t, bed, "ui-held").History), lines)
	if err := sessionFor(t, bed).Sitting("ui-held", record, false); err != nil {
		t.Fatalf("release: %v", err)
	}
	if holds := goal.HoldsOf(readGoal(t, bed, "ui-held")); len(holds) != 0 {
		t.Fatalf("the release left %+v", holds)
	}
	if err := sessionFor(t, bed).Sitting("ui-held", record, false); err != nil {
		t.Fatalf("a second release was refused: %v", err)
	}
}

func TestLandWithoutASittingNeedsItsReasonAndLandsTheLine(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	waitingReview(t, bed, "ui-decided", actReviewedTip)
	err := sessionFor(t, bed).LandWithoutSitting("ui-decided", actReviewedTip, " ")
	if refusal, ok := err.(*Refusal); !ok || refusal.Code != "reason" || !strings.Contains(refusal.Message, "carries your reason") {
		t.Fatalf("no reason = %v", err)
	}
	if err := sessionFor(t, bed).LandWithoutSitting("ui-decided", actReviewedTip, "one-line doc fix"); err != nil {
		t.Fatalf("the decision: %v", err)
	}
	history := readGoal(t, bed, "ui-decided").History
	last := history[len(history)-1]
	testutil.Expect(t, "the line", last.Reason, "landed-without-sitting tip="+actReviewedTip+" by=Wido because=one-line doc fix")
	testutil.Expect(t, "the History outcome", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
	if err := sessionFor(t, bed).LandWithoutSitting("ui-decided", actReviewedTip, "again"); err != nil {
		t.Fatalf("the repeat was refused: %v", err)
	}
	testutil.Expect(t, "one line", len(readGoal(t, bed, "ui-decided").History), len(history))
}
