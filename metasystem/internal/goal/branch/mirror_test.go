package branch_test

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func TestMirrorDeleteReconcilesUnknownOutcome(t *testing.T) {
	t.Parallel()
	t.Run("sweep delete completed", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		commitUnit(t, f, "u1", "metasystem/code.go", "one")
		if _, err := branch.Push(pushRequest(f, "unknown-sweep-origin")); err != nil {
			t.Fatal(err)
		}
		result, err := branch.Sweep(branch.SweepRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", Abandoned: true, CheckClaim: claimAllowed, PushTransport: unknownAfterLanding{}})
		if err != nil || !result.Deleted || git(t, f.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a") != "" {
			t.Fatalf("completed unknown sweep delete = %+v, %v", result, err)
		}
	})
}
