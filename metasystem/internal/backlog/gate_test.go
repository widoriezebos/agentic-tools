package backlog

// The card's gate reading (g1-s70 §6): a Review-lane row carries the gate's
// reading from the history and the settings alone — the clock below the tier,
// waiting for a person at or above it, the hold, and the newest word.

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

var cardSettings = goal.GateSettings{HumanFromTier: 2, AutoAfter: 4 * time.Hour, AutoAfterText: "4h"}

func TestAReviewRowCarriesTheGatesReading(t *testing.T) {
	t.Parallel()
	below := waitingToLand()
	below.Tier = 1
	tree := treeOf(below)
	board := Project(tree, goal.ApprovalHorizon{}, Admission{})
	JoinGates(board.Rows, tree, cardSettings, time.Date(2026, 8, 26, 1, 0, 0, 0, time.UTC), nil)
	gate := board.Rows[0].Gate
	if gate == nil {
		t.Fatalf("the Review row carries no gate: %+v", board.Rows[0])
	}
	testutil.Expect(t, "waits", gate.WaitsForHuman, false)
	testutil.Expect(t, "auto lands at", gate.AutoLandsAt, "2026-08-26T04:00:00Z")
	testutil.Expect(t, "eligible", gate.Eligible, false)
	testutil.Expect(t, "setting", gate.AutoAfter, "4h")

	above := reviewed(waitingToLand(), goal.VerdictClearToLand)
	above.Tier = 3
	tree = treeOf(above)
	board = Project(tree, goal.ApprovalHorizon{}, Admission{})
	JoinGates(board.Rows, tree, cardSettings, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), nil)
	gate = board.Rows[0].Gate
	testutil.Expect(t, "waits above", gate.WaitsForHuman, true)
	testutil.Expect(t, "no clock above", gate.AutoLandsAt, "")
	if gate.Reviewed == nil || gate.Reviewed.Kind != goal.VerdictClearToLand || gate.Reviewed.By != "Wido" || gate.Reviewed.Tip != verdictTip {
		t.Fatalf("the word = %+v", gate.Reviewed)
	}

	held := waitingToLand()
	held.Tier = 1
	held.History = append(held.History, goal.HistoryLine{At: "2026-08-26T00:30:00Z", Opid: "op-hold", Verb: "review", Actor: "human:Wido",
		Reason: goal.SittingReason(true, verdictRecord, "Wido")})
	tree = treeOf(held)
	board = Project(tree, goal.ApprovalHorizon{}, Admission{})
	JoinGates(board.Rows, tree, cardSettings, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), nil)
	gate = board.Rows[0].Gate
	if len(gate.HeldBy) != 1 || gate.HeldBy[0].By != "Wido" || gate.AutoLandsAt != "" || gate.Eligible {
		t.Fatalf("the hold = %+v", gate)
	}
}

// A word to land read against the goal branch's tip at origin (SOL-S70-04):
// where the branch has left the word's tip, the reading says so and where the
// branch is now, for a verdict and a decision to land without a sitting alike;
// at the word's tip, or where the tip was not read, it says nothing more.
func TestAWordAtATipTheBranchLeftNeedsTheWordAgain(t *testing.T) {
	t.Parallel()
	const moved = "a1b2c3d4e5f60718293a4b5c6d7e8f9011223344"
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	wordOf := func(f *goal.GoalFile, branchTip func(string) (string, error)) *GateWord {
		t.Helper()
		f.Tier = 3
		tree := treeOf(f)
		board := Project(tree, goal.ApprovalHorizon{}, Admission{})
		JoinGates(board.Rows, tree, cardSettings, at, branchTip)
		if board.Rows[0].Gate == nil || board.Rows[0].Gate.Reviewed == nil {
			t.Fatalf("the row carries no word: %+v", board.Rows[0].Gate)
		}
		return board.Rows[0].Gate.Reviewed
	}
	branchAt := func(tip string) func(string) (string, error) {
		return func(id string) (string, error) {
			if id != "landing" {
				t.Fatalf("the branch read is goal/%s's", id)
			}
			return tip, nil
		}
	}

	cleared := wordOf(reviewed(waitingToLand(), goal.VerdictClearToLand), branchAt(moved))
	testutil.Expect(t, "a cleared word at a moved tip", *cleared, GateWord{Kind: goal.VerdictClearToLand, By: "Wido", Tip: verdictTip, Moved: true, BranchTip: moved})

	without := waitingToLand()
	without.History = append(without.History, goal.HistoryLine{At: "2026-08-27T00:00:00Z", Opid: reviewOpid, Verb: goal.LandWithoutSittingVerb, Actor: "human:Wido",
		Targets: []string{without.Id}, Reason: "landed-without-sitting tip=" + verdictTip + " by=Wido because=one-line doc fix"})
	decided := wordOf(without, branchAt(moved))
	testutil.Expect(t, "a decision at a moved tip", *decided, GateWord{Kind: goal.LandWithoutSittingVerb, By: "Wido", Tip: verdictTip, Moved: true, BranchTip: moved})

	standing := wordOf(reviewed(waitingToLand(), goal.VerdictClearToLand), branchAt(verdictTip))
	testutil.Expect(t, "a word at the branch's tip", *standing, GateWord{Kind: goal.VerdictClearToLand, By: "Wido", Tip: verdictTip})
	unread := wordOf(reviewed(waitingToLand(), goal.VerdictClearToLand), nil)
	testutil.Expect(t, "a word whose branch was not read", *unread, GateWord{Kind: goal.VerdictClearToLand, By: "Wido", Tip: verdictTip})
}

// JoinGates reads the gate only where it can and only where it applies
// (batch 18, covering g1-s70's new JoinGates): with no tree it fills nothing,
// and a row outside the Review lane carries no gate even when its goal is live.
func TestJoinGatesFillsOnlyReviewRowsOfAKnownTree(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	f := waitingToLand()
	tree := treeOf(f)

	rows := []Row{{ID: f.Id, Lane: LaneReview}}
	JoinGates(rows, nil, cardSettings, at, nil)
	if rows[0].Gate != nil {
		t.Fatalf("no tree, yet a gate: %+v", rows[0].Gate)
	}

	rows = []Row{{ID: f.Id, Lane: LaneInProgress}, {ID: f.Id, Lane: LaneReview}}
	JoinGates(rows, tree, cardSettings, at, nil)
	if rows[0].Gate != nil {
		t.Fatalf("an In Progress row carries a gate: %+v", rows[0].Gate)
	}
	if rows[1].Gate == nil {
		t.Fatal("the Review row of the same live goal carries no gate")
	}
}
