package act

// The second press of one proposal's Apply.
//
// Two tabs read one card. The first press publishes; the second arrives while
// the first act is in flight, or after it landed and its outcome write failed.
// The engine answers the second one with an explicit no-op — the goal already
// carries exactly this budget under exactly this session's authority — and that
// no-op is the act HAVING ITS EFFECT, not a refusal: the goal is approved, with
// the budget the human confirmed. Calling it refused told the page that the
// approval had not been made when it had, and offered the press that would try
// to make another one.

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestASecondIdenticalApproveFromASessionAnswersApplied(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-twice")
	authority := sessionFor(t, bed)

	if err := authority.Approve("ui-twice", box()); err != nil {
		t.Fatalf("the first approve: %v", err)
	}
	first := readGoal(t, bed, "ui-twice")

	if err := authority.Approve("ui-twice", box()); err != nil {
		t.Fatalf("the second identical approve answered %v, want the act having its effect", err)
	}

	second := readGoal(t, bed, "ui-twice")
	testutil.Expect(t, "the goal is approved", second.State, goal.StateApproved)
	testutil.Expect(t, "with the budget the human confirmed", *second.Budget, box())
	testutil.Expect(t, "and one approval stands, not two", second.Approved.Opid, first.Approved.Opid)
	testutil.Expect(t, "nothing else was written", len(second.History), len(first.History))
	testutil.Expect(t, "the goal's revision did not move", second.Revision, first.Revision)
}

// A budget that differs is a different word: it lands, and it is not read as a
// no-op.
func TestAnApproveWithADifferentBudgetStillLands(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-rebudget")
	authority := sessionFor(t, bed)
	if err := authority.Approve("ui-rebudget", box()); err != nil {
		t.Fatalf("the first approve: %v", err)
	}
	before := readGoal(t, bed, "ui-rebudget")

	raised := box()
	raised.AttemptLimit = box().AttemptLimit + 1
	if err := authority.Approve("ui-rebudget", raised); err != nil {
		t.Fatalf("an approve with a different budget: %v", err)
	}

	file := readGoal(t, bed, "ui-rebudget")
	testutil.Expect(t, "the raised budget stands", *file.Budget, raised)
	if file.Approved.Opid == before.Approved.Opid {
		t.Fatal("the second approval did not record its own operation")
	}
}
