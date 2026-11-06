package batchowner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	landingstate "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

var laneStatusNow = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

// The board reports a build beside the lane's running check and drops the
// row when the build completes. It sets the package's lane home and writes
// to the store TestMain's observation counts, so it runs alone.
func TestTheBoardShowsTheLiveRowBesideARunningCheck(t *testing.T) {
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
	pid := int64(os.Getpid())
	exact, live, err := identity.KernelProber{}.Probe(pid)
	if err != nil || live != identity.Alive {
		t.Fatalf("the test process cannot be probed: live=%v error=%v", live, err)
	}
	process, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	const attempt = "board-check"
	data, err = json.Marshal(plain.Running{Attempt: attempt, Pid: pid, Process: process, Since: laneStatusNow.Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plain.Dir(record.Install), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain.Dir(record.Install), "running.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	store := boardLaunchStore
	if err := store.Create(launch.Record{ID: "host-build", Kind: "build", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	home0 := LandingLaneHome
	t.Cleanup(func() { LandingLaneHome = home0 })
	LandingLaneHome = func() (string, error) { return home, nil }

	assertRows := func(now time.Time, want int) {
		t.Helper()
		source := HostBoardSource(t.TempDir())
		if source == nil {
			t.Fatal("the board has no source")
		}
		status := source.Lane(now)
		if len(status.Problems) > 0 {
			t.Fatalf("the board's lane cannot be read: %v", status.Problems)
		}
		if status.Health == nil {
			t.Fatal("the board's lane has no health")
		}
		rows := 0
		for _, row := range status.Health.Drift {
			if row.Row == "proof-beside-builds" {
				rows++
				if act := fmt.Sprintf(landingstate.Phrase("d-builds-and-d-unit-checks-run-beside"), 1, 0, attempt); row.Act != act {
					t.Fatalf("the board's live row says %q, want %q", row.Act, act)
				}
			}
		}
		if rows != want {
			t.Fatalf("the board has %d live rows beside the running check, want %d", rows, want)
		}
	}
	assertRows(laneStatusNow, 1)
	if _, err := store.Update("host-build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	assertRows(laneStatusNow.Add(time.Minute), 0)
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
