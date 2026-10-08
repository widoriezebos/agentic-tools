package dispatch

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The goal parser already rejects a capability that contradicts its claim, so
// no public verb reaches this refusal; it is the launch's own custody guard.
func TestReserveUnitLaunchRefusesWithoutProvenCustody(t *testing.T) {
	t.Parallel()
	claim := &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-28T09:00:00Z", Revision: 7}
	for name, capability := range map[string]*goal.StopCapability{
		"missing":        nil,
		"other machine":  {Generation: 1, Revision: 7, Machine: "bed-m2", ClaimEpoch: 1},
		"other revision": {Generation: 1, Revision: 8, Machine: "bed-m1", ClaimEpoch: 1},
		"unbound epoch":  {Generation: 1, Revision: 7, Machine: "bed-m1", ClaimEpoch: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			file := &goal.GoalFile{Id: "custody", Claimed: claim, StopCapability: capability}
			err := ReserveUnitLaunch(root, "launch-1", "run-1", file, 1, 100, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
			if err == nil || !strings.Contains(err.Error(), "no proven claim custody") || !strings.Contains(err.Error(), "resume or re-claim it before dispatch") {
				t.Fatalf("custody not refused: %v", err)
			}
		})
	}
}
