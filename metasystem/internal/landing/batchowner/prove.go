package batchowner

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// BatchProofOutcomeAccepted is whether a test run child's result stands as
// a pass: it confirmed, or it reused an earlier success whose evidence is
// sufficient.
func BatchProofOutcomeAccepted(child verbresult.Result, result proofrun.TestResult) bool {
	return child.Outcome == verbresult.Confirmed || child.Outcome == verbresult.Unchanged && result.Delivery.Sufficient
}
