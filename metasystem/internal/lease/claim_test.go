package lease

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// readyChild starts a stand-in that reports readiness after exec and then
// remains alive until the test releases it.
func readyChild(t *testing.T) (pid, start int64) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", "printf r; read line")
	hold, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	ready, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn live child: %v", err)
	}
	t.Cleanup(func() {
		_ = hold.Close()
		_ = ready.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	var signal [1]byte
	if _, err := io.ReadFull(ready, signal[:]); err != nil {
		t.Fatalf("live child never reported ready: %v", err)
	}
	pid = int64(cmd.Process.Pid)
	start, ok := StartedAt(pid, nil)
	if !ok {
		t.Fatalf("ready child %d has no readable start", pid)
	}
	return pid, start
}

// liveChild returns a process whose identity is stable and remains live until
// the test ends.
func liveChild(t *testing.T) (pid, start int64) {
	t.Helper()
	return readyChild(t)
}

// deadPid returns a pid that is certainly not alive.
func deadPid(t *testing.T) int64 {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	pid := int64(cmd.Process.Pid)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return pid
}

func ann(mainID string, pid, start int64, lineage string) *Announcement {
	return &Announcement{
		MainId: mainID, Pid: pid, PidStartedAt: start,
		CommandHash: CommandHash("cmd-" + mainID), OwnerLineage: lineage,
	}
}

func mustClaim(t *testing.T, root string, a *Announcement) {
	t.Helper()
	claimer, err := newClaimer(root)
	if err != nil {
		t.Fatalf("claimer(%s): %v", a.MainId, err)
	}
	if err := claimer.claim(a); err != nil {
		t.Fatalf("claim(%s): %v", a.MainId, err)
	}
}

func TestFreshClaim(t *testing.T) {
	root := t.TempDir()
	pid, start := liveChild(t)
	mustClaim(t, root, ann("main-1-1-aaaaaa", pid, start, ""))

	lease, err := loadLease(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if lease.HolderMainId != "main-1-1-aaaaaa" || lease.ClaimEpoch != 1 || lease.Revision != 1 {
		t.Fatalf("fresh claim wrong: %+v", lease)
	}
	if lease.OwnerLineage != "main-1-1-aaaaaa" {
		t.Fatalf("absent lineage should default to the holder mainId: %q", lease.OwnerLineage)
	}
	// The epoch's sweep stamp must be present so a require-holder gate passes.
	stampClaimer, _ := newClaimer(root)
	if !stampClaimer.stampComplete(lease) {
		t.Fatal("fresh claim did not stamp the epoch complete")
	}
}

func TestSameMainRenewal(t *testing.T) {
	root := t.TempDir()
	pid, start := liveChild(t)
	mustClaim(t, root, ann("main-1-1-aaaaaa", pid, start, ""))
	mustClaim(t, root, ann("main-1-1-aaaaaa", pid, start, ""))

	lease, _ := loadLease(root, true)
	if lease.ClaimEpoch != 1 || lease.Revision != 2 {
		t.Fatalf("renewal should keep epoch and bump revision: %+v", lease)
	}
}

func TestLiveHolderKeepsLeaseAgainstDifferentProcess(t *testing.T) {
	root := t.TempDir()
	pid, start := liveChild(t)
	mustClaim(t, root, ann("main-1-1-aaaaaa", pid, start, ""))

	// A different main (different process) tries to claim: refused, silently.
	mustClaim(t, root, ann("main-2-2-bbbbbb", 999999, start+1, ""))

	lease, _ := loadLease(root, true)
	if lease.HolderMainId != "main-1-1-aaaaaa" || lease.Revision != 1 {
		t.Fatalf("a live holder must keep its lease untouched: %+v", lease)
	}
}

// TestSameProcessReannounceReconciles: a live process that re-announces
// under a fresh mainId (a --shutdown then re-arm) must reclaim its own
// checkout rather than be stranded OWNED-ELSEWHERE against its former identity.
func TestSameProcessReannounceReconciles(t *testing.T) {
	root := t.TempDir()
	pid, start := liveChild(t)
	mustClaim(t, root, ann("main-1-1-aaaaaa", pid, start, ""))

	// Same pid+start, new mainId — the shape that would otherwise strand.
	mustClaim(t, root, ann("main-1-1-cccccc", pid, start, ""))

	lease, _ := loadLease(root, true)
	if lease.HolderMainId != "main-1-1-cccccc" {
		t.Fatalf("same-process re-announce must reconcile the holder, got %q", lease.HolderMainId)
	}
	if lease.ClaimEpoch != 1 {
		t.Fatalf("the re-announce reconcile must preserve the epoch, got %d", lease.ClaimEpoch)
	}
	if lease.Revision != 2 {
		t.Fatalf("the re-announce reconcile must bump the revision, got %d", lease.Revision)
	}
	if len(lease.Takeovers) != 0 {
		t.Fatal("the re-announce reconcile is not a takeover; no takeover should be recorded")
	}
}

func TestSameLineageSuccession(t *testing.T) {
	root := t.TempDir()
	dead := deadPid(t)
	// A dead holder of lineage "mission-x".
	seed := &Lease{
		HolderMainId: "main-1-1-aaaaaa", OwnerLineage: "mission-x",
		Pid: dead, PidStartedAt: 111, CommandHash: "x",
		ClaimedAt: "t", RenewedAt: "t", Takeovers: []Takeover{}, Revision: 3, ClaimEpoch: 5,
	}
	if err := saveLease(root, seed); err != nil {
		t.Fatal(err)
	}
	newPid, newStart := liveChild(t)
	mustClaim(t, root, ann("main-9-9-dddddd", newPid, newStart, "mission-x"))

	lease, _ := loadLease(root, true)
	if lease.HolderMainId != "main-9-9-dddddd" || lease.Pid != newPid {
		t.Fatalf("succession should move the holder identity: %+v", lease)
	}
	if lease.ClaimEpoch != 5 {
		t.Fatalf("succession must preserve the epoch (jobs stay valid), got %d", lease.ClaimEpoch)
	}
	if len(lease.Takeovers) != 0 {
		t.Fatal("succession is not a seizure; no takeover entry")
	}
}

func TestForeignTakeover(t *testing.T) {
	root := t.TempDir()
	dead := deadPid(t)
	seed := &Lease{
		HolderMainId: "main-1-1-aaaaaa", OwnerLineage: "main-1-1-aaaaaa",
		Pid: dead, PidStartedAt: 111, CommandHash: "x",
		ClaimedAt: "t", RenewedAt: "t", Takeovers: []Takeover{}, Revision: 3, ClaimEpoch: 5,
	}
	if err := saveLease(root, seed); err != nil {
		t.Fatal(err)
	}
	newPid, newStart := liveChild(t)
	mustClaim(t, root, ann("main-9-9-eeeeee", newPid, newStart, "")) // lineage defaults to its own mainId

	lease, _ := loadLease(root, true)
	if lease.HolderMainId != "main-9-9-eeeeee" {
		t.Fatalf("takeover should install the new holder: %+v", lease)
	}
	if lease.ClaimEpoch != 6 {
		t.Fatalf("takeover must bump the epoch, got %d", lease.ClaimEpoch)
	}
	if len(lease.Takeovers) != 1 || lease.Takeovers[0].FromMainId != "main-1-1-aaaaaa" || lease.Takeovers[0].Reason != "holder-death" {
		t.Fatalf("takeover must be recorded with its predecessor: %+v", lease.Takeovers)
	}
}

func TestATakeoverLeavesAUnitLaunchAlone(t *testing.T) {
	dead := deadPid(t)
	pid, start := readyChild(t)
	ref := hookExact(t, pid).Ref()
	for _, state := range []launch.State{launch.Running, launch.Completed, "absent", "unreadable"} {
		t.Run(string(state), func(t *testing.T) {
			root := t.TempDir()
			if err := saveLease(root, &Lease{HolderMainId: "main-old", OwnerLineage: "old", Pid: dead, PidStartedAt: 111, ClaimEpoch: 3, Revision: 3}); err != nil {
				t.Fatal(err)
			}
			job := "unit-reader"
			path := filepath.Join(root, "artifacts/agents/jobs", job+".json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			before := []byte(`{"jobId":"unit-reader","operationId":"unit-launch:unit-reader","unitRun":"unit","goalId":"bounded","goalRevision":3,"unitRound":1,"capMin":5,"status":"pending","claimEpoch":3}`)
			if err := os.WriteFile(path, before, 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := newClaimer(root)
			if err != nil {
				t.Fatal(err)
			}
			c.launches = launch.Store{Root: t.TempDir()}
			execution := launch.Record{ID: job, State: state, Child: &ref, StartedAt: "2026-10-24T22:14:00Z", FinishedAt: "2026-10-24T22:15:00Z"}
			if state != "absent" {
				if err := c.launches.Create(execution); err != nil {
					t.Fatal(err)
				}
			}
			if state == "unreadable" {
				dir, _ := c.launches.StateDir(job)
				if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err = c.claim(ann("main-new", pid, start, "new"))
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if state == "unreadable" {
				if err == nil || !bytes.Equal(before, after) {
					t.Fatalf("unreadable launch must refuse without changing the reservation: %v, %s", err, after)
				}
				return
			}
			lease, _ := loadLease(root, true)
			if err != nil || lease.ClaimEpoch != 4 {
				t.Fatalf("takeover = %+v, %v", lease, err)
			}
			var record map[string]any
			if err := json.Unmarshal(after, &record); err != nil {
				t.Fatal(err)
			}
			switch state {
			case launch.Running:
				if !bytes.Equal(before, after) {
					t.Fatalf("the takeover changed a running launch's reservation: %s", after)
				}
			case launch.Completed:
				if record["status"] != "completed" || record["startedAt"] != execution.StartedAt || record["endedAt"] != execution.FinishedAt || record["unitExecuted"] != true {
					t.Fatalf("the takeover lost the launch's outcome: %s", after)
				}
				if err := dispatch.ReconcileUnitLaunch(root, job, "unit", "bounded", 3, "completed", "", execution.StartedAt, execution.FinishedAt, true); err != nil {
					t.Fatal(err)
				}
			case "absent":
				if record["status"] != "failed" || record["phase"] != "claim-sweep" {
					t.Fatalf("an unstarted reservation did not keep the sweep: %s", after)
				}
			}
		})
	}
}

func TestSupervisionTagNeverClaims(t *testing.T) {
	root := t.TempDir()
	pid, start := liveChild(t)
	a := ann("main-1-1-aaaaaa", pid, start, "")
	a.InstanceTag = "metasystem-supervision-owner"
	mustClaim(t, root, a)

	if lease, _ := loadLease(root, false); lease != nil {
		t.Fatalf("a supervision component must never claim the lease, got %+v", lease)
	}
}
