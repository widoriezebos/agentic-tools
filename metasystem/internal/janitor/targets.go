package janitor

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// D-4's target selection: reduce the registry, then tear down IN
// ORDER. Selection is pure — reduction, slots, and file states in;
// an ordered work list out — so the sweep's law lives in table tests
// and the process-touching sweep executes the list elsewhere.

// TargetKind is why a claim is being acted on, in D-4's normative
// order.
type TargetKind int

const (
	// CheckoutGone: a live claim whose checkout no longer exists
	// (SLC-R1-011: --shutdown cannot run, its scripts are gone).
	CheckoutGone TargetKind = iota
	// OwnerDead: an open claim on a LIVE checkout whose owner is
	// provably dead (SLC-R9-005 — the janitor must not leave a dead
	// owner's detached components running until a future arm).
	OwnerDead
	// EstablishmentOrphan: an arming-only claim past grace with live
	// tag-matched processes (SLC-R4-005, SLC-R5-015) — guarded by the
	// live-announced-session check (SLC-R13-003).
	EstablishmentOrphan
	// SweepableClaim: a closed claim still marked sweepable
	// (SLC-R4-012); verify survivors and append swept when clear.
	SweepableClaim
	// CustodianDead: a bound custody whose custodian is provably dead
	// on a live checkout, no live announced session, fenced by the
	// held reap-intent marker (D-3).
	CustodianDead
)

// Target is one unit of janitor work.
type Target struct {
	Kind  TargetKind
	Claim *registry.Claim
	// CustodyID is set for CustodianDead targets.
	CustodyID string
}

// World answers the environment questions selection needs; the real
// implementation stats checkouts and probes processes, tests inject.
type World interface {
	// CheckoutState reports the checkout root three-way (D-1's
	// discipline: only a definitive absence selects CheckoutGone).
	CheckoutState(path string) supervise.FileState
	// OwnerLiveness proves the claim's recorded owner three-way.
	OwnerLiveness(ref registry.ProcessRef) identity.Liveness
	// LiveTagged reports whether any live process matches the
	// claim's invocation signatures (REG-6 shapes with its tags).
	LiveTagged(claim *registry.Claim) bool
	// LiveAnnouncedSession reports whether a live announced session
	// exists on the checkout (SLC-R5-006, SLC-R13-003).
	LiveAnnouncedSession(path string) bool
	// ReservationExpired reports whether an arming-only claim is past
	// the grace window (the gate carries reservation times; the
	// reduction does not).
	ReservationExpired(tag string) bool
}

// SelectTargets walks the reduction and returns the ordered work
// list. Indeterminacy NEVER selects: an unreadable checkout or an
// unproven owner is reported by the caller, not acted on.
func SelectTargets(reduction *registry.Reduction, world World) []Target {
	var checkoutGone, ownerDead, orphans, sweepables, custodian []Target

	for _, tag := range reduction.SortedTags() {
		claim := reduction.Claims[tag]
		if claim.Closed {
			if claim.Sweepable() {
				sweepables = append(sweepables, Target{Kind: SweepableClaim, Claim: claim})
			}
			continue
		}
		checkout := world.CheckoutState(claim.CheckoutPath)
		if checkout == supervise.Absent {
			checkoutGone = append(checkoutGone, Target{Kind: CheckoutGone, Claim: claim})
			continue
		}
		if checkout != supervise.Present {
			continue // indeterminate checkout: report, never act
		}
		if claim.Armed {
			if world.OwnerLiveness(claim.Owner) == identity.Dead {
				ownerDead = append(ownerDead, Target{Kind: OwnerDead, Claim: claim})
			}
			continue
		}
		// Arming-only: an establishment orphan needs grace expiry,
		// live tag-matched processes, and NO live announced session
		// (SLC-R13-003's guard).
		if world.ReservationExpired(tag) && world.LiveTagged(claim) &&
			!world.LiveAnnouncedSession(claim.CheckoutPath) {
			orphans = append(orphans, Target{Kind: EstablishmentOrphan, Claim: claim})
		}
	}

	for _, id := range reduction.CustodyOrder {
		custody := reduction.Custodies[id]
		if custody.Released || custody.BoundOwnerTag == "" {
			continue
		}
		claim := reduction.Claims[custody.BoundOwnerTag]
		if claim == nil {
			continue
		}
		if world.CheckoutState(custody.CheckoutPath) != supervise.Present {
			continue // custodian-dead reaps act on LIVE checkouts only (D-3)
		}
		if world.OwnerLiveness(custody.Custodian) != identity.Dead {
			continue
		}
		if world.LiveAnnouncedSession(custody.CheckoutPath) {
			continue // supervision someone is using is never killed (SLC-R5-006)
		}
		custodian = append(custodian, Target{Kind: CustodianDead, Claim: claim, CustodyID: id})
	}

	ordered := append(checkoutGone, ownerDead...)
	ordered = append(ordered, orphans...)
	ordered = append(ordered, sweepables...)
	return append(ordered, custodian...)
}
