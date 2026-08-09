package supervise

import (
	"testing"
	"time"
)

// Classify's table is D-1's decision table verbatim; each row cites
// the Proof or finding it executes.

func TestClassify(t *testing.T) {
	cases := []struct {
		name     string
		root     FileState
		currency CurrencyState
		state    FileState
		want     Verdict
	}{
		// Proof "Purpose gone": a deleted checkout always lands in
		// PurposeGone, never Superseded (SLC-R6-001) — even though the
		// lock vanished with it.
		{"deleted checkout", Absent, NoLock, Absent, PurposeGone},
		{"deleted checkout, lock cached as other", Absent, NamesOther, Absent, PurposeGone},
		// Revocation: our lock, state token deliberately removed.
		{"state revoked under own lock", Present, NamesSelf, Absent, PurposeGone},
		// Healthy current owner.
		{"current", Present, NamesSelf, Present, Continue},
		// SLC-R3-003: replaced after a false-death takeover — leave.
		{"replaced", Present, NamesOther, Present, Superseded},
		{"lock revoked, checkout persists", Present, NoLock, Present, Superseded},
		// Proof "Indeterminacy": chmod-000 state — keep running, no
		// relaunch, no exit.
		{"state unreadable", Present, NamesSelf, Indeterminate, Blind},
		{"lock unreadable", Present, Unreadable, Present, Blind},
		{"root unreadable", Indeterminate, NamesSelf, Present, Blind},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := Classify(row.root, row.currency, row.state); got != row.want {
				t.Fatalf("Classify(%v,%v,%v) = %v, want %v", row.root, row.currency, row.state, got, row.want)
			}
		})
	}
}

// Proof "Breaker on the REAL shape": components that die every cycle
// trip the counter at exactly GiveUpAt.
func TestBreakerTripsAtN(t *testing.T) {
	breaker := &Breaker{GiveUpAt: 5, BaseInterval: time.Second, BackoffCap: 10 * time.Minute}
	for i := 1; i <= 4; i++ {
		if verdict := breaker.Advance(Failing); verdict.GiveUp {
			t.Fatalf("gave up early at observation %d", i)
		}
	}
	if verdict := breaker.Advance(Failing); !verdict.GiveUp {
		t.Fatal("did not give up at N=5")
	}
}

// A full healthy interval resets; UNKNOWN neither increments nor
// resets (SLC-R3-004) and skips the relaunch.
func TestBreakerResetAndIndeterminacy(t *testing.T) {
	breaker := &Breaker{GiveUpAt: 3, BaseInterval: time.Second, BackoffCap: time.Minute}
	breaker.Advance(Failing)
	breaker.Advance(Failing)
	if verdict := breaker.Advance(Indeterminable); !verdict.SkipRelaunch || breaker.Consecutive != 2 {
		t.Fatalf("indeterminacy moved the counter: %+v %d", verdict, breaker.Consecutive)
	}
	breaker.Advance(Healthy)
	if breaker.Consecutive != 0 {
		t.Fatal("healthy interval did not reset the counter")
	}
	if verdict := breaker.Advance(Failing); verdict.GiveUp {
		t.Fatal("counter survived the reset")
	}
}

// Backoff gates RELAUNCHES only: interval × 2^(k-1), capped (D-6).
func TestBackoffSchedule(t *testing.T) {
	breaker := &Breaker{GiveUpAt: 100, BaseInterval: 10 * time.Second, BackoffCap: 10 * time.Minute}
	want := []time.Duration{
		0,                // k=1: immediate
		10 * time.Second, // k=2
		20 * time.Second, // k=3
		40 * time.Second, // k=4
		80 * time.Second, // k=5
	}
	for i, expected := range want {
		verdict := breaker.Advance(Failing)
		if verdict.RelaunchAfter != expected {
			t.Fatalf("observation %d: backoff %v, want %v", i+1, verdict.RelaunchAfter, expected)
		}
	}
	for i := 0; i < 20; i++ {
		if verdict := breaker.Advance(Failing); verdict.RelaunchAfter > 10*time.Minute {
			t.Fatalf("backoff exceeded its cap: %v", verdict.RelaunchAfter)
		}
	}
}

// Proof "Ceiling under forking": membership above the ceiling is never
// tolerated for two consecutive observations — the verdict is
// stop-the-set NOW and an incrementing observation.
func TestCeiling(t *testing.T) {
	if stop, failing := CeilingVerdict(13, 12); !stop || !failing {
		t.Fatal("overshoot must stop the set and count against the breaker")
	}
	if stop, failing := CeilingVerdict(12, 12); stop || failing {
		t.Fatal("at the ceiling is legal — refusal applies to LAUNCHES, not standing sets")
	}
}

// Proof "Establishment bounded": an owner that cannot complete first
// publication within the deadline gives up; publication disarms the
// clock permanently.
func TestEstablishment(t *testing.T) {
	e := &Establishment{Deadline: 5}
	for i := 1; i <= 4; i++ {
		if e.Observe(false) {
			t.Fatalf("gave up early at %d", i)
		}
	}
	if !e.Observe(false) {
		t.Fatal("did not give up at the deadline")
	}
	published := &Establishment{Deadline: 2}
	if published.Observe(true) || published.Observe(false) || published.Observe(false) {
		t.Fatal("a published owner must never fail establishment")
	}
}

// SLC-R9-003: the watermark advances over the verified contiguous
// prefix only; one unverified older generation pins it.
func TestRetireWatermark(t *testing.T) {
	recorded := []int64{1, 2, 3}
	cases := []struct {
		name     string
		current  int64
		verified map[int64]bool
		want     int64
	}{
		{"nothing verified", 0, map[int64]bool{}, 0},
		{"contiguous prefix", 0, map[int64]bool{1: true, 2: true}, 2},
		{"gap pins the watermark", 0, map[int64]bool{2: true, 3: true}, 0},
		{"resume from prior watermark", 1, map[int64]bool{2: true}, 2},
		{"skip beyond recorded ends the walk", 0, map[int64]bool{1: true, 2: true, 3: true}, 3},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := RetireWatermark(row.current, row.verified, recorded); got != row.want {
				t.Fatalf("watermark %d, want %d", got, row.want)
			}
		})
	}
}
