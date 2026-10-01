package main

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
)

// The landing batch owner (internal/landing/batchowner) calls back into the
// engine for what needs the engine's own configuration: the retained
// verification and cost forecast of a testing selection, the cadence tick,
// a goal branch's claim check, the goal ledger's and the landing path's
// owner functions, and the landing path's commit boundary.
func init() {
	batchowner.Engine = batchowner.EngineCalls{
		VerifyRetainedTesting:    verifyRetainedTesting,
		ForecastTestingSelection: forecastTestingSelection,
		CadenceTick:              runProductionCadenceTick,
		BranchClaimCheck:         goalBranchClaimCheck,
	}
	batchowner.BatchOwnerCalls = batchowner.BatchOwnerCallSet{Handover: goalHandoverOwner, EditNext: goalEditNextOwner, Release: goalReleaseOwner, Held: landingHeld}
	batchowner.BatchCommitBoundary = landingPathCommit
	// No batch owner starts while a landing agent runs (A-a, one
	// composition owner).
	agent := newLandingAgent()
	batchowner.LandingAgentLive = agent.liveOrStarting(batchowner.LandingLaneHome)
}

// runProductionCadenceTick runs one cadence tick under the held landing
// owner lease, with the command's trunk fetch, weight threshold, testing
// preparation and worker policy.
func runProductionCadenceTick(root string, held batchowner.BatchOwnerLease, clock func() time.Time) (cadence.TickOutput, error) {
	return cadence.RunTick(root, cadence.Owner{Epoch: held.Epoch, Lineage: batchowner.LandingOwnerLineage,
		Require: func() error { return batchowner.BatchOwnerRequire(held) }, FetchOrigin: batchowner.FetchBatchOrigin, WeightThreshold: weightThreshold,
		Prepare: prepareTestingForCommand, WorkerPolicy: testingWorkerPolicy}, clock)
}
