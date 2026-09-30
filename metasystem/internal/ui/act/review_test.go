package act

// The verdict from the room (g1-s69 D1, §8): goal review under a signed-in
// session, the record resolved in its home and read as it now stands, a
// repeat applied rather than refused, and an Outcome drafted for another tip
// refused in the engine's words.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

const actReviewedTip = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"

func waitingReview(t *testing.T, bed *ledgerBed, id, reviewedAt string) string {
	t.Helper()
	openGoal(t, bed, id)
	if err := sessionFor(t, bed).Approve(id, box()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	claim(t, bed, id)
	if result, err := goal.LandReady(request(t, bed), id); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("land-ready: %+v %v", result, err)
	}
	record := filepath.Join(bed.root, "plans", "reviews", "review-of-"+id+".md")
	write(t, record, "# Review of "+id+"\n\n- Kind: review\n- Goals: "+id+"\n- Reviewed: "+actReviewedTip+" (the tip of goal/"+id+")\n\n"+
		"## Findings\n\n## Outcome\n\nVerdict: clear to land\n\nReviewed at: "+reviewedAt+"\n\nExamined: the change index\n", 0o644)
	return record
}

func TestAVerdictFromASignedInSessionLandsTheLineAndTheRecord(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-reviewed", actReviewedTip)

	recorded, err := sessionFor(t, bed).Review("ui-reviewed", Reviewed{Record: record, Verdict: goal.VerdictClearToLand})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	testutil.Expect(t, "the line", recorded.Line, "reviewed verdict=clear-to-land tip="+actReviewedTip+" record=plans/reviews/review-of-ui-reviewed.md by=Wido")
	last := readGoal(t, bed, "ui-reviewed").History
	testutil.Expect(t, "the History outcome", last[len(last)-1].AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)

	// The same press again is the act having its effect, not a refusal.
	if _, err := sessionFor(t, bed).Review("ui-reviewed", Reviewed{Record: record, Verdict: goal.VerdictClearToLand}); err != nil {
		t.Fatalf("the repeat was refused: %v", err)
	}
	testutil.Expect(t, "one line", len(readGoal(t, bed, "ui-reviewed").History), len(last))
}

func TestAVerdictOnAnOutcomeDraftedForAnotherTipIsRefused(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-retipped", "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2")
	_, err := sessionFor(t, bed).Review("ui-retipped", Reviewed{Record: record, Verdict: goal.VerdictClearToLand})
	refusal, ok := err.(*Refusal)
	if !ok || !strings.Contains(refusal.Message, "but the branch moved to") {
		t.Fatalf("an Outcome drafted for another tip = %v", err)
	}
	outside := filepath.Join(bed.root, "notes.md")
	if err := os.WriteFile(outside, []byte("- Goals: ui-retipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = sessionFor(t, bed).Review("ui-retipped", Reviewed{Record: outside, Verdict: goal.VerdictClearToLand})
	if refusal, ok := err.(*Refusal); !ok || !strings.Contains(refusal.Message, "is not a review record in its home") {
		t.Fatalf("a record outside its home = %v", err)
	}
}
