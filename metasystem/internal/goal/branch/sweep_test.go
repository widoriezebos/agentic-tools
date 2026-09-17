package branch_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func TestGoalBranchSweepRules(t *testing.T) {
	t.Run("last landing", func(t *testing.T) {
		fixture := newLandFixture(t)
		transport := filepath.Join(t.TempDir(), "transport.git")
		git(t, filepath.Dir(transport), "init", "-q", "--bare", transport)
		git(t, fixture.root, "remote", "add", "transport", transport)
		git(t, fixture.root, "push", "-q", "transport", fixture.tip+":refs/heads/goal/goal-a")
		out, prepared := preparedLanding(t, fixture)
		if _, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
			GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		result, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
			Transport: "transport", GoalID: "goal-a", CheckClaim: claimAllowed})
		if err != nil || !result.Deleted || git(t, fixture.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a") != "" ||
			git(t, fixture.root, "ls-remote", "--refs", "transport", "refs/heads/goal/goal-a") != "" {
			t.Fatalf("sweep=%+v err=%v", result, err)
		}
	})

	t.Run("plan tail and abandoned word", func(t *testing.T) {
		fixture := newLandFixture(t)
		out, prepared := preparedLanding(t, fixture)
		if _, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
			GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		stage(t, fixture.branchFixture, "metasystem/plans/unlanded.md", "plan tail\n")
		if _, err := branch.CommitStaged(branch.CommitRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
			GoalID: "goal-a", OpID: "plan-tail", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		if _, err := branch.Push(branch.PushRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
			GoalID: "goal-a", OpID: "plan-tail-push", CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
			GoalID: "goal-a", CheckClaim: claimAllowed})
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.SweepUnlandedCode {
			t.Fatalf("plan-only tail=%v", err)
		}
		if _, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
			GoalID: "goal-a", Abandoned: true, CheckClaim: claimAllowed}); err != nil {
			t.Fatalf("abandoned deletion: %v", err)
		}
	})

	t.Run("tip race", func(t *testing.T) {
		fixture := newLandFixture(t)
		out, prepared := preparedLanding(t, fixture)
		if _, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
			GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		oldTip := git(t, fixture.root, "rev-parse", "refs/heads/goal/goal-a")
		moved := git(t, fixture.root, "commit-tree", oldTip+"^{tree}", "-p", oldTip, "-m", "tail\n\nGoal-Plan: goal-a")
		req := branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
			GoalID: "goal-a", CheckClaim: claimAllowed, Hooks: branch.SweepHooks{AfterRemoteRead: func() error {
				git(t, fixture.root, "push", "-q", "--force", "origin", moved+":refs/heads/goal/goal-a")
				return nil
			}}}
		_, err := branch.Sweep(req)
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.LeaseMovedCode || git(t, fixture.origin, "rev-parse", "refs/heads/goal/goal-a") != moved {
			t.Fatalf("moved-tip sweep=%v", err)
		}
	})

	t.Run("orphan landing word", func(t *testing.T) {
		fixture := newLandFixture(t)
		_, _ = preparedLanding(t, fixture)
		if err := branch.DeleteLanding(branch.DeleteLandingRequest{Repo: fixture.root, Remote: "origin", GoalID: "goal-a", CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		if refs := git(t, fixture.root, "ls-remote", "--refs", "origin", "refs/heads/landing/goal-a"); refs != "" {
			t.Fatalf("orphan landing survived: %s", refs)
		}
	})

	t.Run("unknown commit", func(t *testing.T) {
		fixture := newBranchFixture(t)
		unknown := git(t, fixture.root, "commit-tree", fixture.base+"^{tree}", "-p", fixture.base, "-m", "unknown")
		git(t, fixture.root, "push", "-q", "origin", unknown+":refs/heads/goal/goal-a")
		_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: fixture.base,
			GoalID: "goal-a", CheckClaim: claimAllowed})
		var refusal *branch.RangeError
		if !errors.As(err, &refusal) {
			t.Fatalf("unknown commit sweep=%v", err)
		}
	})
}
