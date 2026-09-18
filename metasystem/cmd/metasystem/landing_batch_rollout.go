package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func init() {
	for _, capability := range requiredBatchCapabilities {
		compiledBatchCapabilities[capability] = struct{}{}
	}
}

// productionBatchLedgerOwner uses the landing identity that mints trunk-red operation identifiers.
func productionBatchLedgerOwner(root string) (batch.LedgerOwner, error) {
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return nil, err
	}
	return newLedgerTrunkRedOwner(root, machine, landingOwnerLineage)
}
