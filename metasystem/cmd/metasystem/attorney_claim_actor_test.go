package main

import (
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// Under a general power of attorney the granted seat's main session still
// claims as itself: a claim needs the agent session that will work the goal,
// so the attorney arm answers for acts that need a person, never for claim.
func TestAttorneyLeavesTheSessionsClaimItsOwn(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	b.lineage = "lin-main"
	b.admitted = &humanauthority.HelmGrant{By: "Wido", Class: lease.ClassMain, Grant: "01M-grant"}
	inv := &intentInvocation{command: mustIntentCommand(t, "goal claim"), owners: b.owners(), stateRoot: b.root(), input: intentInput{values: map[string][]string{}}}
	args, proof, problem := inv.actingAs("claim", "g1", actorEither)
	if problem != nil || proof != nil || slices.Contains(args, "--by") {
		t.Fatalf("under a grant the session's claim became a person's: %v %v %+v", args, proof, problem)
	}
	// A dual act that a person can complete is still the person's.
	args, _, problem = inv.actingAs("release", "g1", actorEither)
	if problem != nil || !slices.Contains(args, "--by") {
		t.Fatalf("under a grant a person's dual act lost the grant: %v %+v", args, problem)
	}
}
