package dispatch

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Rule H1 (verbs-object-action 3.6): a person may order the breach stop the
// custodian would take, and the published fence closure names that person
// as its actor instead of the custodian lineage.
func TestHumanOrderedBreachStopNamesThePersonAsActor(t *testing.T) {
	bed := newGoalMutationBed(t)
	now := time.Date(2026, 8, 28, 21, 0, 0, 0, time.UTC)
	batch, err := bed.stopOrderedBy("bounded", 2, now, "Wido")
	if err != nil {
		t.Fatal(err)
	}
	if batch.State != goal.StopBatchOpen || batch.StopID == "" {
		t.Fatalf("human-ordered stop did not open its batch: %+v", batch)
	}
	accepted := bed.parsedAcceptedGoal(t, "bounded")
	if accepted.StopFence == nil || accepted.StopFence.StopID != batch.StopID {
		t.Fatalf("human-ordered stop did not close the fence: %+v", accepted.StopFence)
	}
	last := accepted.History[len(accepted.History)-1]
	if last.Verb != "breach-stop" || last.Actor != "human:Wido" {
		t.Fatalf("the fence closure's history line = %+v, want verb breach-stop by human:Wido", last)
	}
	retry, err := bed.stop("bounded", 2, now.Add(time.Second))
	if err != nil || retry.StopID != batch.StopID {
		t.Fatalf("the custodian's retry after a human-ordered stop was not idempotent: %+v %v", retry, err)
	}
}

// A custodian-ordered stop keeps the custodian lineage as its actor.
func TestCustodianBreachStopKeepsTheCustodianActor(t *testing.T) {
	bed := newGoalMutationBed(t)
	now := time.Date(2026, 8, 28, 21, 0, 0, 0, time.UTC)
	if _, err := bed.stop("bounded", 2, now); err != nil {
		t.Fatal(err)
	}
	accepted := bed.parsedAcceptedGoal(t, "bounded")
	last := accepted.History[len(accepted.History)-1]
	if last.Verb != "breach-stop" || !strings.HasSuffix(last.Actor, "+"+stopCustodianLineage) {
		t.Fatalf("the custodian's fence closure history line = %+v", last)
	}
}

// The one damage refusal a person can meet here guides: a revision inside
// its budget is not fenced, and the message names the public commands that
// stop the work or park the goal instead.
func TestBreachStopInsideTheBudgetGuidesThePerson(t *testing.T) {
	bed := newGoalMutationBed(t)
	inside := time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC)
	_, err := bed.stopOrderedBy("bounded", 2, inside, "Wido")
	if err == nil || !strings.Contains(err.Error(), "no live-stop breach") ||
		!strings.Contains(err.Error(), "metasystem work stop") || !strings.Contains(err.Error(), "metasystem goal pause bounded") {
		t.Fatalf("an in-budget stop did not guide to the public commands: %v", err)
	}
}

func TestEnsureBreachStopOrderedByNeedsAName(t *testing.T) {
	if _, err := EnsureBreachStopOrderedBy(t.TempDir(), "bounded", 2, time.Now(), ""); err == nil {
		t.Fatal("a human-ordered stop without a name was accepted")
	}
}

// A stranded human-ordered stop is not replayed from journal text: the
// stored name records who intended it and is never the credential that acts
// as that person, so recovery closes it for the person to order again.
func TestStrandedHumanOrderedBreachStopIsNotReplayed(t *testing.T) {
	bed := newGoalMutationBed(t)
	root := bed.root
	stopID, ulid := stopIdentity("bounded", 2, 1)
	opid := goal.Opid(ulid, "bed-m1", stopCustodianLineage)
	intent := goal.Intent{Verb: "breach-stop", Targets: []string{"bounded"}, Args: map[string]string{
		"stopId": stopID, "reason": goal.StopReasonElapsedLimit, "goalRevision": "2",
		"capabilityGeneration": "2", "capabilityMachine": "bed-m1", "claimEpoch": "7", "fenceEpoch": "0", "by": "Wido",
	}}
	if _, err := goal.CreateEntry(root, opid, "bed-m1", stopCustodianLineage, intent); err != nil {
		t.Fatal(err)
	}
	entry, err := goal.ReadEntry(root, opid)
	if err != nil {
		t.Fatal(err)
	}
	entry.Owner = goal.OwnerIdentity{Pid: 999999999, PidStartedAt: 1}
	writeJSON(t, root+"/artifacts/agents/goal-transactions/"+opid+".json", entry)
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	reports, err := goal.RecoverWithPolicy(bed.endpoint(), goalRecoveryPolicyWithReads{GoalRecoveryPolicy: GoalRecoveryPolicy{Now: now}, reads: bed.reads})
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) == 0 || !strings.Contains(reports[len(reports)-1].Detail, "re-run it from the human authority boundary") {
		t.Fatalf("a stranded human-ordered stop was not closed for the person to order again: %+v", reports)
	}
	if binding, err := bed.binding("bounded", now); err != nil || binding.Fence != nil {
		t.Fatalf("journal text replayed a human-ordered stop: %+v %v", binding, err)
	}
}
