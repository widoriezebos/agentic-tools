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
	JoinGates(board.Rows, tree, cardSettings, time.Date(2026, 8, 26, 1, 0, 0, 0, time.UTC))
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
	JoinGates(board.Rows, tree, cardSettings, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
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
	JoinGates(board.Rows, tree, cardSettings, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	gate = board.Rows[0].Gate
	if len(gate.HeldBy) != 1 || gate.HeldBy[0].By != "Wido" || gate.AutoLandsAt != "" || gate.Eligible {
		t.Fatalf("the hold = %+v", gate)
	}
}
