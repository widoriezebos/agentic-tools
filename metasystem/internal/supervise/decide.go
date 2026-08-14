// Package supervise implements the owner lifecycle of
// plans/supervision-lifecycle.md: the purpose/currency check that
// gives the owner a lifetime (D-1) and the breaker that bounds its
// self-healing (D-2). The decision logic is pure — observations in,
// verdicts out — so every Proof row about it runs as a table test;
// the process-touching edges live behind small interfaces.
package supervise

import "time"

// FileState is a three-way observation of one file or directory:
// present, definitively absent (ENOENT on it or any parent), or
// unknown (any other failure). Only a definite negative kills
// (SLC-R1-003, SLC-R2-006).
type FileState int

const (
	Present FileState = iota
	Absent
	Indeterminate
)

// CurrencyState is what the checkout lock's owner file says about us.
type CurrencyState int

const (
	// NamesSelf: owner.json is readable and names this process.
	NamesSelf CurrencyState = iota
	// NamesOther: owner.json is readable and names another identity.
	NamesOther
	// NoLock: the lock or its owner file is definitively absent.
	NoLock
	// Unreadable: any other failure reading the lock.
	Unreadable
)

// Verdict is the owner's per-cycle conclusion (D-1's check, run every
// base interval, independent of backoff).
type Verdict int

const (
	// Continue: current owner; proceed to the breaker's business.
	Continue Verdict = iota
	// PurposeGone: the supervised thing no longer exists or was
	// revoked. Teardown; exit reason purpose-gone.
	PurposeGone
	// Superseded: another owner is current, or the lock was revoked
	// while the checkout persists. Teardown held tags; exit reason
	// superseded.
	Superseded
	// Blind: indeterminacy somewhere. Log, continue, relaunch nothing
	// this cycle.
	Blind
)

// Classify is D-1's decision table. Order is normative: PURPOSE GONE
// is checked FIRST (SLC-R6-001) so a deleted checkout — which takes
// the lock with it — lands there and never in SUPERSEDED; the lock
// vanishing WITH the checkout is not a replacement.
func Classify(checkoutRoot FileState, currency CurrencyState, state FileState) Verdict {
	if checkoutRoot == Absent {
		return PurposeGone
	}
	if checkoutRoot == Indeterminate {
		return Blind
	}
	switch currency {
	case NamesSelf:
		if state == Absent {
			// Deliberate revocation: our own token was removed while
			// we are still the named owner (subsumes E1/E3,
			// SLC-R2-005).
			return PurposeGone
		}
		if state == Indeterminate {
			return Blind
		}
		return Continue
	case NamesOther, NoLock:
		// Replaced or revoked while the checkout persists. A live
		// owner can only be superseded after a FALSE death
		// observation; leaving voluntarily is the insurance for
		// exactly that case (SLC-R3-003).
		return Superseded
	default:
		return Blind
	}
}

// Observation is the breaker's three-way component reading (D-2,
// SLC-R3-004).
type Observation int

const (
	// Healthy: both components alive with fresh heartbeats for a full
	// base interval.
	Healthy Observation = iota
	// Failing: a component is DEAD (exact identity proven dead) or
	// STALE (alive but its heartbeat stale or missing) — or the set
	// broke a structural bound (ceiling overshoot, unrecordable set).
	Failing
	// Indeterminable: state or identity could not be read at all.
	Indeterminable
)

// Breaker counts consecutive failing observations on ONE clock
// (SLC-R3-005): observations tick every base interval and nothing
// else; backoff gates relaunch attempts only.
type Breaker struct {
	// Consecutive failing observations. Reset by a healthy interval;
	// UNKNOWN neither increments nor resets.
	Consecutive int
	// GiveUpAt is D-6's N: the observation count at which the owner
	// stops healing and exits reason giving-up.
	GiveUpAt int
	// BaseInterval and BackoffCap are D-6's numbers: relaunch 1 waits
	// nothing, relaunch k≥2 waits interval × 2^(k-2), capped.
	BaseInterval time.Duration
	BackoffCap   time.Duration
}

// Advance folds one observation and reports the owner's move.
func (b *Breaker) Advance(observation Observation) BreakerVerdict {
	switch observation {
	case Healthy:
		b.Consecutive = 0
		return BreakerVerdict{}
	case Indeterminable:
		// Logged, never acted on: no increment, no reset, no relaunch
		// this cycle.
		return BreakerVerdict{SkipRelaunch: true}
	}
	b.Consecutive++
	if b.Consecutive >= b.GiveUpAt {
		return BreakerVerdict{GiveUp: true}
	}
	return BreakerVerdict{RelaunchAfter: b.backoff()}
}

// backoff returns the delay before the NEXT relaunch attempt: after k
// consecutive incrementing observations, 0 for k=1 and interval ×
// 2^(k-2) for k≥2 (k=2→interval, k=3→2×interval, …), capped (D-6;
// the schedule TestBackoffSchedule pins). Observations and D-1's
// checks never slow down (SLC-R2-008).
func (b *Breaker) backoff() time.Duration {
	if b.Consecutive <= 1 {
		return 0
	}
	delay := b.BaseInterval
	for i := 1; i < b.Consecutive-1; i++ {
		delay *= 2
		if delay >= b.BackoffCap {
			return b.BackoffCap
		}
	}
	if delay > b.BackoffCap {
		return b.BackoffCap
	}
	return delay
}

// BreakerVerdict is what one observation obliges the owner to do.
type BreakerVerdict struct {
	// GiveUp: teardown per D-1, then exited reason giving-up with the
	// last diagnosis and the teardown outcome (SLC-R4-012).
	GiveUp bool
	// SkipRelaunch: indeterminacy — attempt nothing this cycle.
	SkipRelaunch bool
	// RelaunchAfter: the earliest a relaunch may be attempted,
	// relative to the failing observation that triggered it.
	RelaunchAfter time.Duration
}

// CeilingVerdict applies D-2's population bound: the DURATION property.
// A count above the ceiling is itself an incrementing observation and
// the owner stops the set immediately, so a group above the ceiling
// survives at most one base interval (SLC-R3-006, SLC-R3-007,
// SLC-R4-010).
func CeilingVerdict(groupMembers, ceiling int) (stopSet bool, failing bool) {
	if groupMembers > ceiling {
		return true, true
	}
	return false, false
}

// Establishment bounds first publication (SLC-R1-004, SLC-R4-005): an
// owner that cannot COMPLETE first publication within Deadline
// consecutive observations gives up as establishment-failed. The
// purpose/currency check arms only after Published.
type Establishment struct {
	Published    bool
	Observations int
	Deadline     int
}

// Observe advances the establishment clock; it reports true when the
// owner must exit establishment-failed.
func (e *Establishment) Observe(published bool) (giveUp bool) {
	if published {
		e.Published = true
		return false
	}
	if e.Published {
		return false
	}
	e.Observations++
	return e.Observations >= e.Deadline
}

// RetireWatermark advances retiredThrough over the verified contiguous
// prefix ONLY (SLC-R8-002, SLC-R9-003): given the current watermark
// and the set of generation numbers whose every recorded identity was
// VERIFIED DEAD in this pass, it returns the new watermark. One
// unverified older generation pins it, however many newer generations
// were verified.
func RetireWatermark(current int64, verifiedDead map[int64]bool, recorded []int64) int64 {
	watermark := current
	for {
		next := watermark + 1
		if !contains(recorded, next) {
			// A gap in recorded generations cannot be verified; the
			// watermark may not skip it unless nothing was recorded
			// there at all — and generations are minted densely, so
			// absence past the highest recorded number ends the walk.
			if next > highest(recorded) {
				return watermark
			}
			return watermark
		}
		if !verifiedDead[next] {
			return watermark
		}
		watermark = next
	}
}

func contains(values []int64, wanted int64) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func highest(values []int64) int64 {
	var top int64 = -1
	for _, value := range values {
		if value > top {
			top = value
		}
	}
	return top
}
