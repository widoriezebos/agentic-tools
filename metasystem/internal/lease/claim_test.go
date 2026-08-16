package lease

import (
	"os/exec"
	"testing"
	"time"
)

// liveChild spawns a real, long-lived process and returns its pid and true
// start second, so Live(pid, start, nil) is genuinely true until the test ends.
func liveChild(t *testing.T) (pid, start int64) {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn live child: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	pid = int64(cmd.Process.Pid)
	// The child's identity is rightly unreadable inside its fork-to-execve
	// window (the flake dossier's family — sixth instance, first seen on
	// the VM's snapshot gate); the property is steady-state, wait bounded.
	deadline := time.Now().Add(2 * time.Second)
	for {
		s, ok := StartedAt(pid, nil)
		if ok {
			return pid, s
		}
		if time.Now().After(deadline) {
			t.Fatalf("could not read live child %d start", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
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

// TestKI33SameProcessReannounce is the fix: a live process that re-announces
// under a fresh mainId (a --shutdown then re-arm) must reclaim its own
// checkout rather than be stranded OWNED-ELSEWHERE against its former identity.
func TestKI33SameProcessReannounce(t *testing.T) {
	root := t.TempDir()
	pid, start := liveChild(t)
	mustClaim(t, root, ann("main-1-1-aaaaaa", pid, start, ""))

	// Same pid+start, new mainId — the shape KI-33 used to strand.
	mustClaim(t, root, ann("main-1-1-cccccc", pid, start, ""))

	lease, _ := loadLease(root, true)
	if lease.HolderMainId != "main-1-1-cccccc" {
		t.Fatalf("KI-33: same-process re-announce must reconcile the holder, got %q", lease.HolderMainId)
	}
	if lease.ClaimEpoch != 1 {
		t.Fatalf("KI-33 reconcile must preserve the epoch, got %d", lease.ClaimEpoch)
	}
	if lease.Revision != 2 {
		t.Fatalf("KI-33 reconcile must bump the revision, got %d", lease.Revision)
	}
	if len(lease.Takeovers) != 0 {
		t.Fatal("KI-33 reconcile is not a takeover; no takeover should be recorded")
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
