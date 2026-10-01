package dispatch

import (
	"errors"
	"strings"
	"testing"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The design critique's cap is five rounds, a backstop (Wido 2026-10-01): the
// human raise and the fixture exit fire at the chain's frozen limit, never at
// round two of a five-round chain.

func fiveRoundHistory(materials ...int64) []any {
	history := make([]any, 0, len(materials))
	for index, material := range materials {
		history = append(history, map[string]any{"round": int64(index + 1), "material": material})
	}
	return history
}

func TestDesignCapFiveRoundTwoResidueDispatchesNextRound(t *testing.T) {
	t.Parallel()
	findings := []registerFinding{closeFinding("mechanical", "mechanical", "go test ./m", "title", critiqueModel.Bounded)}
	repo, root, _ := writeCloseRoot(t, "design-critic", 2, findings, materialHistory(3, 2), 5, 2)
	var got []goal.ReviewObligation
	_, err := critiqueRegisterClose(repo, root, captureObligations(&got))
	check(t, err != nil && strings.Contains(err.Error(), "dispatch the next round") && len(got) == 0,
		"round two of a five-round design chain did not ask for the next round: obligations=%+v err=%v", got, err)
}

func TestDesignCapFiveRoundTwoBlockerIsNotTheHumanRaise(t *testing.T) {
	t.Parallel()
	findings := []registerFinding{closeFinding("severe", "mechanical", "go test ./s", "title", critiqueModel.Severe)}
	repo, root, _ := writeCloseRoot(t, "design-critic", 2, findings, materialHistory(2, 1), 5, 2)
	_, err := critiqueRegisterClose(repo, root, captureObligations(new([]goal.ReviewObligation)))
	var opErr *OpError
	check(t, err != nil && !errors.As(err, &opErr) && strings.Contains(err.Error(), "blocks close"),
		"a blocker at round two of five raised the cap-exhausted refusal: %v", err)
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
