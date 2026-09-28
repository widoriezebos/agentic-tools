package main

// The covenant family's one verb: a thin structural check whose
// success line must carry the honesty distinction — shape validity is
// not adequacy, and the interview repeats that to the human.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestSystemCheckReadsTheCovenantShape: system check reports the shape of
// the app covenant at its one home (the internal covenant validate it
// replaced read the same file through the same owner): a valid covenant
// names its path and no problem, an absent one reports nothing, and a broken
// one names its path and the parse problem.
func TestSystemCheckReadsTheCovenantShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source, err := os.ReadFile(filepath.Join("..", "..", "internal", "covenant", "testdata", "taskrun-covenant.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "covenant.json"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	if path, problem := checkCovenantShape(processScope{Installation: root}); path != filepath.Join(root, "covenant.json") || problem != nil {
		t.Fatalf("the kit-extracted covenant must validate at the one home: path=%q problem=%v", path, problem)
	}
	if path, problem := checkCovenantShape(processScope{Installation: t.TempDir()}); path != "" || problem != nil {
		t.Fatalf("an absent covenant must report nothing: path=%q problem=%v", path, problem)
	}
	brokenRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(brokenRoot, "covenant.json"), []byte(`{"schemaVersion": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if path, problem := checkCovenantShape(processScope{Installation: brokenRoot}); path == "" || problem == nil {
		t.Fatalf("a broken covenant must name its path and its problem: path=%q problem=%v", path, problem)
	}
}


const evidenceBedCovenant = `{
  "schemaVersion": 1,
  "identity": {"name": "bed-app", "entryPoint": "bash gate.sh", "sourcePaths": ["src/"]},
  "requirements": [
    {"id": "1", "ref": "criterion 1: the app greets by name", "proof": "greets"}
  ],
  "battery": {"command": "bash gate.sh", "metric": "greets", "direction": "max", "threshold": ">=1"},
  "budgets": [],
  "guards": [],
  "guardrails": ["gate.sh", "docs/covenant-evidence.md"]
}
`

const evidenceBedTable = `# Covenant evidence — bed-app

| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | The app greets by name | greets | repo | bash gate.sh | gate.sh,src/app.py | gate.sh runs the entrypoint | observed |

Wired: 1. Floating: 0.
`

func evidenceBed(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"src", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"covenant.json":             evidenceBedCovenant,
		"gate.sh":                   "#!/bin/sh\n",
		"src/app.py":                "print()\n",
		"docs/covenant-evidence.md": evidenceBedTable,
	}
	for name, content := range files {
		if err := testexec.WriteFile(filepath.Join(root, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestSystemCheckJudgesTheCovenantEvidence: system check runs the covenant's
// traceability gate, the one the inception interview promises: every
// requirement backed by a row of docs/covenant-evidence.md with its declared
// dependencies present. A missing row refuses, by requirement.
func TestSystemCheckJudgesTheCovenantEvidence(t *testing.T) {
	t.Parallel()
	root := evidenceBed(t)
	lines, report, traceable := checkCovenantEvidence(root)
	if !traceable || report == nil || report.Outcome != "traceable" || !strings.Contains(strings.Join(lines, "\n"), "requirement 1 (proof greets): ") {
		t.Fatalf("a green table: traceable=%v lines=%q report=%+v", traceable, lines, report)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "covenant-evidence.md"), []byte("# Covenant evidence — bed-app\n\n| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n| 2 | Another thing | other | repo | bash gate.sh | gate.sh | gate.sh runs it | observed |\n\nWired: 1. Floating: 0.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, report, traceable = checkCovenantEvidence(root)
	if traceable || report == nil || !strings.Contains(strings.Join(lines, "\n"), "requirement 1") {
		t.Fatalf("a missing row: traceable=%v lines=%q report=%+v", traceable, lines, report)
	}
}
