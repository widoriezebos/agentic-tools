package main

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
)

var cadenceTick = runProductionCadenceTick

// runProductionCadenceTick runs one cadence tick under the held landing
// owner lease, with the command's trunk fetch, weight threshold, testing
// preparation and worker policy.
func runProductionCadenceTick(root string, held batchOwnerLease, clock func() time.Time) (cadence.TickOutput, error) {
	return cadence.RunTick(root, cadence.Owner{Epoch: held.epoch, Lineage: landingOwnerLineage,
		Require: func() error { return batchOwnerRequire(held) }, FetchOrigin: fetchBatchOrigin, WeightThreshold: weightThreshold,
		Prepare: prepareTestingForCommand, WorkerPolicy: testingWorkerPolicy}, clock)
}
