package branch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestCarriedGateRejectsIncompleteCheckOutcome(t *testing.T) {
	t.Parallel()
	for _, outcome := range []launch.CheckOutcome{"", launch.CheckPassed, launch.CheckBudgetExhausted, launch.CheckFailed, launch.CheckEnvironmentFailure, "unknown"} {
		result := launch.CheckResult{Version: 1, Outcome: outcome, ExecutionID: "check-one",
			Check: launch.UnitCheck{SourceTree: "tree", Cheap: "true", Audits: "true", Minutes: 15, Directory: "installation", Environment: []string{}},
			Exits: []launch.CheckExit{{Name: "cheap"}, {Name: "audits"}}}
		commands, err := json.Marshal(result.Check)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(commands)
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		gate := GateObservation{Kind: "unit-check", Tree: "tree", RunID: "check-one", CommandDigest: hex.EncodeToString(digest[:]), Evidence: string(data)}
		if validGateObservation(gate, "tree") != (outcome == launch.CheckPassed) {
			t.Fatalf("gate accepted wrong outcome: %q", outcome)
		}
		if outcome == launch.CheckPassed {
			result.Version, result.Kind = 2, "bounded"
			result.Check.Declaration = &launch.UnitDeclaration{Values: [6]string{"true", "true", "15", "", "bounded", "6"}}
			result.Exits[0].Name = "static"
			commands, _ = json.Marshal(result.Check)
			digest = sha256.Sum256(commands)
			data, _ = json.Marshal(result)
			gate.CommandDigest, gate.Evidence = hex.EncodeToString(digest[:]), string(data)
			if result.RequireFeedbackPassed() != nil || validGateObservation(gate, "tree") {
				t.Fatal("bounded feedback must pass only its feedback requirement, never the branch gate")
			}
		}
		if outcome == "" {
			result.Version = 0
			data, err = json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			gate.Evidence = string(data)
			if !validGateObservation(gate, "tree") {
				t.Fatal("legacy completed gate was lost")
			}
		}
	}
}
