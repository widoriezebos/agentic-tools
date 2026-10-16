package act

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestBrowserGoalCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			candidate := f.Candidate(reference)
			a, err := Fixture(f.Root, "Wido", "terminal-fixture", f.Now)
			if err != nil {
				t.Fatal(err)
			}
			a.reads.fence = func(stateroot.Installation) error { return nil }
			// Endpoint resolution is the server's production composition.
			err = a.Open(opening("opened"))
			tip := f.Git("rev-parse", "refs/heads/metasystem/goals")
			if reference == "missing" {
				f.AssertRefusal(err)
				if tip != candidate {
					t.Fatalf("refused browser act published %s", tip)
				}
			} else if err != nil || tip == candidate {
				t.Fatalf("valid browser act: tip=%s error=%v", tip, err)
			}
		})
	}
}
