package main

// The evidence verb's exit taxonomy end-to-end: 0 traceable with the
// claims-on-file honesty line, 1 for gate refusals and unreadable
// inputs, 2 for usage — and the JSON contract a tool would consume.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidencetable"
)

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
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCovenantEvidenceVerb(t *testing.T) {
	root := evidenceBed(t)
	out, code := captureStdout(t, func() int {
		return runCovenantEvidence([]string{"--root", root})
	})
	if code != 0 {
		t.Fatalf("the bed must be traceable: code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "evidence traceable") ||
		!strings.Contains(out, "recorded statuses are claims on file, not re-verified here") {
		t.Fatalf("the success line must carry the honesty caveat: %q", out)
	}

	// The JSON contract: verdicts, assessments, counts.
	out, code = captureStdout(t, func() int {
		return runCovenantEvidence([]string{"--root", root, "--json"})
	})
	if code != 0 {
		t.Fatalf("json mode must succeed: %d", code)
	}
	var report evidencetable.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the JSON must parse: %v", err)
	}
	if report.Outcome != "traceable" || len(report.Pairs) != 1 ||
		report.Pairs[0].Verdict != evidencetable.VerdictBound ||
		report.Pairs[0].Assessment != evidencetable.AssessmentRecordedUnverified {
		t.Fatalf("the JSON contract drifted: %+v", report)
	}
	if !report.Counts.Match {
		t.Fatalf("the bed's counts must match: %+v", report.Counts)
	}

	// A positional argument is usage, never a silent judgment.
	_, code = captureStdout(t, func() int {
		return runCovenantEvidence([]string{"--root", root, "garbage"})
	})
	if code != 2 {
		t.Fatalf("a positional argument must refuse with usage exit 2: code=%d", code)
	}
}

func TestCovenantEvidenceVerbRefusals(t *testing.T) {
	// A missing evidence table refuses: the covenant alone is intent
	// with no evidence discipline on file.
	root := evidenceBed(t)
	if err := os.Remove(filepath.Join(root, "docs", "covenant-evidence.md")); err != nil {
		t.Fatal(err)
	}
	_, code := captureStdout(t, func() int {
		return runCovenantEvidence([]string{"--root", root})
	})
	if code != 1 {
		t.Fatalf("a missing table must refuse with exit 1: code=%d", code)
	}

	// A requirement the table cannot back refuses and names the pair.
	root = evidenceBed(t)
	broken := strings.Replace(evidenceBedTable, "| greets |", "| salutes |", 1)
	if err := os.WriteFile(filepath.Join(root, "docs", "covenant-evidence.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func() int {
		return runCovenantEvidence([]string{"--root", root})
	})
	if code != 1 || !strings.Contains(out, "bound to proof greets in the covenant but records proof salutes") {
		t.Fatalf("the wrong-proof refusal must name both proofs: code=%d out=%q", code, out)
	}

	// A broken declared dep refuses.
	root = evidenceBed(t)
	if err := os.Remove(filepath.Join(root, "src", "app.py")); err != nil {
		t.Fatal(err)
	}
	out, code = captureStdout(t, func() int {
		return runCovenantEvidence([]string{"--root", root})
	})
	if code != 1 || !strings.Contains(out, "broken-dep") {
		t.Fatalf("a broken declared dep must refuse broken-dep: code=%d out=%q", code, out)
	}

	// A dangling symlink at the covenant's home refuses by NAME —
	// the same shape validation's presence test must never skip. The
	// loader's refusal speaks on stderr, so that is what the pin reads.
	root = evidenceBed(t)
	if err := os.Remove(filepath.Join(root, "covenant.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("covenant-that-does-not-exist.json", filepath.Join(root, "covenant.json")); err != nil {
		t.Fatal(err)
	}
	errOut, code := captureStderr(t, func() int {
		return runCovenantEvidence([]string{"--root", root})
	})
	if code != 1 || !strings.Contains(errOut, "symlink") {
		t.Fatalf("a symlinked covenant home must refuse with exit 1 naming the symlink: code=%d stderr=%q", code, errOut)
	}
}

// captureStderr mirrors captureStdout for the refusal channel.
func captureStderr(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	original := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = write
	code := fn()
	write.Close()
	os.Stderr = original
	out, _ := io.ReadAll(read)
	return string(out), code
}

func TestCovenantEvidenceProseCarriesOrphanText(t *testing.T) {
	root := evidenceBed(t)
	orphaned := strings.Replace(evidenceBedTable,
		"| gate.sh runs the entrypoint | observed |\n",
		"| gate.sh runs the entrypoint | observed |\n| 9 | The deferred criterion in words | later | repo | (planned) | | no proof yet | planned-floating |\n", 1)
	orphaned = strings.Replace(orphaned, "Wired: 1. Floating: 0.", "Wired: 1. Floating: 1.", 1)
	if err := os.WriteFile(filepath.Join(root, "docs", "covenant-evidence.md"), []byte(orphaned), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func() int {
		return runCovenantEvidence([]string{"--root", root})
	})
	if code != 0 {
		t.Fatalf("an orphan floating row must not refuse: code=%d out=%q", code, out)
	}
	if !strings.Contains(out, `"The deferred criterion in words"`) {
		t.Fatalf("the prose must carry the orphan's criterion text: %q", out)
	}
}
