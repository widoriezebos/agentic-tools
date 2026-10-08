package branch

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestInheritedDestinationRequiresCommittedCritic(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	f.expect("BranchRange", f.root, f.base, f.unit, "goal-a")
	f.expect("BranchSubject", f.root, f.unit)
	request := BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: f.unit, GoalID: "goal-a", UnitCommit: f.unit, Repository: branchPolicyRepository{f}, UnitRead: []byte("{}"), InheritedFindings: []goal.ReviewObligation{{SourceUnit: "source", TargetUnit: "u1", OriginalFinding: "source-read:1", SourceCommit: f.base}}, CheckClaim: func() error { return nil }}
	if _, err := RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "needs a committed read of its inherited change") || !strings.Contains(err.Error(), "metasystem work review goal-a --work u1") {
		t.Fatalf("destination unit read bypassed inherited subject: %v", err)
	}
}
