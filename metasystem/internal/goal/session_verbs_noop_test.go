package goal

// Acts are idempotent: park, unpark and abandon (R-129-ui).
//
// An act whose effect already holds is success, not a refusal. A park of a
// parked goal used to answer LostToCompetitor, naming the operation that got
// there first; an unpark of a running goal refused in its own words; an abandon
// of an abandoned goal answered LostToCompetitor from the archive. All three
// reached the browser as refusals of an act the human had already made, and the
// card offered the press that would try again.
//
// So each is an explicit no-op under a signed-in browser session, in the form
// approve's and edit's already have: NothingToDo, with a sentence that names the
// session, and nothing written. A DIFFERENT request still lands — an unpark of a
// goal that IS parked, a park of a goal that is not — and a park carrying
// another reason is still a repeat of the same effect, so it is a no-op too.
//
// The rule is the browser session's alone. A terminal's second word is still its
// own word, and the second half of each pair below holds that.

import (
	"strings"
	"testing"
	"time"
)

var sessionNoOpNow = time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

// sessionNoOpUlid is one operation id per step of one of these tests. The
// letters keep it out of every other fixture's range in this package.
func sessionNoOpUlid(index, step int) string {
	return "01J5X0000000000000000SN" + string(rune('0'+index)) + string(rune('0'+step/10)) + string(rune('0'+step%10))
}

// sessionNoOpRequest is the request the act layer assembles for a browser: this
// human's name, the browser's lineage, and the session proof as the request's
// own authority.
func sessionNoOpRequest(t *testing.T, endpoint Endpoint, index, step int) VerbRequest {
	t.Helper()
	request := verbReqFor(endpoint, sessionNoOpUlid(index, step), "mac-ui")
	request.Actor.Human = "Wido"
	request.Actor.Lineage = "browser-session"
	request.Now = sessionNoOpNow
	request.Authority = sessionProofForTest(t, endpoint.Root, request.Now)
	return request
}

// sessionNoOpGoal opens one main-origin goal, which is the goal a seat may park
// without a human's proof — so the terminal halves below need nothing else.
func sessionNoOpGoal(t *testing.T, index int, id string) Endpoint {
	t.Helper()
	endpoint, _ := fakeGoalEndpoint(t)
	if result, err := Open(verbReqFor(endpoint, sessionNoOpUlid(index, 1), "mac-ui"), id,
		"Take "+id+" to a working end state.", OriginMain, "Start it."); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the fixture: %+v %v", result, err)
	}
	return endpoint
}

func TestAParkOfAParkedGoalFromASessionIsANoOp(t *testing.T) {
	t.Parallel()
	endpoint := sessionNoOpGoal(t, 1, "ui-park-twice")

	first, err := Park(sessionNoOpRequest(t, endpoint, 1, 2), "ui-park-twice", "waiting for the review")
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("the first park did not land: %+v %v", first, err)
	}
	tree, err := loadTreeFor(endpoint, first.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["ui-park-twice"]
	if file.State != StateParked {
		t.Fatalf("the first park did not park it: %+v", file)
	}
	revision, lines := file.Revision, len(file.History)

	// The second press of the same card: the ledger has nothing to do.
	second, err := Park(sessionNoOpRequest(t, endpoint, 1, 3), "ui-park-twice", "waiting for the review")
	if err != nil || second.Outcome != OutcomeAbandoned ||
		!strings.Contains(second.Detail, "the same pause from this signed-in session") {
		t.Fatalf("the park of a parked goal was not an explicit no-op: %+v %v", second, err)
	}

	// And a park carrying another reason is still a repeat of the same effect.
	third, err := Park(sessionNoOpRequest(t, endpoint, 1, 4), "ui-park-twice", "the design changed under it")
	if err != nil || third.Outcome != OutcomeAbandoned ||
		!strings.Contains(third.Detail, "the same pause from this signed-in session") {
		t.Fatalf("a park with another reason was not a no-op: %+v %v", third, err)
	}

	tree, err = loadTreeFor(endpoint, third.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file = tree.Live["ui-park-twice"]
	if file.Revision != revision || len(file.History) != lines {
		t.Fatalf("a repeated park wrote a second pause: revision %d, %d lines", file.Revision, len(file.History))
	}
	if file.Parked == nil || file.Parked.Because != "waiting for the review" {
		t.Fatalf("the pause the human made did not stand: %+v", file.Parked)
	}
}

func TestAParkOfAParkedGoalFromATerminalIsUnchanged(t *testing.T) {
	t.Parallel()
	endpoint := sessionNoOpGoal(t, 2, "seat-park-twice")

	if result, err := Park(verbReqFor(endpoint, sessionNoOpUlid(2, 2), "mac-ui"), "seat-park-twice",
		"the seat pauses it"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the first park did not land: %+v %v", result, err)
	}

	second, err := Park(verbReqFor(endpoint, sessionNoOpUlid(2, 3), "mac-ui"), "seat-park-twice",
		"the seat pauses it")
	if err != nil || second.Outcome != OutcomeLost || !strings.Contains(second.Detail, "winner: ") {
		t.Fatalf("a seat's repeated park was not the competitor refusal: %+v %v", second, err)
	}
}

func TestAnUnparkOfARunningGoalFromASessionIsANoOp(t *testing.T) {
	t.Parallel()
	endpoint := sessionNoOpGoal(t, 3, "ui-unpark-twice")
	before, err := loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	if err != nil {
		t.Fatal(err)
	}
	running := before.Live["ui-unpark-twice"]
	revision, lines := running.Revision, len(running.History)

	// The goal is queued, not parked: the effect an unpark asks for already
	// holds, so there is nothing to do.
	answered, err := Unpark(sessionNoOpRequest(t, endpoint, 3, 2), "ui-unpark-twice")
	if err != nil || answered.Outcome != OutcomeAbandoned ||
		!strings.Contains(answered.Detail, "the same resume from this signed-in session") {
		t.Fatalf("the unpark of a running goal was not an explicit no-op: %+v %v", answered, err)
	}
	tree, err := loadTreeFor(endpoint, answered.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["ui-unpark-twice"]
	if file.Revision != revision || len(file.History) != lines {
		t.Fatalf("the unpark of a running goal wrote something: revision %d, %d lines", file.Revision, len(file.History))
	}

	// An unpark of a goal that IS parked is a different request, and it lands.
	if result, err := Park(sessionNoOpRequest(t, endpoint, 3, 3), "ui-unpark-twice",
		"waiting for the review"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the park this test lifts: %+v %v", result, err)
	}
	lifted, err := Unpark(sessionNoOpRequest(t, endpoint, 3, 4), "ui-unpark-twice")
	if err != nil || lifted.Outcome != OutcomeConfirmed {
		t.Fatalf("the unpark of a parked goal did not land: %+v %v", lifted, err)
	}
	tree, err = loadTreeFor(endpoint, lifted.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.Live["ui-unpark-twice"].State; got != StateQueued {
		t.Fatalf("the lifted goal rests at %s, want %s", got, StateQueued)
	}
}

func TestAnUnparkOfARunningGoalFromATerminalIsUnchanged(t *testing.T) {
	t.Parallel()
	endpoint := sessionNoOpGoal(t, 4, "seat-unpark-twice")

	result, err := Unpark(verbReqFor(endpoint, sessionNoOpUlid(4, 2), "mac-ui"), "seat-unpark-twice")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not parked") {
		t.Fatalf("a seat's unpark of a running goal was not the engine's own refusal: %+v %v", result, err)
	}
}

func TestAnAbandonOfAnAbandonedGoalFromASessionIsANoOp(t *testing.T) {
	t.Parallel()
	endpoint := sessionNoOpGoal(t, 5, "ui-abandon-twice")

	request := sessionNoOpRequest(t, endpoint, 5, 2)
	first, err := Abandon(request, "ui-abandon-twice",
		AbandonSpec{Because: "the design changed"}, request.Authority)
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("the first abandon did not land: %+v %v", first, err)
	}
	tree, err := loadTreeFor(endpoint, first.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Abandoned["ui-abandon-twice"]
	if file == nil {
		t.Fatalf("the first abandon left the goal live: %+v", tree.Live)
	}
	revision, lines := file.Revision, len(file.History)

	// The second press of the same card, and a second one carrying another
	// reason: both ask for a state the archive is already in.
	repeat := sessionNoOpRequest(t, endpoint, 5, 3)
	second, err := Abandon(repeat, "ui-abandon-twice",
		AbandonSpec{Because: "the design changed"}, repeat.Authority)
	if err != nil || second.Outcome != OutcomeAbandoned ||
		!strings.Contains(second.Detail, "the same abandon from this signed-in session") {
		t.Fatalf("the abandon of an abandoned goal was not an explicit no-op: %+v %v", second, err)
	}
	other := sessionNoOpRequest(t, endpoint, 5, 4)
	third, err := Abandon(other, "ui-abandon-twice",
		AbandonSpec{Because: "nobody will ever work it"}, other.Authority)
	if err != nil || third.Outcome != OutcomeAbandoned ||
		!strings.Contains(third.Detail, "the same abandon from this signed-in session") {
		t.Fatalf("an abandon with another reason was not a no-op: %+v %v", third, err)
	}

	tree, err = loadTreeFor(endpoint, third.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file = tree.Abandoned["ui-abandon-twice"]
	if file.Revision != revision || len(file.History) != lines {
		t.Fatalf("a repeated abandon wrote a second record: revision %d, %d lines", file.Revision, len(file.History))
	}
	if file.Abandoned == nil || file.Abandoned.Because != "the design changed" {
		t.Fatalf("the abandon the human made did not stand: %+v", file.Abandoned)
	}
}

func TestAnAbandonOfAnAbandonedGoalFromATerminalIsUnchanged(t *testing.T) {
	t.Parallel()
	endpoint := sessionNoOpGoal(t, 6, "seat-abandon-twice")

	first := verbReqFor(endpoint, sessionNoOpUlid(6, 2), "mac-ui")
	first.Actor.Human = "Wido"
	first.Now = sessionNoOpNow
	firstProof := goalHumanProof(t, endpoint.Root, first.Now)
	first.Authority = firstProof
	if result, err := Abandon(first, "seat-abandon-twice",
		AbandonSpec{Because: "abandoned at the terminal"}, firstProof); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("the terminal's abandon did not land: %+v %v", result, err)
	}

	second := verbReqFor(endpoint, sessionNoOpUlid(6, 3), "mac-ui")
	second.Actor.Human = "Wido"
	second.Now = sessionNoOpNow
	secondProof := goalHumanProof(t, endpoint.Root, second.Now)
	second.Authority = secondProof
	result, err := Abandon(second, "seat-abandon-twice",
		AbandonSpec{Because: "abandoned at the terminal"}, secondProof)
	if err != nil || result.Outcome != OutcomeLost || !strings.Contains(result.Detail, "winner: ") {
		t.Fatalf("a terminal's repeated abandon was not the competitor refusal: %+v %v", result, err)
	}
}
