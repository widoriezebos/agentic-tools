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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
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

	recorded, err := sessionFor(t, bed).Review("ui-reviewed", seen(t, record, goal.VerdictClearToLand))
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	testutil.Expect(t, "the line", recorded.Line, "reviewed verdict=clear-to-land tip="+actReviewedTip+" record=plans/reviews/review-of-ui-reviewed.md by=Wido")
	last := readGoal(t, bed, "ui-reviewed").History
	testutil.Expect(t, "the History outcome", last[len(last)-1].AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)

	// The same press again is the act having its effect, not a refusal.
	if _, err := sessionFor(t, bed).Review("ui-reviewed", seen(t, record, goal.VerdictClearToLand)); err != nil {
		t.Fatalf("the repeat was refused: %v", err)
	}
	testutil.Expect(t, "one line", len(readGoal(t, bed, "ui-reviewed").History), len(last))
}

func TestAVerdictOnAnOutcomeDraftedForAnotherTipIsRefused(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-retipped", "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2")
	_, err := sessionFor(t, bed).Review("ui-retipped", seen(t, record, goal.VerdictClearToLand))
	refusal, ok := err.(*Refusal)
	if !ok || !strings.Contains(refusal.Message, "but the branch moved to") {
		t.Fatalf("an Outcome drafted for another tip = %v", err)
	}
	outside := filepath.Join(bed.root, "notes.md")
	if err := os.WriteFile(outside, []byte("- Goals: ui-retipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = sessionFor(t, bed).Review("ui-retipped", seen(t, outside, goal.VerdictClearToLand))
	if refusal, ok := err.(*Refusal); !ok || !strings.Contains(refusal.Message, "is not a review record in its home") {
		t.Fatalf("a record outside its home = %v", err)
	}
}

// seen is a verdict as the room asks it: the record, and the version and the
// saved revision of the record the person decided on (RF-02).
func seen(t *testing.T, record, verdict string) Reviewed {
	t.Helper()
	content, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	return Reviewed{Record: record, Verdict: verdict, Tip: actReviewedTip, Revision: project.RevisionOf(content), Branch: actReviewedTip}
}

// Fix round 1, F-3: the goal's branch read at the moment of the press is the
// version that would land, and a verdict on an older one is refused in words
// that name the one act — review the current version — and records nothing:
// the builder pushed between the room's read and the person's press.
func TestAVerdictOnAVersionTheBranchHasMovedPastIsRefused(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-moved", actReviewedTip)
	before := len(readGoal(t, bed, "ui-moved").History)
	for _, verdict := range []string{goal.VerdictClearToLand} {
		asked := seen(t, record, verdict)
		asked.Branch = "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"
		_, err := sessionFor(t, bed).Review("ui-moved", asked)
		refusal, ok := err.(*Refusal)
		if !ok || refusal.Code != "version-moved" || !strings.Contains(refusal.Message, "review the current version") {
			t.Fatalf("a verdict on a version the branch moved past = %v", err)
		}
	}
	testutil.Expect(t, "nothing was recorded", len(readGoal(t, bed, "ui-moved").History), before)
	// A branch that could not be read is not a moved one: the landing gate
	// decides at landing, and the person's word is not refused for it
	// (fix round 2, R-142-m1e).
	unread := seen(t, record, goal.VerdictClearToLand)
	unread.Branch = ""
	if _, err := sessionFor(t, bed).Review("ui-moved", unread); err != nil {
		t.Fatalf("a verdict whose branch could not be read was refused: %v", err)
	}
}

// sentBackReview is a goal waiting to land whose review's Outcome sends it
// back, with one finding answered fix.
func sentBackReview(t *testing.T, bed *ledgerBed, id string) string {
	t.Helper()
	record := waitingReview(t, bed, id, actReviewedTip)
	write(t, record, "# Review of "+id+"\n\n- Kind: review\n- Goals: "+id+"\n- Reviewed: "+actReviewedTip+" (the tip of goal/"+id+")\n\n"+
		"## Findings\n\n- 2026-10-03 · Wido · the lock stays held\n  - Answer: fix — waits for Send back\n\n"+
		"## Outcome\n\nVerdict: send back\n\nReviewed at: "+actReviewedTip+"\n\nExamined: the change index\n", 0o644)
	return record
}

// Fix round 2, R-142-m1e: the two browser paths that name no version keep
// working through the real act. The board card names the work a sent-back goal
// revises (Verdict.tsx: record, send-back, no brief, the work), and a send-back
// is not refused for a branch that moved since, which landing does not read.
func TestTheCardsNamedWorkIsRecordedWithoutAVersion(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := sentBackReview(t, bed, "ui-work")
	room := seen(t, record, goal.VerdictSendBack)
	room.Brief = "# Correction brief\n\n1. the lock stays held\n"
	if _, err := sessionFor(t, bed).Review("ui-work", room); err != nil {
		t.Fatalf("the room's send-back: %v", err)
	}
	card := Reviewed{Record: record, Verdict: goal.VerdictSendBack, Work: "writer",
		Branch: "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"}
	recorded, err := sessionFor(t, bed).Review("ui-work", card)
	if err != nil {
		t.Fatalf("the card's named work was refused: %v", err)
	}
	testutil.Expect(t, "it names the work", recorded.Work, "writer")
}

// Fix round 2, R-142-m1e: applying a review verdict the Partner proposed
// (proposing.ts: record, verdict, work; no brief, no version) is recorded
// through the real act; a clear to land is still refused where a newer
// version exists, as landing would refuse it.
func TestAProposedVerdictIsRecordedWithoutAVersion(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-proposed", actReviewedTip)
	proposed := Reviewed{Record: record, Verdict: goal.VerdictClearToLand, Branch: "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"}
	_, err := sessionFor(t, bed).Review("ui-proposed", proposed)
	if refusal, ok := err.(*Refusal); !ok || refusal.Code != "version-moved" {
		t.Fatalf("a proposed clear to land over a newer version = %v", err)
	}
	proposed.Branch = actReviewedTip
	recorded, err := sessionFor(t, bed).Review("ui-proposed", proposed)
	if err != nil {
		t.Fatalf("the proposed verdict was refused: %v", err)
	}
	testutil.Expect(t, "it is recorded", recorded.Verdict, goal.VerdictClearToLand)
}

// RF-02: the verdict names the version and the saved record the person
// decided on, and the authority compares both with the exact bytes it reads
// and publishes, after its own read: a record that names another version, or
// that another room changed between the person's press and this read — the
// fixture writes it between the two — is not published, and the refusal says
// so in words the page shows before asking again on the version as it stands.
func TestAVerdictIsBoundToTheVersionAndTheRecordThePersonSaw(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-seen", actReviewedTip)
	asked := seen(t, record, goal.VerdictClearToLand)

	other := asked
	other.Tip = "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"
	_, err := sessionFor(t, bed).Review("ui-seen", other)
	refusal, ok := err.(*Refusal)
	if !ok || refusal.Code != "version-changed" || !strings.Contains(refusal.Message, "another version than the one you decided on") {
		t.Fatalf("a verdict on another version = %v", err)
	}
	changed := asked
	write(t, record, "# Review of ui-seen\n\n- Kind: review\n- Goals: ui-seen\n- Reviewed: "+actReviewedTip+" (the tip of goal/ui-seen)\n\n"+
		"## Findings\n\n- 2026-10-03 · Ann · a finding written in another room\n  - Answer: fix — waits for Send back\n\n"+
		"## Outcome\n\nVerdict: clear to land\n\nReviewed at: "+actReviewedTip+"\n\nExamined: the change index\n", 0o644)
	_, err = sessionFor(t, bed).Review("ui-seen", changed)
	refusal, ok = err.(*Refusal)
	if !ok || refusal.Code != "version-changed" || !strings.Contains(refusal.Message, "changed after you decided") {
		t.Fatalf("a verdict on a record changed since = %v", err)
	}
	// A verdict that names only the version, or only the record's revision,
	// is held to the one it names.
	_, err = sessionFor(t, bed).Review("ui-seen", Reviewed{Record: record, Verdict: goal.VerdictClearToLand,
		Revision: asked.Revision, Branch: actReviewedTip})
	if refusal, ok := err.(*Refusal); !ok || refusal.Code != "version-changed" {
		t.Fatalf("a verdict naming a revision the record moved past = %v", err)
	}
	testutil.Expect(t, "nothing was recorded", len(readGoal(t, bed, "ui-seen").History) > 0 &&
		readGoal(t, bed, "ui-seen").History[len(readGoal(t, bed, "ui-seen").History)-1].Verb != "review", true)
	if _, err := sessionFor(t, bed).Review("ui-seen", seen(t, record, goal.VerdictClearToLand)); err != nil {
		t.Fatalf("the verdict on the record as it stands now: %v", err)
	}
}
