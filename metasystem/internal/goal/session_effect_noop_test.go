package goal

// Abandon's no-op is the EFFECT already holding, not the target's absence from
// the live tree (R-129-ui, F-04).
//
// The archive answers for a DONE goal as well as an abandoned one, and an
// abandon carries a successor the record either holds or does not. So the repeat
// is read from the record: the state must be abandoned, and the recorded
// successor must be the one this act asks for. Anything else is still the
// competitor refusal, because it asks for something the ledger does not say.
//
// The rule is the browser session's alone, and the bed below carries both hands
// so the terminal's own answer can be held beside it.
//
// The withdraw and open halves of the same rule are in
// session_repeat_noop_test.go, over this file's bed.

import (
	"strings"
	"testing"
	"time"
)

var sessionEffectNow = time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

// sessionEffectUlid is one operation id per step of one of these tests. The
// letters keep it out of every other fixture's range in this package.
func sessionEffectUlid(index, step int) string {
	return "01J5X0000000000000000SE" + string(rune('0'+index)) + string(rune('0'+step/10)) + string(rune('0'+step%10))
}

// sessionEffectRequest is the request the act layer assembles for a browser:
// this human's name, the browser's lineage, and the session proof as the
// request's own authority.
func sessionEffectRequest(t *testing.T, endpoint Endpoint, index, step int) VerbRequest {
	t.Helper()
	request := verbReqFor(endpoint, sessionEffectUlid(index, step), "mac-ui")
	request.Actor.Human = "Wido"
	request.Actor.Lineage = "browser-session"
	request.Now = sessionEffectNow
	request.Authority = sessionProofForTest(t, endpoint.Root, request.Now)
	return request
}

// sessionEffectTerminalRequest is the same act from the enrolled terminal: a
// person's own hand, with the ancestry proof rather than a session's.
func sessionEffectTerminalRequest(t *testing.T, endpoint Endpoint, index, step int) VerbRequest {
	t.Helper()
	request := verbReqFor(endpoint, sessionEffectUlid(index, step), "mac-ui")
	request.Actor.Human = "Wido"
	request.Now = sessionEffectNow
	request.Authority = goalHumanProof(t, endpoint.Root, request.Now)
	return request
}

// sessionEffectGoals opens main-origin goals, which are the goals a seat may
// act on without a human's proof — so the terminal halves need nothing else.
func sessionEffectGoals(t *testing.T, index int, ids ...string) Endpoint {
	t.Helper()
	endpoint, _ := fakeGoalEndpoint(t)
	for at, id := range ids {
		if result, err := Open(verbReqFor(endpoint, sessionEffectUlid(index, 90+at), "mac-ui"), id,
			"Take "+id+" to a working end state.", OriginMain, "Start it."); err != nil ||
			result.Outcome != OutcomeConfirmed {
			t.Fatalf("open the fixture %s: %+v %v", id, result, err)
		}
	}
	return endpoint
}

func TestARepeatedAbandonCarryingTheSameSuccessorFromASessionIsANoOp(t *testing.T) {
	t.Parallel()
	for _, shape := range []struct {
		named, carried string
		index          int
	}{
		{named: "no-successor", carried: "", index: 1},
		{named: "the-same-successor", carried: "ui-effect-heir", index: 2},
	} {
		shape := shape
		t.Run(shape.named, func(t *testing.T) {
			t.Parallel()
			endpoint := sessionEffectGoals(t, shape.index, "ui-effect-abandon", "ui-effect-heir")
			spec := AbandonSpec{Because: "the design changed", Carried: shape.carried}

			first := sessionEffectRequest(t, endpoint, shape.index, 2)
			landed, err := Abandon(first, "ui-effect-abandon", spec, first.Authority)
			if err != nil || landed.Outcome != OutcomeConfirmed {
				t.Fatalf("the first abandon did not land: %+v %v", landed, err)
			}
			tree, err := loadTreeFor(endpoint, landed.Tip)
			if err != nil {
				t.Fatal(err)
			}
			file := tree.Abandoned["ui-effect-abandon"]
			if file == nil {
				t.Fatalf("the first abandon left the goal live: %+v", tree.Live)
			}
			revision, lines := file.Revision, len(file.History)

			repeat := sessionEffectRequest(t, endpoint, shape.index, 3)
			second, err := Abandon(repeat, "ui-effect-abandon", spec, repeat.Authority)
			if err != nil || second.Outcome != OutcomeAbandoned ||
				!strings.Contains(second.Detail, "the same abandon from this signed-in session") {
				t.Fatalf("the repeated abandon was not an explicit no-op: %+v %v", second, err)
			}

			tree, err = loadTreeFor(endpoint, second.Tip)
			if err != nil {
				t.Fatal(err)
			}
			file = tree.Abandoned["ui-effect-abandon"]
			if file.Revision != revision || len(file.History) != lines {
				t.Fatalf("a repeated abandon wrote a second record: revision %d, %d lines", file.Revision, len(file.History))
			}
			if file.Abandoned == nil || file.Abandoned.Carried != shape.carried {
				t.Fatalf("the successor the human named did not stand: %+v", file.Abandoned)
			}
		})
	}
}

// An abandon asking for a successor the record does not carry is a different
// act, and it is refused: the successor is never recorded, so answering it
// applied would tell the human the ledger points somewhere it does not.
func TestAnAbandonAskingAnotherSuccessorFromASessionIsRefused(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 3, "ui-effect-repoint", "ui-effect-other-heir")

	first := sessionEffectRequest(t, endpoint, 3, 2)
	landed, err := Abandon(first, "ui-effect-repoint", AbandonSpec{Because: "the design changed"}, first.Authority)
	if err != nil || landed.Outcome != OutcomeConfirmed {
		t.Fatalf("the first abandon did not land: %+v %v", landed, err)
	}

	asking := sessionEffectRequest(t, endpoint, 3, 3)
	second, err := Abandon(asking, "ui-effect-repoint",
		AbandonSpec{Because: "the design changed", Carried: "ui-effect-other-heir"}, asking.Authority)
	if err != nil || second.Outcome != OutcomeLost || !strings.Contains(second.Detail, "winner: ") {
		t.Fatalf("an abandon asking another successor was not the competitor refusal: %+v %v", second, err)
	}

	tree, err := loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Abandoned["ui-effect-repoint"]
	if file == nil || file.Abandoned == nil || file.Abandoned.Carried != "" {
		t.Fatalf("the refused successor was recorded anyway: %+v", file.Abandoned)
	}
}

// A done goal is archived too, and an abandon of it asks for a state the
// ledger is not in: the refusal stands and the goal stays done.
func TestAnAbandonOfADoneGoalFromASessionIsRefused(t *testing.T) {
	t.Parallel()
	endpoint := sessionEffectGoals(t, 4, "ui-effect-done")
	budget := testBudget()
	if result, err := claimApprovedForTest(t, verbReqFor(endpoint, sessionEffectUlid(4, 2), "mac-ui"),
		"ui-effect-done", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim the goal this test concludes: %+v %v", result, err)
	}
	if result, err := Done(verbReqFor(endpoint, sessionEffectUlid(4, 3), "mac-ui"), "ui-effect-done",
		"Finished it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("conclude the goal this test abandons: %+v %v", result, err)
	}

	asking := sessionEffectRequest(t, endpoint, 4, 4)
	refused, err := Abandon(asking, "ui-effect-done",
		AbandonSpec{Because: "nobody will ever work it"}, asking.Authority)
	if err != nil || refused.Outcome != OutcomeLost || !strings.Contains(refused.Detail, "winner: ") {
		t.Fatalf("the abandon of a done goal was not the competitor refusal: %+v %v", refused, err)
	}

	tree, err := loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	if err != nil {
		t.Fatal(err)
	}
	if file := tree.Done["ui-effect-done"]; file == nil || file.State != StateDone {
		t.Fatalf("the done goal did not stay done: %+v", file)
	}
	if tree.Abandoned["ui-effect-done"] != nil {
		t.Fatalf("the refused abandon moved the goal to the abandoned set: %+v", tree.Abandoned["ui-effect-done"])
	}
}
