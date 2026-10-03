package main

// Idempotency rows of the deploy object (R-129-ui; landing-deploys-the-engine).

import "testing"

func init() {
	registerIdempotency("deploy status", idemRead, "reads the deploy record, the pause and main's tip; changes nothing", nil)
	registerIdempotency("deploy now", idemStateful, "main's tip is already active: success, nothing built or written", witnessDeployNowRepeat)
	registerIdempotency("deploy rollback", idemStateful, "the rollback already holds: success, nothing written", witnessDeployRollbackRepeat)
	registerIdempotency("deploy pause", idemStateful, "deploys are already paused: success, the pause untouched", witnessDeployPauseRepeat)
	registerIdempotency("deploy resume", idemStateful, "deploys are not paused: success, nothing written", witnessDeployResumeRepeat)
}

// witnessDeployRepeat deploys c1 and c2, runs act's words once, and asserts
// that the repeat is success that leaves the home as it was.
func witnessDeployRepeat(t *testing.T, words ...string) {
	t.Helper()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	b.deployed("c2")
	if code, text := b.run(words...); code != 0 {
		t.Fatalf("first %v = %d\n%s", words, code, text)
	}
	before := idemTreeDigest(t, b.home)
	if code, text := b.run(words...); code != 0 {
		t.Fatalf("repeated %v = %d\n%s", words, code, text)
	}
	idemSameTree(t, "a repeated "+words[0]+" "+words[1], before, idemTreeDigest(t, b.home))
}

func witnessDeployNowRepeat(t *testing.T) { witnessDeployRepeat(t, "deploy", "now") }

func witnessDeployRollbackRepeat(t *testing.T) { witnessDeployRepeat(t, "deploy", "rollback") }

func witnessDeployPauseRepeat(t *testing.T) {
	witnessDeployRepeat(t, "deploy", "pause", "--reason", "maintenance")
}

func witnessDeployResumeRepeat(t *testing.T) {
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	if code, text := b.run("deploy", "pause"); code != 0 {
		t.Fatalf("pause = %d\n%s", code, text)
	}
	if code, text := b.run("deploy", "resume"); code != 0 {
		t.Fatalf("first resume = %d\n%s", code, text)
	}
	before := idemTreeDigest(t, b.home)
	if code, text := b.run("deploy", "resume"); code != 0 {
		t.Fatalf("repeated resume = %d\n%s", code, text)
	}
	idemSameTree(t, "a repeated deploy resume", before, idemTreeDigest(t, b.home))
}
