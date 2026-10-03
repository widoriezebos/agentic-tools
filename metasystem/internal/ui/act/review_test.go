package act

// The verdict from the room (g1-s69 D1, §8): goal review under a signed-in
// session, the record resolved in its home and read as it now stands, a
// repeat applied rather than refused, and an Outcome drafted for another tip
// refused in the engine's words.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// withAnotherRoomsFinding is the review record as another room saves it: the
// same version and Outcome, and one finding added.
func withAnotherRoomsFinding(id string) string {
	return "# Review of " + id + "\n\n- Kind: review\n- Goals: " + id + "\n- Reviewed: " + actReviewedTip + " (the tip of goal/" + id + ")\n\n" +
		"## Findings\n\n- 2026-10-03 · Ann · a finding written in another room\n  - Answer: unanswered\n\n" +
		"## Outcome\n\nVerdict: clear to land\n\nReviewed at: " + actReviewedTip + "\n\nExamined: the change index\n"
}

// F-2 of read 0096f159: the record is compared once before the authority
// takes its lock and once more under it, and only bytes that passed both are
// published. The fixture saves another room's finding into the record after
// the first comparison passed — while the act is assembling its request under
// the lock, which is where the fence is read — so the bytes the first
// comparison passed are no longer the record: nothing is published, in the
// same words as a record changed before the press reached the authority, and
// the person's verdict given again on the record as it now stands publishes
// that record.
func TestAVerdictIsComparedAgainUnderTheAuthoritysOwnLock(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-between", actReviewedTip)
	asked := seen(t, record, goal.VerdictClearToLand)
	authority := sessionFor(t, bed)
	taken := authority.reads
	var once sync.Once
	authority.reads.fence = func(root string) error {
		once.Do(func() { write(t, record, withAnotherRoomsFinding("ui-between"), 0o644) })
		return taken.fence(root)
	}
	before := len(readGoal(t, bed, "ui-between").History)

	_, err := authority.Review("ui-between", asked)
	refusal := refusalOf(t, err)
	testutil.Expect(t, "the refusal's code", refusal.Code, "version-changed")
	testutil.Expect(t, "in the words a record changed before the press reached the authority is refused with", refusal.Message,
		"the review changed after you decided, in another room; nothing was recorded, so decide again")
	testutil.Expect(t, "nothing was recorded", len(readGoal(t, bed, "ui-between").History), before)
	if _, err := goal.ReadPublished(bed.endpoint(), "plans/reviews/review-of-ui-between.md"); err == nil {
		t.Fatal("the record was published although it changed after the first comparison")
	}

	if _, err := authority.Review("ui-between", seen(t, record, goal.VerdictClearToLand)); err != nil {
		t.Fatalf("the verdict given again on the record as it now stands: %v", err)
	}
	published, err := goal.ReadPublished(bed.endpoint(), "plans/reviews/review-of-ui-between.md")
	if err != nil {
		t.Fatalf("reading what was published: %v", err)
	}
	testutil.Expect(t, "the published record is the one the verdict was given on", string(published), withAnotherRoomsFinding("ui-between"))
}

// The room's own save of a review record takes the authority's lock (F-2 of
// read 0096f159), so a save pressed in another room while a verdict is being
// published waits until it is published, and what is published is the record
// the verdict was compared with.
func TestARecordSaveWaitsWhileAVerdictIsPublished(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	record := waitingReview(t, bed, "ui-saving", actReviewedTip)
	original, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	authority := sessionFor(t, bed)
	seam := scripting(bed)
	publishing, release, once := make(chan struct{}), make(chan struct{}), sync.Once{}
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		once.Do(func() {
			close(publishing)
			<-release
		})
		return plain.Capture(opid)
	}
	answered := make(chan error, 1)
	go func() {
		_, err := authority.Review("ui-saving", seen(t, record, goal.VerdictClearToLand))
		answered <- err
	}()

	// No clock decides this (R-104-m1e): the verdict, paused inside its
	// publication, holds the clone's one lock, and the save runs its write
	// under that same lock, so the one cannot run inside the other.
	<-publishing
	if ownerOf(bed.root).publications.TryLock() {
		t.Fatal("the clone's one lock was free while the verdict was being published, so a save could have run inside it")
	}
	close(release)
	if err := <-answered; err != nil {
		t.Fatalf("the verdict: %v", err)
	}
	if err := authority.Holding(func() error {
		if ownerOf(bed.root).publications.TryLock() {
			return errors.New("the save ran without the clone's one lock")
		}
		return os.WriteFile(record, []byte(withAnotherRoomsFinding("ui-saving")), 0o644)
	}); err != nil {
		t.Fatalf("the save once the verdict was published: %v", err)
	}
	published, err := goal.ReadPublished(bed.endpoint(), "plans/reviews/review-of-ui-saving.md")
	if err != nil {
		t.Fatalf("reading what was published: %v", err)
	}
	testutil.Expect(t, "what is published is the record the verdict was compared with", string(published), string(original))
	now, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Expect(t, "the save landed after it", string(now), withAnotherRoomsFinding("ui-saving"))
}
