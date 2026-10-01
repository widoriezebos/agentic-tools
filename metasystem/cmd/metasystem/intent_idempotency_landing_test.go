package main

// Idempotency rows of the landing object (R-129-ui; U12).

import "testing"

func init() {
	registerIdempotency("landing status", idemRead, "reads the host's lane record, its keeper and its batches; changes nothing", nil)
	registerIdempotency("landing set", idemStateful, "the lane is already that checkout: success, the record untouched", witnessLandingSetRepeat)
	registerIdempotency("landing start", idemStateful, "the lane already runs, unpaused: success, nothing written", witnessLandingStartRepeat)
	registerIdempotency("landing stop", idemStateful, "the lane is already stopped: success, the pause untouched", witnessLandingStopRepeat)
	registerIdempotency("landing begin", idemStateful, "the batch's series is already recorded: success, the opening untouched", witnessLandingBeginRepeat)
	registerIdempotency("landing prove", idemCreation,
		"each run is a new test run recorded as its own attempt on the batch, charged to its allowance; a second prove is a second run, not a repeat", nil)
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
