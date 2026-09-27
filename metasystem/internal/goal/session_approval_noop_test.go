package goal

// A signed-in session's identical approval is a no-op, exactly as a proven
// one and an attorney's one are.
//
// The browser is the one hand that can press Apply twice on one proposal: two
// tabs read the same card, and the second press arrives while the first act is
// still in flight. A proven approval answers that with an explicit no-op — the
// goal already carries this exact budget under this exact authority, unexpired,
// so there is nothing to write — and a session approval did not, so the second
// press wrote a SECOND approval record with a second History line. The record a
// human reads then says their one word was given twice.

import (
	"strings"
	"testing"
)

func TestIdenticalSignedInSessionApprovalIsANoOp(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	root := endpoint.Root
	if result, err := Open(verbReqFor(endpoint, "01J5X00000000000000000SN00", "mac-a"),
		"session-twice", "Approve one goal once.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the approval fixture: %+v %v", result, err)
	}

	human := verbReqFor(endpoint, "01J5X00000000000000000SN10", "mac-a")
	human.Actor.Human = "Wido"
	budget := testBudget()
	first, err := Approve(human, []string{"session-twice"}, &budget,
		sessionProofForTest(t, root, human.Now))
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("the first session approval did not land: %+v %v", first, err)
	}
	tree, err := loadTreeFor(endpoint, first.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["session-twice"]
	if file.Approved == nil || file.Approved.Authority != ApprovalAuthoritySession {
		t.Fatalf("the first approval was not the session's: %+v", file.Approved)
	}
	revision, lines, opid := file.Revision, len(file.History), file.Approved.Opid

	// The second press of the same card, under the same session, with the same
	// budget: the ledger has nothing to do.
	human.Ulid = "01J5X00000000000000000SN20"
	second, err := Approve(human, []string{"session-twice"}, &budget,
		sessionProofForTest(t, root, human.Now))
	if err != nil || second.Outcome != OutcomeAbandoned ||
		!strings.Contains(second.Detail, "same approval from this signed-in session") {
		t.Fatalf("the identical session approval was not an explicit no-op: %+v %v", second, err)
	}
	tree, err = loadTreeFor(endpoint, second.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file = tree.Live["session-twice"]
	if file.Revision != revision || len(file.History) != lines || file.Approved.Opid != opid {
		t.Fatalf("the second identical session approval wrote another approval: revision %d, %d lines, opid %s",
			file.Revision, len(file.History), file.Approved.Opid)
	}

	// A different budget is a different word, and it lands.
	human.Ulid = "01J5X00000000000000000SN30"
	other := testBudget()
	other.AttemptLimit = budget.AttemptLimit + 1
	third, err := Approve(human, []string{"session-twice"}, &other,
		sessionProofForTest(t, root, human.Now))
	if err != nil || third.Outcome != OutcomeConfirmed {
		t.Fatalf("a session approval with a different budget was refused: %+v %v", third, err)
	}
	tree, err = loadTreeFor(endpoint, third.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file = tree.Live["session-twice"]
	if file.Budget == nil || *file.Budget != other || file.Approved.Opid == opid {
		t.Fatalf("the changed budget did not land as a new approval: %+v %+v", file.Budget, file.Approved)
	}
	assertSessionApproval(t, file)
}
