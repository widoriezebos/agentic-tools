package main

// Idempotency rows of the landing object (R-129-ui; U12).

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func init() {
	registerIdempotency("landing status", idemRead, "reads the host's lane record and its keeper; changes nothing", nil)
	registerIdempotency("landing set", idemStateful, "the lane is already that checkout: success, the record untouched", witnessLandingSetRepeat)
	registerIdempotency("landing start", idemStateful, "the lane already runs, unpaused: success, nothing written", witnessLandingStartRepeat)
	registerIdempotency("landing drain", idemStateful, "admission is already closed: success, the drain and lane records untouched", witnessLandingDrainRepeat)
	registerIdempotency("landing stop", idemStateful, "the lane is already stopped: success, the pause untouched", witnessLandingStopRepeat)
	registerIdempotency("landing run", idemStateful, "a landing agent already runs: success, no second launch and nothing written", witnessLandingRunRepeat)
	registerIdempotency("landing unset", idemStateful, "no lane is registered any more: success, nothing written", witnessLandingUnsetRepeat)
}

// witnessLandingRepeat runs one landing verb twice on a registered lane and asserts
// the second run is success that leaves the lane's home as it was.
func witnessLandingRepeat(t *testing.T, act func(*laneVerbBed) []string) {
	t.Helper()
	bed := newLaneVerbBed(t)
	bed.alive = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	words := act(bed)
	if code, _, stderr := bed.run(t, words...); code != 0 {
		t.Fatalf("first %v = %d %q", words, code, stderr)
	}
	before := idemTreeDigest(t, bed.home)
	if code, stdout, stderr := bed.run(t, words...); code != 0 {
		t.Fatalf("repeated %v = %d %q %q", words, code, stdout, stderr)
	}
	idemSameTree(t, "a repeated landing "+words[1], before, idemTreeDigest(t, bed.home))
}

func witnessLandingSetRepeat(t *testing.T) {
	witnessLandingRepeat(t, func(bed *laneVerbBed) []string { return []string{"landing", "set", bed.landingA} })
}

func witnessLandingStartRepeat(t *testing.T) {
	witnessLandingRepeat(t, func(*laneVerbBed) []string { return []string{"landing", "start"} })
}

func witnessLandingStopRepeat(t *testing.T) {
	witnessLandingRepeat(t, func(*laneVerbBed) []string { return []string{"landing", "stop", "--by", "Wido"} })
}

func TestLandingDrainRepeatAsSuccess(t *testing.T) {
	t.Parallel()
	witnessLandingDrainRepeat(t)
}

func witnessLandingDrainRepeat(t *testing.T) {
	t.Helper()
	bed := newDrainVerbBed(t)
	code, result := bed.verb(t, "drain", "--reason", "maintenance")
	expectOutcome(t, "first landing drain", code, result, intentConfirmed)
	if drain, err := plain.ReadDrain(bed.install); err != nil || drain == nil || drain.State != plain.DrainDraining || drain.By != "Wido" || drain.Reason != "maintenance" {
		t.Fatalf("first landing drain did not close admission: %+v, %v", drain, err)
	}
	beforeLane := idemTreeDigest(t, bed.root)
	beforeHome := idemTreeDigest(t, bed.home)
	bed.owners.landing.now = func() time.Time { return laneTestNow.Add(time.Hour) }
	code, result = bed.verb(t, "drain", "--reason", "maintenance")
	expectOutcome(t, "repeated landing drain", code, result, intentUnchanged)
	idemSameTree(t, "a repeated landing drain", beforeLane, idemTreeDigest(t, bed.root))
	idemSameTree(t, "a repeated landing drain", beforeHome, idemTreeDigest(t, bed.home))
}

func witnessLandingUnsetRepeat(t *testing.T) {
	witnessLandingRepeat(t, func(*laneVerbBed) []string { return []string{"landing", "unset"} })
}

// witnessLandingRunRepeat: the first landing run starts the landing agent
// for the queued work; the repeat finds it running and writes nothing.
func witnessLandingRunRepeat(t *testing.T) {
	bed, store := landingRunBed(t)
	bed.wake = []string{"queued"}
	if code, _, stderr := bed.run(t, "landing", "run"); code != 0 {
		t.Fatalf("first landing run = %d %q", code, stderr)
	}
	before := idemTreeDigest(t, bed.home)
	if code, stdout, stderr := bed.run(t, "landing", "run"); code != 0 {
		t.Fatalf("repeated landing run = %d %q %q", code, stdout, stderr)
	}
	idemSameTree(t, "a repeated landing run", before, idemTreeDigest(t, bed.home))
	if launches := landingLaunches(t, store); len(launches) != 1 {
		t.Fatalf("a repeated landing run launched again: %d launches", len(launches))
	}
}
