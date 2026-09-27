package act

// No effect twice.
//
// A proposal's Apply can be pressed a second time — the first press's outcome
// write failed, or a second tab had the same card — and the second press asks
// the ledger for a state it is already in. The engine answers that with an
// explicit no-op, which is the act HAVING its effect rather than a refusal of
// it, and the page is told applied: nothing new landed, so nothing new is
// recorded, and no second authority proof is written beside it.
//
// Every act of R-129-ui answers that way, which is what the table at the foot
// holds, act by act. Nothing is excluded any more: park, unpark and abandon
// used to reach the page as the engine's refusals of an act the human had
// already made, and the ruling closed that.

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// repeating runs one act twice from one signed-in session and says that the
// second answered applied without writing anything.
func repeating(t *testing.T, id string, bed *ledgerBed, again func() error) {
	t.Helper()
	if err := again(); err != nil {
		t.Fatalf("the first press: %v", err)
	}
	before := readGoal(t, bed, id)

	if err := again(); err != nil {
		t.Fatalf("the second identical press answered %v, want the act having its effect", err)
	}

	after := readGoal(t, bed, id)
	testutil.Expect(t, "the goal's revision did not move", after.Revision, before.Revision)
	testutil.Expect(t, "and nothing was recorded a second time", len(after.History), len(before.History))
}

func TestASecondIdenticalEditFromASessionAnswersApplied(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-edit-twice")
	authority := sessionFor(t, bed)

	repeating(t, "ui-edit-twice", bed, func() error {
		return authority.Edit("ui-edit-twice", Edited{Intent: intent("One rewritten intent.")})
	})
	testutil.Expect(t, "the intent the human sent stands",
		readGoal(t, bed, "ui-edit-twice").Intent, "One rewritten intent.")
}

func TestASecondIdenticalRerankFromASessionAnswersApplied(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-rank-twice")
	authority := sessionFor(t, bed)

	repeating(t, "ui-rank-twice", bed, func() error {
		return authority.SetPriority("ui-rank-twice", 2, sequence(1))
	})
	testutil.Expect(t, "the goal is where the human put it", rank(t, bed, "ui-rank-twice"), ranked{2, 1})
}

func TestASecondIdenticalBlockFromASessionAnswersApplied(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-waits")
	openGoal(t, bed, "ui-blocker")
	authority := sessionFor(t, bed)

	repeating(t, "ui-waits", bed, func() error {
		return authority.Block("ui-waits", "ui-blocker")
	})
	testutil.Expect(t, "the edge the human asked for stands",
		readGoal(t, bed, "ui-waits").Blocked, []string{"ui-blocker"})
}

func TestASecondIdenticalUnblockFromASessionAnswersApplied(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-freed")
	openGoal(t, bed, "ui-holder")
	authority := sessionFor(t, bed)
	if err := authority.Block("ui-freed", "ui-holder"); err != nil {
		t.Fatalf("the edge this test removes: %v", err)
	}

	repeating(t, "ui-freed", bed, func() error {
		return authority.Unblock("ui-freed", "ui-holder")
	})
	testutil.Expect(t, "the edge is gone", len(readGoal(t, bed, "ui-freed").Blocked), 0)
}

// Every act of the ruling, in one test: a repeat answers applied.
//
// R-129-ui is the whole list — "pausing a paused goal, resuming a running one,
// abandoning an abandoned one, approving with the approval that stands, editing
// to what the goal already says, blocking on a blocker already named, unblocking
// one already absent, prioritizing to the rank already held" — and this says the
// list holds, act by act, through the layer a route calls.
//
// It replaces the test that pinned the three the engine used to refuse in words.
// That test existed so the exclusions this layer kept could not drift from the
// engine's; there are no exclusions left, so what has to be held is the rule
// itself.
func TestEveryActOfTheRulingAnswersItsRepeatApplied(t *testing.T) {
	t.Parallel()
	for _, act := range []struct {
		named string
		// id is the goal the act is made on, and the second goal where the act
		// is an edge.
		id, other string
		// before is what the bed does so that the act's effect can already
		// stand, and again is the act itself, run twice.
		before func(Authority) error
		again  func(Authority) string
	}{
		{named: "approve", id: "ui-ruled-approve",
			again: func(a Authority) string { return errorOf(a.Approve("ui-ruled-approve", box())) }},
		{named: "edit", id: "ui-ruled-edit",
			again: func(a Authority) string {
				return errorOf(a.Edit("ui-ruled-edit", Edited{Intent: intent("One rewritten intent.")}))
			}},
		{named: "prioritize", id: "ui-ruled-rank",
			again: func(a Authority) string { return errorOf(a.SetPriority("ui-ruled-rank", 2, sequence(1))) }},
		{named: "block", id: "ui-ruled-waits", other: "ui-ruled-blocker",
			again: func(a Authority) string { return errorOf(a.Block("ui-ruled-waits", "ui-ruled-blocker")) }},
		{named: "unblock", id: "ui-ruled-freed", other: "ui-ruled-holder",
			before: func(a Authority) error { return a.Block("ui-ruled-freed", "ui-ruled-holder") },
			again:  func(a Authority) string { return errorOf(a.Unblock("ui-ruled-freed", "ui-ruled-holder")) }},
		{named: "pause", id: "ui-ruled-park",
			again: func(a Authority) string { return errorOf(a.Park("ui-ruled-park", "waiting for the review")) }},
		// Resume is the one act whose repeat needs no first press: the goal is
		// running, which is the state an unpark asks for, so the FIRST unpark is
		// already the act whose effect stands.
		{named: "resume", id: "ui-ruled-unpark",
			again: func(a Authority) string { return errorOf(a.Unpark("ui-ruled-unpark")) }},
	} {
		act := act
		t.Run(act.named, func(t *testing.T) {
			t.Parallel()
			bed := ledger(t)
			openGoal(t, bed, act.id)
			if act.other != "" {
				openGoal(t, bed, act.other)
			}
			authority := sessionFor(t, bed)
			if act.before != nil {
				if err := act.before(authority); err != nil {
					t.Fatalf("the state this act repeats over: %v", err)
				}
			}
			if said := act.again(authority); said != "" {
				t.Fatalf("the first press: %s", said)
			}
			before := readGoal(t, bed, act.id)

			if said := act.again(authority); said != "" {
				t.Fatalf("the second identical press answered %s, want the act having its effect", said)
			}

			after := readGoal(t, bed, act.id)
			testutil.Expect(t, "the goal's revision did not move", after.Revision, before.Revision)
			testutil.Expect(t, "and nothing was recorded a second time", len(after.History), len(before.History))
		})
	}
}

// Abandon is the eighth, apart because its goal leaves the live tree: an
// abandoned goal is read from the archive, so the shape above cannot read it.
func TestASecondIdenticalAbandonFromASessionAnswersApplied(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-ruled-abandon")
	authority := sessionFor(t, bed)

	if err := authority.Abandon("ui-ruled-abandon", "the design changed", ""); err != nil {
		t.Fatalf("the first press: %v", err)
	}
	before := readAbandoned(t, bed, "ui-ruled-abandon")

	if err := authority.Abandon("ui-ruled-abandon", "the design changed", ""); err != nil {
		t.Fatalf("the second identical press answered %v, want the act having its effect", err)
	}

	after := readAbandoned(t, bed, "ui-ruled-abandon")
	testutil.Expect(t, "the goal's revision did not move", after.Revision, before.Revision)
	testutil.Expect(t, "and nothing was recorded a second time", len(after.History), len(before.History))
	testutil.Expect(t, "the reason the human gave stands", after.Abandoned.Because, "the design changed")
}

// errorOf is what an act answered, as words: the table above holds one call per
// act and reads only whether it answered at all.
func errorOf(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
