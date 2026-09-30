package counselor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A root outside any repository is told by the filesystem (no .git above
// it), never by git's words: a checkout whose .git git cannot follow is a
// failure named as one, not "outside a Git worktree".
func TestGitResolutionTellsARepositoryByItsGit(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	if _, err := readGitLandingHistory(outside); err == nil || !strings.Contains(err.Error(), "not inside a Git worktree") {
		t.Fatalf("a root outside any repository = %v", err)
	}
	unreadable := t.TempDir()
	if err := os.WriteFile(filepath.Join(unreadable, ".git"), []byte("gitdir: "+filepath.Join(unreadable, "missing")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readGitLandingHistory(unreadable); err == nil || strings.Contains(err.Error(), "not inside a Git worktree") {
		t.Fatalf("an unreadable checkout = %v", err)
	}
}
