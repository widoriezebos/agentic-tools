package landing

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// TestObserveRefusesAGoalMovedOutOfItsClaimedState is the owner half of the
// abandonment-route-recertified port (scripts/agents/land-fixtures.sh): once
// a peer abandons the goal, the base ledger no longer holds it (the goal file
// left plans/goals for records/goals), so a register-carriage landing naming
// it is refused as goal-item-not-held for its former holder; a goal file in
// any state but claimed is refused the same way.
func TestObserveRefusesAGoalMovedOutOfItsClaimedState(t *testing.T) {
	t.Parallel()
	const at = "2026-09-03T08:00:00Z"
	const opid = "01ARZ3NDEKTSV4RRFFQ69G5FAW-m9-00000001"
	abandoned := string(goal.RenderFile(&goal.GoalFile{Id: "fx", State: goal.StateAbandoned,
		Intent: "Fixture ownership.", Origin: goal.OriginMain, NextStep: "none", OpenedAt: at, Revision: 1,
		Abandoned: &goal.AbandonRecord{By: "human:Wido", At: at, Revision: 1, Opid: opid, Because: "fixture"},
		History: []goal.HistoryLine{{At: at, Opid: opid, Verb: "abandon", Actor: "human:Wido",
			Targets: []string{"fx"}, Reason: "fixture", Keep: -1}}}))
	if _, problems := goal.ParseFile([]byte(abandoned)); len(problems) != 0 {
		t.Fatalf("invalid abandoned goal: %v", problems)
	}
	for _, ledgerPath := range []string{"records/goals/fx.md", "plans/goals/fx.md"} {
		t.Run(ledgerPath, func(t *testing.T) {
			f := newRepositoryObservationFixture(t)
			f.base(ledgerPath, abandoned)
			f.base("records/misc/base.md", "base\n")
			c := f.comparison(observeTreeC, "diff --git a/records/misc/base.md b/records/misc/base.md\n+append\n", "records/misc/base.md")
			c.declare("records/misc/base.md", observationText("base\n"), observationText("base\nappend\n"))
			if ledgerPath != "plans/goals/fx.md" {
				c.declare("plans/goals/fx.md", nil, nil)
			}
			got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage", Goal: "fx", Actor: "m9+L1"})
			if got.Code != "goal-item-not-held" || got.Verdict != "would-refuse" || !got.RefusesAgent {
				t.Fatalf("abandoned goal classified as %+v", got)
			}
		})
	}
}
