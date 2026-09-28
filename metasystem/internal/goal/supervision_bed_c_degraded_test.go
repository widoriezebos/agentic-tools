package goal

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// stop-hook-monitor, S4-15(d): a turn verdict whose own state cannot be read
// allows the Stop as infrastructure and never composes an all-clear it cannot
// vouch for, even with open work settled and no goal.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSupCUnreadableVerdictStateNeverComposesAnAllClear(t *testing.T) {
	t.Parallel()
	store, _, _ := fakeServingFixture(t, "bed-m1", nil)
	writeIdleJSON(t, filepath.Join(store.Root, "artifacts", "agents", "turn-verdict-state.json"), map[string]any{
		"schemaVersion": 2,
		"sessions":      map[string]any{},
	})
	verdict, err := store.TurnVerdict(ScanResult{}, "t", "", "main-1", TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1"}})
	if err != nil || verdict.ShouldBlock || verdict.Class != "infrastructure" {
		t.Fatalf("an unreadable verdict state did not allow as infrastructure: %+v %v", verdict, err)
	}
	if strings.Contains(verdict.Display, "NOTHING LEFT") {
		t.Fatalf("the degraded verdict composed an all-clear it cannot vouch for: %s", verdict.Display)
	}
}
