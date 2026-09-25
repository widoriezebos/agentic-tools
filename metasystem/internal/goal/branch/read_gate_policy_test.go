package branch

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// ResolveReadGate runs the fast gate once per goal unit in the detached unit
// tree, replays the recorded run afterwards, and refuses a record whose tree
// no longer matches the requested unit.
func TestResolveReadGateRecordsOneRunAndRefusesAChangedTree(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/internal/a/a_test.go", false)
	gateRuns, ids := 0, 0
	request := ReadGateRequest{Repo: f.root, GoalID: "goal-a", UnitCommit: f.unit, Repository: f,
		Gate: func(dir string) (string, error) {
			gateRuns++
			if dir != f.detached {
				t.Fatalf("gate ran in %q, want detached unit tree %q", dir, f.detached)
			}
			return "go-gate-fast: ok", nil
		},
		NewID: func(prefix string) (string, error) {
			ids++
			return prefix + "-first", nil
		},
	}

	f.expect("BranchSubject", f.root, f.unit)
	f.expect("CommonDir", f.root)
	f.expect("Detached", f.root, f.unit)
	first, err := ResolveReadGate(request)
	if err != nil {
		t.Fatal(err)
	}
	want := GateObservation{Kind: "go-gate-fast", Tree: f.tree, RunID: "goal-read-gate-first"}
	if first != want || gateRuns != 1 || ids != 1 {
		t.Fatalf("first gate resolution = %+v runs=%d ids=%d, want %+v once", first, gateRuns, ids, want)
	}
	record, err := loadBranchReadRecord(filepath.Join(f.root, ".git", "metasystem", "goal-reads", "goal-a", f.unit+".json"))
	if err != nil || record.Goal != "goal-a" || record.UnitCommit != f.unit || record.Tree != f.tree || record.GateRunID != want.RunID {
		t.Fatalf("gate record = %+v err=%v", record, err)
	}

	f.expect("BranchSubject", f.root, f.unit)
	f.expect("CommonDir", f.root)
	replay, err := ResolveReadGate(request)
	if err != nil || replay != want || gateRuns != 1 || ids != 1 {
		t.Fatalf("replayed gate resolution = %+v err=%v runs=%d ids=%d, want recorded %+v without a new run", replay, err, gateRuns, ids, want)
	}

	held, err := lockBranchRead(filepath.Join(f.root, ".git", "metasystem", "goal-reads", "goal-a", f.unit+".json"))
	if err != nil {
		t.Fatal(err)
	}
	f.expect("BranchSubject", f.root, f.unit)
	f.expect("CommonDir", f.root)
	_, err = ResolveReadGate(request)
	// Unlock explicitly: a Git child forked by a parallel test holds a
	// duplicate of this descriptor until it execs, so Close alone can leave
	// the lock held for the next resolution.
	if unlockErr := unix.Flock(int(held.Fd()), unix.LOCK_UN); unlockErr != nil {
		t.Fatal(unlockErr)
	}
	if closeErr := held.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err == nil || !strings.Contains(err.Error(), ReadDispatchPendingCode) || gateRuns != 1 {
		t.Fatalf("concurrent read of the unit was not refused as pending: err=%v runs=%d", err, gateRuns)
	}

	f.tree = policyID("9")
	f.expect("BranchSubject", f.root, f.unit)
	f.expect("CommonDir", f.root)
	if _, err := ResolveReadGate(request); err == nil || !strings.Contains(err.Error(), "does not match the requested unit") || gateRuns != 1 {
		t.Fatalf("changed unit tree was not refused: err=%v runs=%d", err, gateRuns)
	}
}
