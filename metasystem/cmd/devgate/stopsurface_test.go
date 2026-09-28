package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// Rule H1: the gate's Stop-surface refusal names the public command a person
// runs to allow the move, not only the internal declaration.
func TestStopSurfaceRefusalNamesGoalAllow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(relative, content string) {
		t.Helper()
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		command.Env = gittree.ScrubbedEnviron()
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.25\n")
	write("a_test.go", "package fixture\n\nfunc TestFixture() {\n\twant := Verdict{ShouldBlock: true}\n}\n")
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	write("a_test.go", "package fixture\n\nfunc TestFixture() {\n}\n")

	out, ok := stopSurface(root)
	if ok || !strings.Contains(out, "removed: a_test.go: want := Verdict{ShouldBlock: true}") ||
		!strings.Contains(out, "a person allows the move with metasystem goal allow <goal-id> stop-test-changes --reason <text>") {
		t.Fatalf("stop surface refusal = %v:\n%s", ok, out)
	}
}
