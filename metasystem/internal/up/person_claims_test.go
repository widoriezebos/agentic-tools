package up

import (
	"encoding/json"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"strings"
	"testing"
)

func TestPersonClaimUpKeepsArmedPartialResults(t *testing.T) {
	t.Parallel()
	t.Run("pending observation before ids are readable", func(t *testing.T) {
		t.Parallel()
		component := stopCapabilityOutcome(Options{RestampStopCapability: func(string, string, int64) (StopCapabilityRestampResult, error) {
			return StopCapabilityRestampResult{Pending: true}, nil
		}}, "seat", 7)
		result := Result{Outcome: "armed", Components: []ComponentOutcome{component}, Adoption: component.Adoption}
		data, err := json.Marshal(result.Data())
		if err != nil || result.ExitCode() != 0 || component.Outcome != "deferred" || component.Adoption == nil || !component.Adoption.Pending || !strings.Contains(string(data), `"pending":true`) || !strings.Contains(strings.Join(result.Lines(), "\n"), "metasystem session start") {
			t.Fatalf("pending observation was reported as all adopted: %+v %s %v", result, data, err)
		}
	})
	for _, readFailed := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial publication", true: "unreadable observation"}[readFailed], func(t *testing.T) {
			t.Parallel()
			adoption := StopCapabilityRestampResult{Pending: true, Observation: &goal.Observation{Tip: "accepted-tip", Outcome: "offline"}, Goals: []GoalAdoptionOutcome{
				{GoalID: "confirmed", Outcome: "restamped", FromEpoch: 0, ToEpoch: 7},
				{GoalID: "waiting", Outcome: "pending", FromEpoch: 0, ToEpoch: 7, Cause: "confirmation unavailable", Remedy: "metasystem session start", Operation: "journal-operation"},
			}}
			options := Options{RestampStopCapability: func(string, string, int64) (StopCapabilityRestampResult, error) {
				if readFailed {
					return adoption, errors.New("accepted ledger unavailable")
				}
				return adoption, nil
			}}
			component := stopCapabilityOutcome(options, "seat", 7)
			result := Result{Outcome: "armed", Authority: "writer", Components: []ComponentOutcome{component}, Adoption: component.Adoption}
			data, err := json.Marshal(result.Data())
			lines := strings.Join(result.Lines(), "\n")
			if err != nil || result.ExitCode() != 0 || component.Outcome != "deferred" || component.Adoption == nil || !component.Adoption.Pending || len(component.Adoption.Goals) != 2 ||
				!strings.Contains(string(data), `"goal":"confirmed"`) || !strings.Contains(string(data), `"operation":"journal-operation"`) || !strings.Contains(lines, "adoption=restamped") || !strings.Contains(lines, "adoption=pending") || !strings.Contains(lines, "metasystem session start") {
				t.Fatalf("armed supervision erased pending results: %+v %s %s %v", result, data, lines, err)
			}
		})
	}
}
