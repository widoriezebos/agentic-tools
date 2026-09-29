package backlog

// The sent-back reading (g1-s69 D2, §8): a claimed goal whose history carries a
// send-back newer than its Landing is sent back, a phase of In progress, and a
// later land-ready puts it in Review again. The Landing record is never read
// as cleared; the history decides.

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

const (
	verdictTip    = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"
	verdictRecord = "plans/reviews/review-of-landing.md"
	reviewOpid    = "01J5X0000000000000000000R1-mac-ui-1a2b3c4d"
)

func waitingToLand() *goal.GoalFile {
	f := liveGoal("landing", goal.StateClaimed)
	f.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
	f.History = append(f.History, goal.HistoryLine{At: "2026-08-26T00:00:00Z", Opid: "op-land", Verb: "land-ready", Actor: "m1+coordinator"})
	f.Landing = &goal.LandingRecord{At: "2026-08-26T00:00:00Z", Opid: "op-land"}
	return f
}

func reviewed(f *goal.GoalFile, verdict string) *goal.GoalFile {
	reason := "reviewed verdict=" + verdict + " tip=" + verdictTip + " record=" + verdictRecord + " by=Wido"
	if verdict == goal.VerdictSendBack {
		reason += " brief=" + goal.BriefPathFor(verdictRecord)
	}
	f.History = append(f.History, goal.HistoryLine{At: "2026-08-27T00:00:00Z", Opid: reviewOpid, Verb: "review", Actor: "human:Wido", Targets: []string{f.Id}, Reason: reason})
	return f
}

func rowFor(t *testing.T, f *goal.GoalFile) Row {
	t.Helper()
	tree := treeOf(f)
	board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))
	return board.Rows[0]
}

func TestTheBoardReadsASendBackNewerThanTheLandingAsSentBack(t *testing.T) {
	t.Parallel()
	row := rowFor(t, reviewed(waitingToLand(), goal.VerdictSendBack))
	testutil.Expect(t, "lane", row.Lane, LaneInProgress)
	testutil.Expect(t, "phase", row.Phase, PhaseSentBack)
	testutil.Expect(t, "the Landing is read as it stands", row.Claim.LandingAt, "2026-08-26T00:00:00Z")
	testutil.Expect(t, "verdict", row.Verdict, &Verdict{
		Verdict: goal.VerdictSendBack, By: "Wido", At: "2026-08-27T00:00:00Z", Tip: verdictTip, Record: verdictRecord,
		Brief: goal.BriefPathFor(verdictRecord),
	})

	// A later land-ready writes a newer Landing: Review again, no verdict on it.
	f := reviewed(waitingToLand(), goal.VerdictSendBack)
	f.History = append(f.History, goal.HistoryLine{At: "2026-08-28T00:00:00Z", Opid: "op-land-2", Verb: "land-ready", Actor: "m1+coordinator"})
	f.Landing = &goal.LandingRecord{At: "2026-08-28T00:00:00Z", Opid: "op-land-2"}
	row = rowFor(t, f)
	testutil.Expect(t, "lane after a later land-ready", row.Lane, LaneReview)
	testutil.Expect(t, "phase after a later land-ready", row.Phase, "landing")
	if row.Verdict != nil {
		t.Fatalf("a verdict older than the Landing is on the card: %+v", row.Verdict)
	}
}

func TestClearToLandStaysInReviewAndTheCardCarriesTheVerdict(t *testing.T) {
	t.Parallel()
	row := rowFor(t, reviewed(waitingToLand(), goal.VerdictClearToLand))
	testutil.Expect(t, "lane", row.Lane, LaneReview)
	testutil.Expect(t, "phase", row.Phase, "landing")
	if row.Verdict == nil || row.Verdict.Verdict != goal.VerdictClearToLand || row.Verdict.By != "Wido" || row.Verdict.Tip != verdictTip {
		t.Fatalf("the card does not carry the verdict: %+v", row.Verdict)
	}
}

func TestTheCardCarriesTheHoldersAnswer(t *testing.T) {
	t.Parallel()
	f := reviewed(waitingToLand(), goal.VerdictSendBack)
	f.History = append(f.History, goal.HistoryLine{At: "2026-08-27T01:00:00Z", Opid: "op-answer", Verb: "send-back", Actor: "m1+coordinator",
		Targets: []string{f.Id}, Reason: "send-back needs-work candidates=discovery,writer review=" + reviewOpid})
	row := rowFor(t, f)
	testutil.Expect(t, "lane while the holder asks", row.Lane, LaneInProgress)
	testutil.Expect(t, "candidates", row.Verdict.Candidates, []string{"discovery", "writer"})
	testutil.Expect(t, "answered", row.Verdict.Answered, true)

	f = reviewed(waitingToLand(), goal.VerdictSendBack)
	f.History = append(f.History, goal.HistoryLine{At: "2026-08-27T01:00:00Z", Opid: "op-answer", Verb: "send-back", Actor: "m1+coordinator",
		Targets: []string{f.Id}, Reason: "send-back attempt=3 review=" + reviewOpid})
	row = rowFor(t, f)
	testutil.Expect(t, "attempt", row.Verdict.Attempt, 3)
}
