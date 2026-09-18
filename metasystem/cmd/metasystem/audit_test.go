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
