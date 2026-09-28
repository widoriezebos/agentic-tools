package delegation_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// breachStopBed scripts a breached revision whose fence closure opens a batch,
// recording who each closure names as the ordering person.
func breachStopBed(t *testing.T, class string, person func() (string, error)) (*bed, *[]string) {
	t.Helper()
	b := newBed(t)
	b.doubles.Lease.ClassifyFunc = func(delegation.Invocation) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: class}, nil
	}
	b.doubles.Host.OrderingHumanFunc = func(string, int64, time.Time) (string, error) { return person() }
	ordered := &[]string{}
	b.doubles.Goal.BreachStopFunc = func(goalID string, revision uint64, now time.Time, orderedBy string) (goal.StopBatch, error) {
		*ordered = append(*ordered, orderedBy)
		stamp := now.UTC().Format(time.RFC3339)
		batch := goal.StopBatch{
			StopID: "stop-" + goalID + "-r1-f1", GoalID: goalID, GoalRevision: revision, FenceEpoch: 1,
			CapabilityGeneration: 1, Machine: "bed-m1", ClaimEpoch: 1, Reason: goal.StopReasonElapsedLimit,
			State: goal.StopBatchOpen, OpenedAt: stamp, UpdatedAt: stamp, Pass: 1,
		}
		return batch, goal.WriteStopBatch(b.root, batch)
	}
	return b, ordered
}

// Rule H1 (unit U-H1): a person may order the breach stop the steward would
// take. Through the lifecycle's stop-custodian entry a HUMAN caller's stop
// completes, and the fence closure is ordered by the enrolled person's name,
// so it is recorded as human-ordered (dispatch's
// TestHumanOrderedBreachStopNamesThePersonAsActor pins the human:NAME
// history line of that closure).
func TestUH1PersonsBreachStopCompletesRecordedAsHumanOrdered(t *testing.T) {
	t.Parallel()
	b, ordered := breachStopBed(t, lease.ClassHuman, func() (string, error) { return "Wido", nil })
	result := b.run("__breach-stop-goal", "--goal", "ship-widget", "--revision", "1")
	if result.ExitCode != 0 || !strings.Contains(string(result.Stdout), "stop=stop-ship-widget-r1-f1 state=COMPLETE") {
		t.Fatalf("a person's breach stop did not complete: exit %d stdout %q stderr %q", result.ExitCode, result.Stdout, b.stderr.String())
	}
	if len(*ordered) != 1 || (*ordered)[0] != "Wido" {
		t.Fatalf("the person's stop was not ordered by their enrolled name: %q", *ordered)
	}
	batch, err := goal.ReadStopBatch(b.root, "stop-ship-widget-r1-f1")
	if err != nil || batch.State != goal.StopBatchComplete {
		t.Fatalf("the person's stop batch is not complete: %+v %v", batch, err)
	}
}

// An unproven person is guided to the enrolled terminal and nothing is
// fenced; the steward's stop records the custodian and never reads a
// person's proof.
func TestUH1BreachStopNamesOnlyAProvenPerson(t *testing.T) {
	t.Parallel()
	unproven, ordered := breachStopBed(t, lease.ClassHuman, func() (string, error) {
		return "", errors.New("the stop is admitted for a person and records who ordered it, and no enrolled person was proven here; run it at the enrolled terminal, or enroll this one with metasystem system enroll --name NAME")
	})
	result := unproven.run("__breach-stop-goal", "--goal", "ship-widget", "--revision", "1")
	if result.ExitCode == 0 || len(*ordered) != 0 || !strings.Contains(unproven.stderr.String(), "metasystem system enroll --name NAME") {
		t.Fatalf("an unproven person's stop was not guided: exit %d closures %q stderr %q", result.ExitCode, *ordered, unproven.stderr.String())
	}
	proofRead := false
	steward, ordered := breachStopBed(t, lease.ClassSteward, func() (string, error) { proofRead = true; return "Wido", nil })
	result = steward.run("__breach-stop-goal", "--goal", "ship-widget", "--revision", "1")
	if result.ExitCode != 0 || len(*ordered) != 1 || (*ordered)[0] != "" || proofRead {
		t.Fatalf("the steward's stop was not custodian-ordered: exit %d closures %q proofRead=%t stderr %q", result.ExitCode, *ordered, proofRead, steward.stderr.String())
	}
}
