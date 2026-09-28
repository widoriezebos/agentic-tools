package dispatch

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The observation form of the custody proof (Part B 3.3, DL2-13): with
// ObserveMarkerExpiry the proof records that it would expire the pre-fork
// marker, answers DEFERRED, and leaves the marker in place, so a preview
// changes nothing; the apply form then expires it as before.
func TestCustodyDeathObservationLeavesTheMarker(t *testing.T) {
	root := t.TempDir()
	tag := "metasystem-job-observed-marker-nonce"
	supervisor := stage4Exact(57, 360_000_001)
	record := stage4RecordWithPrimary(map[string]any{
		"jobId": "observed-marker", "status": "pending", "instanceTag": tag,
		"custodyProcesses": []any{},
	}, supervisor, 57)
	marker := writeStage4Marker(t, root, tag, supervisor, 57, 0)
	table := &stage4ProcessTable{
		startStates: map[int64]identity.Liveness{57: identity.Dead},
		groups:      map[int64]int64{},
		pids:        []int64{},
	}
	dependencies := stage4DeathDependencies(table)
	var wouldExpire []string
	dependencies.ExpireMarker = ObserveMarkerExpiry(&wouldExpire)
	for pass := 1; pass <= 2; pass++ {
		got := ProveCustodyDeath(root, record, dependencies)
		if got.Outcome != CustodyDeathDeferred || got.Reason != "prefork-marker-would-expire" {
			t.Fatalf("observation pass %d = %+v", pass, got)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("an observation removed the marker: %v", err)
		}
	}
	if len(wouldExpire) != 2 || wouldExpire[0] != marker {
		t.Fatalf("observations recorded %v, want the marker twice", wouldExpire)
	}
	dependencies.ExpireMarker = nil
	if got := ProveCustodyDeath(root, record, dependencies); got.Reason != "prefork-marker-expired" {
		t.Fatalf("apply = %+v", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the apply left the marker: %v", err)
	}
}
