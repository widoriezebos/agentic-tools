package dispatch

import (
	"testing"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestCloseFixtureRidesOnlyTheDesignRoundTwoBranch(t *testing.T) {
	t.Run("design_round_two", func(t *testing.T) {
		findings := []registerFinding{
			closeFinding("one", "mechanical", "group:fixture/one", "title one", critiqueModel.Bounded),
			closeFinding("two", "mechanical", "group:fixture/two", "title two", critiqueModel.Bounded),
		}
		repo, root, _ := writeCloseRoot(t, "design-critic", 2, findings, materialHistory(3, 2), 2, 2)
		var got []goal.ReviewObligation
		outcome, err := critiqueRegisterClose(repo, root, captureObligations(&got))
		check(t, err == nil && outcome == "deferred" && len(got) == len(findings), "design round-two branch did not defer both findings: outcome=%q obligations=%+v err=%v", outcome, got, err)
		for i, finding := range findings {
			check(t, got[i].Fixture == finding.Fixture && got[i].Test == "prove: "+finding.Fixture, "design round-two branch obligation %d did not carry its own fixture: %+v", i, got[i])
		}
	})
	t.Run("generic_exhaustion", func(t *testing.T) {
		finding := closeFinding("generic", "mechanical", "group:must-not-ride", "title first line\ntitle second line", critiqueModel.Bounded)
		repo, root, _ := writeCloseRoot(t, "code-critic", 2, []registerFinding{finding}, materialHistory(3, 2), 2, 2)
		var got []goal.ReviewObligation
		outcome, err := critiqueRegisterClose(repo, root, captureObligations(&got))
		check(t, err == nil && outcome == "deferred" && len(got) == 1, "generic exhaustion branch did not defer its finding: outcome=%q obligations=%+v err=%v", outcome, got, err)
		check(t, got[0].Fixture == "" && got[0].Test == "prove: title first line", "generic exhaustion branch carried fixture or wrong title: %+v", got[0])
	})
}
