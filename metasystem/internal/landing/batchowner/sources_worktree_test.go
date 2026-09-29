package batchowner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// An owner killed during a retained verification leaves that worktree
// registered in the lane and on disk. The next owner of the lane removes it
// at start, and a later verification of the same batch reuses the path.
func TestBatchRetainedSourcesWorktreeLeftByAKilledOwnerIsSwept(t *testing.T) {
	t.Parallel()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "--quiet", "-b", "main")
	git("config", "user.email", "seat@invalid")
	git("config", "user.name", "Seat")
	if err := os.MkdirAll(filepath.Join(repo, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "metasystem", "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "metasystem/base.txt")
	git("commit", "--quiet", "-m", "base")
	tree := git("rev-parse", "HEAD^{tree}")
	const id = "01j5x00000000000000000ba01"
	t.Cleanup(func() { _ = sweepBatchSourcesWorktrees(repo) })

	// A verification whose process dies: the worktree is never closed.
	killed, err := openBatchSourcesWorktree(repo, id, tree)
	if err != nil {
		t.Fatal(err)
	}
	left := killed.Workspace().Dir
	if _, err := os.Stat(filepath.Join(left, "metasystem", "base.txt")); err != nil || !strings.Contains(git("worktree", "list"), left) {
		t.Fatalf("the verification worktree was not materialized at %s: %v", left, err)
	}
	// The same batch verifies again before any sweep: the stale path is taken over.
	again, err := openBatchSourcesWorktree(repo, id, tree)
	if err != nil || again.Workspace().Dir != left {
		t.Fatalf("a second verification of the batch after a kill: dir=%v err=%v", again, err)
	}
	if err := BatchOwnerSweepSources(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(left); !os.IsNotExist(err) || strings.Contains(git("worktree", "list"), left) {
		t.Fatalf("the owner's start sweep left %s: stat=%v list=%s", left, err, git("worktree", "list"))
	}
}
