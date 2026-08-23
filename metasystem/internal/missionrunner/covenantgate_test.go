package missionrunner

// The covenant gate at preflight: no declaration passes untouched; a
// declared covenant must exist, its battery must BE the contract's
// gate, and every contract-custodied guardrail must be one the
// covenant declares.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func covenantGateBed(t *testing.T) (*Engine, string) {
	t.Helper()
	root := t.TempDir()
	body := `{
  "schemaVersion": 1,
  "identity": {"name": "app", "entryPoint": "./run", "sourcePaths": ["src/"]},
  "requirements": [{"id": "1", "ref": "spec 1", "proof": "check-1"}],
  "battery": {"command": "bash gate.sh", "metric": "score", "direction": "max", "threshold": ">=1"},
  "budgets": [],
  "guards": [],
  "guardrails": ["gate.sh", "goldens/"]
}`
	if err := os.WriteFile(filepath.Join(root, "covenant.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Engine{Root: root, Mission: "app"}, "covenant.json"
}

func TestCovenantPreflight(t *testing.T) {
	engine, path := covenantGateBed(t)

	// No declaration: covenant-less repos stay lawful.
	if err := engine.covenantPreflight(map[string]string{"gate.command": "anything"}); err != nil {
		t.Fatalf("no declaration must pass: %v", err)
	}

	// Full agreement passes.
	agree := map[string]string{
		"covenant.path":   path,
		"gate.command":    "bash gate.sh",
		"wall.guardrails": "gate.sh, goldens/",
	}
	if err := engine.covenantPreflight(agree); err != nil {
		t.Fatalf("agreement must pass: %v", err)
	}

	// A declared covenant that does not exist refuses by name.
	missing := map[string]string{"covenant.path": "elsewhere.json", "gate.command": "bash gate.sh"}
	if err := engine.covenantPreflight(missing); err == nil || !strings.Contains(err.Error(), "declares a covenant") {
		t.Fatalf("missing covenant must refuse: %v", err)
	}

	// A gate that is not the battery refuses with both named.
	drift := map[string]string{"covenant.path": path, "gate.command": "bash weaker-gate.sh"}
	if err := engine.covenantPreflight(drift); err == nil || !strings.Contains(err.Error(), "covenant's battery") {
		t.Fatalf("gate drift must refuse: %v", err)
	}

	// A contract custodying a guardrail the covenant never declared
	// refuses: the covenant is the net's one home.
	invented := map[string]string{
		"covenant.path":   path,
		"gate.command":    "bash gate.sh",
		"wall.guardrails": "gate.sh, invented.txt",
	}
	if err := engine.covenantPreflight(invented); err == nil || !strings.Contains(err.Error(), "the net's one home") {
		t.Fatalf("an invented guardrail must refuse: %v", err)
	}

	// Traversal in the declaration itself refuses before any read.
	traversal := map[string]string{"covenant.path": "../outside.json"}
	if err := engine.covenantPreflight(traversal); err == nil || !strings.Contains(err.Error(), "repository-relative") {
		t.Fatalf("traversal must refuse: %v", err)
	}
}
