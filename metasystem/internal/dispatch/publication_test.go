package dispatch

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
	"testing"
)

func TestDispatchGoalCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			candidate := f.Candidate(reference)
			_, err := deferReviewObligations(f.Root, "review-fixture", "held", "fixture", "session", 0, []goal.ReviewObligation{{Finding: "one", Chain: "review-fixture", Artifact: "the change", Test: "TestOne"}})
			tip := f.Git("rev-parse", goal.LocalLedgerBranch)
			if reference == "missing" {
				f.AssertRefusal(err)
				if tip != candidate {
					t.Fatalf("refused dispatch published %s", tip)
				}
			} else if err != nil || tip == candidate {
				t.Fatalf("valid dispatch: tip=%s error=%v", tip, err)
			}
		})
	}
}
