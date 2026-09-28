package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// TestIncidentListCountsOnlyTrunkRedsAsOnMain: a pending flake seen only on a
// batch tip, and a hang, are tracked defects listed apart under a plain label;
// the "on main" count and the claim hint cover trunk reds alone.
func TestIncidentListCountsOnlyTrunkRedsAsOnMain(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	stamp := "2026-09-17T09:00:00Z"
	sighting := goal.TrunkRedSighting{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: stamp,
		Opid: goal.Opid("01J5X0000000000000000000X1", "mac-cli", "m1")}
	register := goal.RenderTrunkRed([]goal.TrunkRedEntry{
		{ID: "tr-fast-main000001", Identity: "tr-fast-main000001", Group: "fast", Status: "failed", Failures: []goal.TrunkRedFailure{},
			Sightings: []goal.TrunkRedSighting{sighting}, Owner: goal.TrunkRedOwner{Machine: "mac-other", Since: stamp, How: "joiner"},
			Holds: []string{"batch-1"}, Opened: stamp},
		{ID: "tr-fast-tip0000002", Identity: "tr-fast-tip0000002", Class: goal.TrunkRedClassPendingFlake, Group: "fast", Status: "failed",
			Failures: []goal.TrunkRedFailure{}, Sightings: []goal.TrunkRedSighting{sighting}, Opened: stamp},
	})
	parent := b.repo.accepted
	tip, err := b.repo.Build(goal.Opid("01J5X0000000000000000000X2", "mac-cli", "m1"), parent,
		[]goal.Change{{Path: "plans/goals/trunk-red.json", Content: register}}, "incident classes fixture")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := b.repo.Publish(parent, tip); err != nil || outcome != goal.CASLanded {
		t.Fatalf("publish incident fixture: %v %v", outcome, err)
	}
	if err := b.repo.AcceptedCAS(parent, tip); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := b.run(b.owners(), "incident", "list")
	if code != 0 || !strings.Contains(stdout, "1 incident(s) on main; 1 tracked flake or hang entry") ||
		!strings.Contains(stdout, "tr-fast-tip0000002  fast  pending flake, seen on a batch tip, not on main, open") {
		t.Fatalf("incident list = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.Contains(stdout, "incident claim tr-fast-tip0000002") {
		t.Fatalf("a tip-only pending flake was offered as an incident to claim: %q", stdout)
	}
}
