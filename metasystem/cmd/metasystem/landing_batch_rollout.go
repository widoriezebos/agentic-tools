package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

var batchRolloutRequirements = [...]batchCapability{
	assemblyConflictCeilingAndSeal,
	prefixReceipts,
	ejectionAndRedScheduling,
	atomicSeriesAndRecovery,
}

func init() {
	registerBatchRollout(compiledBatchCapabilities, requiredBatchCapabilities[:])
}

func registerBatchRollout(registry map[batchCapability]struct{}, available []batchCapability) bool {
	for _, requirement := range batchRolloutRequirements {
		found := false
		for _, capability := range available {
			found = found || capability == requirement
		}
		if !found {
			return false
		}
	}
	for _, capability := range available {
		registry[capability] = struct{}{}
	}
	return true
}

// productionBatchLedgerOwner uses the landing identity that mints trunk-red operation identifiers.
func productionBatchLedgerOwner(root string) (batch.LedgerOwner, error) {
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return nil, err
	}
	return newLedgerTrunkRedOwner(root, machine, landingOwnerLineage)
}
