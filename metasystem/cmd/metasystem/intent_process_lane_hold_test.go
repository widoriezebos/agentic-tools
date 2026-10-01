package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// TestRestartHoldsWhileALaneProofRuns (moving main cancels a running lane
// proof, 2026-10-01): main moved, the lane checkout was brought to it and
// its engine restarted, and the restart's stop transition cancelled the
// running lane proof. While a lane proof runs, system restart of the lane
// checkout stops and starts nothing and says when it can run; once the
// proof ended the same restart goes through.
func TestRestartHoldsWhileALaneProofRuns(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	running := true
	var asked []string
	owners := b.owners()
	owners.processes.process.laneProof = func(scope processScope) (lane.ProofHold, bool, error) {
		asked = append(asked, scope.Checkout)
		return lane.ProofHold{Attempt: "proof-mupsn9pf-35c1e86efc9223c7", StartedAt: "2026-10-01T17:14:09Z"}, running, nil
	}

	code, result := b.runJSON(owners, "system", "restart")
	if code == 0 || result.Outcome != intentRefused || b.armCalls != 0 {
		t.Fatalf("restart under a running lane proof = %d %+v, arm calls %d", code, result, b.armCalls)
	}
	if record := b.fence(); record.State != stopfence.StateOpen || record.Generation != 0 {
		t.Fatalf("restart under a running lane proof stopped the checkout: %+v", record)
	}
	if !strings.Contains(result.Summary, "the landing lane's test run") || !strings.Contains(result.Summary, "proof-mupsn9pf-35c1e86efc9223c7") {
		t.Fatalf("the refusal does not name the running proof: %q", result.Summary)
	}
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "landing", "status"}) {
		t.Fatalf("the refusal's next = %+v", result.Next)
	}
	if len(asked) != 1 {
		t.Fatalf("the hold was read %d times for one restart: %v", len(asked), asked)
	}

	// The proof ended: the next restart stops and starts the checkout.
	running = false
	code, result = b.runJSON(owners, "system", "restart")
	if code != 0 || result.Outcome != intentConfirmed || b.armCalls != 1 || b.fence().Generation != 2 {
		t.Fatalf("restart after the proof ended = %d %+v, arm calls %d, fence %+v", code, result, b.armCalls, b.fence())
	}

	// system stop is the brake and is never held.
	running = true
	if code, result := b.runJSON(owners, "system", "stop"); code != 0 || result.Outcome != intentConfirmed || b.fence().State != stopfence.StateClosed {
		t.Fatalf("stop under a running lane proof = %d %+v", code, result)
	}
}

// The production owners read the hold: a nil hold would let every restart
// through.
func TestProductionProcessOwnersReadTheLaneProofHold(t *testing.T) {
	t.Parallel()
	if defaultProcessOwners().laneProof == nil {
		t.Fatal("the production process owners do not read whether a lane proof runs")
	}
}
