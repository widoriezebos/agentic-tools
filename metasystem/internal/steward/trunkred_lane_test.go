package steward

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// TestTrunkRedReadsTheHostLaneForAnUnsetSeat (U12): a seat with no
// landing.batch-root of its own lands through the host's lane, so the
// trunk-red check reads that lane's held batches, resolved by the one lane
// resolver, and never falls back to the seat's own (absent) setting.
func TestTrunkRedReadsTheHostLaneForAnUnsetSeat(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	bed := newRoleTrunkRedBed(t, now, nil, healthCadence(now, "passed"))
	landing := t.TempDir()
	writeHealthBatch(t, landing, now.Add(-2*time.Minute), "held-opid")
	projection, err := bed.project()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	laneRoot := func(repoRoot string, at time.Time) (string, bool, error) {
		calls++
		if repoRoot != bed.root || !at.Equal(now) {
			t.Fatalf("lane resolver arguments: %q %s", repoRoot, at)
		}
		return landing, true, nil
	}
	seatOnly := func(string, string, func() time.Time) (config.BatchLanding, error) {
		t.Fatal("the seat's own setting was read instead of the host lane")
		return config.BatchLanding{}, nil
	}
	role := checkTrunkRedFromProjection(bed.root, now, projection, nil, seatOnly, laneRoot)
	if calls == 0 || role.Status != HealthDead || !strings.Contains(role.Reason, "batch batch-held opid") {
		t.Fatalf("unset seat on the host lane: calls=%d role=%+v", calls, role)
	}
}
