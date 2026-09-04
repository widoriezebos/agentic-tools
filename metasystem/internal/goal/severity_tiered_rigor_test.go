package goal

import (
	"strings"
	"testing"
)

func TestConcludeRefusesWithReviewObligations(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "review-goal")
	req := obligationAuthorityVerbReq(root, "01J5X00000000000000000SR10", "mac-a")
	obligation := ReviewObligation{Finding: "F-1", Chain: "critic-a", Artifact: "NEW metasystem/a file.go", Test: "prove: it works", State: "open"}
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{obligation}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("defer = %+v, %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000SR11"
	result, err := Done(req, "review-goal", "premature")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "open review obligation") || !strings.Contains(result.Detail, "F-1") {
		t.Fatalf("done with obligation = %+v, %v", result, err)
	}
}

func TestSTR3GapDischargeSelectVerb(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "review-goal")
	req := obligationAuthorityVerbReq(root, "01J5X00000000000000000SR20", "mac-a")
	obligations := []ReviewObligation{
		{Finding: "F-1", Chain: "critic-a", Artifact: "metasystem/a.go", Test: "prove: a", State: "open"},
		{Finding: "F-1", Chain: "critic-b", Artifact: "metasystem/b.go", Test: "prove: b", State: "open"},
	}
	if result, err := DeferFindings(req, "review-goal", obligations); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("defer = %+v, %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000SR21"
	if result, err := DischargeReviewObligation(req, "review-goal", "F-1", "critic-b", "mac-a", `test=result="green"`); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("discharge = %+v, %v", result, err)
	}
	projection, err := Project(req.Endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	got := projection.Tree.Live["review-goal"].ReviewObligations
	if len(got) != 2 || got[0].State != "open" || got[1].State != "discharged" || got[1].Test != `test=result="green"` {
		t.Fatalf("chain-qualified discharge = %+v", got)
	}
}
