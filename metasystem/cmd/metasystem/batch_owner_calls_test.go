package main

import (
	"strconv"
	"testing"
)

// stubBatchOwnerCalls replaces the landing path's in-process owner calls for
// one test with run, which sees each call as the argv its former child
// carried, so a test can keep asserting on the words of the call.
func stubBatchOwnerCalls(t *testing.T, run func(invocation ownerInvocation, argv ...string) error) {
	t.Helper()
	original := batchOwnerCalls
	t.Cleanup(func() { batchOwnerCalls = original })
	batchOwnerCalls = batchOwnerCallSet{
		handover: func(invocation ownerInvocation, request goalHandoverRequest) error {
			argv := []string{"goal", "handover", "--root", request.Root, "--id", request.GoalID, "--lineage", invocation.lineage,
				"--target-machine", request.TargetMachine, "--target-lineage", request.TargetLineage,
				"--target-claim-epoch", strconv.FormatInt(request.TargetEpoch, 10), "--batch", request.Batch}
			if request.TargetRoot != "" {
				argv = append(argv, "--target-root", request.TargetRoot)
			}
			return run(invocation, argv...)
		},
		editNext: func(invocation ownerInvocation, root, goalID, next string) error {
			return run(invocation, "internal", "goal", "edit", "--root", root, "--id", goalID, "--next", next, "--lineage", invocation.lineage)
		},
		release: func(invocation ownerInvocation, root, goalID string) error {
			return run(invocation, "internal", "goal", "release", "--root", root, "--id", goalID, "--lineage", invocation.lineage)
		},
		held: func(root, base, commit, remote, ref string) error {
			return run(ownerInvocation{}, "landing", "held", "--root", root, "--base", base, "--commit", commit, "--remote", remote, "--ref", ref)
		},
	}
}
