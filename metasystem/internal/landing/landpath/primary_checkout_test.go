package landpath

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPrimaryCheckoutIsTheExportedThreeAnswerPredicate: the predicate the
// guard's helm admission uses, reachable by the person-proof yield: one
// directory three times names the common dir; a linked worktree's own
// administrative directory, or a failed answer, is not the primary checkout.
func TestPrimaryCheckoutIsTheExportedThreeAnswerPredicate(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	common := filepath.Join(top, ".git")
	linked := filepath.Join(common, "worktrees", "w")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(common)
	if err != nil {
		t.Fatal(err)
	}
	answering := func(entry string, code int) func(args ...string) GitResult {
		return func(args ...string) GitResult {
			if args[len(args)-2] == "--resolve-git-dir" {
				if args[len(args)-1] != filepath.Join(top, ".git") {
					t.Errorf("the on-disk entry asked is %s", args[len(args)-1])
				}
				return GitResult{Stdout: []byte(entry + "\n"), Code: code}
			}
			return GitResult{Stdout: []byte(common + "\n")}
		}
	}
	if got, ok := PrimaryCheckout(answering(common, 0), top); !ok || got != resolved {
		t.Fatalf("the primary checkout: %q %t", got, ok)
	}
	if got, ok := PrimaryCheckout(answering(linked, 0), top); ok || got != "" {
		t.Fatalf("a linked worktree's entry: %q %t", got, ok)
	}
	if _, ok := PrimaryCheckout(answering(common, 128), top); ok {
		t.Fatal("a failed answer was admitted")
	}
}
