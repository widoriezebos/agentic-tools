package goal

// Withdraw and open answer their repeat too (R-129-ui, F-05).
//
// They are the two acts whose repeat is decided by a COMPARISON rather than by
// the target's mere existence, which is why they were excluded for a round: an
// unapprove reads the LIVE goal's own approval, and an open reads the record the
// id already names against what the request states. Anything that differs is a
// second act on one id, and it keeps the engine's own refusal.
//
// The bed is session_effect_noop_test.go's: the browser's session request, the
// enrolled terminal's, and a ledger of goals to act on.

import (
	"strings"
	"testing"
)

// A withdrawal of a goal that carries no approval asks for the state the goal
// is already in: under a session that is the act having its effect. This is the
// retry the ruling names — the withdrawal landed and its answer was lost — so
// the shape is the act twice over, not a goal that was never approved.
func TestAWithdrawOfAnUnapprovedGoalFromASessionIsANoOp(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 5, "ui-effect-withdraw")
	budget := testBudget()
	approveGoalForTest(t, verbReqFor(endpoint, sessionEffectUlid(5, 2), "mac-ui"), "ui-effect-withdraw", budget)

	landed, err := Unapprove(sessionEffectRequest(t, endpoint, 5, 3), "ui-effect-withdraw",
		"the board changed", sessionProofForTest(t, endpoint.Root, sessionEffectNow))
	if err != nil || landed.Outcome != OutcomeConfirmed {
		t.Fatalf("the first withdrawal did not land: %+v %v", landed, err)
	}
	tree, err := loadTreeFor(endpoint, landed.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["ui-effect-withdraw"]
	if file == nil || file.Approved != nil {
		t.Fatalf("the first withdrawal did not remove the approval: %+v", file)
	}
	revision, lines := file.Revision, len(file.History)

	// The second press of the same card: the goal carries no approval, which
	// is what this act asks for.
	second, err := Unapprove(sessionEffectRequest(t, endpoint, 5, 4), "ui-effect-withdraw",
		"the board changed", sessionProofForTest(t, endpoint.Root, sessionEffectNow))
	if err != nil || second.Outcome != OutcomeAbandoned ||
		!strings.Contains(second.Detail, "the same withdrawal from this signed-in session") {
		t.Fatalf("the withdrawal of an unapproved goal was not an explicit no-op: %+v %v", second, err)
	}

	tree, err = loadTreeFor(endpoint, second.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file = tree.Live["ui-effect-withdraw"]
	if file.Revision != revision || len(file.History) != lines {
		t.Fatalf("a repeated withdrawal wrote a second record: revision %d, %d lines", file.Revision, len(file.History))
	}
}

// The rule reaches a LIVE goal and no other: a goal that has left the live tree
// is not a goal whose approval this act can speak about, and it keeps the
// engine's own refusal.
func TestAWithdrawOfAGoalThatIsNotLiveFromASessionIsRefused(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 6, "ui-effect-gone")

	leaving := sessionEffectRequest(t, endpoint, 6, 2)
	if result, err := Abandon(leaving, "ui-effect-gone",
		AbandonSpec{Because: "the design changed"}, leaving.Authority); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("the abandon this test withdraws over: %+v %v", result, err)
	}

	refused, err := Unapprove(sessionEffectRequest(t, endpoint, 6, 3), "ui-effect-gone",
		"the board changed", sessionProofForTest(t, endpoint.Root, sessionEffectNow))
	if err != nil || refused.Outcome != OutcomeRejected ||
		!strings.Contains(refused.Detail, "APPROVAL_REQUIRED") {
		t.Fatalf("the withdrawal of a goal that is not live was not the engine's own refusal: %+v %v", refused, err)
	}
}

func TestAWithdrawOfAnUnapprovedGoalFromATerminalIsUnchanged(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 7, "seat-effect-withdraw")

	request := sessionEffectTerminalRequest(t, endpoint, 7, 2)
	refused, err := Unapprove(request, "seat-effect-withdraw", "the board changed", request.Authority)
	if err != nil || refused.Outcome != OutcomeRejected ||
		!strings.Contains(refused.Detail, "APPROVAL_REQUIRED") {
		t.Fatalf("a terminal's withdrawal of an unapproved goal was not the engine's own refusal: %+v %v", refused, err)
	}
}

// sessionEffectOpened is the one open this file repeats: every field the act
// layer sends, so the comparison behind the no-op is exercised whole.
type sessionEffectOpened struct {
	intent, nextStep, why string
	tier                  uint8
	labels                []string
	blocks, blockedBy     []string
}

func sessionEffectOpen() sessionEffectOpened {
	return sessionEffectOpened{
		intent: "The defect both goals wait for.", nextStep: "Fix it.",
		labels: []string{"ui", "browser"},
		blocks: []string{"ui-effect-holds"}, blockedBy: []string{"ui-effect-waits-for"},
	}
}

func sessionEffectOpening(t *testing.T, endpoint Endpoint, index, step int, id string, opened sessionEffectOpened) (PublishResult, error) {
	t.Helper()
	budget := testBudget()
	request := sessionEffectRequest(t, endpoint, index, step)
	return OpenRisked(request, id, opened.intent, OriginHuman, opened.nextStep,
		opened.blocks, opened.blockedBy, edgeRisk(), opened.tier, opened.why, &budget,
		request.Authority, opened.labels...)
}

// An open whose goal already reads exactly this way is the open HAVING its
// effect: the second press of one intake sheet, after the first press's answer
// was lost.
func TestARepeatedOpenOfTheSameGoalFromASessionIsANoOp(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 8, "ui-effect-holds", "ui-effect-waits-for")

	landed, err := sessionEffectOpening(t, endpoint, 8, 2, "ui-effect-open", sessionEffectOpen())
	if err != nil || landed.Outcome != OutcomeConfirmed {
		t.Fatalf("the first open did not land: %+v %v", landed, err)
	}
	tree, err := loadTreeFor(endpoint, landed.Tip)
	if err != nil {
		t.Fatal(err)
	}
	opened, held := tree.Live["ui-effect-open"], tree.Live["ui-effect-holds"]
	if opened == nil || held == nil || !contains(held.Blocked, "ui-effect-open") {
		t.Fatalf("the first open did not record its edges: opened=%+v held=%+v", opened, held)
	}
	revision, lines := opened.Revision, len(opened.History)
	heldRevision, heldLines := held.Revision, len(held.History)

	second, err := sessionEffectOpening(t, endpoint, 8, 3, "ui-effect-open", sessionEffectOpen())
	if err != nil || second.Outcome != OutcomeAbandoned ||
		!strings.Contains(second.Detail, "the same open from this signed-in session") {
		t.Fatalf("the repeated open was not an explicit no-op: %+v %v", second, err)
	}

	tree, err = loadTreeFor(endpoint, second.Tip)
	if err != nil {
		t.Fatal(err)
	}
	opened, held = tree.Live["ui-effect-open"], tree.Live["ui-effect-holds"]
	if opened.Revision != revision || len(opened.History) != lines {
		t.Fatalf("a repeated open wrote a second record: revision %d, %d lines", opened.Revision, len(opened.History))
	}
	if held.Revision != heldRevision || len(held.History) != heldLines {
		t.Fatalf("a repeated open parked the blocked goal again: revision %d, %d lines", held.Revision, len(held.History))
	}
}

// An open of an id the ledger carries that states anything else is a SECOND
// act on one id, not a repeat of the first: the competitor refusal stands, and
// nothing the second open asked for is written.
func TestAnOpenOfAnExistingGoalThatDiffersFromASessionIsRefused(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 9, "ui-effect-holds", "ui-effect-waits-for", "ui-effect-spare")

	if result, err := sessionEffectOpening(t, endpoint, 9, 2, "ui-effect-differs", sessionEffectOpen()); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("the open this test repeats over: %+v %v", result, err)
	}

	for at, differing := range []struct {
		named  string
		opened sessionEffectOpened
	}{
		{named: "another intent", opened: func() sessionEffectOpened {
			opened := sessionEffectOpen()
			opened.intent = "Something else entirely."
			return opened
		}()},
		{named: "another next step", opened: func() sessionEffectOpened {
			opened := sessionEffectOpen()
			opened.nextStep = "Do something else."
			return opened
		}()},
		{named: "another tier", opened: func() sessionEffectOpened {
			opened := sessionEffectOpen()
			opened.tier, opened.why = 2, "the blast radius is wider than the answers say"
			return opened
		}()},
		{named: "another label set", opened: func() sessionEffectOpened {
			opened := sessionEffectOpen()
			opened.labels = []string{"ui"}
			return opened
		}()},
		{named: "another goal to wait for", opened: func() sessionEffectOpened {
			opened := sessionEffectOpen()
			opened.blockedBy = nil
			return opened
		}()},
		{named: "another goal to unblock", opened: func() sessionEffectOpened {
			opened := sessionEffectOpen()
			opened.blocks = []string{"ui-effect-spare"}
			return opened
		}()},
	} {
		result, err := sessionEffectOpening(t, endpoint, 9, 10+at, "ui-effect-differs", differing.opened)
		if err != nil || result.Outcome != OutcomeLost || !strings.Contains(result.Detail, "winner: ") {
			t.Fatalf("an open stating %s was not the competitor refusal: %+v %v", differing.named, result, err)
		}
	}

	tree, err := loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	if err != nil {
		t.Fatal(err)
	}
	opened := tree.Live["ui-effect-differs"]
	if opened == nil || opened.Intent != "The defect both goals wait for." || opened.NextStep != "Fix it." ||
		opened.Tier != 1 || len(opened.Labels) != 2 {
		t.Fatalf("a refused open changed the goal the human opened: %+v", opened)
	}
	if spare := tree.Live["ui-effect-spare"]; spare == nil || len(spare.Blocked) != 0 {
		t.Fatalf("a refused open wrote its edge anyway: %+v", spare)
	}
}

func TestARepeatedOpenFromATerminalIsUnchanged(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 0, "seat-effect-holds")
	budget := testBudget()

	first := sessionEffectTerminalRequest(t, endpoint, 0, 2)
	if result, err := OpenRisked(first, "seat-effect-open", "The work a person asked for.", OriginHuman,
		"Start it.", nil, nil, edgeRisk(), 0, "", &budget, first.Authority); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("the terminal's open did not land: %+v %v", result, err)
	}

	second := sessionEffectTerminalRequest(t, endpoint, 0, 3)
	result, err := OpenRisked(second, "seat-effect-open", "The work a person asked for.", OriginHuman,
		"Start it.", nil, nil, edgeRisk(), 0, "", &budget, second.Authority)
	if err != nil || result.Outcome != OutcomeLost || !strings.Contains(result.Detail, "winner: ") {
		t.Fatalf("a terminal's repeated open was not the competitor refusal: %+v %v", result, err)
	}
}
