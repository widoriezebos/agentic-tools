package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func withinTheTierBox() goal.Budget {
	return goal.Budget{ElapsedLimit: "2h", AttemptLimit: 3, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1, ReviewRoundLimit: 1}
}

func beyondTheTierNorm() goal.Budget {
	return goal.Budget{ElapsedLimit: "2h", AttemptLimit: 3, ReservedJobMinutesLimit: 5000, ActiveJobLimit: 1, ReviewRoundLimit: 1}
}

// approvedFor is a goal a human admitted at the enrolled terminal: the
// approval record binds its own History event and the exact budget tuple, so
// the claim gate judges it rather than refusing its shape.
func approvedFor(id string, budget goal.Budget) *goal.GoalFile {
	f := liveGoal(id, goal.StateApproved)
	f.Tier = 3
	f.Revision = 2
	f.Budget = &budget
	f.History = []goal.HistoryLine{
		{At: "2026-08-23T00:00:00Z", Opid: "op-open-" + id, Verb: "open", Actor: "m1+coordinator"},
		{At: "2026-08-24T00:00:00Z", Opid: "op-approve-" + id, Verb: "approve", Actor: "human:wido"},
	}
	f.Approved = &goal.ApprovalRecord{
		By: "human:wido", At: "2026-08-24T00:00:00Z", Revision: 2, Opid: "op-approve-" + id,
		Authority: goal.ApprovalAuthorityProven,
		Digest:    goal.ApprovalDigest(f.Intent, f.Tier, budget),
	}
	return f
}

// relayedApprovedFor is the same admission carried by a relayed human word,
// which a review date can retire.
func relayedApprovedFor(id string, budget goal.Budget, reviewBy string) *goal.GoalFile {
	f := approvedFor(id, budget)
	f.History[1].AuthorityOutcome = goal.AuthorityOutcomeTemporaryHumanWord
	f.History[1].AuthorityReviewBy = reviewBy
	f.Approved.Authority = goal.ApprovalAuthorityRelayed
	f.Approved.ReviewBy = reviewBy
	return f
}

func projectionOver(root string, tree *goal.TreeGoals) goal.Projection {
	return goal.Projection{
		Root: root, Tip: "0000000000000000000000000000000000000000",
		Tree: tree, Horizon: goal.NewApprovalHorizon(tree, observedAt),
	}
}

// TestAdmitTakesEveryVerdictFromTheClaimGate runs the engine's own frontier
// over one tree and proves each approved goal arrives in the bucket the gate
// put it in, including the ones a single unpinned read would skip.
func TestAdmitTakesEveryVerdictFromTheClaimGate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pinned := approvedFor("pinned", withinTheTierBox())
	pinned.Pinned = "m1"
	blocked := approvedFor("blocked", withinTheTierBox())
	blocked.Blocked = []string{"blocker"}
	tree := treeOf(
		approvedFor("ready", withinTheTierBox()),
		pinned,
		approvedFor("over-norm", beyondTheTierNorm()),
		blocked,
		relayedApprovedFor("expired", withinTheTierBox(), "2026-08-01"),
		liveGoal("blocker", goal.StateQueued),
	)

	admission := Admit(projectionOver(root, tree))

	testutil.Require(t, "answered", admission.Answered, true)
	testutil.Expect(t, "ready", admission.Ready, map[string]bool{"ready": true, "pinned": true})
	testutil.Expect(t, "blocked", admission.Blocked, map[string]bool{"blocked": true})
	testutil.Expect(t, "awaiting", admission.Awaiting, map[string]bool{"expired": true})
	testutil.Require(t, "one refusal", len(admission.Refused), 1)
	if cause := admission.Refused["over-norm"]; !strings.Contains(cause, "norm") {
		t.Fatalf("the refusal does not carry the gate's cause: %q", cause)
	}
}

// TestAdmitPlacesAPinnedGoalInItsOwnReadersBucket proves a pin changes who
// may claim the work, never whether it is shown.
func TestAdmitPlacesAPinnedGoalInItsOwnReadersBucket(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pinned := approvedFor("pinned", withinTheTierBox())
	pinned.Pinned = "m1"
	tree := treeOf(pinned)

	unpinnedRead, err := goal.Next(projectionOver(root, tree), "")
	testutil.Require(t, "the unpinned frontier", err, nil)
	testutil.Require(t, "the unpinned frontier skips the pin", unpinnedRead.Ready, []string(nil))

	admission := Admit(projectionOver(root, tree))
	testutil.Expect(t, "ready", admission.Ready, map[string]bool{"pinned": true})

	lane, _, gaps := LaneOf(pinned, tree, goal.NewApprovalHorizon(tree, observedAt), admission)
	testutil.Expect(t, "lane", lane, LaneReady)
	testutil.Expect(t, "gaps", gaps, []string(nil))
}

// TestAdmitReportsAnUnanswerableFrontierWhole holds this projection to the
// engine's own rule: uncertainty about the admission law is uncertainty about
// every candidate, so no goal is placed on a guess.
func TestAdmitReportsAnUnanswerableFrontierWhole(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.tier-3=nonsense\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tree := treeOf(approvedFor("ready", withinTheTierBox()))

	admission := Admit(projectionOver(root, tree))

	testutil.Expect(t, "answered", admission.Answered, false)
	if !strings.HasPrefix(admission.Message, "cannot answer claimable backlog") {
		t.Fatalf("the message is not the engine's: %q", admission.Message)
	}
	testutil.Expect(t, "ready", len(admission.Ready), 0)
	testutil.Expect(t, "blocked", len(admission.Blocked), 0)
	testutil.Expect(t, "awaiting", len(admission.Awaiting), 0)
	testutil.Expect(t, "refused", len(admission.Refused), 0)

	board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), admission)
	testutil.Require(t, "rows", len(board.Rows), 1)
	testutil.Expect(t, "lane", board.Rows[0].Lane, LaneUnknown)
	testutil.Expect(t, "gaps", board.Rows[0].Gaps, []string{"claim admission not answered: " + admission.Message})
	testutil.Expect(t, "ready is empty", board.Counts[LaneReady], 0)
}

func TestAdmitAnswersAnAbsentTreeWithoutPlacingAnything(t *testing.T) {
	t.Parallel()
	admission := Admit(goal.Projection{Root: t.TempDir()})
	testutil.Expect(t, "answered", admission.Answered, false)
	testutil.Expect(t, "message", admission.Message, "")
}
