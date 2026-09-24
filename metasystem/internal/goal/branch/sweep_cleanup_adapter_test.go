package branch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sweepNativeGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func sweepNativeRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	sweepNativeGit(t, repo, "init", "-q")
	sweepNativeGit(t, repo, "config", "user.name", "fixture")
	sweepNativeGit(t, repo, "config", "user.email", "fixture@example.invalid")
	sweepNativeGit(t, repo, "commit", "--allow-empty", "-qm", "base")
	tip := sweepNativeGit(t, repo, "rev-parse", "HEAD")
	sweepNativeGit(t, repo, "branch", "goal/goal-a", tip)
	return repo, tip
}

func TestSweepCleanupNativeAdapter(t *testing.T) {
	t.Parallel()
	t.Run("local_expected_tip_deleted", func(t *testing.T) {
		t.Parallel()
		repo, tip := sweepNativeRepo(t)
		if err := cleanupGoalWorktrees(repo, "goal-a", tip, tip, nil, defaultSweepDependencies()); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", goalBranchRef("goal-a")).CombinedOutput(); err == nil {
			t.Fatalf("unchanged local ref survived expected-tip deletion: %s", out)
		} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("could not check deleted local ref: %v: %s", err, out)
		}
	})
	t.Run("local_moved_tip", func(t *testing.T) {
		t.Parallel()
		repo, tip := sweepNativeRepo(t)
		sweepNativeGit(t, repo, "commit", "--allow-empty", "-qm", "moved")
		moved := sweepNativeGit(t, repo, "rev-parse", "HEAD")
		sweepNativeGit(t, repo, "update-ref", goalBranchRef("goal-a"), moved, tip)
		err := cleanupGoalWorktrees(repo, "goal-a", tip, tip, nil, defaultSweepDependencies())
		var refusal *OpError
		if !errors.As(err, &refusal) || refusal.Code != LeaseMovedCode ||
			sweepNativeGit(t, repo, "rev-parse", goalBranchRef("goal-a")) != moved {
			t.Fatalf("moved local deletion = %v", err)
		}
	})
	t.Run("root_and_linked_worktrees", func(t *testing.T) {
		t.Parallel()
		repo, tip := sweepNativeRepo(t)
		sweepNativeGit(t, repo, "switch", "--quiet", "goal/goal-a")
		if err := cleanupGoalWorktrees(repo, "goal-a", tip, "", []string{repo}, defaultSweepDependencies()); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "-C", repo, "symbolic-ref", "--quiet", "HEAD").CombinedOutput(); err == nil {
			t.Fatalf("root worktree is still attached: %s", out)
		} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("could not check detached root: %v: %s", err, out)
		}
		linked := filepath.Join(t.TempDir(), "linked")
		sweepNativeGit(t, repo, "worktree", "add", "--quiet", linked, "goal/goal-a")
		if err := cleanupGoalWorktrees(repo, "goal-a", tip, "", []string{linked}, defaultSweepDependencies()); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(linked); !os.IsNotExist(err) {
			t.Fatalf("linked worktree still exists: %v", err)
		}
		if strings.Contains(sweepNativeGit(t, repo, "worktree", "list", "--porcelain"), linked) {
			t.Fatal("removed worktree remains registered")
		}
	})
}
