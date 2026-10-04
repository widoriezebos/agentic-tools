package landing

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
)

func TestLandingDesignCheckGoverned(t *testing.T) {
	t.Parallel()
	fixture := newRepositoryObservationFixture(t)
	path := "plans/handoff-fixture-1.md"
	c := fixture.comparison(observeTreeB, "diff --git a/"+path+" b/"+path+"\n+# handoff\n", path)
	c.declare(path, nil, observationText("# handoff\n"))
	held := observationText(string(observationHeldGoal("g", "m1", "L")))
	c.declare("plans/goals/g.md", held, held)
	r := c.observe(ObserveParams{RepoRoot: fixture.root, CandidateTree: c.candidate, Goal: "g", Actor: "m1+L", DirectFix: "register-carriage", DesignFacts: func() DesignFacts {
		return DesignFacts{Facts: designgate.Facts{Goal: "g", Tier: 2, Mode: "refuse"}}
	}})
	if r.Code != "LANDING_DESIGN_NOT_STANDING" || r.Detail != "governed-by=R-146-m1k" || !r.RefusesAgent {
		t.Fatalf("landing refusal has no governing ruling: %+v", r)
	}
}
