package landpath

import (
	"os"
	"path/filepath"
	"testing"
)

// A checkout whose repository records no linked worktree serves none, and
// the reaper's tick there starts no Git process to find that out: every
// armed checkout ticks, and paths that only wait run under Git stubs that
// admit no such call.
func TestServedInstallationsAskGitOnlyWhenWorktreesAreRecorded(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	installation := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	calls := 0
	git := func(GitCall) GitResult {
		calls++
		return GitResult{Code: 1}
	}
	if served := ServedInstallations(git, installation); served != nil || calls != 0 {
		t.Fatalf("no linked worktree: served %v after %d Git call(s)", served, calls)
	}
	if err := os.MkdirAll(filepath.Join(checkout, ".git", "worktrees", "goal-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if ServedInstallations(git, installation); calls == 0 {
		t.Fatal("a recorded linked worktree must be looked up through Git")
	}
}
