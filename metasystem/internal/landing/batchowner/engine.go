package batchowner

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// EngineCalls are the engine's own functions the landing batch owner calls:
// each one needs the engine's command configuration (its proof limits,
// worker policy, validation weight and goal-branch holder), which this
// package does not own.
type EngineCalls struct {
	// VerifyRetainedTesting is test verify's retained verification of one
	// selection.
	VerifyRetainedTesting func(testrun.SelectionRequest) (proofrun.TestResult, error)
	// ForecastTestingSelection forecasts one selection's proof cost without
	// running it.
	ForecastTestingSelection func(root string, selection testrun.CostSelection, proofCapMinutes uint64) (testrun.CostEvidence, error)
	// BranchClaimCheck proves, before each goal-branch write, that this
	// checkout still holds the goal's claim.
	BranchClaimCheck func(root, goalID string, endpoint goal.Endpoint) func() error
}

// Engine is set once by the engine at start, with BatchOwnerCalls and
// BatchCommitBoundary, before any landing owner runs.
var Engine EngineCalls
