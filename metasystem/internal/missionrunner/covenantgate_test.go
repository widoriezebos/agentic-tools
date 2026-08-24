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
  "battery": {"command": "bash gate.sh", "metric": "score", "direction": "max", "threshold": ">=3"},
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

	// A TRULY covenant-less repo stays lawful.
	bare := &Engine{Root: t.TempDir(), Mission: "bare"}
	if err := bare.covenantPreflight(map[string]string{"gate.command": "anything"}); err != nil {
		t.Fatalf("a covenant-less repo must pass: %v", err)
	}

	// An app CARRYING covenant.json cannot be opted out by omission.
	if err := engine.covenantPreflight(map[string]string{"gate.command": "anything"}); err == nil || !strings.Contains(err.Error(), "not optional once it exists") {
		t.Fatalf("omission over an existing covenant must refuse: %v", err)
	}

	// Full agreement — command, threshold for the metric, and the whole
	// net in both directions — passes.
	agree := map[string]string{
		"covenant.path":        path,
		"gate.command":         "bash gate.sh",
		"gate.threshold.score": ">=3",
		"gate.direction":       "max",
		"wall.guardrails":      "gate.sh, goldens/",
	}
	if err := engine.covenantPreflight(agree); err != nil {
		t.Fatalf("agreement must pass: %v", err)
	}

	// A matching command with a weakened threshold refuses: green must
	// mean what the covenant says.
	weakened := map[string]string{
		"covenant.path":        path,
		"gate.command":         "bash gate.sh",
		"gate.threshold.score": ">=1",
		"gate.direction":       "max",
		"wall.guardrails":      "gate.sh, goldens/",
	}
	if err := engine.covenantPreflight(weakened); err == nil || !strings.Contains(err.Error(), "covenant's threshold") {
		t.Fatalf("a weakened threshold must refuse: %v", err)
	}

	// A contract omitting a covenant guardrail refuses: an omitted entry
	// would ride the ordinary authorization lane.
	underCustody := map[string]string{
		"covenant.path":        path,
		"gate.command":         "bash gate.sh",
		"gate.threshold.score": ">=3",
		"gate.direction":       "max",
		"wall.guardrails":      "gate.sh",
	}
	if err := engine.covenantPreflight(underCustody); err == nil || !strings.Contains(err.Error(), "does not custody it") {
		t.Fatalf("under-custody must refuse: %v", err)
	}

	// An inverted direction refuses: it would turn every measurement
	// upside down.
	inverted := map[string]string{
		"covenant.path":        path,
		"gate.command":         "bash gate.sh",
		"gate.threshold.score": ">=3",
		"gate.direction":       "min",
		"wall.guardrails":      "gate.sh, goldens/",
	}
	if err := engine.covenantPreflight(inverted); err == nil || !strings.Contains(err.Error(), "upside down") {
		t.Fatalf("an inverted direction must refuse: %v", err)
	}

	// The covenant has one home: a declaration naming any other file
	// refuses — a movable covenant is a selectable one.
	relocated := map[string]string{"covenant.path": "policies/other.json"}
	if err := engine.covenantPreflight(relocated); err == nil || !strings.Contains(err.Error(), "one home") {
		t.Fatalf("a relocated covenant must refuse: %v", err)
	}

	// A symlink AT the one home refuses before any read.
	linkRoot := t.TempDir()
	os.WriteFile(filepath.Join(linkRoot, "real.json"), []byte("{}"), 0o644)
	os.Symlink("real.json", filepath.Join(linkRoot, "covenant.json"))
	linked := &Engine{Root: linkRoot, Mission: "app"}
	if err := linked.covenantPreflight(map[string]string{"covenant.path": "covenant.json"}); err == nil {
		t.Fatal("a symlinked covenant must refuse")
	}

	// A declaration whose one-home file does not exist refuses by name.
	empty := &Engine{Root: t.TempDir(), Mission: "app"}
	missing := map[string]string{"covenant.path": "covenant.json", "gate.command": "bash gate.sh"}
	if err := empty.covenantPreflight(missing); err == nil || !strings.Contains(err.Error(), "declares a covenant") {
		t.Fatalf("missing covenant must refuse: %v", err)
	}

	// A gate that is not the battery refuses with both named.
	drift := map[string]string{"covenant.path": path, "gate.command": "bash weaker-gate.sh", "gate.threshold.score": ">=3"}
	if err := engine.covenantPreflight(drift); err == nil || !strings.Contains(err.Error(), "covenant's battery") {
		t.Fatalf("gate drift must refuse: %v", err)
	}

	// A contract custodying a guardrail the covenant never declared
	// refuses: the covenant is the net's one home.
	invented := map[string]string{
		"covenant.path":        path,
		"gate.command":         "bash gate.sh",
		"gate.threshold.score": ">=3",
		"gate.direction":       "max",
		"wall.guardrails":      "gate.sh, goldens/, invented.txt",
	}
	if err := engine.covenantPreflight(invented); err == nil || !strings.Contains(err.Error(), "the net's one home") {
		t.Fatalf("an invented guardrail must refuse: %v", err)
	}

	// Traversal in the declaration refuses under the one-home rule too.
	traversal := map[string]string{"covenant.path": "../outside.json"}
	if err := engine.covenantPreflight(traversal); err == nil || !strings.Contains(err.Error(), "one home") {
		t.Fatalf("traversal must refuse: %v", err)
	}
}
