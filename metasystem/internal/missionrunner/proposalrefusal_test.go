package missionrunner

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Issue #3 part 2: the ProposalError boundary is REACHABLE through the
// runner's writeState — a transition-violating proposal keeps its type so
// concludeCycle's guard can park instead of dying — while a
// compare-and-write miss stays a plain runner failure (the ramp's).
func TestWriteStatePreservesProposalRefusal(t *testing.T) {
	root := t.TempDir()
	contract := filepath.Join(root, "mission-mr-prop.contract.md")
	os.WriteFile(contract, []byte("```mission\ncandidate.branch=feature-x\nstream.alpha=Do alpha\n```\n"), 0o644)
	stateDir := filepath.Join(root, "artifacts", "agents", "missions", "mr-prop")
	os.MkdirAll(stateDir, 0o755)
	statePath := filepath.Join(stateDir, "state.json")
	if err := mission.InitStateWithBaseline(statePath, contract, filepath.Join(stateDir, "ledger.md"), "", "", strings.Repeat("b", 40), testAdmissionOrigins()); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{Root: root, Mission: "mr-prop"}
	state, err := engine.verifyState(statePath, false)
	if err != nil {
		t.Fatal(err)
	}

	violating := deepCopyDoc(state)
	violating["missionId"] = "someone-else" // immutable identity: a proposal fault
	_, err = engine.writeState(statePath, violating)
	var proposal *mission.ProposalError
	if !errors.As(err, &proposal) {
		t.Fatalf("transition violation lost its type: %T %v", err, err)
	}

	// A stale expected hash is the runner's own CAS problem, never a
	// proposal refusal. The runner's writeState always targets the live
	// disk hash, so the miss is driven at the mission layer directly.
	lawful := deepCopyDoc(state)
	delete(lawful, "integrity")
	source := filepath.Join(t.TempDir(), "proposal.json")
	data, _ := json.MarshalIndent(lawful, "", "  ")
	os.WriteFile(source, append(data, byte(10)), 0o644)
	err = mission.WriteState(statePath, source, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("stale CAS accepted")
	}
	if errors.As(err, &proposal) {
		t.Fatalf("CAS mismatch misclassified as a proposal refusal: %v", err)
	}
}
