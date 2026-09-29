package main

// The holder's step on its Stop path (g1-s70 D3, SOL-S70-02): the Stop takes
// the step itself through the public command under the session's identity,
// over the bed's own ledger; nothing is typed. A due landing joins the batch,
// a refused one is shown at every Stop while it stands, and a send-back's
// revision is started once: its answer on the goal ends it.

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// holderStop is one Stop of the session holding the bed's goal (mac-cli+m1),
// judged over the bed's ledger with the production step taker.
func holderStop(t *testing.T, bed *intentBed, owners intentOwners, now time.Time) goal.Verdict {
	t.Helper()
	return holderStopOver(t, bed, owners, now, goal.ScanResult{})
}

// holderStopOver is holderStop over what the Stop's scan found.
func holderStopOver(t *testing.T, bed *intentBed, owners intentOwners, now time.Time, scan goal.ScanResult) goal.Verdict {
	t.Helper()
	endpoint, err := owners.dependencies.endpoint(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	store := &goal.Store{Root: bed.root(), Now: func() time.Time { return now }, TakeHolderStep: holderStepTaker(bed.root(), owners)}
	verdict, err := store.TurnVerdictAtEndpoint(endpoint, "mac-cli", scan, "holder-session", "", "main-1",
		goal.TurnVerdictOptions{SeatActor: goal.Actor{Machine: "mac-cli", Lineage: "m1"}})
	if err != nil {
		t.Fatal(err)
	}
	return verdict
}

// deliveryOwners are the owners the bed's public commands run with.
func (b *deliveryBed) deliveryOwners() intentOwners {
	owners := b.intentBed.owners()
	owners.delivery, owners.connection = b.owners, b.connection
	return owners
}

// The Landing record is at 09:30; with the default four hours the goal is due
// from 13:30.
var holderDue = time.Date(2026, 9, 1, 14, 0, 0, 0, time.UTC)

func TestADueGoalIsAdmittedAtTheHoldersStopWithNoCommandTyped(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) { waitingToLandBed(file); retier(file, 1) })
	b.lineage = "m1"
	verdict := holderStop(t, b.intentBed, b.deliveryOwners(), holderDue)
	if len(owners.joins) != 1 || owners.joins[0].GoalID != bedGoal {
		t.Fatalf("the due goal was not admitted to the batch on the Stop: %+v\n%s", owners.joins, verdict.Display)
	}
	if !strings.Contains(verdict.Display, "LANDED "+bedGoal+": joined batch b-1") || strings.Contains(verdict.Display, "metasystem work land") {
		t.Fatalf("the Stop does not say what was done: %s", verdict.Display)
	}
}

func TestARefusedLandingIsShownAtEveryStopAndTheHumansVerb(t *testing.T) {
	t.Parallel()
	// Cleared at the tip, then held by a sitting the human opened again: the
	// word makes the landing worth taking, and the gate refuses it.
	b, owners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) {
		clearedAt(gateBedTip)(file)
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW2", "review", goal.SittingReason(true, reviewBedRecord, "Wido"))
	})
	b.lineage = "m1"
	for stop := 1; stop <= 2; stop++ {
		verdict := holderStop(t, b.intentBed, b.deliveryOwners(), holderDue.Add(time.Duration(stop)*time.Minute))
		if !strings.Contains(verdict.Display, "LANDING REFUSED "+bedGoal+" [LANDING_HELD_BY_SITTING]: goal "+bedGoal+" is held by Wido's review sitting") ||
			!strings.Contains(verdict.Display, "metasystem goal review "+bedGoal+" --release --record "+reviewBedRecord) {
			t.Fatalf("stop %d: the refusal, its code and the human's verb are not shown: %s", stop, verdict.Display)
		}
		if strings.Contains(verdict.Display, "metasystem work land") {
			t.Fatalf("stop %d: the step is advised rather than taken: %s", stop, verdict.Display)
		}
	}
	if len(owners.joins) != 0 {
		t.Fatalf("a held goal joined: %+v", owners.joins)
	}
}

func TestASendBacksRevisionIsStartedOnceAtTheHoldersStop(t *testing.T) {
	t.Parallel()
	bed := sentBackBed(t)
	var calls []reviseCall
	owners := holderOwners(bed, &calls, func(int) intentResult { return attemptStarted(3) })
	first := holderStop(t, bed, owners, holderDue)
	if len(calls) != 1 || !strings.Contains(calls[0].brief, "Read the reviewed tree") {
		t.Fatalf("the revision was not started from the published brief on the Stop: %+v\n%s", calls, first.Display)
	}
	if !strings.Contains(first.Display, "REVISION STARTED "+bedGoal) || !strings.Contains(first.Display, "attempt 3 is recorded on the goal") {
		t.Fatalf("the Stop does not say the revision started: %s", first.Display)
	}
	second := holderStop(t, bed, owners, holderDue.Add(time.Minute))
	if len(calls) != 1 || strings.Contains(second.Display, "REVISION") {
		t.Fatalf("the second Stop started the revision again: calls=%d\n%s", len(calls), second.Display)
	}
}

// The holder takes its steps on every Stop, whatever the scan found (SOL-S70-02
// round 2): a machine holding a landing claim while it works on another goal
// has open work or a busy checkout, and the scan's own verdict stands beside
// the step.
var (
	openWorkScan = goal.ScanResult{Open: []goal.Item{{Detail: "plans/other.md: 1 open item"}}}
	busyScan     = goal.ScanResult{Busy: []goal.Item{{Detail: "a delegate job runs"}}}
)

func TestADueGoalIsAdmittedAtAStopWithOpenWorkAndTheOpenWorkStillReported(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) { waitingToLandBed(file); retier(file, 1) })
	b.lineage = "m1"
	verdict := holderStopOver(t, b.intentBed, b.deliveryOwners(), holderDue, openWorkScan)
	if len(owners.joins) != 1 || owners.joins[0].GoalID != bedGoal {
		t.Fatalf("the due goal was not admitted on a Stop with open work: %+v\n%s", owners.joins, verdict.Display)
	}
	if !strings.Contains(verdict.Display, "LANDED "+bedGoal+": joined batch b-1") || !strings.Contains(verdict.Display, "OPEN WORK (1)") || !strings.Contains(verdict.Display, "plans/other.md: 1 open item") {
		t.Fatalf("the Stop does not say both the landing and the open work: %s", verdict.Display)
	}
	if verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
		t.Fatalf("the open work's own verdict changed: %+v", verdict)
	}
}

func TestADueGoalIsAdmittedAtAStopWhileTheCheckoutIsBusy(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) { waitingToLandBed(file); retier(file, 1) })
	b.lineage = "m1"
	verdict := holderStopOver(t, b.intentBed, b.deliveryOwners(), holderDue, busyScan)
	if len(owners.joins) != 1 || owners.joins[0].GoalID != bedGoal {
		t.Fatalf("the due goal was not admitted on a busy Stop: %+v\n%s", owners.joins, verdict.Display)
	}
	if verdict.ShouldBlock || !strings.Contains(verdict.Display, "LANDED "+bedGoal+": joined batch b-1") || !strings.Contains(verdict.Display, "STILL WORKING: a delegate job runs") {
		t.Fatalf("the busy Stop blocks or does not say both: %+v", verdict)
	}
}

func TestASendBacksRevisionIsStartedOnceAtStopsWithOpenWork(t *testing.T) {
	t.Parallel()
	bed := sentBackBed(t)
	var calls []reviseCall
	owners := holderOwners(bed, &calls, func(int) intentResult { return attemptStarted(3) })
	first := holderStopOver(t, bed, owners, holderDue, openWorkScan)
	if len(calls) != 1 || !strings.Contains(first.Display, "REVISION STARTED "+bedGoal) || !strings.Contains(first.Display, "OPEN WORK (1)") {
		t.Fatalf("the revision was not started on a Stop with open work: calls=%d\n%s", len(calls), first.Display)
	}
	second := holderStopOver(t, bed, owners, holderDue.Add(time.Minute), openWorkScan)
	if len(calls) != 1 || strings.Contains(second.Display, "REVISION") {
		t.Fatalf("the second Stop started the revision again: calls=%d\n%s", len(calls), second.Display)
	}
}
