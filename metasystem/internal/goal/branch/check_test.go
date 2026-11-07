package branch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestValidGateObservationFlake(t *testing.T) {
	t.Parallel()
	tree := strings.Repeat("b", 40)
	check := launch.UnitCheck{SourceTree: tree, Cheap: "metasystem test impact", Audits: "true", Minutes: 15, Directory: "/subject", Environment: []string{}}
	commands, err := json.Marshal(check)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(commands)
	green, audits := launch.CheckExit{Name: "cheap"}, launch.CheckExit{Name: "audits"}
	red := launch.CheckExit{Name: "cheap", Exit: 1, Error: "exit status 1"}
	gate := func(cheap, audits launch.CheckExit, change func(*CheckFlake)) GateObservation {
		var flake *CheckFlake
		if change != nil {
			flake = &CheckFlake{Cause: "flake", Flakes: []string{"example.com/flaky TestFlaky"}, Red: "checks passed after a flake rerun", Alone: "alone-1", Selectors: []string{"LANDING_ONLY=example.com/flaky"}, AloneExit: launch.CheckExit{Name: "cheap"}}
			change(flake)
		}
		evidence, err := json.Marshal(struct {
			ExecutionID string             `json:"executionId"`
			Check       launch.UnitCheck   `json:"check"`
			Exits       []launch.CheckExit `json:"exits"`
			Flake       *CheckFlake        `json:"flake,omitempty"`
		}{"check-1", check, []launch.CheckExit{cheap, audits}, flake})
		if err != nil {
			t.Fatal(err)
		}
		return GateObservation{Kind: "unit-check", Tree: tree, RunID: "check-1", CommandDigest: hex.EncodeToString(digest[:]), Evidence: string(evidence)}
	}
	well := func(*CheckFlake) {}
	for _, c := range []struct {
		name string
		gate GateObservation
		want bool
	}{
		{"green", gate(green, audits, nil), true},
		{"flake", gate(red, audits, well), true},
		{"red without flake", gate(red, audits, nil), false},
		{"flake with red audits", gate(red, launch.CheckExit{Name: "audits", Exit: 2, Error: "exit status 2"}, well), false},
		{"empty flaked list", gate(red, audits, func(f *CheckFlake) { f.Flakes = nil }), false},
		{"missing alone execution", gate(red, audits, func(f *CheckFlake) { f.Alone = "" }), false},
		{"red alone exit", gate(red, audits, func(f *CheckFlake) { f.AloneExit.Exit, f.AloneExit.Error = 1, "exit status 1" }), false},
		{"erroring alone exit", gate(red, audits, func(f *CheckFlake) { f.AloneExit.Error = "cheap timed out" }), false},
		{"not a flake", gate(red, audits, func(f *CheckFlake) { f.Cause = "own" }), false},
	} {
		if got := validGateObservation(c.gate, tree); got != c.want {
			t.Errorf("%s: valid=%v, want %v", c.name, got, c.want)
		}
	}
}
