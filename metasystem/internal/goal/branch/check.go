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

// CheckFlake is a red cheap command whose failed tests passed when they ran
// alone on the same tree: the check passes over it as the unit run does.
type CheckFlake struct {
	Cause     string           `json:"cause"`
	Flakes    []string         `json:"flakes"`
	Red       string           `json:"red"`
	Alone     string           `json:"aloneExecutionId"`
	Selectors []string         `json:"selectors"`
	AloneExit launch.CheckExit `json:"aloneExit"`
}

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
		Flake       *CheckFlake        `json:"flake"`
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
		if result.Exits[i].Name != name {
			return false
		}
	}
	cheap, audits := result.Exits[0], result.Exits[1]
	if audits.Exit != 0 || audits.Error != "" {
		return false
	}
	if cheap.Exit == 0 && cheap.Error == "" {
		return true
	}
	// A red cheap command passes only over a flake its alone rerun cleared.
	flake := result.Flake
	return flake != nil && flake.Cause == "flake" && len(flake.Flakes) > 0 && flake.Alone != "" && flake.AloneExit.Exit == 0 && flake.AloneExit.Error == ""
}
