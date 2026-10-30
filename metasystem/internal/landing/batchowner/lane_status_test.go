package batchowner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

var laneStatusNow = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

// The board reads the lane's hold through the host's observation: it shows
// the hold while a build runs and drops it once the build's launch record
// completes, well inside the hold's deadline. It sets the package's lane home
// and writes to the store TestMain's observation counts, so it runs alone.
func TestTheBoardReadsTheLanesCurrentHold(t *testing.T) {
	home, checkout := t.TempDir(), laneStatusCheckout(t)
	record := lane.Record{Root: checkout, Install: filepath.Join(checkout, "metasystem"), CustodyEpoch: 1, RegisteredBy: "Wido"}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := plain.RecordIntervention(record.Install, plain.Intervention{At: laneStatusNow.Add(-10 * time.Minute).Format(time.RFC3339Nano),
		Row: "proof-beside-builds", Subject: "main", Outcome: "held: 1 builds and 0 unit checks run"}); err != nil {
		t.Fatal(err)
	}
	store := boardLaunchStore
	if err := store.Create(launch.Record{ID: "host-build", Kind: "build", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	home0 := LandingLaneHome
	t.Cleanup(func() { LandingLaneHome = home0 })
	LandingLaneHome = func() (string, error) { return home, nil }

	held := func(now time.Time) bool {
		t.Helper()
		source := HostBoardSource(t.TempDir())
		if source == nil {
			t.Fatal("the board has no source")
		}
		status := source.Lane(now)
		if status.Health == nil {
			t.Fatalf("the board's lane has no health: problems=%v", status.Problems)
		}
		if len(status.Problems) > 0 {
			t.Fatalf("the board's lane cannot be read: %v", status.Problems)
		}
		for _, row := range status.Health.Drift {
			if row.Row == "proof-beside-builds" {
				return true
			}
		}
		return false
	}
	if !held(laneStatusNow) {
		t.Fatal("the board omits the hold while the build runs")
	}
	if _, err := store.Update("host-build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	if held(laneStatusNow.Add(time.Minute)) {
		t.Fatal("the board shows the hold after the build completed, inside the deadline")
	}
}

// laneStatusCheckout is a landing checkout the lane's own Git reads: a
// configured installation committed on main, with origin's main at it.
func laneStatusCheckout(t *testing.T) string {
	t.Helper()
	checkout := t.TempDir()
	if err := os.MkdirAll(filepath.Join(checkout, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "metasystem", "metasystem.conf"), []byte("metasystem.template=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "lane"}, {"update-ref", "refs/remotes/origin/main", "HEAD"}} {
		if out, err := exec.Command("git", append([]string{"-C", checkout}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return checkout
}
