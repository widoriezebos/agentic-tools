package registry

import (
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// D-4's arming gate: a SLOT IS CONSUMED BY a live-verified claim, an
// OPEN reservation (grace expiry makes it actionable, never free), a
// claim whose owner liveness is UNKNOWN (indeterminacy counts TOWARD
// the cap — the gate under-admits, never over-admits), and a closed
// claim still marked sweepable (SLC-R5-001, SLC-R5-002, SLC-R6-003,
// SLC-R6-007, SLC-R7-N001).

// SlotClass says why a claim consumes (or does not consume) a slot.
type SlotClass int

const (
	// Free: closed, not sweepable — consumes nothing.
	Free SlotClass = iota
	// LiveVerified: armed and its owner proven alive.
	LiveVerified
	// OpenReservation: arming without armed, not closed.
	OpenReservation
	// UnknownLiveness: armed but the owner's liveness is unreadable.
	UnknownLiveness
	// DeadOwner: armed, owner DEFINITIVELY dead — consumes until the
	// gate or janitor closes it (only provable death frees a slot,
	// and the close also stops the recorded set).
	DeadOwner
	// SweepableClosed: closed with possible survivors, until swept.
	SweepableClosed
)

// Slot is one claim's accounting.
type Slot struct {
	OwnerTag string
	Class    SlotClass
	// ActionableOrphan: an OpenReservation past the grace window —
	// the gate resolves these before granting slots (SLC-R6-003).
	ActionableOrphan bool
}

// Slots classifies every claim. Probe answers three-way owner
// liveness; now and grace bound reservation staleness (reservation
// age is read from the arming record's at field, carried on the
// claim's checkout... the reduction does not retain at-times, so the
// caller passes reservationAge per tag via the ages map — the gate
// reads them from the raw frames it already holds).
func Slots(reduction *Reduction, probe identity.Prober, now time.Time, grace time.Duration, reservedAt map[string]time.Time) []Slot {
	var slots []Slot
	tags := append([]string(nil), reduction.Order...)
	sort.Strings(tags)
	for _, tag := range tags {
		claim := reduction.Claims[tag]
		slot := Slot{OwnerTag: tag}
		switch {
		case claim.Closed && claim.Sweepable():
			slot.Class = SweepableClosed
		case claim.Closed:
			slot.Class = Free
		case !claim.Armed:
			slot.Class = OpenReservation
			if at, known := reservedAt[tag]; known && now.Sub(at) > grace {
				slot.ActionableOrphan = true
			}
		default:
			switch identity.AliveRef(probe, identity.Ref{Pid: claim.Owner.Pid, StartedAtSec: claim.Owner.PidStartedAt}) {
			case identity.Alive:
				slot.Class = LiveVerified
			case identity.Dead:
				slot.Class = DeadOwner
			default:
				slot.Class = UnknownLiveness
			}
		}
		slots = append(slots, slot)
	}
	return slots
}

// Consumed counts the slots the cap compares against K (D-6).
func Consumed(slots []Slot) int {
	count := 0
	for _, slot := range slots {
		switch slot.Class {
		case LiveVerified, OpenReservation, UnknownLiveness, DeadOwner, SweepableClosed:
			count++
		}
	}
	return count
}
