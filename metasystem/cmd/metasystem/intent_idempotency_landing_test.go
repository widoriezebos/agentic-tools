package main

// Idempotency rows of the landing object (R-129-ui; U12).

import "testing"

func init() {
	registerIdempotency("landing status", idemRead, "reads the host's lane record, its keeper and its batches; changes nothing", nil)
	registerIdempotency("landing restart", idemCreation,
		"an explicit request to stop and start the lane's owner again; a second restart is a second cycle, not a repeat; a start of what already runs is landing start", nil)
	registerIdempotency("landing set", idemStateful, "the lane is already that checkout: success, the record untouched", witnessLandingSetRepeat)
	registerIdempotency("landing start", idemStateful, "the owner already runs, unpaused and without restarts: success, nothing started or written", witnessLandingStartRepeat)
	registerIdempotency("landing stop", idemStateful, "the lane is already stopped: success, the pause untouched", witnessLandingStopRepeat)
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
	before, starts := idemTreeDigest(t, bed.home), bed.starts
	if code, stdout, stderr := bed.run(t, words...); code != 0 || bed.starts != starts {
		t.Fatalf("repeated %v = %d %q %q, starts %d -> %d", words, code, stdout, stderr, starts, bed.starts)
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
