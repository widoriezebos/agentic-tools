package ledgerfence

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestGitAdapterEnsureEnrollsTheGuardInAFreshCheckout is the goal CLI shell
// bed's R2-11 row: a fresh clone has no hooks, and the first ledger mutation
// enrolls the shipped guard before publishing. The claim is Git's own hooks
// resolution (rev-parse --git-path hooks) and the behavioural probe that runs
// the installed chain, which no stub can prove, so this adapter test runs the
// real Git and the real guard in a private repository.
func TestGitAdapterEnsureEnrollsTheGuardInAFreshCheckout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	init := exec.Command("git", "init", "-q", "-b", "main", root)
	init.Env = EnvironWithoutGitSteering()
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	shipped, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "pre-commit-guard.sh"))
	if err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh")
	if err := os.MkdirAll(filepath.Dir(guard), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(guard, shipped, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hook); !os.IsNotExist(err) {
		t.Fatalf("a fresh checkout already has a pre-commit hook: %v", err)
	}
	if err := Ensure(root); err != nil {
		t.Fatalf("Ensure did not enroll the guard: %v", err)
	}
	installed, err := os.ReadFile(hook)
	if err != nil {
		t.Fatalf("Ensure wrote no pre-commit hook: %v", err)
	}
	if !strings.Contains(string(installed), "pre-commit-guard.sh") || !isOurComposer(string(installed)) {
		t.Fatalf("the installed hook is not the guard's composer:\n%s", installed)
	}
	if err := Ensure(root); err != nil {
		t.Fatalf("a second enrollment of an enrolled checkout refused: %v", err)
	}
}
