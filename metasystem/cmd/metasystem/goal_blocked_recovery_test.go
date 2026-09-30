package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	reads := 0
	commandNow := func(root string) (time.Time, error) {
		reads++
		return bed.commandNow(root)
	}
	req, err := syncReqWithProofAtWithDependencies("handover", bed.root, "", "fixture-lineage", nil, commandNow, bed.dependencies(&stdout, &stderr))
	if err != nil {
		t.Fatal(err)
	}
	// Binding reads no clock of its own: the request's clock reads are the
	// verb's (TestSyncReqLineage and its peers count them exactly), and only a
	// publish that meets a dead owner's pushed entry reads the command's clock,
	// when it recovers it.
	bindReads := reads
	bound, ok := req.Endpoint.BlockedRecovery().(blockedRecoveryAtCommandClock)
	if !ok {
		t.Fatalf("the handover request carries no blocked-recovery policy: %#v", req.Endpoint.BlockedRecovery())
	}
	policy, err := bound.policy()
	if err != nil {
		t.Fatal(err)
	}
	if reads != bindReads+1 || policy.root != bed.root || !policy.Now.Equal(bed.clock()) {
		t.Fatalf("the resolved policy is at root %q time %s after %d clock reads, want %q %s after %d", policy.root, policy.Now, reads, bed.root, bed.clock(), bindReads+1)
	}
	if _, ok := goal.SensitiveRecoveryPolicy(bound).(goal.ParkRecoveryPolicy); !ok {
		t.Fatal("the bound policy supplies no park branch check to a recovered park")
	}
}
