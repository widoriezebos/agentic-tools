package main

// Idempotency rows of the landing object (R-129-ui; U12).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func init() {
	registerIdempotency("landing batch", idemRead, "viewing the batch changes nothing; seal and reopen are stateful forms with their own repeat witnesses", nil)
	registerIdempotency("landing restart", idemStateful, "the same restart is already applied or pending for this proof: success, nothing written", witnessLandingRestartRepeat)
	registerIdempotency("landing status", idemRead, "reads the host's lane record and its keeper; changes nothing", nil)
	registerIdempotency("landing set", idemStateful, "the lane is already that checkout: success, the record untouched", witnessLandingSetRepeat)
	registerIdempotency("landing start", idemStateful, "the lane already runs, unpaused: success, nothing written", witnessLandingStartRepeat)
	registerIdempotency("landing drain", idemStateful, "admission is already closed: success, the drain and lane records untouched", witnessLandingDrainRepeat)
	registerIdempotency("landing stop", idemStateful, "the lane is already stopped: success, the pause untouched", witnessLandingStopRepeat)
	registerIdempotency("landing run", idemStateful, "a landing agent already runs: success, no second launch and nothing written", witnessLandingRunRepeat)
	registerIdempotency("landing unset", idemStateful, "no lane is registered any more: success, nothing written", witnessLandingUnsetRepeat)
}

func TestLandingBatchSealRepeatAsSuccess(t *testing.T) {
	t.Parallel()
	witnessLandingBatchRepeat(t, "seal", "open")
}

func TestLandingBatchReopenRepeatAsSuccess(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"sealed", "running"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			witnessLandingBatchRepeat(t, "reopen", state)
		})
	}
}

func TestLandingRestartRepeatAsSuccess(t *testing.T) {
	t.Parallel()
	witnessLandingRestartRepeat(t)
}

func witnessLandingRestartRepeat(t *testing.T) {
	t.Helper()
	for _, state := range []string{"sealed", "running"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			witnessLandingBatchRepeat(t, "restart", state)
		})
	}
}

// The stateful batch forms repeat through the public router. Both applied
// decisions and requests pending on a live proof must leave records untouched.
func witnessLandingBatchRepeat(t *testing.T, kind, state string) {
	t.Helper()
	bed, _ := batchVerbFixture(t, state)
	words := []string{"landing", "batch", kind}
	want := "already sealed"
	if kind == "reopen" {
		words = append(words, "--with", "shared-state-in-the-ledger")
		want = "already a member"
	} else if kind == "restart" {
		words = []string{"landing", "restart"}
		want = "already a member"
	}
	if state == "running" {
		want = "request already pending"
	}
	words = append(words, "--by", "Wido", "--json")
	code, out := bed.run(t, bed.seat, words...)
	var first intentResult
	helmMust(t, json.Unmarshal([]byte(out), &first))
	expectOutcome(t, "first batch "+kind, code, first, intentConfirmed)
	batch := bed.batch(t)
	if len(batch.Actions) != 1 || batch.Actions[0].Kind != kind || batch.Actions[0].Pending != (state == "running") {
		t.Fatalf("first %s did not record its decision: %+v", kind, batch)
	}
	beforeLane, beforeHome, beforeSeat := idemTreeDigest(t, bed.lane), idemTreeDigest(t, bed.home), idemTreeDigest(t, bed.seat)
	path := filepath.Join(plain.Dir(bed.lane), "batch.json")
	before, err := os.Stat(path)
	helmMust(t, err)
	bed.now = bed.now.Add(time.Minute)
	code, out = bed.run(t, bed.seat, words...)
	var second intentResult
	helmMust(t, json.Unmarshal([]byte(out), &second))
	expectOutcome(t, "repeated batch "+kind, code, second, intentConfirmed)
	if !strings.Contains(second.Summary, want) {
		t.Fatalf("repeat did not answer as applied: %+v; want %q", second, want)
	}
	idemSameTree(t, "a repeated batch "+kind, beforeLane, idemTreeDigest(t, bed.lane))
	idemSameTree(t, "a repeated batch "+kind, beforeHome, idemTreeDigest(t, bed.home))
	idemSameTree(t, "a repeated batch "+kind, beforeSeat, idemTreeDigest(t, bed.seat))
	after, err := os.Stat(path)
	helmMust(t, err)
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("repeat rewrote the batch even though its contents are unchanged")
	}
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
