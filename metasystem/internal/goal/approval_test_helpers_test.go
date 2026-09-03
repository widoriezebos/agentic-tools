package goal

import (
	"strings"
	"testing"
)

func testULIDVariant(ulid string, suffix byte) string {
	if len(ulid) == 0 {
		return ulid
	}
	prefix := byte('7')
	if suffix == 'X' {
		prefix = '6'
	}
	return string(prefix) + ulid[1:]
}

func approveGoalForTest(t *testing.T, req VerbRequest, id string, budget Budget) {
	t.Helper()
	human := req
	human.Actor.Human = "Wido"
	human.Ulid = testULIDVariant(req.Ulid, 'Y')
	proof := testHumanAuthority(t, req.Endpoint.Root, req.Now)
	result, err := Approve(human, []string{id}, &budget, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve fixture goal %s: %+v %v", id, result, err)
	}
	tree, err := loadTree(req.Endpoint.Root, result.Tip)
	if err != nil || tree.Live[id] == nil || tree.Live[id].State != StateApproved {
		t.Fatalf("approval fixture did not materialize approved state for %s: result=%+v goal=%+v err=%v", id, result, tree.Live[id], err)
	}
}

func claimApprovedForTest(t *testing.T, req VerbRequest, id string, budget Budget) (PublishResult, error) {
	t.Helper()
	approveGoalForTest(t, req, id, budget)
	return Claim(req, id)
}

func claimArcApprovedForTest(t *testing.T, req VerbRequest, id string, budget Budget) (PublishResult, error) {
	t.Helper()
	tree, err := loadTree(req.Endpoint.Root, acceptedTip(t, req.Endpoint.Root))
	if err != nil {
		t.Fatal(err)
	}
	members := arcMembers(tree, id)
	ids := make([]string, 0, len(members))
	for _, member := range members {
		if member.State == StateQueued {
			ids = append(ids, member.Id)
		}
	}
	if len(ids) > 0 {
		human := req
		human.Actor.Human = "Wido"
		human.Ulid = testULIDVariant(req.Ulid, 'Y')
		proof := testHumanAuthority(t, req.Endpoint.Root, req.Now)
		result, approveErr := Approve(human, ids, &budget, proof)
		if approveErr != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("approve fixture arc %s: %+v %v", id, result, approveErr)
		}
	}
	return ClaimArc(req, id)
}

func openClaimForTest(t *testing.T, req VerbRequest, id, intent, origin, nextStep string, budget Budget, labels ...string) (PublishResult, error) {
	t.Helper()
	openReq := req
	openReq.Ulid = testULIDVariant(req.Ulid, 'X')
	opened, err := Open(openReq, id, intent, origin, nextStep, labels...)
	if err != nil || opened.Outcome != OutcomeConfirmed {
		return opened, err
	}
	approveGoalForTest(t, req, id, budget)
	return Claim(req, id)
}

func setBudgetApprovedForTest(t *testing.T, req VerbRequest, id string, budget Budget) (PublishResult, error) {
	t.Helper()
	req.Actor.Human = "Wido"
	proof := testHumanAuthority(t, req.Endpoint.Root, req.Now)
	return SetBudgetApproved(req, id, budget, proof)
}

func acceptedTip(t *testing.T, root string) string {
	t.Helper()
	out, err := gitIn(root, "rev-parse", "--verify", AcceptedRef)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func approvedGoalFixture(f *GoalFile, budget Budget) *GoalFile {
	f.State = StateApproved
	f.Budget = &budget
	f.Revision++
	event := HistoryLine{
		At: "2026-08-20T10:01:00Z", Opid: "01J5X0000000000000000000BY-human-1a2b3c4d",
		Verb: "approve", Actor: "human:wido", Targets: []string{f.Id}, Keep: -1,
	}
	f.History = append(f.History, event)
	f.Approved = &ApprovalRecord{
		By: event.Actor, At: event.At, Revision: f.Revision, Opid: event.Opid,
		Authority: ApprovalAuthorityProven, Digest: ApprovalDigest(f.Intent, budget),
	}
	return f
}
