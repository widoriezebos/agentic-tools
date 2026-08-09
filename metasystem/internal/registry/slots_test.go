package registry

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type scheduleProber map[int64]identity.Liveness

func (s scheduleProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, known := s[pid]
	if !known || state == identity.Dead {
		return identity.Exact{}, identity.Dead, nil
	}
	if state == identity.Unknown {
		return identity.Exact{}, identity.Unknown, errUnknown
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(100, 0)}, identity.Alive, nil
}

var errUnknown = &probeError{}

type probeError struct{}

func (*probeError) Error() string { return "probe failed" }

// Proof "Gate indeterminacy" (SLC-R5-002) and the four-class cap
// (SLC-R7-N001): every consuming class counts; only clean closure
// frees.
func TestSlotClassification(t *testing.T) {
	reduction := reduceOrFail(t,
		// live-verified
		raw(EventArming, "live", nil), armedRow("live", 41),
		// unknown liveness
		raw(EventArming, "unknown", nil), armedRow("unknown", 42),
		// dead owner
		raw(EventArming, "dead", nil), armedRow("dead", 43),
		// open reservation
		raw(EventArming, "reserved", nil),
		// sweepable closed
		raw(EventArming, "sweepable", nil),
		raw(EventReaped, "sweepable", map[string]any{"reason": "owner-dead", "sweepPending": true}),
		// clean closed — the only free slot
		raw(EventArming, "clean", nil),
		raw(EventExited, "clean", map[string]any{"reason": "purpose-gone", "teardownComplete": true}),
	)
	probe := scheduleProber{41: identity.Alive, 42: identity.Unknown, 43: identity.Dead}
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	slots := Slots(reduction, probe, now, 10*time.Minute, map[string]time.Time{
		"reserved": now.Add(-time.Minute),
	})
	classes := map[string]SlotClass{}
	for _, slot := range slots {
		classes[slot.OwnerTag] = slot.Class
	}
	want := map[string]SlotClass{
		"live": LiveVerified, "unknown": UnknownLiveness, "dead": DeadOwner,
		"reserved": OpenReservation, "sweepable": SweepableClosed, "clean": Free,
	}
	for tag, class := range want {
		if classes[tag] != class {
			t.Fatalf("%s: class %v, want %v", tag, classes[tag], class)
		}
	}
	if got := Consumed(slots); got != 5 {
		t.Fatalf("consumed %d, want 5 (only the clean close frees)", got)
	}
}

// Proof "Reservation is a slot" (SLC-R5-001): open means NOT CLOSED —
// grace expiry makes it actionable, never free.
func TestExpiredReservationStillConsumes(t *testing.T) {
	reduction := reduceOrFail(t, raw(EventArming, "stale", nil))
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	slots := Slots(reduction, scheduleProber{}, now, 10*time.Minute, map[string]time.Time{
		"stale": now.Add(-time.Hour),
	})
	if slots[0].Class != OpenReservation || !slots[0].ActionableOrphan {
		t.Fatalf("expired reservation misclassified: %+v", slots[0])
	}
	if Consumed(slots) != 1 {
		t.Fatal("an actionable orphan freed its slot")
	}
}
