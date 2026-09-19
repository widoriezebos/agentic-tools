package branch_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func TestGoalBranchSweepRules(t *testing.T) {
	t.Parallel()
	t.Run("last landing", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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

func TestSweepAbandonedStillValidatesRemoteRange(t *testing.T) {
	t.Parallel()
	fixture := newBranchFixture(t)
	unknown := git(t, fixture.root, "commit-tree", fixture.base+"^{tree}", "-p", fixture.base, "-m", "unknown")
	git(t, fixture.root, "push", "-q", "origin", unknown+":refs/heads/goal/goal-a")

	_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: fixture.base,
		GoalID: "goal-a", Abandoned: true, CheckClaim: claimAllowed})
	var refusal *branch.RangeError
	if !errors.As(err, &refusal) {
		t.Fatalf("abandoned unknown commit sweep=%v", err)
	}
	if got := git(t, fixture.origin, "rev-parse", "refs/heads/goal/goal-a"); got != unknown {
		t.Fatalf("origin ref after refused sweep = %s, want %s", got, unknown)
	}
}

func landedSweepFixture(t *testing.T) (landFixture, branch.LandResult) {
	t.Helper()
	fixture := newLandFixture(t)
	out, prepared := preparedLanding(t, fixture)
	if _, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
		GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	return fixture, prepared
}

func addSweepTail(t *testing.T, fixture landFixture, endpoint string) string {
	t.Helper()
	stage(t, fixture.branchFixture, "metasystem/plans/unlanded.md", "unlanded plan\n")
	tip, err := branch.CommitStaged(branch.CommitRequest{Repo: fixture.root, Remote: "origin", EndpointTip: endpoint,
		GoalID: "goal-a", OpID: "sweep-tail", Kind: branch.Plan, CheckClaim: claimAllowed})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(branch.PushRequest{Repo: fixture.root, Remote: "origin", EndpointTip: endpoint,
		GoalID: "goal-a", OpID: "sweep-tail-push", CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	return tip
}

func endpointWithConclusion(t *testing.T, root, endpoint, goalID string) string {
	t.Helper()
	worktree := filepath.Join(t.TempDir(), "conclusion")
	git(t, root, "worktree", "add", "--quiet", "--detach", worktree, endpoint)
	write(t, worktree, "metasystem/records/goals/"+goalID+".md", "# "+goalID+"\n\n- State: done\n- Concluded: Finished.\n")
	git(t, worktree, "add", ".")
	git(t, worktree, "commit", "-qm", "goal conclusion")
	return git(t, worktree, "rev-parse", "HEAD")
}

func TestSweepRefusesUnlandedLocalTip(t *testing.T) {
	t.Parallel()
	fixture, prepared := landedSweepFixture(t)
	unlanded := addSweepTail(t, fixture, prepared.Landing)
	git(t, fixture.root, "push", "-q", "--force", "origin", fixture.tip+":refs/heads/goal/goal-a")

	_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
		GoalID: "goal-a", CheckClaim: claimAllowed})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.SweepUnlandedCode ||
		!strings.Contains(refusal.Message, "goal/goal-a") || !strings.Contains(refusal.Message, "local") ||
		!strings.Contains(refusal.Message, unlanded+" (plan )") {
		t.Fatalf("unlanded local sweep = %v", err)
	}
}

func TestSweepRefusalPreservesRefsAndWorktree(t *testing.T) {
	t.Parallel()
	fixture, prepared := landedSweepFixture(t)
	transport := filepath.Join(t.TempDir(), "transport.git")
	git(t, filepath.Dir(transport), "init", "-q", "--bare", transport)
	git(t, fixture.root, "remote", "add", "transport", transport)
	git(t, fixture.root, "push", "-q", "transport", fixture.tip+":refs/heads/goal/goal-a")
	unlanded := addSweepTail(t, fixture, prepared.Landing)
	git(t, fixture.root, "push", "-q", "--force", "origin", fixture.tip+":refs/heads/goal/goal-a")
	worktreesBefore := git(t, fixture.root, "worktree", "list", "--porcelain")

	_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", Transport: "transport",
		EndpointTip: prepared.Landing, GoalID: "goal-a", CheckClaim: claimAllowed})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.SweepUnlandedCode {
		t.Fatalf("unlanded local sweep = %v", err)
	}
	for _, remote := range []string{"origin", "transport"} {
		if got := git(t, fixture.root, "ls-remote", "--refs", remote, "refs/heads/goal/goal-a"); !strings.HasPrefix(got, fixture.tip+"\t") {
			t.Fatalf("%s ref after refusal = %q, want %s", remote, got, fixture.tip)
		}
	}
	if got, present, refErr := localFixtureRef(fixture.root, "refs/heads/goal/goal-a"); refErr != nil || !present || got != unlanded {
		t.Fatalf("local ref after refusal = %s, present=%v, err=%v", got, present, refErr)
	}
	if worktrees := git(t, fixture.root, "worktree", "list", "--porcelain"); worktrees != worktreesBefore {
		t.Fatalf("goal worktree changed after refusal:\nbefore:\n%s\nafter:\n%s", worktreesBefore, worktrees)
	}
}

func TestSweepRefusalNamesEveryUnlandedCommit(t *testing.T) {
	t.Parallel()
	fixture, prepared := landedSweepFixture(t)
	var commits []string
	for index := 0; index < 10; index++ {
		commits = append(commits, commitUnit(t, fixture.branchFixture, fmt.Sprintf("tail-%d", index),
			fmt.Sprintf("metasystem/tail-%d.go", index), fmt.Sprintf("tail %d\n", index)))
	}

	_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
		GoalID: "goal-a", CheckClaim: claimAllowed})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.SweepUnlandedCode {
		t.Fatalf("unlanded local sweep = %v", err)
	}
	wantPrefix := fmt.Sprintf("goal/goal-a at local tip %s has unlanded commits: ", commits[9])
	if !strings.HasPrefix(refusal.Message, wantPrefix) {
		t.Fatalf("refusal = %q, want prefix %q", refusal.Message, wantPrefix)
	}
	previous := -1
	for index, commit := range commits[:8] {
		entry := fmt.Sprintf("%s (unit tail-%d)", commit, index)
		at := strings.Index(refusal.Message, entry)
		if at <= previous {
			t.Fatalf("refusal does not name commits oldest first: %q", refusal.Message)
		}
		previous = at
	}
	if strings.Contains(refusal.Message, commits[8]) || strings.Contains(refusal.Message, commits[9]+" (unit tail-9)") ||
		!strings.HasSuffix(refusal.Message, "and 2 more") {
		t.Fatalf("refusal cap = %q", refusal.Message)
	}
}

func TestSweepKeepsLocalRefMovedAfterCheck(t *testing.T) {
	t.Parallel()
	fixture, prepared := landedSweepFixture(t)
	checked := git(t, fixture.root, "rev-parse", "refs/heads/goal/goal-a")
	moved := git(t, fixture.root, "commit-tree", checked+"^{tree}", "-p", checked, "-m", "moved\n\nGoal-Plan: goal-a")

	_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
		GoalID: "goal-a", CheckClaim: claimAllowed, Hooks: branch.SweepHooks{AfterRemoteRead: func() error {
			git(t, fixture.root, "update-ref", "refs/heads/goal/goal-a", moved, checked)
			return nil
		}}})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.LeaseMovedCode {
		t.Fatalf("moved local sweep = %v", err)
	}
	if remote := git(t, fixture.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a"); remote != "" {
		t.Fatalf("origin ref survived its leased deletion: %q", remote)
	}
	if got, present, refErr := localFixtureRef(fixture.root, "refs/heads/goal/goal-a"); refErr != nil || !present || got != moved {
		t.Fatalf("moved local ref = %s, present=%v, err=%v", got, present, refErr)
	}
}

func TestSweepLocalTipExemptions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		run  func(*testing.T, landFixture, branch.LandResult, string)
	}{
		{name: "abandoned", run: func(t *testing.T, fixture landFixture, prepared branch.LandResult, _ string) {
			result, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
				GoalID: "goal-a", Abandoned: true, CheckClaim: claimAllowed})
			if err != nil || !result.Deleted {
				t.Fatalf("abandoned local sweep = %+v, %v", result, err)
			}
		}},
		{name: "declared dropped", run: func(t *testing.T, fixture landFixture, prepared branch.LandResult, tail string) {
			digest, err := branch.UnitDigest(fixture.root, tail)
			if err != nil {
				t.Fatal(err)
			}
			endpoint := endpointWithConclusion(t, fixture.root, prepared.Landing, "goal-a")
			result, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: endpoint,
				GoalID: "goal-a", Dropped: "dropped:" + tail + ":" + digest, CheckClaim: claimAllowed})
			if err != nil || !result.Deleted {
				t.Fatalf("dropped local sweep = %+v, %v", result, err)
			}
		}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture, prepared := landedSweepFixture(t)
			tail := addSweepTail(t, fixture, prepared.Landing)
			git(t, fixture.root, "push", "-q", "--force", "origin", fixture.tip+":refs/heads/goal/goal-a")
			test.run(t, fixture, prepared, tail)
			if _, present, refErr := localFixtureRef(fixture.root, "refs/heads/goal/goal-a"); refErr != nil || present {
				t.Fatalf("local ref after exempt sweep present=%v, err=%v", present, refErr)
			}
		})
	}
}

func TestSweepRefusesDirtyGoalWorktree(t *testing.T) {
	t.Parallel()
	fixture, prepared := landedSweepFixture(t)
	write(t, fixture.root, "metasystem/one.go", "dirty worktree\n")
	_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: prepared.Landing,
		GoalID: "goal-a", CheckClaim: claimAllowed})
	if err == nil || !strings.Contains(err.Error(), fixture.root) || !strings.Contains(err.Error(), "metasystem/one.go") {
		t.Fatalf("dirty worktree sweep = %v", err)
	}
	if remote := git(t, fixture.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a"); remote == "" {
		t.Fatal("dirty worktree refusal deleted origin first")
	}
}

func TestSweepRefusesTransportTipWithUnlandedCommit(t *testing.T) {
	t.Parallel()
	fixture, prepared := landedSweepFixture(t)
	transport := filepath.Join(t.TempDir(), "transport.git")
	git(t, filepath.Dir(transport), "init", "-q", "--bare", transport)
	git(t, fixture.root, "remote", "add", "transport", transport)
	unlanded := addSweepTail(t, fixture, prepared.Landing)
	git(t, fixture.root, "push", "-q", "transport", unlanded+":refs/heads/goal/goal-a")
	git(t, fixture.root, "push", "-q", "origin", fixture.tip+":refs/heads/goal/goal-a", "--force")
	_, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", Transport: "transport",
		EndpointTip: prepared.Landing, GoalID: "goal-a", CheckClaim: claimAllowed})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.SweepUnlandedCode {
		t.Fatalf("unlanded transport sweep = %v", err)
	}
	if git(t, fixture.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a") == "" {
		t.Fatal("transport refusal deleted origin first")
	}
}

func TestSweepCleansLeftoversWhenOriginBranchIsGone(t *testing.T) {
	t.Parallel()
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
	git(t, fixture.root, "push", "-q", "origin", ":refs/heads/goal/goal-a")
	result, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", Transport: "transport",
		EndpointTip: prepared.Landing, GoalID: "goal-a", CheckClaim: claimAllowed})
	if err != nil || !result.Deleted {
		t.Fatalf("leftover sweep = %+v, %v", result, err)
	}
	for _, remote := range []string{"origin", "transport"} {
		if got := git(t, fixture.root, "ls-remote", "--refs", remote, "refs/heads/goal/goal-a"); got != "" {
			t.Fatalf("%s leftover = %s", remote, got)
		}
	}
	if _, present, _ := localFixtureRef(fixture.root, "refs/heads/goal/goal-a"); present {
		t.Fatal("local goal ref survived")
	}
}

func TestSweepRequiresExactConcludedDroppedDeclaration(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		declaration func(string, string) string
		concluded   bool
		wantSweep   bool
	}{
		{name: "prose mention", declaration: func(commit, digest string) string { return "commit " + commit + " was landed with digest " + digest }, concluded: true},
		{name: "exact concluded entry", declaration: func(commit, digest string) string { return "resume later dropped:" + commit + ":" + digest }, concluded: true, wantSweep: true},
		{name: "exact entry without conclusion", declaration: func(commit, digest string) string { return "dropped:" + commit + ":" + digest }},
		{name: "last landing takes no exemption", declaration: func(commit, digest string) string { return "dropped:" + commit + ":" + digest }},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture, prepared := landedSweepFixture(t)
			tail := addSweepTail(t, fixture, prepared.Landing)
			digest, err := branch.UnitDigest(fixture.root, tail)
			if err != nil {
				t.Fatal(err)
			}
			endpoint := prepared.Landing
			if test.concluded {
				endpoint = endpointWithConclusion(t, fixture.root, endpoint, "goal-a")
			}
			result, err := branch.Sweep(branch.SweepRequest{Repo: fixture.root, Remote: "origin", EndpointTip: endpoint,
				GoalID: "goal-a", Dropped: test.declaration(tail, digest), CheckClaim: claimAllowed})
			if test.wantSweep {
				if err != nil || !result.Deleted {
					t.Fatalf("exact concluded sweep = %+v, %v", result, err)
				}
				return
			}
			var refusal *branch.OpError
			if !errors.As(err, &refusal) || refusal.Code != branch.SweepUnlandedCode {
				t.Fatalf("dropped declaration refusal = %v", err)
			}
		})
	}
}
