package steward

// Witnesses of the second B3 read (the reader's probes, kept as they
// failed on 118b71a4f).

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// R2-P3: the goal's own checkout (where it was reopened, so it is Live
// there) cannot be read this pass; another armed checkout's unfetched
// ledger still reads it Done. The unit's goal is judged ended.
func TestB3Read2UnitGoalEndedIgnoresAnUnreadableLedger(t *testing.T) {
	t.Parallel()
	ledgers := map[string]*ledgerView{
		"/own":   fixedLedger(nil, errors.New("ledger projection failed")),
		"/other": fixedLedger(&goal.TreeGoals{Done: map[string]*goal.GoalFile{"g": {}}}, nil),
	}
	ended, known := unitGoalEnded(func(root string) *ledgerView { return ledgers[root] })("g", "")
	if ended && known {
		t.Fatalf("goal judged ended and known while the ledger of a checkout could not be read")
	}
}
