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
// The acts below are the ones whose repeat the engine answers that way. The
// three that answer something else are at the foot, held by a test of their
// own so that the list this layer keeps cannot drift from the engine's.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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

// The three whose repeat is NOT the engine's no-op, held here so that the
// list this layer keeps stays the engine's own. Park and abandon name the
// operation that got there first; unpark says the goal is not parked. Each
// reaches the page as the engine's refusal, in its own words, which is what a
// human can act on.
func TestTheActsWhoseRepeatTheEngineRefusesInWords(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	for _, id := range []string{"ui-parked", "ui-resumed", "ui-gone"} {
		openGoal(t, bed, id)
	}
	authority := sessionFor(t, bed)
	if err := authority.Park("ui-parked", "waiting for the review"); err != nil {
		t.Fatalf("the park this test repeats: %v", err)
	}
	if err := authority.Abandon("ui-gone", "the design changed", ""); err != nil {
		t.Fatalf("the abandon this test repeats: %v", err)
	}

	for name, answered := range map[string]struct {
		err  error
		says string
	}{
		"a park of a parked goal":        {authority.Park("ui-parked", "waiting for the review"), "winner: "},
		"an unpark of a running goal":    {authority.Unpark("ui-resumed"), "not parked"},
		"an abandon of an abandoned one": {authority.Abandon("ui-gone", "the design changed", ""), "winner: "},
	} {
		refusal := refusalOf(t, answered.err)
		testutil.Expect(t, name+" is the engine refusing", refusal.Kind, KindEngine)
		if !strings.Contains(refusal.Message, answered.says) {
			t.Fatalf("%s answered %q, which does not carry %q", name, refusal.Message, answered.says)
		}
	}
	testutil.Expect(t, "the park stands", readGoal(t, bed, "ui-parked").State, goal.StateParked)
}
