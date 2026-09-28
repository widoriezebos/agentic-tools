package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// TestWorkStopGoalCompletesItsRecordedStop: a breach-stopped goal's fence
// lifts only once its stop batch is COMPLETE. The refusal names the public
// command that finishes the stop, work stop G, which advances the batch from
// the job records (the former internal job stop-batch-reconcile); a repeat
// changes nothing, and the goal's resume then succeeds.
func TestWorkStopGoalCompletesItsRecordedStop(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	gcliForgivingMust(t, bed, "goal", "budget", "ship-widget", "4h/6/600m/1/2", "--by", "Wido", gcliForgivingFixture)
	bed.setNow(gcliForgivingBreachAt)
	stopID := gcliForgivingOpenStop(t, bed, "ship-widget", "01ARZ3NDEKTSV4RRFFQ69G7S03")

	code, stdout, stderr := gcliForgivingPublic(bed, "goal", "budget", "ship-widget", "keep", "--by", "Wido", gcliForgivingFixture)
	if code == 0 || !strings.Contains(stdout+stderr, "metasystem work stop ship-widget") {
		t.Fatalf("the fenced goal's resume did not name work stop G: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	gcliForgivingMust(t, bed, "work", "stop", "ship-widget")
	batch, err := goal.ReadStopBatch(bed.root, stopID)
	if err != nil || batch.State != goal.StopBatchComplete {
		t.Fatalf("work stop G did not complete stop batch %s: %+v %v", stopID, batch, err)
	}
	tip := bed.tip()
	if out := gcliForgivingMust(t, bed, "work", "stop", "ship-widget"); !strings.Contains(out, "already") || bed.tip() != tip {
		t.Fatalf("a repeated work stop G was not a no-op: %q", out)
	}

	gcliForgivingMust(t, bed, "goal", "budget", "ship-widget", "keep", "--by", "Wido", gcliForgivingFixture)
	if resumed := bed.goalRecord("ship-widget"); goalCLILine(resumed, "- StopFence:") != "" {
		t.Fatalf("the goal's fence did not lift once its stop completed:\n%s", resumed)
	}

	code, stdout, stderr = gcliForgivingPublic(bed, "work", "stop", "ship-widget")
	if code == 0 || !strings.Contains(stdout+stderr, "no stop in progress") {
		t.Fatalf("work stop G without a recorded stop did not refuse with its reason: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
