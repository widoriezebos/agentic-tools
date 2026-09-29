package branch_test

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// A unit whose earlier read came from a reader record can be read again by a
// critic: the collection commits the critic's attestation instead of taking
// the earlier read as a conflicting installed read, and the branch then
// attests the unit through the critic root.
func TestBranchReadCollectsACriticReReadOfAReaderRecordUnit(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	readUnit(t, f, "u1", unit)
	tip := git(t, f.root, "rev-parse", "HEAD")
	request := branch.BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: tip,
		GoalID: "goal-a", UnitCommit: unit, CheckClaim: claimAllowed,
		Gate:  func(string) (string, error) { return "go gate: fast mode passed", nil },
		NewID: func(prefix string) (string, error) { return prefix + "-reread", nil },
		Delegate: func(brief, goal, commit, runtime, model string) (string, error) {
			writeReadJob(t, f.root, "critic-reread", unit, "running", false)
			return "critic-reread", nil
		}}
	dispatched, err := branch.RunBranchRead(request)
	if err != nil || dispatched.State != "dispatched" || dispatched.RootJob != "critic-reread" {
		t.Fatalf("dispatch=%+v err=%v", dispatched, err)
	}
	writeReadJob(t, f.root, "critic-reread", unit, "completed", false)
	request.Collect = true
	collected, err := branch.RunBranchRead(request)
	if err != nil || collected.State != "collected" || collected.AttestationCommit == "" {
		t.Fatalf("collect=%+v err=%v", collected, err)
	}
	att, err := branch.ValidateAttestation(f.root, f.base, "goal-a", "u1", unit)
	if err != nil || att.Source.Kind != "critic-root" || att.Source.RootJob != "critic-reread" {
		t.Fatalf("attestation after the critic re-read=%+v err=%v", att.Source, err)
	}
	// The landing router and the batch read the branch at its new tip: the
	// unit is read clean, and its attestation there names the critic root.
	newTip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	status, err := branch.InspectStatus(f.root, f.base, newTip, "goal-a")
	if err != nil || status.Prefix != 1 || len(status.Units) != 1 {
		t.Fatalf("status after the critic re-read=%+v err=%v", status, err)
	}
	at, err := branch.ValidateAttestationAt(f.root, newTip, f.base, "goal-a", "u1", unit)
	if err != nil || at.Source.Kind != "critic-root" {
		t.Fatalf("attestation at the new tip=%+v err=%v", at.Source, err)
	}
	request.Collect = false
	again, err := branch.RunBranchRead(request)
	if err != nil || again.State != "already-collected" || again.AttestationCommit != collected.AttestationCommit {
		t.Fatalf("repeat=%+v err=%v", again, err)
	}
}
