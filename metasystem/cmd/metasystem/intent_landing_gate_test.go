package main

// The landing gate at every form of work land (g1-s70 D2, D4, §8): the bed's
// claimed goal is tier 3, so with the default settings it waits for a person;
// the production gate reads the bed's ledger.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

const gateBedTip = "2222222222222222222222222222222222222222"

// gatedDeliveryBed is a delivery bed whose landing gate is the production one
// over the bed's own ledger, with the goal's history amended.
func gatedDeliveryBed(t *testing.T, amend func(*goal.GoalFile)) (*deliveryBed, *landingOwners, *[]string) {
	t.Helper()
	b := newDeliveryBedWith(t, amend)
	owners := &landingOwners{configured: true, status: readBranch(2, "critic-root", "critic-root")}
	owners.install(b)
	b.owners.branchTip = func(string, string) (string, error) { return gateBedTip, nil }
	recorded := &[]string{}
	b.owners.recordLanded = func(_ *intentInvocation, id string) error { *recorded = append(*recorded, id); return nil }
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
	if !strings.Contains(result.Summary, want) {
		t.Fatalf("%s: the refusal does not say %q: %+v", label, want, result)
	}
}

func TestWorkLandMeetsTheGateAtEveryForm(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, waitingToLandBed)
	code, result := b.do("work", "land", bedGoal)
	expectGateRefusal(t, "work land G", code, result, "waits for a person")
	if len(owners.joins) != 0 || owners.candidates != 0 || result.Decision != "metasystem goal land-without-sitting "+bedGoal+" --reason TEXT" {
		t.Fatalf("the refusal joined or proved something, or names no human verb: %+v", result)
	}

	owners.status = readBranch(2, "reader-record", "reader-record")
	code, result = b.do("work", "land", bedGoal)
	expectGateRefusal(t, "work land G by hand", code, result, "waits for a person")
	if owners.candidates != 0 || len(owners.pushes) != 0 {
		t.Fatalf("the hand route proved or pushed past the gate: %+v", owners)
	}

	b.writeJob(map[string]any{"jobId": "impl1", "role": "implementer", "status": "completed", "goalId": bedGoal})
	code, result = b.do("work", "land", "j2:impl1")
	expectGateRefusal(t, "work land j2:J", code, result, "waits for a person")
	if len(owners.joins) != 0 {
		t.Fatalf("a certified chain of a tier-3 goal with no Landing word joined: %+v", owners.joins)
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

// A tier-2 claimed goal with a certified chain and no Landing record, landed
// directly with work land j2:J, refuses without the human's word (S70-06).
func TestAChainOfAClaimedGoalWithNoLandingRecordMeetsTheGate(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) { retier(file, 2) })
	b.writeJob(map[string]any{"jobId": "impl2", "role": "implementer", "status": "completed", "goalId": bedGoal})
	code, result := b.do("work", "land", "j2:impl2")
	expectGateRefusal(t, "tier-2 chain", code, result, "tier 2, at or above landing.review.human-from-tier=2")
	if len(owners.joins) != 0 {
		t.Fatalf("the chain joined: %+v", owners.joins)
	}
}

// chainCandidate is a delegate's worktree whose branch holds one commit: the
// head a certified chain publishes, as its job record names it (workspaceRoot
// and branch).
func chainCandidate(t *testing.T) (workspace, head string) {
	t.Helper()
	workspace = t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "delegate-impl"},
		{"-c", "user.name=Bed", "-c", "user.email=bed@example.invalid", "commit", "-q", "--allow-empty", "-m", "the certified change"}} {
		if output, err := exec.Command("git", append([]string{"-C", workspace}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	output, err := exec.Command("git", "-C", workspace, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return workspace, strings.TrimSpace(string(output))
}

// The human's word on a certified chain is bound to the commit the chain
// publishes, the head of its candidate branch (SOL-S70-03): at a tier-2 goal a
// word at that head lets work land j2:J join, and the batch carries the head
// its publication gate reads; a word at another commit refuses naming both;
// below the tier a chain lands as before, with no word.
func TestAChainLandsWithTheWordAtItsHead(t *testing.T) {
	t.Parallel()
	workspace, head := chainCandidate(t)
	job := map[string]any{"jobId": "impl3", "role": "implementer", "status": "completed", "goalId": bedGoal,
		"workspaceRoot": workspace, "branch": "delegate-impl"}

	cleared, clearedOwners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) { clearedAt(head)(file); retier(file, 2) })
	cleared.writeJob(job)
	code, result := cleared.do("work", "land", "j2:impl3")
	expectOutcome(t, "a chain cleared at its head", code, result, intentInProgress)
	if len(clearedOwners.joins) != 1 || clearedOwners.joins[0].ChainID != "impl3" || clearedOwners.joins[0].ChainHead != head {
		t.Fatalf("the chain cleared at its head did not join carrying its head: %+v", clearedOwners.joins)
	}

	elsewhere, elsewhereOwners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) { clearedAt(gateBedTip)(file); retier(file, 2) })
	elsewhere.writeJob(job)
	code, result = elsewhere.do("work", "land", "j2:impl3")
	expectGateRefusal(t, "a word at another commit", code, result,
		"Wido's word was given at "+gateBedTip[:12]+" and the branch is now at "+head[:12]+"; a moved tip needs the word again")
	if len(elsewhereOwners.joins) != 0 {
		t.Fatalf("a chain joined on a word at another commit: %+v", elsewhereOwners.joins)
	}

	below, belowOwners, _ := gatedDeliveryBed(t, func(file *goal.GoalFile) { retier(file, 1) })
	below.writeJob(job)
	code, result = below.do("work", "land", "j2:impl3")
	expectOutcome(t, "a chain below the tier", code, result, intentInProgress)
	if len(belowOwners.joins) != 1 {
		t.Fatalf("a chain below the tier did not land as before: %+v", belowOwners.joins)
	}
}

func TestWorkLandProceedsWithTheWordAtTheTipAndWritesTheLandedLineOnlyOnceLanded(t *testing.T) {
	t.Parallel()
	b, owners, recorded := gatedDeliveryBed(t, clearedAt(gateBedTip))
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "cleared at the tip", code, result, intentInProgress)
	if len(owners.joins) != 1 || len(*recorded) != 0 {
		t.Fatalf("the cleared goal did not join, or its landing was recorded before publication: %+v %v", owners.joins, *recorded)
	}
	owners.member.State = batch.UnitLanded
	code, result = b.do("work", "land", bedGoal)
	expectOutcome(t, "landed", code, result, intentUnchanged)
	if len(*recorded) != 1 || (*recorded)[0] != bedGoal {
		t.Fatalf("the confirmed landing wrote no landed line: %v", *recorded)
	}

	moved, movedOwners, _ := gatedDeliveryBed(t, clearedAt(strings.Repeat("3", 40)))
	code, result = moved.do("work", "land", bedGoal)
	expectGateRefusal(t, "a moved tip", code, result, "a moved tip needs the word again")
	if len(movedOwners.joins) != 0 {
		t.Fatalf("a word at another tip joined: %+v", movedOwners.joins)
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
	if len(owners.joins) != 0 {
		t.Fatalf("a held goal joined: %+v", owners.joins)
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
	delivery.recordLanded = func(*intentInvocation, string) error { return nil }
	return delivery
}

// The landing path reads the gate immediately before each push of the staged
// --message form and the exceptional forms (SOL-S70-01): production wires it,
// and it reads the fresh ledger at the goal branch's tip, so a hold recorded
// after admission refuses, its release lets the retry pass, and a word at
// another tip refuses naming both commits.
func TestTheLandingPathsGateReadsTheFreshLedgerBeforeEachPush(t *testing.T) {
	t.Parallel()
	if landingPathOwners().LandingGate == nil {
		t.Fatal("the production landing path reads no gate before its pushes")
	}
	hold := func(file *goal.GoalFile) {
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW2", "review", goal.SittingReason(true, reviewBedRecord, "Wido"))
	}
	release := func(file *goal.GoalFile) {
		humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW3", "review", goal.SittingReason(false, reviewBedRecord, "Wido"))
	}
	gateOf := func(amend func(*goal.GoalFile)) error {
		b := newDeliveryBedWith(t, amend)
		owners := b.intentBed.owners()
		return landingPathGateAt(b.root(), b.install, bedGoal, owners.dependencies, owners.commandNow,
			func(string, string) (string, error) { return gateBedTip, nil })
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
