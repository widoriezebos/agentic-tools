package branch_test

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func TestApplyScopePreservesDropHistoryAndRestore(t *testing.T) {
	t.Parallel()
	optional := goal.UnitDrop{Unit: "optional", Operation: "drop-optional", Commit: "optional-inverse", Covered: []string{"optional-build"}}
	required := goal.UnitDrop{Unit: "required", Operation: "drop-required", Commit: "required-inverse", Covered: []string{"required-build"},
		Requirements: "requirements", Proof: "proof", Actor: "Wido", At: "2026-08-20T10:05:00Z"}
	file := &goal.GoalFile{UnitDrops: []goal.UnitDrop{optional, required}, ScopeExclusions: []goal.ScopeExclusion{{
		Unit: required.Unit, Operation: required.Operation, Result: required.Commit, Requirements: required.Requirements,
		Proof: required.Proof, Actor: required.Actor, Authority: "SIGNED_IN_SESSION", At: required.At,
		Reason: "Remove required scope", Impact: "Required behavior will be missing", Designs: []string{"design"},
	}}}
	status := branch.Status{Units: []branch.UnitStatus{
		{Unit: required.Unit, Commit: "required-build", ReadState: "dropped", PriorReadState: "needs read", Drop: &required},
		{Unit: optional.Unit, Commit: "optional-build", ReadState: "dropped", PriorReadState: "built", Drop: &optional},
		{Unit: "independent", Commit: "independent-build", ReadState: "built"},
	}}
	for range 2 {
		branch.ApplyScope(&status, file)
		if status.Prefix != 2 || status.Units[0].PriorReadState != "needs read" || status.Units[0].ScopeOperation != required.Operation || !status.Units[0].Resolved() || !status.Units[1].Resolved() || status.Units[2].Resolved() {
			t.Fatalf("scope projection lost prior reads or optional completion: %+v", status)
		}
	}
	file.ScopeExclusions[0].RestoredAt, file.ScopeExclusions[0].RestoredBy = "2026-08-20T11:00:00Z", "Wido"
	for range 2 {
		branch.ApplyScope(&status, file)
		if status.Prefix != 0 || status.Units[0].ReadState != "needs read" || status.Units[0].PriorReadState != "needs read" || status.Units[0].Resolved() || status.Units[0].Drop == nil || status.Units[0].ScopeOperation != "" || !status.Units[1].Resolved() || status.Units[2].Resolved() {
			t.Fatalf("restoration lost the inverse or kept required completion waived: %+v", status)
		}
	}
}
