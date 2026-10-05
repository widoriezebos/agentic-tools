package dispatch

import (
	"path/filepath"
	"strings"
	"testing"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A wider review budget cannot admit more than two design examinations.

func fiveRoundHistory(materials ...int64) []any {
	history := make([]any, 0, len(materials))
	for index, material := range materials {
		history = append(history, map[string]any{"round": int64(index + 1), "material": material})
	}
	return history
}

func TestDesignCapFiveRoundTwoResidueDefersFixtures(t *testing.T) {
	t.Parallel()
	findings := []registerFinding{closeFinding("mechanical", "mechanical", "go test ./m", "title", critiqueModel.Bounded)}
	repo, root, _ := writeCloseRoot(t, "design-critic", 2, findings, materialHistory(3, 2), 5, 2)
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", root, "rounds", "1"), "return.json", map[string]any{"findings": []any{map[string]any{"severity": "critical"}}})
	var got []goal.ReviewObligation
	outcome, err := critiqueRegisterClose(repo, root, captureObligations(&got))
	check(t, err == nil && outcome == "deferred" && len(got) == 1,
		"terminal design residue did not defer: obligations=%+v err=%v", got, err)
}

func TestDesignCapFiveRoundTwoBlockerRaisesTheHuman(t *testing.T) {
	t.Parallel()
	findings := []registerFinding{closeFinding("severe", "mechanical", "go test ./s", "title", critiqueModel.Severe)}
	repo, root, _ := writeCloseRoot(t, "design-critic", 2, findings, materialHistory(2, 1), 5, 2)
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", root, "rounds", "1"), "return.json", map[string]any{"findings": []any{map[string]any{"severity": "critical"}}})
	_, err := critiqueRegisterClose(repo, root, captureObligations(new([]goal.ReviewObligation)))
	assertHumanRaise(t, err, "severe")
}

func TestDesignCapFiveFinalRoundFallingMechanicalDefersFixtures(t *testing.T) {
	t.Parallel()
	findings := []registerFinding{closeFinding("finding-a", "mechanical", "go test ./a", "title a", critiqueModel.Bounded)}
	repo, root, _ := writeCloseRoot(t, "design-critic", 5, findings, fiveRoundHistory(6, 5, 4, 3, 2), 5, 5)
	var got []goal.ReviewObligation
	outcome, err := critiqueRegisterClose(repo, root, captureObligations(&got))
	check(t, err == nil && outcome == "deferred" && len(got) == 1 && got[0].Test == "prove: go test ./a" && got[0].Fixture == "go test ./a",
		"final round five did not take the fixture exit: outcome=%q obligations=%+v err=%v", outcome, got, err)
}

func TestDesignCapFiveFinalRoundRaisesTheHuman(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		findings []registerFinding
		history  []any
	}{
		"not_falling": {[]registerFinding{closeFinding("flat", "mechanical", "go test ./f", "title", critiqueModel.Bounded)}, fiveRoundHistory(6, 5, 4, 2, 2)},
		"severe":      {[]registerFinding{closeFinding("severe", "mechanical", "go test ./s", "title", critiqueModel.Severe)}, fiveRoundHistory(6, 5, 4, 3, 1)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo, root, _ := writeCloseRoot(t, "design-critic", 5, tc.findings, tc.history, 5, 5)
			_, err := critiqueRegisterClose(repo, root, captureObligations(new([]goal.ReviewObligation)))
			assertHumanRaise(t, err, tc.findings[0].FindingID)
			check(t, strings.Contains(err.Error(), "design round 5 left findings"), "human raise did not name round five: %v", err)
		})
	}
}
