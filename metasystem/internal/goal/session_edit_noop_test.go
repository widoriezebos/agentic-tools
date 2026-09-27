package goal

// A signed-in session's identical edit is a no-op, as its identical approval
// already is.
//
// The browser is the one hand that can press Apply twice on one proposal: two
// tabs read the same card, and the second press arrives while the first act is
// still in flight or after its outcome write failed. Approve answers that with
// an explicit no-op; edit did not, so the second press wrote a SECOND edit with
// a second History line and a second revision, and the goal's record then says
// the same rewrite was made twice (Astra A-01, confirmation read).
//
// Only the fields that hand sends are read here — the intent, the next step and
// the labels — and only under a session proof. An edit that carries anything
// else, or a field that differs, is not this repeat and lands as it always did.

import (
	"strings"
	"testing"
)

func TestAnIdenticalSignedInSessionEditIsANoOp(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	root := endpoint.Root
	if result, err := Open(verbReqFor(endpoint, "01J5X00000000000000000SE00", "mac-a"),
		"session-edit-twice", "Rewrite one goal once.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the edit fixture: %+v %v", result, err)
	}

	human := verbReqFor(endpoint, "01J5X00000000000000000SE10", "mac-a")
	human.Actor.Human = "Wido"
	human.Authority = sessionProofForTest(t, root, human.Now)
	intent, next, labels := "The goal says what done looks like.", "Take it to a working end state.", []string{"ui"}
	fields := func() EditFields {
		held := append([]string(nil), labels...)
		return EditFields{QueuedOnly: true, Intent: &intent, NextStep: &next, Labels: &held}
	}

	first, err := Edit(human, "session-edit-twice", fields())
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("the first session edit did not land: %+v %v", first, err)
	}
	tree, err := loadTreeFor(endpoint, first.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["session-edit-twice"]
	if file.Intent != intent || file.NextStep != next {
		t.Fatalf("the first edit did not write the fields it carried: %+v", file)
	}
	revision, lines := file.Revision, len(file.History)

	// The second press of the same card, under the same session, with the same
	// three fields: the ledger has nothing to do.
	human.Ulid = "01J5X00000000000000000SE20"
	second, err := Edit(human, "session-edit-twice", fields())
	if err != nil || second.Outcome != OutcomeAbandoned ||
		!strings.Contains(second.Detail, "the same edit from this signed-in session") {
		t.Fatalf("the identical session edit was not an explicit no-op: %+v %v", second, err)
	}
	tree, err = loadTreeFor(endpoint, second.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file = tree.Live["session-edit-twice"]
	if file.Revision != revision || len(file.History) != lines {
		t.Fatalf("the second identical session edit wrote another edit: revision %d, %d lines",
			file.Revision, len(file.History))
	}

	// A field that differs is a different word, and it lands.
	human.Ulid = "01J5X00000000000000000SE30"
	changed := fields()
	other := "Take it to a working end state, then say so."
	changed.NextStep = &other
	third, err := Edit(human, "session-edit-twice", changed)
	if err != nil || third.Outcome != OutcomeConfirmed {
		t.Fatalf("a session edit with a changed next step was refused: %+v %v", third, err)
	}
	tree, err = loadTreeFor(endpoint, third.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file = tree.Live["session-edit-twice"]
	if file.NextStep != other || file.Revision != revision+1 {
		t.Fatalf("the changed next step did not land: %+v", file)
	}
}

// The no-op is the browser session's alone. A terminal repeats an edit under
// its own proven authority, and the engine records it exactly as it always
// has: this rule is about the one hand that has two tabs, and it does not
// quietly swallow anybody else's second word.
func TestAnIdenticalEditFromATerminalStillLands(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	if result, err := Open(verbReqFor(endpoint, "01J5X00000000000000000SE40", "mac-a"),
		"terminal-edit-twice", "Rewrite one goal twice.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the edit fixture: %+v %v", result, err)
	}

	human := verbReqFor(endpoint, "01J5X00000000000000000SE50", "mac-a")
	human.Actor.Human = "Wido"
	intent := "The goal says what done looks like."
	first, err := Edit(human, "terminal-edit-twice", EditFields{Intent: &intent})
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("the first edit did not land: %+v %v", first, err)
	}
	tree, err := loadTreeFor(endpoint, first.Tip)
	if err != nil {
		t.Fatal(err)
	}
	revision := tree.Live["terminal-edit-twice"].Revision

	human.Ulid = "01J5X00000000000000000SE60"
	second, err := Edit(human, "terminal-edit-twice", EditFields{Intent: &intent})
	if err != nil || second.Outcome != OutcomeConfirmed {
		t.Fatalf("the same edit from a terminal was not recorded: %+v %v", second, err)
	}
	tree, err = loadTreeFor(endpoint, second.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.Live["terminal-edit-twice"].Revision; got != revision+1 {
		t.Fatalf("the terminal's second edit left the revision at %d, want %d", got, revision+1)
	}
}
