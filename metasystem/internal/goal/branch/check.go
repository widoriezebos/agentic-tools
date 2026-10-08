package branch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// DeclarationUnavailableError means carry cannot prove this subject's check.
// The rebase keeps the unit and asks for a new read instead of carrying one.
type DeclarationUnavailableError struct{ Err error }

func (e *DeclarationUnavailableError) Error() string { return e.Err.Error() }
func (e *DeclarationUnavailableError) Unwrap() error { return e.Err }

func validGateObservation(gate GateObservation, tree string) bool {
	if gate.RunID == "" || gate.Tree != tree {
		return false
	}
	if gate.Kind == "go-gate-fast" {
		return true
	}
	if gate.Kind != "unit-check" {
		return false
	}
	var result struct {
		ExecutionID string             `json:"executionId"`
		Check       launch.UnitCheck   `json:"check"`
		Exits       []launch.CheckExit `json:"exits"`
	}
	if json.Unmarshal([]byte(gate.Evidence), &result) != nil || result.ExecutionID != gate.RunID || result.Check.SourceTree != tree || result.Check.Cheap == "" || result.Check.Audits == "" || result.Check.Minutes <= 0 || result.Check.Directory == "" || result.Check.Environment == nil || len(result.Exits) != 2 {
		return false
	}
	commands, err := json.Marshal(result.Check)
	digest := sha256.Sum256(commands)
	if err != nil || hex.EncodeToString(digest[:]) != gate.CommandDigest {
		return false
	}
	for i, name := range []string{"cheap", "audits"} {
		if result.Exits[i].Name != name || result.Exits[i].Exit != 0 || result.Exits[i].Error != "" {
			return false
		}
	}
	return true
}
