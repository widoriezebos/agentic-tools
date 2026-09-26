package branch_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// TestGoalBranchLandingEventVerifier drives the landing event owner against a
// real published goal-branch landing: only the goal's last landing matches,
// another goal and an earlier unit do not, and a commit carrying the same
// message over different entries is refused instead of matched.
func TestGoalBranchLandingEventVerifier(t *testing.T) {
	t.Parallel()
	fixture := newLandFixture(t)
	out, prepared := preparedLanding(t, fixture)
	if _, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
		GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	matched, provenance, err := goal.VerifiedGoalBranchLanding(ctx, fixture.root, prepared.Landing, "goal-a")
	if err != nil || !matched || !strings.HasPrefix(provenance, "goal-branch goal=goal-a units=") {
		t.Fatalf("last landing: matched=%v provenance=%q err=%v", matched, provenance, err)
	}
	if matched, _, err := goal.VerifiedGoalBranchLanding(ctx, fixture.root, prepared.Landing, "goal-b"); matched || err != nil {
		t.Fatalf("another goal matched=%v err=%v", matched, err)
	}
	series, err := branch.VerifyLandedSeries(fixture.root, prepared.Landing)
	if err != nil || len(series) < 2 {
		t.Fatalf("series=%+v err=%v", series, err)
	}
	if matched, _, err := goal.VerifiedGoalBranchLanding(ctx, fixture.root, series[0].Commit, "goal-a"); matched || err != nil {
		t.Fatalf("an earlier unit matched=%v err=%v", matched, err)
	}

	worktree := filepath.Join(t.TempDir(), "mutation")
	git(t, fixture.root, "worktree", "add", "--quiet", "--detach", worktree, prepared.Landing+"^")
	write(t, worktree, "metasystem/one.go", "mutated\n")
	git(t, worktree, "add", "metasystem/one.go")
	tree := git(t, worktree, "write-tree")
	messageFile := filepath.Join(t.TempDir(), "message")
	if err := os.WriteFile(messageFile, []byte(git(t, fixture.root, "show", "-s", "--format=%B", prepared.Landing)), 0o644); err != nil {
		t.Fatal(err)
	}
	forged := git(t, fixture.root, "commit-tree", tree, "-p", prepared.Landing+"^", "-F", messageFile)
	if matched, _, err := goal.VerifiedGoalBranchLanding(ctx, fixture.root, forged, "goal-a"); matched || err == nil {
		t.Fatalf("a forged landing matched=%v err=%v", matched, err)
	}
}
