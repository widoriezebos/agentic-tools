package ledgerfence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitAdapterEnsureEnrollsTheGuardInAFreshCheckout is the goal CLI shell
// bed's R2-11 row: a fresh clone has no hooks, and the first ledger mutation
// enrolls the guard before publishing. The claim is Git's own hooks
// resolution (rev-parse --git-path hooks) and the behavioural probe that runs
// the installed chain, which no stub can prove, so this adapter test runs the
// real Git in a private repository. Since U5 the guard is the engine's
// `internal pre-commit` entry, run from the checkout's own bin/metasystem.
func TestGitAdapterEnsureEnrollsTheGuardInAFreshCheckout(t *testing.T) {
	t.Parallel()
	root, _ := fenceGitAdapterRepo(t, 0)
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
	if !strings.Contains(string(installed), `internal pre-commit --root "$installation"`) || !isCurrentComposer(string(installed)) {
		t.Fatalf("the installed hook is not the engine guard's composer:\n%s", installed)
	}
	if err := Ensure(root); err != nil {
		t.Fatalf("a second enrollment of an enrolled checkout refused: %v", err)
	}
}
