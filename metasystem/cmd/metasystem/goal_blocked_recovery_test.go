package main

import (
	"bytes"
	"testing"
)

// fencedflake (2026-09-30): the batch owner's handover publishes through the
// synced-ledger request, and a dead owner's pushed breach-stop is recovered
// only with the live budget policy. Every sync-verb request binds the same
// policy `goal sync --recover` carries, at the command's clock and root, so
// the publish that meets a dead owner's pushed entry can recover it.
func TestSyncVerbRequestsBindTheBlockedRecoveryPolicy(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	var stdout, stderr bytes.Buffer
	req, err := syncReqWithProofAtWithDependencies("handover", bed.root, "", "fixture-lineage", nil, bed.commandNow, bed.dependencies(&stdout, &stderr))
	if err != nil {
		t.Fatal(err)
	}
	policy, ok := req.Endpoint.BlockedRecovery().(goalRecoveryPolicy)
	if !ok {
		t.Fatalf("the handover request carries no blocked-recovery policy: %#v", req.Endpoint.BlockedRecovery())
	}
	if policy.root != bed.root || !policy.Now.Equal(bed.clock()) {
		t.Fatalf("the bound policy is at root %q time %s, want %q %s", policy.root, policy.Now, bed.root, bed.clock())
	}
}
