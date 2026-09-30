package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// disk clean --leases (U6a-2, DL2-19, DL3B-02): a person's act at the
// enrolled terminal; a lease proven settled is reclaimed, one a live owner,
// custodian or worker holds is kept with the holder and the command, one
// whose census is incomplete is kept with its remedy; with admission.lock
// held by a proof nothing is reclaimed; a repeat with nothing left succeeds
// and writes nothing; an agent is refused.
func TestDiskLeasesByAPerson(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	var reports []proofrun.HostLeaseReport
	busy := false
	calls := 0
	bed.owners.disk.leases = func(string) ([]proofrun.HostLeaseReport, bool, error) { calls++; return reports, busy, nil }
	reports = []proofrun.HostLeaseReport{
		{Lease: "lease-heavy-a", State: proofrun.HostLeaseReclaimed, Reason: "pid 11 is not running", Owner: proofrun.ProcessIdentity{Pid: 11}},
		{Lease: "lease-heavy-b", State: proofrun.HostLeaseLive, Reason: "a live owner, custodian or worker holds the lease", Owner: proofrun.ProcessIdentity{Pid: 12}},
		{Lease: "lease-heavy-c", State: proofrun.HostLeaseUnknown, Reason: "the owner's fixture census failed (pid 13 unreadable)",
			Remedy: "stop pid 13, then metasystem disk clean --leases (or the next admission pass) reclaims it", Owner: proofrun.ProcessIdentity{Pid: 14}},
	}
	code, out := bed.run("disk", "clean", "--leases", "--verbose")
	if code != 0 || !strings.Contains(out, "leases: 1 done, 2 kept") || !strings.Contains(out, "end owner pid 12 and what it started, then metasystem disk clean --leases") ||
		!strings.Contains(out, "pid 13 unreadable") || !strings.Contains(out, "stop pid 13, then metasystem disk clean --leases") {
		t.Fatalf("--leases = %d:\n%s", code, out)
	}
	reports, busy = []proofrun.HostLeaseReport{{Lease: "lease-heavy-d", State: proofrun.HostLeaseDead, Reason: "pid 15 is not running"}}, true
	if code, out := bed.run("disk", "clean", "--leases", "--verbose"); code != 0 || !strings.Contains(out, "a proof holds admission.lock") || strings.Contains(out, "1 done") {
		t.Fatalf("a busy admission lock = %d:\n%s", code, out)
	}
	reports, busy = nil, false
	if code, out := bed.run("disk", "clean", "--leases"); code != 0 || !strings.Contains(out, "no test-run lease is left over") {
		t.Fatalf("a repeat with nothing left = %d:\n%s", code, out)
	}
	before := calls
	bed.person = errors.New("this shell is an agent's")
	if code, out := bed.run("disk", "clean", "--leases"); code != 3 || !strings.Contains(out, "this shell is an agent's, so nothing was done") || !strings.Contains(out, "run: metasystem disk clean --leases") || calls != before {
		t.Fatalf("an agent's --leases = %d (calls %d):\n%s", code, calls, out)
	}
	if code, out := bed.run("disk", "clean", "--leases", "--strays"); code != 2 || !strings.Contains(out, "one thing at a time") {
		t.Fatalf("two acts at once = %d:\n%s", code, out)
	}
}

// With no fixture proofs, --release judges a store by the proofs the
// checkout's pass has: the process, goal, session and delegate kinds, the
// goal's and session's by store class.
func TestDiskReleaseDefaultsToTheEnginesOwnerProofs(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	proofs := diskOwners{now: bed.owners.disk.now}.proofsFor(bed.inst)
	for _, kind := range []diskstore.OwnerKind{diskstore.OwnerProcess, diskstore.OwnerGoal, diskstore.OwnerSession} {
		if proofs[kind] == nil {
			t.Fatalf("no default proof for owner kind %s: %v", kind, proofs)
		}
	}
	classes, ok := proofs[diskstore.OwnerGoal].(diskstore.ClassProofs)
	if !ok || classes.ByClass[diskstore.GoalWorktreeClass] == nil || classes.ByClass[diskstore.WorkspaceClass] == nil {
		t.Fatalf("the goal's proofs by class = %+v", proofs[diskstore.OwnerGoal])
	}
}
