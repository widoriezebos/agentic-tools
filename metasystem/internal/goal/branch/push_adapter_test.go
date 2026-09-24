package branch_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// TestPushGitAdapter checks the physical Git operations used by Push. Policy
// decisions are covered by the Git-denied tests in the internal package.
func TestPushGitAdapter(t *testing.T) {
	t.Run("transport_adopt", func(t *testing.T) {
		f := newBranchFixture(t)
		tip := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		result, err := branch.Push(pushRequest(f, "adapter-publish"))
		if err != nil || result.Tip != tip || remoteGoalTip(t, f) != tip {
			t.Fatalf("force-with-lease publish = %+v, %v", result, err)
		}
		other := cloneBranchFixture(t, f)
		write(t, other.root, "notes/scratch.txt", "keep me")
		result, err = branch.Push(pushRequest(other, "adapter-adopt"))
		if err != nil || result.State != "adopted" || result.Tip != tip {
			t.Fatalf("second clone adoption = %+v, %v", result, err)
		}
		if got := git(t, other.root, "rev-parse", "refs/heads/goal/goal-a"); got != tip {
			t.Fatalf("adopted ref = %s, want %s", got, tip)
		}
		if got := goalRef(t, other.root, "refs/metasystem/goals/fetch/"); got != "" {
			t.Fatalf("adoption left fetch ref %s", got)
		}
		bytes, err := os.ReadFile(filepath.Join(other.root, "notes/scratch.txt"))
		if err != nil || string(bytes) != "keep me" {
			t.Fatalf("scratch bytes = %q, %v", bytes, err)
		}
		if got := git(t, other.root, "status", "--porcelain=v1", "--untracked-files=all"); got != "?? notes/scratch.txt" {
			t.Fatalf("scratch status = %q", got)
		}

		malformed := newBranchFixture(t)
		invalid := git(t, malformed.root, "commit-tree", malformed.base+"^{tree}", "-p", malformed.base, "-m", "invalid")
		git(t, malformed.root, "push", "-q", "origin", invalid+":refs/heads/goal/goal-a")
		invalidClone := cloneBranchFixture(t, malformed)
		_, err = branch.Push(pushRequest(invalidClone, "adapter-invalid-adoption"))
		var refusal *branch.RangeError
		if !errors.As(err, &refusal) {
			t.Fatalf("invalid adoption = %v", err)
		}
		if _, present, _ := localFixtureRef(invalidClone.root, "refs/heads/goal/goal-a"); present {
			t.Fatal("invalid remote tip was adopted")
		}
		if got := goalRef(t, invalidClone.root, "refs/metasystem/goals/fetch/"); got != "" {
			t.Fatalf("invalid adoption left fetch ref %s", got)
		}
	})

	t.Run("failed_ref_move", func(t *testing.T) {
		f := newBranchFixture(t)
		first := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		if _, err := branch.Push(pushRequest(f, "adapter-restore-first")); err != nil {
			t.Fatal(err)
		}
		other := cloneBranchFixture(t, f)
		if _, err := branch.Push(pushRequest(other, "adapter-restore-adopt")); err != nil {
			t.Fatal(err)
		}
		commitUnit(t, other, "u2", "metasystem/other.go", "two")
		if _, err := branch.Push(pushRequest(other, "adapter-restore-second")); err != nil {
			t.Fatal(err)
		}
		request := pushRequest(f, "adapter-restore-failed-update")
		var moved string
		request.Hooks.BeforeAdoptionRefMove = func() error {
			moved = git(t, f.root, "commit-tree", first+"^{tree}", "-p", first, "-m", "local move\n\nGoal-Plan: goal-a")
			git(t, f.root, "update-ref", "refs/heads/goal/goal-a", moved, first)
			return nil
		}
		if _, err := branch.Push(request); err == nil {
			t.Fatal("adoption with failed ref transaction succeeded")
		}
		if got := git(t, f.root, "symbolic-ref", "-q", "HEAD"); got != "refs/heads/goal/goal-a" {
			t.Fatalf("HEAD stayed detached at %s", got)
		}
		if got := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a"); got != moved {
			t.Fatalf("failed transaction overwrote competing ref: %s", got)
		}
		if got := git(t, f.root, "rev-parse", "refs/metasystem/goals/origin/goal-a"); got != first {
			t.Fatalf("failed transaction moved origin: %s", got)
		}
	})

	t.Run("after_ref_move_crash", func(t *testing.T) {
		f := newBranchFixture(t)
		first := commitUnit(t, f, "u1", "metasystem/one.go", "one")
		if _, err := branch.Push(pushRequest(f, "adapter-publish-first")); err != nil {
			t.Fatal(err)
		}
		other := cloneBranchFixture(t, f)
		if _, err := branch.Push(pushRequest(other, "adapter-adopt-first")); err != nil {
			t.Fatal(err)
		}
		second := commitUnit(t, other, "u2", "metasystem/two.go", "two")
		if _, err := branch.Push(pushRequest(other, "adapter-publish-second")); err != nil {
			t.Fatal(err)
		}
		request := pushRequest(f, "adapter-crash-after-ref")
		request.Hooks.AfterAdoptionRefMove = func() error { return errors.New("simulated adoption crash") }
		if _, err := branch.Push(request); err == nil || !strings.Contains(err.Error(), "simulated adoption crash") {
			t.Fatalf("crash result = %v", err)
		}
		if got := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a"); got != second {
			t.Fatalf("goal ref = %s, want %s (first=%s)", got, second, first)
		}
		if got := git(t, f.root, "rev-parse", "HEAD"); got != second {
			t.Fatalf("detached checkout = %s, want %s", got, second)
		}
		if out, err := exec.Command("git", "-C", f.root, "symbolic-ref", "-q", "HEAD").CombinedOutput(); err == nil || strings.TrimSpace(string(out)) != "" {
			t.Fatalf("HEAD was reattached: %s, %v", out, err)
		} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("could not inspect detached HEAD: %s, %v", out, err)
		}
		if status := git(t, f.root, "status", "--porcelain=v1"); status != "" {
			t.Fatalf("adoption crash staged a reversal: %q", status)
		}
	})
}

// TestPushCommitFollowonGitAdapter checks the Git ref and parent handoffs
// that the shared policy fixtures cannot establish from symbolic commits.
func TestPushCommitFollowonGitAdapter(t *testing.T) {
	t.Run("lease_refusal", func(t *testing.T) {
		f := newBranchFixture(t)
		first := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		if _, err := branch.Push(pushRequest(f, "adapter-lease-seed")); err != nil {
			t.Fatal(err)
		}
		stage(t, f, "metasystem/code.go", "two")
		local, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "adapter-lease-amend", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
		if err != nil {
			t.Fatal(err)
		}
		req := pushRequest(f, "adapter-lease-race")
		var competitor string
		req.Hooks.AfterRemoteRead = func() error {
			competitor = git(t, f.root, "commit-tree", first+"^{tree}", "-p", first, "-m", "competitor\n\nGoal-Plan: goal-a")
			git(t, f.root, "push", "-q", "--force", "origin", competitor+":refs/heads/goal/goal-a")
			return nil
		}
		_, err = branch.Push(req)
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.LeaseMovedCode || remoteGoalTip(t, f) != competitor || remoteGoalTip(t, f) == local {
			t.Fatalf("lease refusal=%v remote=%s competitor=%s local=%s", err, remoteGoalTip(t, f), competitor, local)
		}
		if got := goalRef(t, f.root, "refs/metasystem/goals/txn/"); got != "" {
			t.Fatalf("lease refusal left transaction %s", got)
		}
	})
	t.Run("remote_adoption_parent", func(t *testing.T) {
		f := newBranchFixture(t)
		first := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		if _, err := branch.Push(pushRequest(f, "adapter-parent-first")); err != nil {
			t.Fatal(err)
		}
		other := cloneBranchFixture(t, f)
		if result, err := branch.Push(pushRequest(other, "adapter-parent-adopt-first")); err != nil || result.Tip != first {
			t.Fatalf("second client adoption=%+v,%v", result, err)
		}
		stage(t, other, "metasystem/code.go", "two")
		second, err := branch.CommitStaged(branch.CommitRequest{Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u1", OpID: "adapter-parent-second", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := branch.Push(pushRequest(other, "adapter-parent-push-second")); err != nil {
			t.Fatal(err)
		}
		if result, err := branch.Push(pushRequest(f, "adapter-parent-adopt-second")); err != nil || result.State != "adopted" || result.Tip != second {
			t.Fatalf("first client adoption=%+v,%v", result, err)
		}
		stage(t, other, "metasystem/code.go", "three")
		third, err := branch.CommitStaged(branch.CommitRequest{Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u1", OpID: "adapter-parent-third", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := branch.Push(pushRequest(other, "adapter-parent-push-third")); err != nil {
			t.Fatal(err)
		}
		stage(t, f, "metasystem/next.go", "next")
		next, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u2", OpID: "adapter-parent-next", Kind: branch.Unit, CheckClaim: claimAllowed})
		if err != nil || git(t, f.root, "rev-parse", next+"^") != third || remoteGoalTip(t, f) != third {
			t.Fatalf("commit parent=%s err=%v remote=%s want parent=%s", next, err, remoteGoalTip(t, f), third)
		}
	})
	t.Run("crash_recovery_parent", func(t *testing.T) {
		f := newBranchFixture(t)
		commitUnit(t, f, "u0", "metasystem/seed.go", "seed")
		if _, err := branch.Push(pushRequest(f, "adapter-crash-seed")); err != nil {
			t.Fatal(err)
		}
		landed := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		req := pushRequest(f, "adapter-crash-landed")
		req.Hooks.AfterPush = func() error { return errors.New("process stopped after push") }
		if _, err := branch.Push(req); err == nil || remoteGoalTip(t, f) != landed {
			t.Fatalf("landed crash=%v remote=%s", err, remoteGoalTip(t, f))
		}
		if got := goalRef(t, f.root, "refs/metasystem/goals/txn/"); got == "" {
			t.Fatal("crash did not retain transaction")
		}
		other := cloneBranchFixture(t, f)
		if _, err := branch.Push(pushRequest(other, "adapter-crash-adopt")); err != nil {
			t.Fatal(err)
		}
		commitUnit(t, other, "u2", "metasystem/other.go", "two")
		if _, err := branch.Push(pushRequest(other, "adapter-crash-advance")); err != nil {
			t.Fatal(err)
		}
		remote := remoteGoalTip(t, f)
		if result, err := branch.Push(pushRequest(f, "adapter-crash-recover")); err != nil || result.State != "reconciled" || result.Tip != remote {
			t.Fatalf("recovery=%+v,%v", result, err)
		}
		if got := goalRef(t, f.root, "refs/metasystem/goals/txn/"); got != "" {
			t.Fatalf("recovery left transaction %s", got)
		}
		if got := git(t, f.root, "rev-parse", "refs/metasystem/goals/origin/goal-a"); got != remote {
			t.Fatalf("recorded origin=%s want %s", got, remote)
		}
		stage(t, f, "metasystem/next.go", "three")
		next, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u3", OpID: "adapter-crash-next", Kind: branch.Unit, CheckClaim: claimAllowed})
		if err != nil || git(t, f.root, "rev-parse", next+"^") != remote {
			t.Fatalf("recovered commit=%s err=%v want parent=%s", next, err, remote)
		}
	})
}
