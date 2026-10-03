package main

// The landing gate at every form of work land (g1-s70 D2, D4, §8): the bed's
// claimed goal is tier 3, so with the default settings it waits for a person;
// the production gate reads the bed's ledger.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

const gateBedTip = "2222222222222222222222222222222222222222"

// gatedDeliveryBed is a delivery bed whose landing gate is the production one
// over the bed's own ledger, with the goal's history amended.
func gatedDeliveryBed(t *testing.T, amend func(*goal.GoalFile)) (*deliveryBed, *landingOwners, *[]string) {
	t.Helper()
	b := newDeliveryBedWith(t, amend)
	owners := &landingOwners{status: readBranch(2, "critic-root", "critic-root")}
	owners.install(b)
	b.owners.branchTip = func(string, string) (string, error) { return gateBedTip, nil }
	recorded := &[]string{}
	b.owners.recordLanded = func(_ *intentInvocation, id, _ string) error { *recorded = append(*recorded, id); return nil }
	return b, owners, recorded
}

// retier moves the bed's goal to another tier with its approval rebound.
func retier(file *goal.GoalFile, tier uint8) {
	file.Tier = tier
	file.Approved.Digest = goal.ApprovalDigest(file.Intent, tier, *file.Budget, file.Risk)
}

func humanWord(file *goal.GoalFile, ulid, verb, reason string) {
	file.History = append(file.History, goal.HistoryLine{At: "2026-09-01T10:00:00Z", Opid: goal.Opid(ulid, "mac-ui", "m9"),
		Verb: verb, Actor: "human:Wido", Targets: []string{file.Id}, Keep: -1, Reason: reason})
	file.Revision++
}

func clearedAt(tip string) func(*goal.GoalFile) {
	return func(file *goal.GoalFile) {
		waitingToLandBed(file)
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW1", "review", "reviewed verdict=clear-to-land tip="+tip+" record="+reviewBedRecord+" by=Wido")
	}
}

func expectGateRefusal(t *testing.T, label string, code int, result intentResult, want string) {
	t.Helper()
	expectOutcome(t, label, code, result, intentRefused)
	if !strings.Contains(result.Summary+"\n"+strings.Join(result.Details, "\n"), want) {
		t.Fatalf("%s: the refusal does not say %q: %+v", label, want, result)
	}
}

func TestWorkLandMeetsTheGateAtEveryForm(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, waitingToLandBed)
	code, result := b.do("work", "land", bedGoal)
	expectGateRefusal(t, "work land G", code, result, "waits for a person")
	if len(owners.pushes) != 0 || owners.candidates != 0 || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem goal land-without-sitting "+bedGoal+" --reason TEXT" {
		t.Fatalf("the refusal pushed or proved something, or names no human verb: %+v", result)
	}

	owners.status = readBranch(2, "reader-record", "reader-record")
	code, result = b.do("work", "land", bedGoal)
	expectGateRefusal(t, "work land G by hand", code, result, "waits for a person")
	if owners.candidates != 0 || len(owners.pushes) != 0 {
		t.Fatalf("the hand route proved or pushed past the gate: %+v", owners)
	}

	message := filepath.Join(b.root(), "message.txt")
	if err := os.WriteFile(message, []byte("fix: one line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, result = b.do("work", "land", bedGoal, "--message", message, "--staged")
	expectGateRefusal(t, "work land G --message", code, result, "waits for a person")

	code, result = b.do("work", "land", bedGoal, "--exception", "goal-item-not-held", "--reason", "carry it", "--by", "Wido")
	expectGateRefusal(t, "work land G --exception", code, result, "waits for a person")
}

func TestWorkLandProceedsWithTheWordAtTheTipAndWritesTheLandedLineOnlyOnceLanded(t *testing.T) {
	t.Parallel()
	b, owners, recorded := gitlessDeliveryBed(t, clearedAt(gateBedTip)), &landingOwners{status: readBranch(2, "critic-root", "critic-root")}, &[]string{}
	owners.install(b)
	gitNeverCalled(t)
	b.owners.recordLanded = func(_ *intentInvocation, id, under string) error {
		*recorded = append(*recorded, id+"@"+under)
		return nil
	}
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "cleared at the tip", code, result, intentConfirmed)
	if len(owners.pushes) != 1 || len(*recorded) != 1 || !strings.HasPrefix((*recorded)[0], bedGoal+"@reviewed verdict=clear-to-land by=Wido tip=222222222222") {
		t.Fatalf("the cleared goal did not land, or its landed line was not written: %+v %v", owners.pushes, *recorded)
	}

	moved, movedOwners := gitlessDeliveryBed(t, clearedAt(strings.Repeat("3", 40))), &landingOwners{status: readBranch(2, "critic-root", "critic-root")}
	movedOwners.install(moved)
	code, result = moved.do("work", "land", bedGoal)
	expectGateRefusal(t, "a moved tip", code, result, "a moved tip needs the word again")
	if len(movedOwners.pushes) != 0 {
		t.Fatalf("a word at another tip landed: %+v", movedOwners.pushes)
	}
}

// The landed line names what the gate admitted the publication under: a
// send-back recorded at the same tip after the gate let a clear-to-land
// through, before the line is written, does not change it.
func TestTheLandedLineNamesTheWordTheGateAdmitted(t *testing.T) {
	t.Parallel()
	b, owners := gitlessDeliveryBed(t, clearedAt(gateBedTip)), &landingOwners{status: readBranch(2, "critic-root", "critic-root")}
	owners.install(b)
	gitNeverCalled(t)
	b.lineage = "m1"
	push := b.owners.landPush
	b.owners.landPush = func(args []string) (branch.PreparedLanding, string, int, error) {
		file := b.goalFile(bedGoal)
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW2", "review", "reviewed verdict=send-back tip="+gateBedTip+" record="+reviewBedRecord+" by=Wido brief="+goal.BriefPathFor(reviewBedRecord))
		b.addGoal(file)
		return push(args)
	}
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "landed", code, result, intentConfirmed)
	history := b.goalFile(bedGoal).History
	if last := history[len(history)-1]; last.Verb != goal.LandedVerb || !strings.HasPrefix(last.Reason, "landed under reviewed verdict=clear-to-land by=Wido tip=222222222222") {
		t.Fatalf("the landed line does not name the clear the gate admitted: %+v %+v", last, result)
	}
}

// A landing record that does not carry what the gate admitted it under, from
// a landing that stopped before its landed line was written, swept or not:
// the repeat words the line from the ledger at the tip that landed, not at a
// newer word's tip.
func TestAnOlderLandingRecordStillWritesItsLandedLine(t *testing.T) {
	t.Parallel()
	for _, swept := range []bool{true, false} {
		b := gitlessDeliveryBed(t, func(file *goal.GoalFile) {
			clearedAt(gateBedTip)(file)
			humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW2", "review", "reviewed verdict=clear-to-land tip="+strings.Repeat("3", 40)+" record="+reviewBedRecord+" by=Wido")
		})
		owners := &landingOwners{status: readBranch(2, "critic-root", "critic-root"), branchDeleted: swept}
		owners.install(b)
		gitNeverCalled(t)
		b.lineage = "m1"
		dir := filepath.Join(b.install, "artifacts", "agents", "landing-intent", bedGoal, shortCommit(gateBedTip)+"-"+shortCommit(owners.status.EndpointTip))
		b.writeFile(filepath.Join(dir, "landed.json"), fmt.Sprintf(`{"Landing":"land1","Endpoint":"main","Branch":"goal/%s","Subject":"%s","Swept":%t}`, bedGoal, gateBedTip, swept))
		code, result := b.do("work", "land", bedGoal)
		history := b.goalFile(bedGoal).History
		if last := history[len(history)-1]; code != 0 || last.Verb != goal.LandedVerb || !strings.HasPrefix(last.Reason, "landed under reviewed verdict=clear-to-land by=Wido tip=222222222222") {
			t.Fatalf("swept %t: the repeat did not write the landed line from the ledger at the landed tip: %d %+v %+v", swept, code, last, result)
		}
	}
}

func TestWorkLandIsRefusedUnderAHoldAtEveryTier(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) {
		clearedAt(gateBedTip)(file)
		retier(file, 1)
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW2", "review", goal.SittingReason(true, reviewBedRecord, "Wido"))
	})
	code, result := b.do("work", "land", bedGoal)
	expectGateRefusal(t, "held", code, result, "held by Wido's review sitting")
	if len(owners.pushes) != 0 {
		t.Fatalf("a held goal landed: %+v", owners.pushes)
	}
}

// The hand route reads the gate again right before its push: a hold that
// arrived while the proof ran stops the publication.
func TestTheHandRouteReadsTheGateAgainBeforeItsPush(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	owners := &landingOwners{status: readBranch(2, "reader-record", "reader-record")}
	owners.install(b)
	calls := 0
	b.owners.landingGate = func(*intentInvocation, string, string) (string, error) {
		calls++
		if calls > 1 {
			return "", &goal.GateRefusal{Code: goal.GateHeldBySitting, Reason: "goal standing-validation is held by Wido's review sitting"}
		}
		return "below the tier", nil
	}
	code, result := b.do("work", "land", bedGoal)
	expectGateRefusal(t, "held before the push", code, result, "held by Wido's review sitting")
	if len(owners.preps) != 1 || len(owners.pushes) != 0 {
		t.Fatalf("the hold did not stop the push: preps=%d pushes=%d", len(owners.preps), len(owners.pushes))
	}
}

func TestQueueOnlyStillEntersReviewPastTheGate(t *testing.T) {
	t.Parallel()
	b, _, _ := gatedDeliveryBed(t, nil)
	_, result := b.do("work", "land", bedGoal, "--queue-only")
	if strings.Contains(result.Summary, "waits for a person") || strings.Contains(result.Summary, "review sitting") {
		t.Fatalf("--queue-only met the landing gate: %+v", result)
	}
}

// belowTheGate stubs the landing gate and the landed line on a bed that
// proves a landing's mechanics rather than the gate: its fixture goal is tier
// 3, which with the default settings waits for a person.
func belowTheGate(delivery *intentDeliveryOwners) *intentDeliveryOwners {
	delivery.landingGate = func(*intentInvocation, string, string) (string, error) { return "the bed's landing", nil }
	delivery.recordLanded = func(*intentInvocation, string, string) error { return nil }
	return delivery
}

// The landing path reads the gate immediately before each push of the staged
// --message form and the exceptional forms (SOL-S70-01): the gate it is handed
// is the invocation's own, at the goal branch's tip against the fresh ledger,
// so a hold recorded after admission refuses, its release lets the retry pass,
// and a word at another tip refuses naming both commits.
func TestThePushGateReadsTheFreshLedgerBeforeEachPush(t *testing.T) {
	t.Parallel()
	hold := func(file *goal.GoalFile) {
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW2", "review", goal.SittingReason(true, reviewBedRecord, "Wido"))
	}
	release := func(file *goal.GoalFile) {
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW3", "review", goal.SittingReason(false, reviewBedRecord, "Wido"))
	}
	gateOf := func(amend func(*goal.GoalFile)) error {
		b, _, _ := gatedDeliveryBed(t, amend)
		inv := &intentInvocation{owners: b.deliveryOwners(), layout: stateroot.Layout{InstallationRoot: b.install}, stateRoot: b.root()}
		return inv.pushGate()(b.install, bedGoal)
	}

	held := gateOf(func(file *goal.GoalFile) { clearedAt(gateBedTip)(file); hold(file) })
	if held == nil || !strings.Contains(held.Error(), goal.GateHeldBySitting) || !strings.Contains(held.Error(), "metasystem goal review "+bedGoal+" --release") {
		t.Fatalf("a hold recorded after admission did not refuse with its code and the human verb: %v", held)
	}
	if released := gateOf(func(file *goal.GoalFile) { clearedAt(gateBedTip)(file); hold(file); release(file) }); released != nil {
		t.Fatalf("a hold released before the retry still refuses: %v", released)
	}
	moved := gateOf(clearedAt(strings.Repeat("3", 40)))
	if moved == nil || !strings.Contains(moved.Error(), goal.GateWaitsForHuman) || !strings.Contains(moved.Error(), "given at 333333333333 and the branch is now at 222222222222") ||
		!strings.Contains(moved.Error(), "metasystem goal land-without-sitting "+bedGoal+" --reason TEXT") {
		t.Fatalf("a word at another tip did not refuse naming both commits and the human verb: %v", moved)
	}
}

// wordsThenMark gives the person's words on the bed's goal, then the mark a
// hand-in writes after them.
func wordsThenMark(reasons ...string) func(*goal.GoalFile) {
	return func(file *goal.GoalFile) {
		for index, reason := range reasons {
			humanWord(file, fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5FW%d", index+1), "review", reason)
		}
		waitingToLandBed(file)
	}
}

// At the tier, a person's word at the branch tip, then a --through hand-in
// that marks the goal: the hand-in of the tip still passes the gate. A word at
// another tip still refuses, and so does a send-back at this tip.
func TestAThroughHandInKeepsThePersonsWordForTheTip(t *testing.T) {
	t.Parallel()
	clearTo := func(tip string) string {
		return "reviewed verdict=clear-to-land tip=" + tip + " record=" + reviewBedRecord + " by=Wido"
	}
	b, owners, install := gitlessLaneBed(t, func(file *goal.GoalFile) {
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW1", "review", clearTo(gateBedTip))
	}, "critic-root", "critic-root")
	b.owners.landMark = nil
	prefix := owners.status.Status.Units[0].Commit
	code, result := b.do("work", "land", bedGoal, "--through", prefix)
	expectOutcome(t, "the --through hand-in", code, result, intentConfirmed)
	if b.goalFile(bedGoal).Landing == nil {
		t.Fatalf("the --through hand-in did not mark the goal: %+v", result)
	}
	code, result = b.do("work", "land", bedGoal)
	expectOutcome(t, "the tip's hand-in", code, result, intentConfirmed)
	if latest, _, _ := plain.Latest(install, bedGoal); latest.SHA != gateBedTip {
		t.Fatalf("the tip was not handed in: %+v %+v", latest, result)
	}

	sendBack := "reviewed verdict=send-back tip=" + gateBedTip + " record=" + reviewBedRecord + " by=Wido brief=" + goal.BriefPathFor(reviewBedRecord)
	for _, row := range []struct {
		label, want string
		amend       func(*goal.GoalFile)
	}{
		{"a word at another tip", "a moved tip needs the word again", wordsThenMark(clearTo(strings.Repeat("3", 40)))},
		{"a send-back at this tip", "send-back", wordsThenMark(clearTo(gateBedTip), sendBack)},
	} {
		b, _, install := gitlessLaneBed(t, row.amend, "critic-root", "critic-root")
		code, result := b.do("work", "land", bedGoal)
		expectGateRefusal(t, row.label, code, result, row.want)
		if entries, _ := plain.Entries(install); len(entries) != 0 {
			t.Fatalf("%s: handed in past the gate: %+v", row.label, entries)
		}
	}
}
