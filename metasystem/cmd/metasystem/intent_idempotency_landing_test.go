package main

// Idempotency rows of the landing object (R-129-ui; U12).

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func init() {
	registerIdempotency("landing status", idemRead, "reads the host's lane record, its keeper and its batches; changes nothing", nil)
	registerIdempotency("landing set", idemStateful, "the lane is already that checkout: success, the record untouched", witnessLandingSetRepeat)
	registerIdempotency("landing start", idemStateful, "the lane already runs, unpaused: success, nothing written", witnessLandingStartRepeat)
	registerIdempotency("landing stop", idemStateful, "the lane is already stopped: success, the pause untouched", witnessLandingStopRepeat)
	registerIdempotency("landing prove", idemCreation,
		"each run is a new test run of the checkout's tree, recorded as its own attempt; a second prove is a second run, not a repeat", nil)
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

func witnessLandingUnsetRepeat(t *testing.T) {
	witnessLandingRepeat(t, func(*laneVerbBed) []string { return []string{"landing", "unset"} })
}

// witnessLandingRunRepeat: the first landing run starts the landing agent
// for the queued work; the repeat finds it running and writes nothing.
func witnessLandingRunRepeat(t *testing.T) {
	bed, store := landingRunBed(t)
	bed.records = []batch.Record{{BatchID: "b-one", State: batch.StateOpen, Units: []batch.Unit{{GoalID: "g-one", State: batch.UnitJoined}}}}
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
