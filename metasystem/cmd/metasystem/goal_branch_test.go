package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGoalBranchCheckPrintsKinds(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	goalSyncMutationGit(t, root, "config", "goal.sync-remote", "origin")
	base := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "update-ref", "refs/remotes/origin/main", base)
	commit := func(path, body, message string) string {
		full := filepath.Join(root, filepath.FromSlash(path))
		writeTestingFixtureFile(t, full, []byte(body), 0o644)
		goalSyncMutationGit(t, root, "add", ".")
		goalSyncMutationGit(t, root, "commit", "-qm", message)
		return goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	}
	commit("metasystem/plans/x.md", "plan", "plan\n\nGoal-Plan: goal-a")
	unit := commit("metasystem/code.go", "unit", "unit\n\nGoal-Unit: goal-a/u1")
	tip := commit("metasystem/records/reads/goal-a/"+unit+".json", "{}", "read\n\nGoal-Read: goal-a/u1 "+unit)
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runGoalBranch([]string{"check", "--goal", "goal-a", "--root", root, "--no-fetch", "--tip", tip})
	})
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if code != 0 || stderr != "" || len(lines) != 3 || !strings.Contains(lines[0], " plan ") || !strings.Contains(lines[1], unit[:12]+" unit u1 ") || !strings.Contains(lines[2], " read u1") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	bad := commit("metasystem/plans/bad.md", "bad", "bad\n\nGoal-Unit: goal-a/u2")
	stderr, code = captureStderr(t, func() int {
		return runGoalBranch([]string{"check", "--goal", "goal-a", "--root", root, "--no-fetch", "--tip", bad})
	})
	if code == 0 || !strings.Contains(stderr, "GOAL_BRANCH_RANGE") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	goalSyncMutationGit(t, root, "config", "goal.sync-branch", "refs/heads/develop")
	stderr, code = captureStderr(t, func() int {
		return runGoalBranch([]string{"check", "--goal", "goal-a", "--root", root, "--no-fetch", "--tip", tip})
	})
	if code == 0 || !strings.Contains(stderr, "GOAL_BRANCH_ENDPOINT_UNSUPPORTED") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}
