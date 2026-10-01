package batch

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Dispatch is one run the owner starts: the batch, the window and sample it
// was admitted under, the plan token its completion must carry, and the
// runner whose capacity admitted it.
type Dispatch struct {
	ID, Window, Token, Runner string
	Sample                    proofrun.LoadSample
}

// Completion is a finished run as it returns to the owner's loop.
type Completion struct {
	ID, Token string
	Err       error
}

// RunProbe is that answer: a terminal run carries its result file's result.
type RunProbe struct {
	State, Detail string
	Result        proofrun.TestResult
	Err           error
}

// RunnerCapacity is one proof runner's own capacity: the CPUs it gives
// proofs, its sampled load, the proofs its census sees that are not this
// owner's, and its CPU-derived ceiling. The host runner reports the local
// cores and load; any other runner reports its own, so no core counts twice.
type RunnerCapacity struct {
	Runner      string
	Cores       int
	Load        float64
	LoadKnown   bool
	Overlapping int
	Ceiling     int
}
