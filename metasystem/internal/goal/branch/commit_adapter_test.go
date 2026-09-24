package branch_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestCommitStagedGitAdapter(t *testing.T) {
	t.Run("create_adopt", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		stage(t, f, "metasystem/plans/goal-a.md", "plan")
		plan, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", OpID: "commit-plan", Kind: branch.Plan, CheckClaim: claimAllowed})
		if err != nil {
			t.Fatal(err)
		}
		if parent := git(t, f.root, "rev-parse", plan+"^"); parent != f.base {
			t.Fatalf("plan parent=%s want %s", parent, f.base)
		}
		if message := git(t, f.root, "show", "-s", "--format=%B", plan); !strings.Contains(message, "Goal-Plan: goal-a") {
			t.Fatalf("plan message=%q", message)
		}
		if head := git(t, f.root, "symbolic-ref", "HEAD"); head != "refs/heads/goal/goal-a" {
			t.Fatalf("HEAD=%s", head)
		}
		if got := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a"); got != plan {
			t.Fatalf("goal ref=%s want %s", got, plan)
		}
		if _, present, _ := localFixtureRef(f.root, "refs/metasystem/goals/origin/goal-a"); present {
			t.Fatal("fresh goal has origin ref")
		}
		if tree, index := git(t, f.root, "rev-parse", plan+"^{tree}"), git(t, f.root, "write-tree"); tree != index {
			t.Fatalf("plan tree=%s index=%s", tree, index)
		}
		if status := git(t, f.root, "status", "--porcelain"); status != "" {
			t.Fatalf("plan checkout status=%q", status)
		}
		stage(t, f, "metasystem/code.go", "one")
		unit, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "commit-u1", Kind: branch.Unit, CheckClaim: claimAllowed})
		if err != nil {
			t.Fatal(err)
		}
		if parent := git(t, f.root, "rev-parse", unit+"^"); parent != plan {
			t.Fatalf("unit parent=%s want %s", parent, plan)
		}
		if msg := git(t, f.root, "show", "-s", "--format=%B", unit); !strings.Contains(msg, "Goal-Unit: goal-a/u1") {
			t.Fatalf("unit message=%q", msg)
		}
		if tree, index := git(t, f.root, "rev-parse", unit+"^{tree}"), git(t, f.root, "write-tree"); tree != index {
			t.Fatalf("unit tree=%s index=%s", tree, index)
		}
		if got := git(t, f.root, "rev-parse", "HEAD"); got != unit {
			t.Fatalf("HEAD tip=%s want %s", got, unit)
		}
		if got := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a"); got != unit {
			t.Fatalf("goal tip=%s want %s", got, unit)
		}

		remoteFixture := newBranchFixture(t)
		remote := commitUnit(t, remoteFixture, "u1", "metasystem/remote.go", "remote")
		if _, err := branch.Push(pushRequest(remoteFixture, "seed-remote")); err != nil {
			t.Fatal(err)
		}
		other := cloneBranchFixture(t, remoteFixture)
		other.commit(t, "local-only.txt", "local", "unrelated local commit")
		unrelated := git(t, other.root, "rev-parse", "HEAD")
		write(t, other.root, "unrelated-untracked.txt", "keep local bytes")
		stage(t, other, "metasystem/next.go", "next")
		adopted, err := branch.CommitStaged(branch.CommitRequest{Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u2", OpID: "adopt-away-from-endpoint", Kind: branch.Unit, CheckClaim: claimAllowed})
		if err != nil {
			t.Fatal(err)
		}
		if parent := git(t, other.root, "rev-parse", adopted+"^"); parent != remote {
			t.Fatalf("adopted parent=%s want remote %s", parent, remote)
		}
		if unrelated == remote || unrelated == other.base {
			t.Fatalf("unrelated HEAD witness=%s", unrelated)
		}
		if got := git(t, other.root, "rev-parse", "refs/heads/goal/goal-a"); got != adopted {
			t.Fatalf("adopted goal ref=%s", got)
		}
		if got := git(t, other.root, "rev-parse", "refs/metasystem/goals/origin/goal-a"); got != remote {
			t.Fatalf("adopted origin=%s want %s", got, remote)
		}
		if head := git(t, other.root, "symbolic-ref", "HEAD"); head != "refs/heads/goal/goal-a" {
			t.Fatalf("adopted HEAD=%s", head)
		}
		if tree, index := git(t, other.root, "rev-parse", adopted+"^{tree}"), git(t, other.root, "write-tree"); tree != index {
			t.Fatalf("adopted tree=%s index=%s", tree, index)
		}
		if got, readErr := os.ReadFile(filepath.Join(other.root, "unrelated-untracked.txt")); readErr != nil || string(got) != "keep local bytes" {
			t.Fatalf("unrelated file=%q err=%v", got, readErr)
		}
		if got, readErr := os.ReadFile(filepath.Join(other.root, "metasystem/next.go")); readErr != nil || string(got) != "next" {
			t.Fatalf("adopted file=%q err=%v", got, readErr)
		}

		collision := newBranchFixture(t)
		commitUnit(t, collision, "u1", "metasystem/one.go", "one")
		if _, err := branch.Push(pushRequest(collision, "collision-first")); err != nil {
			t.Fatal(err)
		}
		peer := cloneBranchFixture(t, collision)
		if _, err := branch.Push(pushRequest(peer, "collision-adopt")); err != nil {
			t.Fatal(err)
		}
		commitUnit(t, peer, "u2", "metasystem/collision.go", "remote")
		if _, err := branch.Push(pushRequest(peer, "collision-remote")); err != nil {
			t.Fatal(err)
		}
		write(t, collision.root, "metasystem/collision.go", "local untracked")
		stage(t, collision, "metasystem/next.go", "next")
		before := snapshotCheckout(t, collision.root)
		_, err = branch.CommitStaged(branch.CommitRequest{Repo: collision.root, Remote: "origin", EndpointTip: collision.base, GoalID: "goal-a", Unit: "u3", OpID: "collision-local", Kind: branch.Unit, CheckClaim: claimAllowed})
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.StaleCode || !strings.Contains(err.Error(), "Untracked working tree") {
			t.Fatalf("collision refusal=%v", err)
		}
		requireCheckoutUnchanged(t, collision.root, before)
		if got, readErr := os.ReadFile(filepath.Join(collision.root, "metasystem/collision.go")); readErr != nil || string(got) != "local untracked" {
			t.Fatalf("collision file=%q err=%v", got, readErr)
		}
	})
	t.Run("pre_commit", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		hooks := filepath.Join(f.root, ".fixture-hooks")
		if err := os.MkdirAll(hooks, 0755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\necho fixture hook refusal >&2\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
		git(t, f.root, "config", "core.hooksPath", hooks)
		stage(t, f, "metasystem/code.go", "one")
		before := snapshotCheckout(t, f.root)
		_, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "hook-refusal", Kind: branch.Unit, CheckClaim: claimAllowed})
		if err == nil || !strings.Contains(err.Error(), "fixture hook refusal") {
			t.Fatalf("hook refusal=%v", err)
		}
		requireCheckoutUnchanged(t, f.root, before)
		if _, present, _ := localFixtureRef(f.root, "refs/heads/goal/goal-a"); present {
			t.Fatal("failed commit created goal branch")
		}
	})
	t.Run("replay", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		readPath := "metasystem/records/reads/goal-a/" + unit + ".json"
		f.commit(t, readPath, "{}", "read\n\nGoal-Read: goal-a/u1 "+unit)
		plan := f.commit(t, "metasystem/plans/later.md", "later", "later plan\n\nGoal-Plan: goal-a")
		stage(t, f, "metasystem/code.go", "two")
		tip, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-stale-read", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
		if err != nil {
			t.Fatal(err)
		}
		commits, err := branch.ValidateRange(f.root, f.base, tip, "goal-a")
		if err != nil || len(commits) != 2 || commits[0].Kind != branch.Unit || commits[1].Kind != branch.Plan || commits[0].ID == unit || commits[1].ID == plan {
			t.Fatalf("replayed suffix=%+v err=%v", commits, err)
		}
		if parent := git(t, f.root, "rev-parse", commits[0].ID+"^"); parent != f.base {
			t.Fatalf("amended parent=%s want %s", parent, f.base)
		}
		if parent := git(t, f.root, "rev-parse", tip+"^"); parent != commits[0].ID {
			t.Fatalf("replayed plan parent=%s want %s", parent, commits[0].ID)
		}
		if msg := git(t, f.root, "show", "-s", "--format=%B", commits[0].ID); !strings.Contains(msg, "Goal-Unit: goal-a/u1") {
			t.Fatalf("amended message=%q", msg)
		}
		if got := git(t, f.root, "show", tip+":metasystem/code.go"); got != "two" {
			t.Fatalf("amended bytes=%q", got)
		}
		if got := git(t, f.root, "show", tip+":metasystem/plans/later.md"); got != "later" {
			t.Fatalf("replayed plan=%q", got)
		}
		if _, err := execGit(f.root, "cat-file", "-e", tip+":"+readPath); err == nil {
			t.Fatal("replaced build read survived replay")
		}
		if tree, index := git(t, f.root, "rev-parse", tip+"^{tree}"), git(t, f.root, "write-tree"); tree != index {
			t.Fatalf("replayed tree=%s index=%s", tree, index)
		}
		if got, err := os.ReadFile(filepath.Join(f.root, "metasystem/plans/later.md")); err != nil || string(got) != "later" {
			t.Fatalf("later plan bytes=%q err=%v", got, err)
		}
	})
	t.Run("rollback_goal", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		prepareTrackedAmendFixture(t, f)
		commitUnit(t, f, "u1", "metasystem/code.go", "one")
		write(t, f.root, "metasystem/keep-a.go", "local a")
		stage(t, f, "metasystem/code.go", "two")
		lockPath := filepath.Join(f.root, ".git", "refs", "heads", "goal", "goal-a.lock")
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		before := snapshotCheckout(t, f.root)
		if before.head != "refs/heads/goal/goal-a" {
			t.Fatalf("starting HEAD=%s", before.head)
		}
		_, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-install-failure", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
		if err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
			t.Fatalf("locked ref error=%v", err)
		}
		requireCheckoutUnchanged(t, f.root, before)
		for path, want := range map[string]string{"metasystem/keep-a.go": "local a", "metasystem/code.go": "two"} {
			if got, readErr := os.ReadFile(filepath.Join(f.root, path)); readErr != nil || string(got) != want {
				t.Fatalf("rollback %s=%q err=%v", path, got, readErr)
			}
		}
	})
	t.Run("rollback_other", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		prepareTrackedAmendFixture(t, f)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		readPath := "metasystem/records/reads/goal-a/" + unit + ".json"
		f.commit(t, readPath, "{}", "read\n\nGoal-Read: goal-a/u1 "+unit)
		git(t, f.root, "switch", "--quiet", "-c", "other")
		stage(t, f, "metasystem/code.go", "two")
		lockPath := filepath.Join(f.root, ".git", "refs", "heads", "goal", "goal-a.lock")
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		before := snapshotCheckout(t, f.root)
		if before.head != "refs/heads/other" {
			t.Fatalf("starting HEAD=%s", before.head)
		}
		readBefore, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(readPath)))
		if err != nil {
			t.Fatal(err)
		}
		_, err = branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-moved-install-failure", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
		if err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
			t.Fatalf("locked moved error=%v", err)
		}
		requireCheckoutUnchanged(t, f.root, before)
		if got, readErr := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(readPath))); readErr != nil || string(got) != string(readBefore) {
			t.Fatalf("rollback read=%q err=%v", got, readErr)
		}
		if got, readErr := os.ReadFile(filepath.Join(f.root, "metasystem/code.go")); readErr != nil || string(got) != "two" {
			t.Fatalf("rollback code=%q err=%v", got, readErr)
		}
	})
}
