package goal

// The read-side advance for callers that run it on a schedule. FetchAdvance
// waits on its transport for as long as the transport takes; a caller that
// ticks unattended needs the pass to end either way, because a fetch that
// never returns holds the caller's one in-flight slot forever, stops it
// retrying, and keeps it from shutting down. The bound belongs to the engine
// that owns the transport, not to the caller that schedules it.

import "time"

// FetchAdvanceBounded is FetchAdvance's acceptance sequence with a
// process-group bound around the one network operation: on expiry the
// transport's whole process group is killed, the per-operation ref is
// cleaned up, and the error names the timeout, so the pass ends as a
// refusal like any other and the accepted ref is untouched.
//
// Single-machine mode runs no transport — its capture is git against paths
// on this machine — and keeps the unbounded path.
//
// processTimeout is the caller's to choose and is never defaulted here: a
// non-positive budget expires the remote fetch at once rather than becoming
// a licence to run unbounded.
func FetchAdvanceBounded(e Endpoint, processTimeout time.Duration) (AdvanceResult, error) {
	return boundedFetchAdvance(e, processTimeout)
}
