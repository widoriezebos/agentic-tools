package goal

import "time"

// Owners of packages that import goal cannot be imported by package goal's
// own tests. The external test package (cross_owner_hooks_external_test.go)
// binds these before any test runs, so a goal test reaches the lease and
// dispatch owners in its own process instead of through an engine verb.
var (
	// LeaseAnnounceForTest records a process as a main of root and claims
	// the checkout lease (lease.AnnounceWithPair).
	LeaseAnnounceForTest func(root, session string, pid, start, startTicks int64, bootID, tag, runtime, ownerLineage string) error
	// GoalRevisionAdmissionForTest judges one exact goal revision for a
	// fresh MECHANICAL implementer dispatch with a one-minute cap at now,
	// and returns an error naming the refusal when it is refused.
	GoalRevisionAdmissionForTest func(root, id string, revision uint64, now time.Time) error
)
