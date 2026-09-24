package goal

import (
	"strings"
	"testing"
	"time"
)

// The relation a goal file already carried both ways gains the verbs that
// write it: an open that blocks several goals, an open that waits for
// several, and goal block and goal unblock on goals that already exist.
//
// Every test here writes through the real verbs onto a fixture ledger and
// reads the published tree back, because what these verbs are about is what
// the record says afterwards: which goals park, which park lifts, which
// marker the park carries, and which hand was allowed to write it.

func edgeRisk() RiskRecord {
	return RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "fixture"}
}

// personReq is the enrolled human at the keyboard: the same request a seat
// makes, with the name that makes it a person's act.
func personReq(root, ulid, machine string) VerbRequest {
	request := verbReq(root, ulid, machine)
	request.Actor.Human = "Wido"
	return request
}

// assertFilesParse reads every file the tree holds back through the parser.
// A verb that writes a record the reader refuses has written a ledger nobody
// can open, which is worse than the refusal it avoided.
func assertFilesParse(t *testing.T, tree *TreeGoals) {
	t.Helper()
	for _, set := range []map[string]*GoalFile{tree.Live, tree.Done, tree.Abandoned} {
		for _, id := range sortedGoalIds(set) {
			if _, problems := ParseFile(RenderFile(set[id])); len(problems) != 0 {
				t.Fatalf("goal %s does not parse after the act: %v", id, problems)
			}
		}
	}
}

// liveGoalForEdges opens one goal through the compatibility entry point,
// which takes no blocker and so models neither a seat's open nor a person's.
func liveGoalForEdges(t *testing.T, root, ulid, id, origin string) {
	t.Helper()
	if result, err := Open(verbReq(root, ulid, "mac-a"), id, "Work called "+id+".", origin, "Do "+id+"."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open %s: %+v %v", id, result, err)
	}
}

// concludeForEdges takes one live goal all the way to done, which is the only
// thing that satisfies an edge.
func concludeForEdges(t *testing.T, root, ulidClaim, ulidDone, id string) PublishResult {
	t.Helper()
	budget := testBudget()
	if result, err := claimApprovedForTest(t, verbReq(root, ulidClaim, "mac-a"), id, budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim %s: %+v %v", id, result, err)
	}
	result, err := Done(verbReq(root, ulidDone, "mac-a"), id, "Finished "+id+".")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("done %s: %+v %v", id, result, err)
	}
	return result
}

// An open names every goal it unblocks, and each of them parks in the one
// publish with the new goal recorded as its blocker.
func TestOpenBlocksSeveralGoalsInOnePublish(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BK00", "holds-one", OriginHuman)
	liveGoalForEdges(t, root, "01J5X00000000000000000BK01", "holds-two", OriginHuman)

	result, err := OpenRisked(personReq(root, "01J5X00000000000000000BK02", "mac-a"), "one-fix",
		"The defect both goals wait for.", OriginHuman, "Fix it.",
		[]string{"holds-one,holds-two"}, nil, edgeRisk(), 0, "", &budget, nil)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocks two goals: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"holds-one", "holds-two"} {
		held := tree.Live[id]
		if held.State != StateParked || held.Parked == nil || held.Parked.Blocker != "one-fix" ||
			strings.Join(held.Blocked, ",") != "one-fix" {
			t.Fatalf("goal %s did not park behind the one open: state=%s parked=%+v blocked=%v", id, held.State, held.Parked, held.Blocked)
		}
		if last := held.History[len(held.History)-1]; last.Verb != "park" || last.Opid != tree.Live["one-fix"].History[0].Opid {
			t.Fatalf("goal %s did not park on the open's own operation: %+v", id, last)
		}
	}
	opened := tree.Live["one-fix"]
	if opened.State != StateQueued || opened.Parked != nil || len(opened.Blocked) != 0 {
		t.Fatalf("the goal that blocks waits for nothing itself: %+v", opened)
	}
	if got := strings.Join(opened.History[0].Targets, ","); got != "one-fix,holds-one,holds-two" {
		t.Fatalf("the open's history names every goal it touched: %s", got)
	}
	assertFilesParse(t, tree)
}

// One fenced target refuses the whole open: the claim teardown a blocker park
// runs is the same one that will not clear a breach stop, and nothing of the
// open publishes (S37-05).
func TestOpenBlocksRefusesAFencedTargetAndPublishesNothing(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BF00", "plain-work", OriginHuman)
	liveGoalForEdges(t, root, "01J5X00000000000000000BF01", "fenced-work", OriginHuman)

	claim := verbReq(root, "01J5X00000000000000000BF02", "mac-a")
	claim.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claim, "fenced-work", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim the goal that will be fenced: %+v %v", result, err)
	}
	projection, err := Project(endpointFor(root), true, claim.Now)
	if err != nil {
		t.Fatal(err)
	}
	claimed := projection.Tree.Live["fenced-work"]
	stop := CloseStopRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"},
			Ulid: "01J5X00000000000000000BF03", Now: claim.Now.Add(time.Minute), ClaimEpoch: 9,
		},
		GoalID: "fenced-work", StopID: "stop-fenced-work-r3-f1",
		Reason: StopReasonElapsedLimit, Capability: *claimed.StopCapability,
	}
	if result, err := CloseStop(stop); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("breach-stop the claim: %+v %v", result, err)
	}

	before := acceptedTip(t, root)
	result, err := OpenRisked(personReq(root, "01J5X00000000000000000BF04", "mac-a"), "two-fix",
		"A defect two goals wait for.", OriginHuman, "Fix it.",
		[]string{"plain-work", "fenced-work"}, nil, edgeRisk(), 0, "", &budget, nil)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "breach-stopped") {
		t.Fatalf("a fenced target did not refuse the open: %+v %v", result, err)
	}
	if acceptedTip(t, root) != before {
		t.Fatal("the refused open moved the ledger")
	}
	tree, err := loadTree(root, before)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["two-fix"] != nil {
		t.Fatal("the refused open published its own goal")
	}
	if held := tree.Live["plain-work"]; held.State == StateParked || len(held.Blocked) != 0 {
		t.Fatalf("the refused open parked the target it reached first: %+v", held)
	}
}

// An open may also name the goals it waits for. It parks at once, with the
// marker on the first named blocker and the reason naming them all, and
// returns only when the last of them is done.
func TestOpenBlockedByManyParksAtOnceAndReturnsWhenTheLastIsDone(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BB00", "dep-a", OriginMain)
	liveGoalForEdges(t, root, "01J5X00000000000000000BB01", "dep-b", OriginMain)

	result, err := OpenRisked(personReq(root, "01J5X00000000000000000BB02", "mac-a"), "waiter",
		"Work that waits for two.", OriginHuman, "Wait.",
		nil, []string{"dep-a", "dep-b"}, edgeRisk(), 0, "", &budget, nil)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocked-by two goals: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	waiter := tree.Live["waiter"]
	if waiter.State != StateParked || waiter.Parked == nil || waiter.Parked.Blocker != "dep-a" ||
		strings.Join(waiter.Blocked, ",") != "dep-a,dep-b" {
		t.Fatalf("the opened goal did not park behind both: state=%s parked=%+v blocked=%v", waiter.State, waiter.Parked, waiter.Blocked)
	}
	if !strings.Contains(waiter.Parked.Because, "blocked by dep-a, dep-b") || !strings.Contains(waiter.Parked.Because, "they are done") {
		t.Fatalf("the park's reason does not name both goals: %q", waiter.Parked.Because)
	}
	assertFilesParse(t, tree)

	first := concludeForEdges(t, root, "01J5X00000000000000000BB03", "01J5X00000000000000000BB04", "dep-a")
	tree, err = loadTree(root, first.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if held := tree.Live["waiter"]; held.State != StateParked {
		t.Fatalf("one blocker of two did not lift the park, and must not: %s", held.State)
	}
	second := concludeForEdges(t, root, "01J5X00000000000000000BB05", "01J5X00000000000000000BB06", "dep-b")
	tree, err = loadTree(root, second.Tip)
	if err != nil {
		t.Fatal(err)
	}
	returned := tree.Live["waiter"]
	if returned.State != StateQueued || returned.Parked != nil || strings.Join(returned.Blocked, ",") != "dep-a,dep-b" {
		t.Fatalf("the last blocker did not return the goal with its edges kept: state=%s parked=%+v blocked=%v", returned.State, returned.Parked, returned.Blocked)
	}
	assertFilesParse(t, tree)
}

// A blocker that finished before the open is a satisfied edge: it is recorded
// and it parks nothing, because the completion this park would wait for has
// already happened (S37-06).
func TestOpenBlockedByASatisfiedBlockerParksNothing(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BS00", "already-done", OriginMain)
	concludeForEdges(t, root, "01J5X00000000000000000BS01", "01J5X00000000000000000BS02", "already-done")

	result, err := OpenRisked(personReq(root, "01J5X00000000000000000BS03", "mac-a"), "later-work",
		"Work behind finished work.", OriginHuman, "Start it.",
		nil, []string{"already-done"}, edgeRisk(), 0, "", &budget, nil)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocked-by a done goal: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	later := tree.Live["later-work"]
	if later.State != StateQueued || later.Parked != nil || strings.Join(later.Blocked, ",") != "already-done" {
		t.Fatalf("a satisfied edge parked the new goal: state=%s parked=%+v blocked=%v", later.State, later.Parked, later.Blocked)
	}
	assertFilesParse(t, tree)
}

// goal block writes the same edge on a goal that already exists, through the
// same path: the claim is cleared, the displaced claimant is recorded, and
// the park carries its marker.
func TestBlockParksALiveGoalAndRecordsTheDisplacement(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BC00", "held-work", OriginHuman)
	liveGoalForEdges(t, root, "01J5X00000000000000000BC01", "new-blocker", OriginMain)
	if result, err := claimApprovedForTest(t, verbReq(root, "01J5X00000000000000000BC02", "mac-b"), "held-work", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("another pair claims the goal: %+v %v", result, err)
	}

	// A seat reaches only the goal it holds: naming a stranger's claim is the
	// refusal the open already gives (S37-01).
	if result, err := Block(verbReq(root, "01J5X00000000000000000BC03", "mac-a"), "held-work", "new-blocker"); err != nil ||
		result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not this seat's claim") {
		t.Fatalf("a seat blocked a goal it does not hold: %+v %v", result, err)
	}

	result, err := Block(personReq(root, "01J5X00000000000000000BC04", "mac-a"), "held-work", "new-blocker")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("goal block: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	held := tree.Live["held-work"]
	if held.State != StateParked || held.Parked == nil || held.Parked.Blocker != "new-blocker" ||
		held.Claimed != nil || strings.Join(held.Blocked, ",") != "new-blocker" {
		t.Fatalf("goal block did not park and clear the claim: state=%s parked=%+v claimed=%+v blocked=%v", held.State, held.Parked, held.Claimed, held.Blocked)
	}
	if !strings.HasPrefix(held.Parked.Displaced, "mac-b+") {
		t.Fatalf("the displaced claimant is not recorded: %q", held.Parked.Displaced)
	}
	if last := held.History[len(held.History)-1]; last.Verb != "park" || last.Displaced != held.Parked.Displaced {
		t.Fatalf("the park's history line does not carry the displacement: %+v", last)
	}
	// The same edge twice is nothing to do rather than a second revision.
	if result, err := Block(personReq(root, "01J5X00000000000000000BC05", "mac-a"), "held-work", "new-blocker"); err != nil ||
		result.Outcome != OutcomeAbandoned {
		t.Fatalf("a repeated block wrote a revision: %+v %v", result, err)
	}
	assertFilesParse(t, tree)
}

// Removing a satisfied edge from a park that still waits for live work keeps
// the park and rebinds its marker, because a marker outside BlockedBy is a
// record the reader refuses and a cleared one never returns (S37-03).
func TestUnblockOfASatisfiedEdgeRebindsTheMarkerAndKeepsThePark(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BR00", "dep-1", OriginMain)
	liveGoalForEdges(t, root, "01J5X00000000000000000BR01", "dep-2", OriginMain)
	if result, err := OpenRisked(personReq(root, "01J5X00000000000000000BR02", "mac-a"), "rebound",
		"Work behind two.", OriginHuman, "Wait.", nil, []string{"dep-1", "dep-2"}, edgeRisk(), 0, "", &budget, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocked-by: %+v %v", result, err)
	}
	concludeForEdges(t, root, "01J5X00000000000000000BR03", "01J5X00000000000000000BR04", "dep-1")

	result, err := Unblock(personReq(root, "01J5X00000000000000000BR05", "mac-a"), "rebound", "dep-1", nil)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("unblock a satisfied edge: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	stood := tree.Live["rebound"]
	if stood.State != StateParked || stood.Parked == nil || stood.Parked.Blocker != "dep-2" ||
		strings.Join(stood.Blocked, ",") != "dep-2" {
		t.Fatalf("the park did not stand on the remaining blocker: state=%s parked=%+v blocked=%v", stood.State, stood.Parked, stood.Blocked)
	}
	if !strings.Contains(stood.Parked.Because, "blocked by dep-2") {
		t.Fatalf("the park's reason was not repaired with the marker: %q", stood.Parked.Because)
	}
	if last := stood.History[len(stood.History)-1]; last.Verb != "unblock" || !strings.Contains(last.Reason, "drops dep-1") {
		t.Fatalf("the removal is not in the history as its own verb: %+v", last)
	}
	assertFilesParse(t, tree)
}

// Removing the last unsatisfied edge of a dependency-created park returns the
// goal, because the park's own condition is satisfied the moment the edge is
// gone. It is an early lift, so it is a person's act with a proof.
func TestUnblockOfTheLastUnsatisfiedEdgeReturnsTheGoal(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BN00", "dep-x", OriginMain)
	liveGoalForEdges(t, root, "01J5X00000000000000000BN01", "dep-y", OriginMain)
	if result, err := OpenRisked(personReq(root, "01J5X00000000000000000BN02", "mac-a"), "released",
		"Work behind two.", OriginHuman, "Wait.", nil, []string{"dep-x", "dep-y"}, edgeRisk(), 0, "", &budget, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocked-by: %+v %v", result, err)
	}
	concludeForEdges(t, root, "01J5X00000000000000000BN03", "01J5X00000000000000000BN04", "dep-x")

	request := personReq(root, "01J5X00000000000000000BN05", "mac-a")
	result, err := Unblock(request, "released", "dep-y", goalHumanProof(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("unblock the last unsatisfied edge: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	returned := tree.Live["released"]
	if returned.State != StateQueued || returned.Parked != nil || strings.Join(returned.Blocked, ",") != "dep-x" {
		t.Fatalf("the goal did not return with its satisfied edge kept: state=%s parked=%+v blocked=%v", returned.State, returned.Parked, returned.Blocked)
	}
	if last := returned.History[len(returned.History)-1]; last.Verb != "unblock" || !strings.Contains(last.Reason, "the park lifts") {
		t.Fatalf("the return is not said in the history: %+v", last)
	}
	assertFilesParse(t, tree)
}

// A person's own pause is not a dependency's. It carries no marker, so it
// survives both the blocker's completion and the edge's removal, and goal
// unpark is still the only thing that lifts it (S37-02).
func TestUnblockOnAnOrdinaryParkRemovesTheEdgeAndLeavesThePark(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	liveGoalForEdges(t, root, "01J5X00000000000000000BP00", "paused", OriginHuman)
	liveGoalForEdges(t, root, "01J5X00000000000000000BP01", "side-fix", OriginMain)

	park := personReq(root, "01J5X00000000000000000BP02", "mac-a")
	park.Authority = testHumanAuthority(t, root, park.Now)
	if result, err := Park(park, "paused", "the shape of the answer is still being decided"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("a person's park: %+v %v", result, err)
	}
	if result, err := Block(personReq(root, "01J5X00000000000000000BP03", "mac-a"), "paused", "side-fix"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("block a goal that is already paused: %+v %v", result, err)
	}
	done := concludeForEdges(t, root, "01J5X00000000000000000BP04", "01J5X00000000000000000BP05", "side-fix")
	tree, err := loadTree(root, done.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if held := tree.Live["paused"]; held.State != StateParked || held.Parked == nil || held.Parked.Blocker != "" {
		t.Fatalf("the blocker's completion lifted a person's pause: state=%s parked=%+v", held.State, held.Parked)
	}

	result, err := Unblock(personReq(root, "01J5X00000000000000000BP06", "mac-a"), "paused", "side-fix", nil)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("unblock on an ordinary park: %+v %v", result, err)
	}
	tree, err = loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	stood := tree.Live["paused"]
	if stood.State != StateParked || stood.Parked == nil || stood.Parked.Blocker != "" || len(stood.Blocked) != 0 {
		t.Fatalf("removing the edge lifted a person's pause: state=%s parked=%+v blocked=%v", stood.State, stood.Parked, stood.Blocked)
	}
	if !strings.Contains(stood.Parked.Because, "still being decided") {
		t.Fatalf("the person's own reason was rewritten: %q", stood.Parked.Because)
	}
	assertFilesParse(t, tree)
}

// An early unblock is a person's act, admitted by the approval gate, so the
// signed-in session reaches it and its provenance lands on the line.
func TestEarlyUnblockNeedsAHumanAndRecordsItsSessionProvenance(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BE00", "dep-live", OriginMain)
	if result, err := OpenRisked(personReq(root, "01J5X00000000000000000BE01", "mac-a"), "waits",
		"Work behind live work.", OriginHuman, "Wait.", nil, []string{"dep-live"}, edgeRisk(), 0, "", &budget, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocked-by: %+v %v", result, err)
	}
	before := acceptedTip(t, root)

	if result, err := Unblock(verbReq(root, "01J5X00000000000000000BE02", "mac-a"), "waits", "dep-live", nil); err != nil ||
		result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "early lift and a human act") {
		t.Fatalf("a seat ran an early unblock: %+v %v", result, err)
	}
	if result, err := Unblock(personReq(root, "01J5X00000000000000000BE03", "mac-a"), "waits", "dep-live", nil); err != nil ||
		result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "freshly observed enrolled-human authority") {
		t.Fatalf("a name without a proof ran an early unblock: %+v %v", result, err)
	}
	if acceptedTip(t, root) != before {
		t.Fatal("a refused early unblock moved the ledger")
	}

	request := personReq(root, "01J5X00000000000000000BE04", "mac-a")
	result, err := Unblock(request, "waits", "dep-live", sessionProofForTest(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("a signed-in session could not run an early unblock: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	lifted := tree.Live["waits"]
	if lifted.State != StateQueued || lifted.Parked != nil || len(lifted.Blocked) != 0 {
		t.Fatalf("the early unblock did not return the goal: state=%s parked=%+v blocked=%v", lifted.State, lifted.Parked, lifted.Blocked)
	}
	last := lifted.History[len(lifted.History)-1]
	if last.Verb != "unblock" {
		t.Fatalf("the act is not recorded under its own verb: %+v", last)
	}
	assertSessionLine(t, last)
	assertFilesParse(t, tree)
}

// A seat's open reaches only the goal it holds, whichever of the named goals
// that is: naming one it holds never authorises the others (S37-01).
func TestSeatOpenBlocksRefusesAGoalItDoesNotHold(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BM00", "mine", OriginHuman)
	liveGoalForEdges(t, root, "01J5X00000000000000000BM01", "theirs", OriginHuman)
	if result, err := claimApprovedForTest(t, verbReq(root, "01J5X00000000000000000BM02", "mac-a"), "mine", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the seat claims its goal: %+v %v", result, err)
	}
	before := acceptedTip(t, root)
	result, err := OpenRisked(verbReq(root, "01J5X00000000000000000BM03", "mac-a"), "seat-fix",
		"The defect that blocks two goals.", OriginMain, "Fix it.",
		[]string{"mine", "theirs"}, nil, edgeRisk(), 0, "", &budget, nil)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not this seat's claim") {
		t.Fatalf("a seat parked a goal it does not hold: %+v %v", result, err)
	}
	if acceptedTip(t, root) != before {
		t.Fatal("the refused seat open moved the ledger")
	}
	tree, err := loadTree(root, before)
	if err != nil {
		t.Fatal(err)
	}
	if held := tree.Live["mine"]; held.State != StateClaimed {
		t.Fatalf("the refused open parked the goal the seat does hold: %s", held.State)
	}
}

// A recovered open rebuilds the same mutation, which means both directions of
// its dependencies and not whichever one the old argument carried (S37-09).
func TestRecoveryReplaysAnOpenCarryingBothLists(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	liveGoalForEdges(t, root, "01J5X00000000000000000BJ00", "held-by-n", OriginHuman)
	liveGoalForEdges(t, root, "01J5X00000000000000000BJ01", "needed-by-n", OriginMain)
	// The stranded open is the seat's own, so it names the goal that seat
	// holds: recovery replays the actor the entry carries and the seat rule
	// is judged again, exactly as it was live.
	if result, err := claimApprovedForTest(t, verbReq(root, "01J5X00000000000000000BJ03", "mac-a"), "held-by-n", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the seat claims the goal its blocker will name: %+v %v", result, err)
	}

	opid := Opid("01J5X00000000000000000BJ02", "mac-a", "lin-1")
	strandEntry(t, root, opid, PhaseCreated, Intent{
		Verb: "open", Targets: []string{"n", "held-by-n", "needed-by-n"},
		Args: map[string]string{
			"intent": "The dead owner's blocker.", "origin": "main", "next": "Finish it.",
			"blocks": "held-by-n", "blockedBy": "needed-by-n",
		},
	})
	if _, err := Recover(endpointFor(root)); err != nil {
		t.Fatal(err)
	}
	projection, err := Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	replayed := projection.Tree.Live["n"]
	if replayed == nil || replayed.State != StateParked || replayed.Parked == nil ||
		replayed.Parked.Blocker != "needed-by-n" || strings.Join(replayed.Blocked, ",") != "needed-by-n" {
		t.Fatalf("the replayed open lost the goals it waits for: %+v", replayed)
	}
	if held := projection.Tree.Live["held-by-n"]; held.State != StateParked || strings.Join(held.Blocked, ",") != "n" {
		t.Fatalf("the replayed open lost the goals that wait for it: state=%s blocked=%v", held.State, held.Blocked)
	}
	if replayed.History[0].Opid != opid {
		t.Fatalf("the replay did not carry the original operation: %s", replayed.History[0].Opid)
	}
	assertFilesParse(t, projection.Tree)
}

// A stored human name is not a credential: a recovered unblock that carries
// one is refused and the person runs it again from the boundary.
func TestRecoveryRefusesAJournaledUnblock(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := testBudget()
	liveGoalForEdges(t, root, "01J5X00000000000000000BV00", "dep-r", OriginMain)
	if result, err := OpenRisked(personReq(root, "01J5X00000000000000000BV01", "mac-a"), "waits-r",
		"Work behind live work.", OriginHuman, "Wait.", nil, []string{"dep-r"}, edgeRisk(), 0, "", &budget, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocked-by: %+v %v", result, err)
	}
	opid := Opid("01J5X00000000000000000BV02", "mac-a", "lin-1")
	strandEntry(t, root, opid, PhaseCreated, Intent{
		Verb: "unblock", Targets: []string{"waits-r", "dep-r"},
		Args: map[string]string{"blocker": "dep-r", "by": "Wido"},
	})
	reports, err := Recover(endpointFor(root))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, report := range reports {
		if report.Opid == opid {
			found = true
		}
	}
	if !found {
		t.Fatalf("recovery did not visit the stranded unblock: %+v", reports)
	}
	entry, err := ReadEntry(root, opid)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "cannot be replayed from journal text") {
		t.Fatalf("a journaled unblock was replayed: %+v", entry)
	}
	projection, err := Project(endpointFor(root), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if held := projection.Tree.Live["waits-r"]; held.State != StateParked || len(held.Blocked) != 1 {
		t.Fatalf("the refused replay changed the record: state=%s blocked=%v", held.State, held.Blocked)
	}
}
