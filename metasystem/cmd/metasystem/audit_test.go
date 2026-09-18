package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditDependencyRatchetRelayReportsPathLineAndExit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "scripts", "fixture.sh")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("node -v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRelay(t, func() int {
		return runAuditDependencyRatchet([]string{"--root", root})
	})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "scripts/fixture.sh:1") {
		t.Fatalf("dependency audit relay = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if _, _, code = captureRelay(t, func() int {
		return runAuditDependencyRatchet([]string{"--root", root, "extra"})
	}); code != 2 {
		t.Fatalf("dependency audit relay accepted an extra argument with code %d", code)
	}
	if err := os.WriteFile(path, []byte("printf '%s' node\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = captureRelay(t, func() int {
		return runAuditDependencyRatchet([]string{"--root", root})
	})
	if code != 0 || stdout != "dependency ratchet passed\n" || stderr != "" {
		t.Fatalf("clean dependency audit relay = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestAuditParallelRatchetVerbRefusesAndLowers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAuditStopFixture(t, root, "go.mod", "module example.test/ratchet\n\ngo 1.27\n")
	writeAuditStopFixture(t, root, "pkg/serial_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestSerial(t *testing.T) {}\n")
	baseline := filepath.Join(root, "testing-parallel-ratchet.json")
	writeAuditStopFixture(t, root, "testing-parallel-ratchet.json", "{\n  \"packages\": {\"example.test/ratchet/pkg\": 0},\n  \"exempt\": []\n}\n")

	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runAuditParallelRatchet([]string{"--root", root})
	})
	for _, want := range []string{"package example.test/ratchet/pkg", "test TestSerial", "pkg/serial_test.go:5", "recorded serial count 0, actual 1", "PARALLEL_RATCHET_REFUSED"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("refusal lacks %q: %s", want, stderr)
		}
	}
	if code != 1 || stdout != "" {
		t.Fatalf("refusal = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	writeAuditStopFixture(t, root, "testing-parallel-ratchet.json", "{\n  \"packages\": {\"example.test/ratchet/pkg\": 2},\n  \"exempt\": []\n}\n")
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runAuditParallelRatchet([]string{"--root", root, "--update"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "package example.test/ratchet/pkg dropped from 2 to 1 serial tests") {
		t.Fatalf("lowering update = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	updated, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), `"example.test/ratchet/pkg": 1`) {
		t.Fatalf("updated baseline did not lower the count:\n%s", updated)
	}

	writeAuditStopFixture(t, root, "testing-parallel-ratchet.json", "{\n  \"packages\": {\"example.test/ratchet/pkg\": 0},\n  \"exempt\": []\n}\n")
	before, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr = captureCommandOutput(t, true, true, func() int {
		return runAuditParallelRatchet([]string{"--root", root, "--update"})
	})
	after, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(stderr, "edit the baseline by hand to raise a count") || string(after) != string(before) {
		t.Fatalf("raising update = code %d, stderr %q, changed=%t", code, stderr, string(after) != string(before))
	}
}

func TestAuditStopDecisionSurfaceVerb(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	writeAuditStopFixture(t, root, "scripts/agents/stop-decision-surface.txt", "go\ta_test.go\n")
	writeAuditStopFixture(t, root, "a_test.go", auditStopGoFixture("base := Verdict{ShouldBlock: true}\n"))
	runAuditStopGit(t, root, "init", "-q", "-b", "main")
	runAuditStopGit(t, root, "config", "--local", "user.name", "Stop Surface Fixture")
	runAuditStopGit(t, root, "config", "--local", "user.email", "stop-surface@invalid")
	runAuditStopGit(t, root, "config", "--local", "commit.gpgsign", "false")
	runAuditStopGit(t, root, "add", "-A")
	runAuditStopGit(t, root, "commit", "-q", "-m", "base")
	writeAuditStopFixture(t, root, "a_test.go", auditStopGoFixture("base := Verdict{ShouldBlock: true}\nadded := Verdict{BlockSource: source}\n"))

	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runAuditStopDecisionSurface([]string{"--root", root})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "added: a_test.go: added := Verdict{BlockSource: source}") ||
		!strings.Contains(stdout, "stop decision surface: base ") {
		t.Fatalf("additive verb = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runAuditStopDecisionSurface([]string{"--root", root, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("JSON verb = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	for _, field := range []string{"base", "added", "moved", "removed", "problems"} {
		if _, ok := document[field]; !ok {
			t.Errorf("JSON output lacks %q: %s", field, stdout)
		}
	}

	writeAuditStopFixture(t, root, "a_test.go", auditStopGoFixture(""))
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runAuditStopDecisionSurface([]string{"--root", root})
	})
	if code != 1 || !strings.Contains(stdout, "removed 1") ||
		!strings.Contains(stderr, "removed: a_test.go: base := Verdict{ShouldBlock: true}") ||
		!strings.Contains(stderr, "--declare --goal") {
		t.Fatalf("refusing verb = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	code, _, _ = captureCommandOutput(t, true, true, func() int {
		return runAuditStopDecisionSurface([]string{"--root", root, "extra"})
	})
	if code != 2 {
		t.Fatalf("invalid invocation exit = %d, want 2", code)
	}
}

func auditStopGoFixture(body string) string {
	return "package fixture\n\nfunc TestFixture() {\n\t" + strings.ReplaceAll(strings.TrimSuffix(body, "\n"), "\n", "\n\t") + "\n}\n"
}

func writeAuditStopFixture(t *testing.T, root, path, content string) {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runAuditStopGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
