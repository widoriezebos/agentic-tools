package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// A goal's status reads its batch line through the delivery owners the
// invocation carries; an owner a caller left unset selects the production
// one, as every other delivery owner does, so a status read with partial
// owners (every bed that sets only the owners it drives) neither panics nor
// invents a batch: an installation with no landing.batch-root has no line.
func TestGoalBatchLineWithUnsetOwnersReadsTheProductionOnes(t *testing.T) {
	t.Parallel()
	inv := &intentInvocation{owners: intentOwners{delivery: &intentDeliveryOwners{}}, layout: stateroot.Layout{InstallationRoot: t.TempDir()}}
	line, batchID := inv.goalBatchLine("standing-validation")
	if line != "" || batchID != "" {
		t.Fatalf("an installation with no batch root has a batch line: %q %q", line, batchID)
	}
}
