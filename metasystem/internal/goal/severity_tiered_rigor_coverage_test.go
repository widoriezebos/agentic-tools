package goal

import (
	"strings"
	"testing"
)

func TestSeverityTieredRigorAcceptedRiskLifecycle(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "risk-goal")
	proof := proveObligationHuman(t, root)
	req := obligationAuthorityVerbReq(root, "01J5X00000000000000000SR30", "mac-a")

	if _, err := AcceptedRiskDecision(req, "risk-goal", "F-1", "critic-a", "Wido", "bounded risk", &proof); err == nil || !strings.Contains(err.Error(), "human act") {
		t.Fatalf("accept-risk without the human actor = %v", err)
	}
	req.Actor.Human = "Wido"
	if _, err := AcceptedRiskDecision(req, "risk-goal", "F-1", "critic-a", "Wido", "bounded risk", nil); err == nil || !strings.Contains(err.Error(), "human approval requires") {
		t.Fatalf("accept-risk without authority proof = %v", err)
	}

	result, err := AcceptedRiskDecision(req, "risk-goal", "F-1", "critic-a", "Wido", "bounded risk", &proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("accept-risk = %+v, %v", result, err)
	}
	wantOpID := req.opid()
	gotOpID, err := AcceptedRiskDecisionOpID(root, "risk-goal", "F-1", "critic-a", req.Now)
	if err != nil || gotOpID != wantOpID {
		t.Fatalf("accepted-risk operation identifier = %q, %v; want %q", gotOpID, err, wantOpID)
	}
	if _, err := AcceptedRiskDecisionOpID(root, "risk-goal", "F-missing", "critic-a", req.Now); err == nil || !strings.Contains(err.Error(), "has no accepted-risk decision") {
		t.Fatalf("missing accepted-risk lookup = %v", err)
	}
	if _, err := AcceptedRiskDecisionOpID(root, "absent-goal", "F-1", "critic-a", req.Now); err == nil || !strings.Contains(err.Error(), "is absent") {
		t.Fatalf("absent goal lookup = %v", err)
	}
	req.Ulid = "01J5X00000000000000000SR35"
	result, err = AcceptedRiskDecision(req, "absent-goal", "F-1", "critic-a", "Wido", "absent risk", &proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "is not live") {
		t.Fatalf("accept-risk on an absent goal = %+v, %v", result, err)
	}

	req.Ulid = "01J5X00000000000000000SR31"
	result, err = AcceptedRiskDecision(req, "risk-goal", "F-1", "critic-a", "Wido", "same decision", &proof)
	if err != nil || result.Outcome != OutcomeConfirmed || result.Detail != "idempotent" {
		t.Fatalf("matching accepted-risk replay = %+v, %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000SR32"
	result, err = AcceptedRiskDecision(req, "risk-goal", "F-1", "critic-a", "Another human", "conflicting decision", &proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "already accepted by Wido") {
		t.Fatalf("conflicting accepted-risk replay = %+v, %v", result, err)
	}

	release := obligationAuthorityVerbReq(root, "01J5X00000000000000000SR33", "mac-a")
	if result, err = Release(release, "risk-goal"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("release risk fixture = %+v, %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000SR34"
	result, err = AcceptedRiskDecision(req, "risk-goal", "F-2", "critic-a", "Wido", "unclaimed risk", &proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "is not claimed") {
		t.Fatalf("accept-risk on an unclaimed goal = %+v, %v", result, err)
	}
	obligationAuthorityGit(t, root, "config", "goal.sync-branch", "main")
	if _, err := AcceptedRiskDecisionOpID(root, "risk-goal", "F-1", "critic-a", req.Now); err == nil || !strings.Contains(err.Error(), "must be fully qualified") {
		t.Fatalf("accepted-risk lookup with malformed endpoint = %v", err)
	}
}

func TestSeverityTieredRigorReviewObligationRefusals(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "review-refusals")
	owner := obligationAuthorityVerbReq(root, "01J5X00000000000000000SR40", "mac-a")
	obligation := ReviewObligation{Finding: "F-1", Chain: "critic-a", Artifact: "metasystem/a.go", Test: "prove: a"}

	result, err := DeferFindings(owner, "missing-goal", []ReviewObligation{obligation})
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "is not live") {
		t.Fatalf("defer on missing goal = %+v, %v", result, err)
	}
	owner.Ulid = "01J5X00000000000000000SR41"
	result, err = DeferFindings(owner, "review-refusals", []ReviewObligation{obligation, obligation})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("defer duplicate obligation = %+v, %v", result, err)
	}
	projection, err := Project(owner.Endpoint, false, owner.Now)
	if err != nil {
		t.Fatal(err)
	}
	if got := projection.Tree.Live["review-refusals"].ReviewObligations; len(got) != 1 || got[0].State != "open" {
		t.Fatalf("duplicate defer did not retain one open obligation: %+v", got)
	}

	foreign := obligationAuthorityVerbReq(root, "01J5X00000000000000000SR42", "mac-b")
	result, err = DeferFindings(foreign, "review-refusals", []ReviewObligation{{Finding: "F-2", Chain: "critic-a"}})
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "requires its owning pair") {
		t.Fatalf("foreign defer = %+v, %v", result, err)
	}
	if _, err := DischargeReviewObligation(owner, "review-refusals", "", "critic-a", "mac-a", "green"); err == nil || !strings.Contains(err.Error(), "requires --finding") {
		t.Fatalf("incomplete discharge = %v", err)
	}

	owner.Ulid = "01J5X00000000000000000SR43"
	result, err = DischargeReviewObligation(owner, "missing-goal", "F-1", "critic-a", "mac-a", "green")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "is not live") {
		t.Fatalf("discharge on missing goal = %+v, %v", result, err)
	}
	foreign.Ulid = "01J5X00000000000000000SR44"
	result, err = DischargeReviewObligation(foreign, "review-refusals", "F-1", "critic-a", "mac-b", "green")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "requires the human or owning pair") {
		t.Fatalf("foreign discharge = %+v, %v", result, err)
	}
	owner.Ulid = "01J5X00000000000000000SR45"
	result, err = DischargeReviewObligation(owner, "review-refusals", "F-missing", "critic-a", "mac-a", "green")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "no such obligation") {
		t.Fatalf("unknown discharge = %+v, %v", result, err)
	}
	owner.Ulid = "01J5X00000000000000000SR46"
	result, err = DischargeReviewObligation(owner, "review-refusals", "F-1", "critic-a", "mac-a", "green")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("owner discharge = %+v, %v", result, err)
	}
}

func TestSeverityTieredRigorUtilityWrappers(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "utility-goal")
	req := obligationAuthorityVerbReq(root, "01J5X00000000000000000SR50", "mac-a")
	if _, err := SetBudget(req, "utility-goal", testBudget()); err == nil || !strings.Contains(err.Error(), "human authority proof") {
		t.Fatalf("retired set-budget wrapper = %v", err)
	}

	ids := []string{"zeta", "alpha", "middle"}
	SortIds(ids)
	if got := strings.Join(ids, ","); got != "alpha,middle,zeta" {
		t.Fatalf("canonical identifier order = %q", got)
	}

	if _, err := SetBudgetApproved(req, "utility-goal", Budget{}, nil); err == nil || !strings.Contains(err.Error(), "invalid budget") {
		t.Fatalf("invalid approved budget = %v", err)
	}
	if _, err := SetBudgetApproved(req, "utility-goal", testBudget(), nil); err == nil || !strings.Contains(err.Error(), "requires --by") {
		t.Fatalf("approved budget without a human actor = %v", err)
	}
	req.Actor.Human = "Wido"
	if _, err := SetBudgetApproved(req, "utility-goal", testBudget(), nil); err == nil || !strings.Contains(err.Error(), "human approval requires") {
		t.Fatalf("approved budget without authority proof = %v", err)
	}

	proof := proveObligationHuman(t, root)
	req.Ulid = "01J5X00000000000000000SR51"
	result, err := SetBudgetApproved(req, "missing-goal", testBudget(), &proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "is not live") {
		t.Fatalf("approved budget on missing goal = %+v, %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000SR52"
	result, err = SetBudgetApproved(req, "utility-goal", testBudget(), &proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approved budget on claimed goal = %+v, %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000SR53"
	result, err = SetBudgetApproved(req, "utility-goal", testBudget(), &proof)
	if err != nil || result.Outcome != OutcomeAbandoned || !strings.Contains(result.Detail, "already reads exactly") {
		t.Fatalf("unchanged approved budget = %+v, %v", result, err)
	}
	unclaimedRoot := obligationAuthorityLocalRoot(t, "unclaimed-budget")
	release := obligationAuthorityVerbReq(unclaimedRoot, "01J5X00000000000000000SR54", "mac-a")
	if result, err = Release(release, "unclaimed-budget"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("release budget fixture = %+v, %v", result, err)
	}
	unclaimedProof := proveObligationHuman(t, unclaimedRoot)
	unclaimed := obligationAuthorityVerbReq(unclaimedRoot, "01J5X00000000000000000SR55", "mac-a")
	unclaimed.Actor.Human = "Wido"
	result, err = SetBudgetApproved(unclaimed, "unclaimed-budget", testBudget(), &unclaimedProof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "budgets on unclaimed work") {
		t.Fatalf("approved budget on unclaimed goal = %+v, %v", result, err)
	}

	if _, err := Prune(req, -1); err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("negative prune retention = %v", err)
	}
	if _, err := ParkArc(req, "utility-goal", " "); err == nil || !strings.Contains(err.Error(), "needs its reason") {
		t.Fatalf("arc park without reason = %v", err)
	}
	if _, err := SetArc(req, "utility-goal", ""); err == nil || !strings.Contains(err.Error(), "names its arc") {
		t.Fatalf("empty arc = %v", err)
	}

	noHuman := req
	noHuman.Actor.Human = ""
	if _, err := SetPin(noHuman, "utility-goal", "mac-a"); err == nil || !strings.Contains(err.Error(), "human act") {
		t.Fatalf("agent set-pin = %v", err)
	}
	if _, err := SetPin(req, "utility-goal", ""); err == nil || !strings.Contains(err.Error(), "names its machine") {
		t.Fatalf("empty pin = %v", err)
	}
	if _, err := SetPin(req, "utility-goal", "two words"); err == nil || !strings.Contains(err.Error(), "not a machine nickname") {
		t.Fatalf("whitespace pin = %v", err)
	}

	for _, invalid := range []string{
		"no-dashes",
		"01J5X00000000000000000SR50-mac-a-zzzzzzzz",
		"short-mac-a-12345678",
		"01J5X00000000000000000SRI0-mac-a-12345678",
	} {
		if validOpidShape(invalid) {
			t.Fatalf("invalid operation identifier accepted: %q", invalid)
		}
	}
	if validState("unknown") {
		t.Fatal("unknown goal state accepted")
	}
	if got := lastOpid(&GoalFile{}); got != "unknown" {
		t.Fatalf("empty goal history winner = %q", got)
	}
	if got := lastOpid(&GoalFile{History: []HistoryLine{{Opid: "winner"}}}); got != "winner" {
		t.Fatalf("recorded goal history winner = %q", got)
	}
	if !validPinnedNickname("mac-a") {
		t.Fatal("valid machine nickname refused")
	}
	tree := &TreeGoals{
		Live: map[string]*GoalFile{"live": {State: StateParked}},
		Done: map[string]*GoalFile{"done": {State: StateDone}},
	}
	if got := depState(tree, "live"); got != StateParked {
		t.Fatalf("live dependency state = %q", got)
	}
	if got := depState(tree, "done"); got != StateDone {
		t.Fatalf("done dependency state = %q", got)
	}
	if got := depState(tree, "missing"); got != "" {
		t.Fatalf("missing dependency state = %q", got)
	}
}
