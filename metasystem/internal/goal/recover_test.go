package goal

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type fixtureParkRecoveryPolicy struct {
	check func(string, string) (string, error)
}

func (p fixtureParkRecoveryPolicy) BreachStop(Endpoint, Entry) (PublishRequest, func(), error) {
	return PublishRequest{}, nil, errors.New("breach-stop is outside this fixture")
}

func (p fixtureParkRecoveryPolicy) ParkBranchCheck(Endpoint) func(string, string) (string, error) {
	return p.check
}

// strandEntry journals an operation as a dead foreign owner left it.
func strandEntry(t *testing.T, root string, opid string, phase Phase, intent Intent) {
	t.Helper()
	strandEntryAt(t, root, opid, "mac-a", phase, intent)
}

// strandEntryAt strands an entry under an explicit machine — the
// cross-clone recovery legs need the entry's identity to be the
// OTHER clone's.
func strandEntryAt(t *testing.T, root, opid, machine string, phase Phase, intent Intent) {
	t.Helper()
	if intent.Verb == "claim" || intent.Verb == "open-claim" || intent.Verb == "steal" {
		if intent.Args == nil {
			intent.Args = map[string]string{}
		}
		if intent.Args["claimEpoch"] == "" {
			intent.Args["claimEpoch"] = "1"
		}
	}
	if _, err := CreateEntry(root, opid, machine, "lin-1", intent); err != nil {
		t.Fatal(err)
	}
	if phase == PhasePushed {
		if err := MarkPushed(root, opid, "sometip", 1, time.Now().Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	cmd := spawnForeignOwner(t, root, opid)
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func TestRecoveryCompletesADeadOwnersOpen(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// A dead owner's CREATED open: never pushed, fully rebuildable.
	opid := Opid("01J5X00000000000000000Q000", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{
		Verb: "open", Targets: []string{"orphaned"},
		Args: map[string]string{"intent": "The dead owner's work.", "origin": "main", "next": "Continue.", "labels": "custody,recovery"},
	})
	// The pushed-block gate: a CREATED stranded entry does not block,
	// but recovery still completes it (dead owner + created =
	// complete, R10-M04).
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rep := range reports {
		if rep.Opid == opid {
			found = true
			if rep.Action != ActionComplete || !strings.Contains(rep.Detail, "confirmed") {
				t.Fatalf("the dead owner's open completes: %+v", rep)
			}
		}
	}
	if !found {
		t.Fatalf("recovery visits the stranded entry: %+v", reports)
	}
	// The work LANDED under the ORIGINAL opid.
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	orphaned := p.Tree.Live["orphaned"]
	if orphaned == nil || orphaned.Intent != "The dead owner's work." || strings.Join(orphaned.Labels, ",") != "custody,recovery" ||
		orphaned.Tier != 3 || orphaned.Budget == nil || orphaned.Budget.ReviewRoundLimit != 3 {
		t.Fatal("the recovered open is on the canonical branch")
	}
	if orphaned.History[0].Opid != opid {
		t.Fatalf("the recovered history carries the ORIGINAL opid: %s", orphaned.History[0].Opid)
	}
	// The entry is terminal confirmed.
	entry, err := ReadEntry(a, opid)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("the recovered entry confirms: %+v %v", entry, err)
	}
}

func TestRecoveryRebuildsAnswer(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X0000000000000000000K0", "mac-a"), "answer-recovery", "Recover an answer.", "main", "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	opid := Opid("01J5X0000000000000000000K1", "mac-a", "lin-1")
	intent := Intent{Verb: "answer", Targets: []string{"answer-recovery"}, Args: map[string]string{
		"question": "question-1", "text": "approved", "wants": "goal=answer-recovery resume elapsed=4h attempts=4 minutes=240 active=2",
		"provider": "slack", "user": "UWIDO", "ref": "1/2", "step": "42",
	}}
	strandEntry(t, root, opid, PhaseCreated, intent)
	entry, err := ReadEntry(root, opid)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := requestForEntry(endpointFor(root), entry)
	if err != nil || rebuilt.Opid != opid || !reflect.DeepEqual(rebuilt.Intent, intent) {
		t.Fatalf("rebuilt answer = %+v, want opid=%s intent=%+v, err=%v", rebuilt, opid, intent, err)
	}
	if _, err := Recover(endpointFor(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Recover(endpointFor(root)); err != nil {
		t.Fatal("repeated recovery failed:", err)
	}
	projection, err := Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live["answer-recovery"]
	answers := 0
	for _, history := range file.History {
		if history.Verb == "answer" {
			answers++
			if history.Opid != opid || history.Reason != "approved "+intent.Args["wants"] {
				t.Fatalf("recovered answer changed identity or reason: %+v", history)
			}
		}
	}
	if answers != 1 || strings.Count(file.NextStep, "ANSWERED question-1: approved") != 1 {
		t.Fatalf("recovery did not land once: answers=%d next=%q", answers, file.NextStep)
	}
	if err := ValidateCommit(root, projection.Tip); err != nil {
		t.Fatalf("recovered ledger does not parse: %v", err)
	}
}

func TestRecoveryRefusesOpenClaimWithoutHumanApproval(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	opid := Opid("01J5X00000000000000000Q005", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{
		Verb: "open-claim", Targets: []string{"recovered-claim"},
		Args: mergeIntentArgs(map[string]string{
			"intent": "The dead owner's claimed work.", "origin": "main",
			"next": "Continue.", "labels": "custody,recovery",
		}, budgetIntentArgs(testBudget())),
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if f := p.Tree.Live["recovered-claim"]; f != nil {
		t.Fatalf("recovery must not manufacture approval through open-claim: %+v", f)
	}
	entry, err := ReadEntry(a, opid)
	if err != nil || entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "APPROVAL_REQUIRED") {
		t.Fatalf("recovery did not close open-claim with the approval remedy: %+v %v", entry, err)
	}
}

func TestRecoveryUnblocksAStrandedPush(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// A dead owner's PUSHED claim on a goal that exists: the block
	// clears through recovery, not through waiting.
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q010", "mac-a"), "claimable", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000Q020", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhasePushed, Intent{
		Verb: "claim", Targets: []string{"claimable"}, Args: budgetIntentArgs(testBudget()),
	})

	// The stranded push blocks ordinary mutations…
	_, err := Open(verbReq(a, "01J5X00000000000000000Q030", "mac-a"), "blocked-out", "Waits.", "main", "Go.")
	if err == nil || !strings.Contains(err.Error(), opid) {
		t.Fatalf("a pushed entry blocks by name: %v", err)
	}
	// …until recovery completes the dead owner's claim.
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	var clear bool
	for _, rep := range reports {
		if rep.Opid == opid && rep.Action == ActionComplete {
			clear = true
		}
	}
	if !clear {
		t.Fatalf("recovery completes the stranded push: %+v", reports)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	claimed := p.Tree.Live["claimable"]
	if claimed == nil || claimed.State != StateQueued || claimed.Claimed != nil {
		t.Fatalf("the recovered claim crossed the approval gate: %+v", claimed)
	}
	entry, err := ReadEntry(a, opid)
	if err != nil || entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "APPROVAL_REQUIRED") {
		t.Fatalf("the refused recovery did not terminalize with its remedy: %+v %v", entry, err)
	}
	// The clone mutates again.
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q040", "mac-a"), "unblocked", "Flows.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the clone mutates after recovery: %+v %v", res, err)
	}
}

func TestRecoveryClosesUnrebuildableVerbsByName(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	opid := Opid("01J5X00000000000000000Q050", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{Verb: "reconcile", Targets: []string{"x"},
		Args: map[string]string{"rows": "1"}})
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	for _, rep := range reports {
		if rep.Opid == opid {
			if !strings.Contains(rep.Detail, "not rebuildable") {
				t.Fatalf("the unrebuildable verb closes by name: %+v", rep)
			}
		}
	}
	entry, err := ReadEntry(a, opid)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeRejected {
		t.Fatalf("the entry closes rejected, never wedges: %+v %v", entry, err)
	}
}

func TestRecoveryRefusesToReplayAbandonEngineFloorAndAbandonedReopen(t *testing.T) {
	endpoint := endpointFor(t.TempDir())
	for index, test := range []struct {
		verb string
		args map[string]string
	}{
		{verb: "abandon", args: map[string]string{}},
		{verb: "engine-floor", args: map[string]string{}},
		{verb: "carry", args: map[string]string{"to": "successor"}},
		{verb: "reopen", args: map[string]string{"from": "abandoned"}},
	} {
		t.Run(test.verb, func(t *testing.T) {
			ulid := fmt.Sprintf("01J5X00000000000000001Q0%d0", index)
			entry := Entry{
				Opid: Opid(ulid, "mac-a", "lin-1"), Machine: "mac-a", Lineage: "lin-1",
				Intent: Intent{Verb: test.verb, Targets: []string{"target"}, Args: test.args},
			}
			_, err := requestForEntry(endpoint, entry)
			want := test.verb + " is proof-bearing and cannot be replayed from journal text; re-run it from the enrolled terminal"
			if err == nil || err.Error() != want {
				t.Fatalf("recovery refusal = %v, want %q", err, want)
			}
		})
	}

	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000001Q100", "mac-a"), "normal-reopen", "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	if result, err := Done(verbReq(root, "01J5X00000000000000001Q110", "mac-a"), "normal-reopen", "done"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", result, err)
	}
	reopen := verbReq(root, "01J5X00000000000000001Q120", "mac-a")
	result, err := Reopen(reopen, "normal-reopen")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("ordinary reopen: %+v %v", result, err)
	}
	entry, err := ReadEntry(root, reopen.opid())
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := requestForEntry(endpointFor(root), entry)
	if err != nil {
		t.Fatalf("ordinary reopen did not rebuild: %v", err)
	}
	_, err = rebuilt.Mutate(result.Tip)
	var already AlreadyApplied
	if !errors.As(err, &already) {
		t.Fatalf("landed ordinary reopen was not confirmed by its operation id: %v", err)
	}
}

func TestRecoveryRebuildsParkAndEdit(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q100", "mac-a"), "target", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// A dead owner's park of a main-origin, never-claimed goal needs no human
	// proof, so recovery carries its reason through the real verb.
	parkOpid := Opid("01J5X00000000000000000Q110", "mac-a", "lin-1")
	strandEntry(t, a, parkOpid, PhaseCreated, Intent{
		Verb: "park", Targets: []string{"target"},
		Args: map[string]string{"because": "recovered pause"},
	})
	if _, err := RecoverWithPolicy(endpointFor(a), fixtureParkRecoveryPolicy{check: func(string, string) (string, error) { return "", nil }}); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	parked := p.Tree.Live["target"]
	if parked.State != StateParked || parked.Parked.Because != "recovered pause" {
		t.Fatalf("the recovered park carries its reason: %+v", parked.Parked)
	}

	// The machine-authored park needs no human proof to lift, so its stranded
	// unpark also replays before the ordinary edit.
	unparkOpid := Opid("01J5X00000000000000000Q120", "mac-a", "lin-1")
	strandEntry(t, a, unparkOpid, PhaseCreated, Intent{Verb: "unpark", Targets: []string{"target"}})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	editOpid := Opid("01J5X00000000000000000Q130", "mac-a", "lin-1")
	strandEntry(t, a, editOpid, PhaseCreated, Intent{
		Verb: "edit", Targets: []string{"target"},
		Deltas: []FieldDelta{
			{Target: "target", Field: "next", New: "Recovered next step."},
			{Target: "target", Field: "blockedBy", New: ""},
		},
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err = Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	edited := p.Tree.Live["target"]
	if edited.State != StateQueued || edited.NextStep != "Recovered next step." {
		t.Fatalf("the recovered unpark and edit landed in order: %+v", edited)
	}
	carries := func(f *GoalFile, opid string) bool {
		for _, h := range f.History {
			if h.Opid == opid {
				return true
			}
		}
		return false
	}
	if !carries(edited, parkOpid) || !carries(edited, unparkOpid) || !carries(edited, editOpid) {
		t.Fatal("each replayable recovery carries its original opid")
	}
}

func claimedParkRecoveryFixture(t *testing.T, id string) (string, string) {
	t.Helper()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000PR00", "mac-a"), id, "Branch-backed work.", OriginMain, "Build u1."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	if result, err := claimApprovedForTest(t, verbReq(root, "01J5X00000000000000000PR01", "mac-a"), id, testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	opid := Opid("01J5X00000000000000000PR02", "mac-a", "lin-1")
	strandEntry(t, root, opid, PhaseCreated, Intent{Verb: "park", Targets: []string{id}, Args: map[string]string{"because": "recover handoff"}})
	return root, opid
}

func TestRecoveryRefusesParkWhenGoalBranchIsUnpushed(t *testing.T) {
	root, opid := claimedParkRecoveryFixture(t, "unpushed-park")
	reports, err := RecoverWithPolicy(endpointFor(root), fixtureParkRecoveryPolicy{check: func(goalID, next string) (string, error) {
		if goalID != "unpushed-park" || next != "Build u1." {
			t.Fatalf("branch check goal=%q next=%q", goalID, next)
		}
		return "", errors.New("GOAL_PARK_UNPUSHED: local goal branch is ahead of origin")
	}})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ReadEntry(root, opid)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "GOAL_PARK_UNPUSHED") {
		t.Fatalf("entry=%+v reports=%+v err=%v", entry, reports, err)
	}
	projection, err := Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live["unpushed-park"]
	if file == nil || file.State != StateClaimed || file.Claimed == nil || file.NextStep != "Build u1." {
		t.Fatalf("refused recovery changed goal: %+v", file)
	}
}

func TestRecoveryCompletesParkWhenGoalBranchIsPushed(t *testing.T) {
	root, opid := claimedParkRecoveryFixture(t, "pushed-park")
	summary := "goal/pushed-park last unit u1 commit " + strings.Repeat("a", 40) + " is read clean"
	if _, err := RecoverWithPolicy(endpointFor(root), fixtureParkRecoveryPolicy{check: func(goalID, next string) (string, error) {
		return summary, nil
	}}); err != nil {
		t.Fatal(err)
	}
	entry, err := ReadEntry(root, opid)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("entry=%+v err=%v", entry, err)
	}
	projection, err := Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live["pushed-park"]
	if file == nil || file.State != StateParked || file.Claimed != nil || file.NextStep != summary {
		t.Fatalf("recovered pushed park: %+v", file)
	}
}

func TestParkFailsClosedWithoutBranchChecker(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000PN00", "mac-a"), "nil-check", "Work.", OriginMain, "Build."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	request := verbReq(root, "01J5X00000000000000000PN01", "mac-a")
	request.ParkBranchCheck = nil
	result, err := Park(request, "nil-check", "handoff")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "run goal park again") {
		t.Fatalf("nil checker park: %+v %v", result, err)
	}
}

func TestRecoveryReplaysOwnReleaseAndRefusesHumanRequiredStoppingActs(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)

	// A dead pair's own release remains machine-authorized and heals the claim.
	if res, err := Open(verbReq(root, "01J5X00000000000000000Q140", "mac-a"), "own-release", "Release after a crash.", OriginMain, "Continue."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open own release: %+v %v", res, err)
	}
	claimReq := verbReq(root, "01J5X00000000000000000Q141", "mac-a")
	if res, err := claimApprovedForTest(t, claimReq, "own-release", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim own release: %+v %v", res, err)
	}
	releaseOpid := Opid("01J5X00000000000000000Q142", "mac-a", "lin-1")
	strandEntry(t, root, releaseOpid, PhaseCreated, Intent{Verb: "release", Targets: []string{"own-release"}})
	if _, err := Recover(endpointFor(root)); err != nil {
		t.Fatal(err)
	}
	projection, err := Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	released := projection.Tree.Live["own-release"]
	releaseEntry, err := ReadEntry(root, releaseOpid)
	if err != nil || released == nil || released.Claimed != nil || releaseEntry.Outcome != OutcomeConfirmed {
		t.Fatalf("the dead pair's own release did not heal: goal=%+v entry=%+v err=%v", released, releaseEntry, err)
	}

	// A stored human name is evidence of intent, not authority to create a
	// human-attributed park.
	if res, err := Open(verbReq(root, "01J5X00000000000000000Q151", "mac-a"), "recovered-human-park", "Recover the admitted human park.", OriginMain, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open recovered human park: %+v %v", res, err)
	}
	recoveredHumanParkTip := mustGit(t, root, "rev-parse", "origin/main")
	recoveredHumanParkOpid := Opid("01J5X00000000000000000Q152", "mac-a", "lin-1")
	strandEntry(t, root, recoveredHumanParkOpid, PhaseCreated, Intent{Verb: "park", Targets: []string{"recovered-human-park"}, Args: map[string]string{"because": "human pause", "by": "Wido"}})
	reports, err := Recover(endpointFor(root))
	if err != nil {
		t.Fatal(err)
	}
	recoveredHumanParkEntry, err := ReadEntry(root, recoveredHumanParkOpid)
	wantHumanParkRefusal := "park is proof-bearing and cannot be replayed from journal text; re-run it from the human authority boundary"
	if err != nil || recoveredHumanParkEntry.Phase != PhaseTerminal || recoveredHumanParkEntry.Outcome != OutcomeRejected || recoveredHumanParkEntry.Evidence != wantHumanParkRefusal {
		t.Fatalf("journaled human park did not close rejected: entry=%+v err=%v reports=%+v", recoveredHumanParkEntry, err, reports)
	}
	if afterTip := mustGit(t, root, "rev-parse", "origin/main"); afterTip != recoveredHumanParkTip {
		t.Fatalf("journaled human park wrote to the goal branch: before=%s after=%s", recoveredHumanParkTip, afterTip)
	}
	projection, err = Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	recoveredHumanPark := projection.Tree.Live["recovered-human-park"]
	if recoveredHumanPark == nil || recoveredHumanPark.State != StateQueued || recoveredHumanPark.Parked != nil {
		t.Fatalf("journaled human park changed the goal: %+v", recoveredHumanPark)
	}

	assertHumanBoundaryRefusal := func(opid, verb, row, grade, beforeTip string) {
		t.Helper()
		reports, recoverErr := Recover(endpointFor(root))
		if recoverErr != nil {
			t.Fatal(recoverErr)
		}
		entry, readErr := ReadEntry(root, opid)
		want := fmt.Sprintf("%s is proof-bearing and cannot be replayed from journal text; re-run it from the human authority boundary because %s requires %s-grade human authority", verb, row, grade)
		if readErr != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeRejected || entry.Evidence != want {
			t.Fatalf("%s did not close with the fresh human-boundary remedy: entry=%+v err=%v reports=%+v", verb, entry, readErr, reports)
		}
		if afterTip := mustGit(t, root, "rev-parse", "origin/main"); afterTip != beforeTip {
			t.Fatalf("refused %s changed the goal branch: before=%s after=%s", verb, beforeTip, afterTip)
		}
	}

	// A foreign release requires the human and leaves the other pair's claim.
	if res, err := Open(verbReq(root, "01J5X00000000000000000Q143", "mac-a"), "foreign-release", "Keep the foreign claim.", OriginMain, "Continue."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open foreign release: %+v %v", res, err)
	}
	foreignClaim := verbReq(root, "01J5X00000000000000000Q144", "mac-b")
	if res, err := claimApprovedForTest(t, foreignClaim, "foreign-release", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim foreign release: %+v %v", res, err)
	}
	foreignTip := mustGit(t, root, "rev-parse", "origin/main")
	foreignOpid := Opid("01J5X00000000000000000Q145", "mac-a", "lin-1")
	strandEntry(t, root, foreignOpid, PhaseCreated, Intent{Verb: "release", Targets: []string{"foreign-release"}, Args: map[string]string{"by": "Wido"}})
	assertHumanBoundaryRefusal(foreignOpid, "release", "foreign release", humanauthority.GradeTerminal, foreignTip)
	projection, err = Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	foreignGoal := projection.Tree.Live["foreign-release"]
	if foreignGoal == nil || foreignGoal.Claimed == nil || foreignGoal.Claimed.Machine != "mac-b" {
		t.Fatalf("refused foreign release changed its claim: %+v", foreignGoal)
	}

	// A human-origin goal retains its standing reservation when journal replay
	// cannot carry a fresh human proof.
	if res, err := Open(verbReq(root, "01J5X00000000000000000Q146", "mac-a"), "human-origin", "Human-reserved work.", OriginHuman, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open human-origin goal: %+v %v", res, err)
	}
	humanOriginTip := mustGit(t, root, "rev-parse", "origin/main")
	humanOriginOpid := Opid("01J5X00000000000000000Q147", "mac-a", "lin-1")
	strandEntry(t, root, humanOriginOpid, PhaseCreated, Intent{Verb: "park", Targets: []string{"human-origin"}, Args: map[string]string{"because": "journal pause", "by": "Wido"}})
	assertHumanBoundaryRefusal(humanOriginOpid, "park", "park of a human-origin goal", humanauthority.GradeTerminal, humanOriginTip)
	projection, err = Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	humanOrigin := projection.Tree.Live["human-origin"]
	if humanOrigin == nil || humanOrigin.State != StateQueued || humanOrigin.Parked != nil {
		t.Fatalf("refused human-origin park changed the goal: %+v", humanOrigin)
	}

	// A human's live park remains in force when an authority-free journal
	// unpark tries to lift it.
	if res, err := Open(verbReq(root, "01J5X00000000000000000Q148", "mac-a"), "human-park", "Keep the human pause.", OriginMain, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open human-park goal: %+v %v", res, err)
	}
	humanPark := verbReq(root, "01J5X00000000000000000Q149", "mac-a")
	humanPark.Actor.Human = "Wido"
	humanPark.Authority = testTerminalAuthority(t, root, humanPark.Now)
	if res, err := Park(humanPark, "human-park", "human pause"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("record human park: %+v %v", res, err)
	}
	humanParkTip := mustGit(t, root, "rev-parse", "origin/main")
	unparkOpid := Opid("01J5X00000000000000000Q150", "mac-a", "lin-1")
	strandEntry(t, root, unparkOpid, PhaseCreated, Intent{Verb: "unpark", Targets: []string{"human-park"}, Args: map[string]string{"by": "Wido"}})
	assertHumanBoundaryRefusal(unparkOpid, "unpark", "unpark of a human park to queued", humanauthority.GradeTerminal, humanParkTip)
	projection, err = Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	humanParked := projection.Tree.Live["human-park"]
	if humanParked == nil || humanParked.State != StateParked || humanParked.Parked == nil || humanParked.Parked.By != "human:Wido" {
		t.Fatalf("refused unpark changed the human's park: %+v", humanParked)
	}
}

func TestRecoveryNamedStoppingIntentsPreserveTypedOutcomes(t *testing.T) {
	t.Run("already applied park confirms", func(t *testing.T) {
		_, root, _ := twoClones(t)
		seedLedger(t, root)
		if res, err := Open(verbReq(root, "01J5X00000000000000000Q154", "mac-a"), "landed-human-park", "The delayed human park.", OriginMain, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", res, err)
		}
		park := verbReq(root, "01J5X00000000000000000Q155", "mac-a")
		park.Actor.Human = "Wido"
		park.Authority = testTerminalAuthority(t, root, park.Now)
		if res, err := Park(park, "landed-human-park", "human pause"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("land the human park: %+v %v", res, err)
		}
		entry, err := ReadEntry(root, park.opid())
		if err != nil {
			t.Fatal(err)
		}
		entry.Phase = PhaseCreated
		entry.FetchedOid = ""
		entry.TxnCommit = ""
		entry.ExpectedOldTip = ""
		entry.Attempts = 0
		entry.Deadline = ""
		entry.Outcome = ""
		entry.Evidence = ""
		entry.TerminalAt = ""
		if err := writeEntry(root, entry); err != nil {
			t.Fatal(err)
		}
		beforeTip := mustGit(t, root, "rev-parse", "origin/main")
		detail, err := completeFromIntent(endpointFor(root), entry, nil)
		if err != nil || !strings.Contains(detail, "confirmed") {
			t.Fatalf("recovery did not preserve already-applied: detail=%q err=%v", detail, err)
		}
		terminal, err := ReadEntry(root, park.opid())
		if err != nil || terminal.Outcome != OutcomeConfirmed || !strings.Contains(terminal.Evidence, "already applied") {
			t.Fatalf("already-applied park did not close confirmed: entry=%+v err=%v", terminal, err)
		}
		if afterTip := mustGit(t, root, "rev-parse", "origin/main"); afterTip != beforeTip {
			t.Fatalf("already-applied recovery wrote a second park: before=%s after=%s", beforeTip, afterTip)
		}
	})

	t.Run("release lost to competitor stays lost", func(t *testing.T) {
		_, root, _ := twoClones(t)
		seedLedger(t, root)
		opid := Opid("01J5X00000000000000000Q156", "mac-a", "lin-1")
		strandEntry(t, root, opid, PhaseCreated, Intent{Verb: "release", Targets: []string{"lost-release"}, Args: map[string]string{"by": "Wido"}})
		entry, err := TakeOver(root, opid)
		if err != nil {
			t.Fatal(err)
		}
		req, err := requestForEntry(endpointFor(root), entry)
		if err != nil {
			t.Fatal(err)
		}
		winner := "01J5X00000000000000000Q157-mac-b-lin-2"
		req.Mutate = func(string) ([]Change, error) {
			return nil, LostToCompetitor{Winner: winner}
		}
		req = recoveryHumanBoundaryRequest(req, entry.Intent.Args["by"] != "")
		beforeTip := mustGit(t, root, "rev-parse", "origin/main")
		result, err := runTransaction(endpointFor(root), req)
		if err != nil || result.Outcome != OutcomeLost || !strings.Contains(result.Detail, winner) {
			t.Fatalf("recovery did not preserve lost-to-competitor: result=%+v err=%v", result, err)
		}
		terminal, err := ReadEntry(root, opid)
		if err != nil || terminal.Outcome != OutcomeLost || terminal.Evidence != "winner: "+winner {
			t.Fatalf("lost release did not keep its typed outcome: entry=%+v err=%v", terminal, err)
		}
		if afterTip := mustGit(t, root, "rev-parse", "origin/main"); afterTip != beforeTip {
			t.Fatalf("lost release changed the goal branch: before=%s after=%s", beforeTip, afterTip)
		}
	})
}

func TestRecoveryConfirmsASightedPushWithoutRebuilding(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// The operation LANDED (its opid is canonical) but the owner
	// died between the push and the confirm: recovery confirms on
	// sight — no rebuild, no second commit.
	res, err := Open(verbReq(a, "01J5X00000000000000000Q200", "mac-a"), "landed", "Landed work.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000Q200", "mac-a", "lin-1")
	// Rewind the entry's belief to pushed with a dead foreign owner.
	entry, err := ReadEntry(a, opid)
	if err != nil {
		t.Fatal(err)
	}
	entry.Phase = PhasePushed
	entry.Outcome = ""
	entry.TerminalAt = ""
	if err := writeEntry(a, entry); err != nil {
		t.Fatal(err)
	}
	cmd := spawnForeignOwner(t, a, opid)
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()

	tipBefore := mustGit(t, a, "ls-remote", "origin", "refs/heads/main")
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	confirmed := false
	for _, rep := range reports {
		if rep.Opid == opid && rep.Action == ActionConfirm {
			confirmed = true
		}
	}
	if !confirmed {
		t.Fatalf("a sighted push confirms without rebuilding: %+v", reports)
	}
	if tipAfter := mustGit(t, a, "ls-remote", "origin", "refs/heads/main"); tipAfter != tipBefore {
		t.Fatal("confirmation on sight moves nothing on the canonical branch")
	}
}

func TestRecoveryHandlesOwnEntriesAndDoneRebuild(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// The test process OWNS these entries: recovery abandons its own
	// never-pushed work and expires its own overdue push.
	ownCreated := Opid("01J5X00000000000000000Q300", "mac-a", "lin-1")
	if _, err := CreateEntry(a, ownCreated, "mac-a", "lin-1", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	ownPushed := Opid("01J5X00000000000000000Q310", "mac-a", "lin-1")
	if _, err := CreateEntry(a, ownPushed, "mac-a", "lin-1", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	if err := MarkPushed(a, ownPushed, "sometip", 1, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]RecoveryAction{}
	for _, rep := range reports {
		got[rep.Opid] = rep.Action
	}
	if got[ownCreated] != ActionAbandonOwn {
		t.Fatalf("the owner abandons its own never-pushed work: %v", got[ownCreated])
	}
	if got[ownPushed] != ActionExpireOwn {
		t.Fatalf("the owner expires its own overdue push: %v", got[ownPushed])
	}

	// A dead owner's DONE rebuild, conclusion in the stored intent.
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q320", "mac-a"), "finish-me", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	doneOpid := Opid("01J5X00000000000000000Q330", "mac-a", "lin-1")
	strandEntry(t, a, doneOpid, PhaseCreated, Intent{
		Verb: "done", Targets: []string{"finish-me"},
		Args: map[string]string{"conclusion": "Recovered conclusion."},
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	archived := p.Tree.Done["finish-me"]
	if archived == nil || archived.Conclude != "Recovered conclusion." {
		t.Fatalf("the recovered done archived with its conclusion: %+v", archived)
	}
}

func TestRecoveryRunsTheRealVerbSemanticsAcrossAnArc(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	seedGoalNormConfig(t, a)
	seedGoalNormConfig(t, b)
	for i, id := range []string{"rv-one", "rv-two"} {
		ulid := fmt.Sprintf("01J5X00000000000000000Q%d40", i)
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Arc "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if res, err := SetArc(verbReq(a, ulid[:len(ulid)-1]+"5", "mac-a"), id, "rv-arc"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set-arc %s: %+v %v", id, res, err)
		}
	}
	// A dead owner's CASCADE claim cannot reconstruct human approval.
	claimOpid := Opid("01J5X00000000000000000Q160", "mac-a", "lin-1")
	strandEntry(t, a, claimOpid, PhaseCreated, Intent{
		Verb: "claim", Targets: []string{"rv-one"},
		Args: mergeIntentArgs(map[string]string{"cascade": "arc"}, budgetIntentArgs(testBudget())),
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"rv-one", "rv-two"} {
		f := p.Tree.Live[id]
		if f.State != StateQueued || f.Claimed != nil {
			t.Fatalf("recovery manufactured approval across the arc: %s %+v", id, f)
		}
	}
	claimEntry, err := ReadEntry(a, claimOpid)
	if err != nil || claimEntry.Outcome != OutcomeRejected || !strings.Contains(claimEntry.Evidence, "APPROVAL_REQUIRED") {
		t.Fatalf("the recovered arc claim did not close with the approval remedy: %+v %v", claimEntry, err)
	}

	// A dead owner's STEAL from the other machine cannot replay. The journal's
	// by string makes the act proof-bearing but is not human authority.
	stealOpid := Opid("01J5X00000000000000000Q170", "mac-b", "lin-1")
	strandEntryAt(t, b, stealOpid, "mac-b", PhaseCreated, Intent{
		Verb: "steal", Targets: []string{"rv-one"},
		Args: map[string]string{"by": "wido", "approvedRef": "R-25b"},
	})
	reports, err := Recover(endpointFor(b))
	if err != nil {
		t.Fatal(err)
	}
	p, err = Project(endpointFor(b), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	stolen := p.Tree.Live["rv-one"]
	if stolen.State != StateQueued || stolen.Claimed != nil {
		t.Fatalf("journal text manufactured or reassigned execution: %+v", stolen)
	}
	entry, err := ReadEntry(b, stealOpid)
	if err != nil || entry.Outcome != OutcomeRejected {
		t.Fatalf("the unauthorized steal journal did not close rejected: entry=%+v err=%v reports=%+v", entry, err, reports)
	}
	wantStealRefusal := "steal is proof-bearing and cannot be replayed from journal text; re-run it from the human authority boundary"
	if entry.Evidence != wantStealRefusal {
		t.Fatalf("steal recovery did not direct a fresh human-boundary rerun: %+v", entry)
	}
}

func TestRecoveryRefusesAStrandedOriginRewrite(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q200", "mac-a"), "prov", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// A pre-fold journal's origin delta: recovery must refuse it by
	// name, never replay it — origin is immutable through recovery
	// exactly as it is through the live verbs.
	opid := Opid("01J5X00000000000000000Q210", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{
		Verb: "edit", Targets: []string{"prov"},
		Deltas: []FieldDelta{{Target: "prov", Field: "origin", New: "main"}},
	})
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rep := range reports {
		if rep.Opid == opid {
			found = true
			if !strings.Contains(rep.Detail, "Origin") || !strings.Contains(rep.Detail, "immutable") {
				t.Fatalf("the origin rewrite refuses by name: %+v", rep)
			}
		}
	}
	if !found {
		t.Fatalf("recovery visits the stranded origin rewrite: %+v", reports)
	}
}

func TestRecoveryRefusesJournaledHumanDone(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q300", "mac-a"), "hand-held", "Work.", OriginMain, "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	parkReq := verbReq(a, "01J5X00000000000000000Q310", "mac-a")
	if res, err := Park(parkReq, "hand-held", "waiting on review"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	doneReq := verbReq(a, "01J5X00000000000000000Q320", "mac-a")
	doneReq.Actor.Human = "Wido"
	storedDone := doneRequest(doneReq, "hand-held", "Journal text must not conclude this goal.")
	beforeTip := mustGit(t, a, "rev-parse", "origin/main")
	strandEntryAt(t, a, storedDone.Opid, "mac-a", PhaseCreated, storedDone.Intent)
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ReadEntry(a, storedDone.Opid)
	want := "done is proof-bearing and cannot be replayed from journal text; re-run it from the human authority boundary"
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeRejected || entry.Evidence != want {
		t.Fatalf("journaled human done did not close rejected: entry=%+v err=%v reports=%+v", entry, err, reports)
	}
	if afterTip := mustGit(t, a, "rev-parse", "origin/main"); afterTip != beforeTip {
		t.Fatalf("journaled human done wrote to the goal branch: before=%s after=%s", beforeTip, afterTip)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f := p.Tree.Live["hand-held"]
	if f == nil || f.State != StateParked || p.Tree.Done["hand-held"] != nil {
		t.Fatalf("journaled human done changed the goal: live=%+v done=%+v", f, p.Tree.Done["hand-held"])
	}
}

func TestRecoveryCompletesMainSplitAndRejectsHumanOrDoctoredDrafts(t *testing.T) {
	t.Run("created main split completes", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if res, err := Open(verbReq(root, "01J5X00000000000000000RM00", "mac-a"), "recover-main-split", "Recover the split.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", res, err)
		}
		members := testMembers("recover-main-split")
		request := verbReq(root, "01J5X00000000000000000RM10", "mac-a")
		rebuilt, err := splitRequest(request, "recover-main-split", members, mainRatification("recover-main-split", members), nil)
		if err != nil {
			t.Fatal(err)
		}
		strandEntry(t, root, rebuilt.Opid, PhaseCreated, rebuilt.Intent)
		reports, err := Recover(endpointFor(root))
		if err != nil {
			t.Fatal(err)
		}
		entry, readErr := ReadEntry(root, rebuilt.Opid)
		projection, projectErr := Project(endpointFor(root), false, time.Now())
		if readErr != nil || projectErr != nil || entry.Outcome != OutcomeConfirmed ||
			projection.Tree.Done["recover-main-split"] == nil || projection.Tree.Live["recover-main-split-one"] == nil {
			t.Fatalf("main split did not recover atomically: entry=%+v reports=%+v read=%v project=%v", entry, reports, readErr, projectErr)
		}
	})

	t.Run("human ratification remains fresh authority", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if res, err := Open(verbReq(root, "01J5X00000000000000000RH00", "mac-a"), "recover-human-split", "Human-ratified split.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", res, err)
		}
		members := testMembers("recover-human-split")
		request := verbReq(root, "01J5X00000000000000000RH10", "mac-a")
		request.Actor.Human = "wido"
		ratification := SplitRatification{Tier: RatifierHuman, By: "wido", DraftSHA256: SplitDraftSHA256("recover-human-split", members)}
		rebuilt, err := splitRequest(request, "recover-human-split", members, ratification, testHumanAuthority(t, root, request.Now))
		if err != nil {
			t.Fatal(err)
		}
		strandEntry(t, root, rebuilt.Opid, PhaseCreated, rebuilt.Intent)
		reports, err := Recover(endpointFor(root))
		if err != nil {
			t.Fatal(err)
		}
		entry, readErr := ReadEntry(root, rebuilt.Opid)
		projection, projectErr := Project(endpointFor(root), false, time.Now())
		if readErr != nil || projectErr != nil || entry.Outcome != OutcomeRejected || projection.Tree.Live["recover-human-split"] == nil || projection.Tree.Done["recover-human-split"] != nil {
			t.Fatalf("human journal text changed the parent: entry=%+v reports=%+v read=%v project=%v", entry, reports, readErr, projectErr)
		}
		if entry.Evidence != "split is proof-bearing and cannot be replayed from journal text; re-run it from the human authority boundary" {
			t.Fatalf("human recovery did not name the fresh-authority remedy: %+v", entry)
		}
	})

	t.Run("doctored stored members fail their digest", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if res, err := Open(verbReq(root, "01J5X00000000000000000RD00", "mac-a"), "recover-doctored", "Digest-bound split.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", res, err)
		}
		members := testMembers("recover-doctored")
		request := verbReq(root, "01J5X00000000000000000RD10", "mac-a")
		rebuilt, err := splitRequest(request, "recover-doctored", members, mainRatification("recover-doctored", members), nil)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt.Intent.Args["members"] = strings.Replace(rebuilt.Intent.Args["members"], "Deliver the first part.", "Doctored first part.", 1)
		strandEntry(t, root, rebuilt.Opid, PhaseCreated, rebuilt.Intent)
		if _, err := Recover(endpointFor(root)); err != nil {
			t.Fatal(err)
		}
		entry, readErr := ReadEntry(root, rebuilt.Opid)
		projection, projectErr := Project(endpointFor(root), false, time.Now())
		if readErr != nil || projectErr != nil || entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "digest") ||
			projection.Tree.Live["recover-doctored"] == nil || projection.Tree.Live["recover-doctored-one"] != nil {
			t.Fatalf("doctored members did not close rejected by digest with parent untouched: entry=%+v read=%v project=%v", entry, readErr, projectErr)
		}
	})
}

func TestRecoveryClassifiesOldArcDebtAfterDecomposedParentWasPruned(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if res, err := Open(verbReq(root, "01J5X00000000000000000RP00", "mac-a"), "pruned-split", "Prune after split.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := SetArc(verbReq(root, "01J5X00000000000000000RP10", "mac-a"), "pruned-split", "old-pruned-arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set old arc: %+v %v", res, err)
	}
	members := testMembers("pruned-split")
	request := verbReq(root, "01J5X00000000000000000RP20", "mac-a")
	if res, err := Split(request, "pruned-split", members, mainRatification("pruned-split", members), nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("split: %+v %v", res, err)
	}
	if res, err := Prune(verbReq(root, "01J5X00000000000000000RP30", "mac-a"), 0); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("prune parent: %+v %v", res, err)
	}
	entry, err := ReadEntry(root, request.opid())
	if err != nil {
		t.Fatal(err)
	}
	entry.Phase = PhasePushed
	entry.Outcome = ""
	entry.Evidence = ""
	entry.TerminalAt = ""
	entry.Owner = OwnerIdentity{Pid: 99999999, PidStartedAt: 1}
	if err := writeEntry(root, entry); err != nil {
		t.Fatal(err)
	}
	reports, err := Recover(endpointFor(root))
	if err != nil {
		t.Fatalf("pruned-parent recovery wedged: %v", err)
	}
	entry, err = ReadEntry(root, request.opid())
	if err != nil || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("registry coordinates did not let recovery terminalize: %+v %v reports=%+v", entry, err, reports)
	}
}

func TestRecoveryCompletesADeadOwnersBlockerOpen(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q100", "mac-a"), "held", "Work the blocker holds.", OriginHuman, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, verbReq(a, "01J5X00000000000000000Q101", "mac-a"), "held", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	// The dead owner held that goal and its open named the blocker:
	// recovery replays the whole publish, the park included, under the
	// original opid.
	opid := Opid("01J5X00000000000000000Q110", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{
		Verb: "open", Targets: []string{"fixer", "held"},
		Args: map[string]string{"intent": "The dead owner's blocker.", "origin": "main", "next": "Fix.", "blocks": "held", "labels": ""},
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	fixer, held := p.Tree.Live["fixer"], p.Tree.Live["held"]
	if fixer == nil || fixer.State != StateQueued || fixer.History[0].Opid != opid {
		t.Fatalf("the recovered blocker opens under the original opid: %+v", fixer)
	}
	if held == nil || held.State != StateParked || held.Claimed != nil || held.Parked == nil || held.Parked.Blocker != "fixer" || strings.Join(held.Blocked, ",") != "fixer" {
		t.Fatalf("the recovered open parks the blocked goal with its blocker: %+v", held)
	}
	if held.History[len(held.History)-1].Opid != opid {
		t.Fatalf("the park rides the original opid: %+v", held.History[len(held.History)-1])
	}
}
