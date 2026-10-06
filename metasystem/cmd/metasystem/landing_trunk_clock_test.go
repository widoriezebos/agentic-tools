package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// seedRecentTrunkCheck records a trunk check and a keeper launch that ended
// at the same second (so the check is not a proof finished after it). A lane that never ran a trunk check is
// due at once (design Decision 4's clock); a bed that tests other wakes
// seeds this so its wake answers only the queue, the holds and the proofs.
func seedRecentTrunkCheck(t *testing.T, install, home string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	trunk, err := json.Marshal(plain.Result{Result: plain.Green, Trunk: true, Tree: "trunk-tree", Commit: "trunk-commit", Scope: "full", At: now})
	if err != nil {
		t.Fatal(err)
	}
	results := filepath.Join(plain.Dir(install), "results.jsonl")
	if err := os.MkdirAll(filepath.Dir(results), 0o755); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(results)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(results, append(append(previous, trunk...), '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(lane.AgentState{StartedAt: now, ReapedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lane.AgentStatePath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.AgentStatePath(home), state, 0o644); err != nil {
		t.Fatal(err)
	}
}
