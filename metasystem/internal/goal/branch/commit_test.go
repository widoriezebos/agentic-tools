package branch_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func claimAllowed() error { return nil }

func stage(t *testing.T, f *branchFixture, path, body string) {
	t.Helper()
	write(t, f.root, path, body)
	git(t, f.root, "add", "--", path)
}

func commitUnit(t *testing.T, f *branchFixture, unit, path, body string) string {
	t.Helper()
	stage(t, f, path, body)
	commit, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: unit, OpID: "commit-" + unit,
		Kind: branch.Unit, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

type checkoutSnapshot struct {
	head, index, status, refs string
}

func snapshotCheckout(t *testing.T, root string) checkoutSnapshot {
	t.Helper()
	head, err := execGit(root, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		head, _ = execGit(root, "rev-parse", "HEAD")
	}
	return checkoutSnapshot{
		head: strings.TrimSpace(head), index: git(t, root, "write-tree"),
		status: git(t, root, "status", "--porcelain=v1", "--untracked-files=all"),
		refs:   git(t, root, "for-each-ref", "--format=%(refname) %(objectname)"),
	}
}

func requireCheckoutUnchanged(t *testing.T, root string, before checkoutSnapshot) {
	t.Helper()
	if after := snapshotCheckout(t, root); after != before {
		t.Fatalf("refusal changed checkout:\nbefore=%+v\nafter=%+v", before, after)
	}
}

func TestCommitCreatesBranchAndAmendsUnit(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	stage(t, f, "metasystem/plans/goal-a.md", "plan")
	plan, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", OpID: "commit-plan", Kind: branch.Plan, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if parent := git(t, f.root, "rev-parse", plan+"^"); parent != f.base {
		t.Fatalf("first parent = %s, want endpoint %s", parent, f.base)
	}
	if message := git(t, f.root, "show", "-s", "--format=%B", plan); !strings.Contains(message, "Goal-Plan: goal-a") {
		t.Fatalf("plan message = %q", message)
	}
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	stage(t, f, "metasystem/code.go", "two")
	replacement, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-u1",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replacement == unit || git(t, f.root, "rev-parse", replacement+"^") != plan {
		t.Fatalf("amend did not replace one unit: old=%s new=%s", unit, replacement)
	}
	commits, err := branch.ValidateRange(f.root, f.base, replacement, "goal-a")
	if err != nil || len(commits) != 2 || commits[1].Unit != "u1" {
		t.Fatalf("commits=%+v err=%v", commits, err)
	}
	stage(t, f, "metasystem/plans/after.md", "later plan")
	tail, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", OpID: "commit-tail", Kind: branch.Plan, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	stage(t, f, "metasystem/code.go", "three")
	rewritten, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-tail",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	commits, err = branch.ValidateRange(f.root, f.base, rewritten, "goal-a")
	if err != nil || len(commits) != 3 || commits[1].Unit != "u1" || rewritten == tail {
		t.Fatalf("rewritten commits=%+v oldTail=%s newTip=%s err=%v", commits, tail, rewritten, err)
	}
	if status := git(t, f.root, "status", "--porcelain"); status != "" {
		t.Fatalf("rewritten branch left workspace changes: %q", status)
	}
}

func prepareTrackedAmendFixture(t *testing.T, f *branchFixture) {
	t.Helper()
	for path, body := range map[string]string{
		"metasystem/code.go":   "zero",
		"metasystem/keep-a.go": "original a",
		"metasystem/keep-b.go": "original b",
	} {
		write(t, f.root, path, body)
	}
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "tracked amend fixture")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
}

func TestAmendKeepsUnstagedTrackedEdits(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		away bool
	}{{name: "on goal branch"}, {name: "away from goal branch", away: true}} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newBranchFixture(t)
			prepareTrackedAmendFixture(t, f)
			commitUnit(t, f, "u1", "metasystem/code.go", "one")
			if test.away {
				git(t, f.root, "switch", "--quiet", "-c", "other")
			}
			wantA, wantB := "local a\nwith another line\n", "local b\n"
			write(t, f.root, "metasystem/keep-a.go", wantA)
			write(t, f.root, "metasystem/keep-b.go", wantB)
			stage(t, f, "metasystem/code.go", "two")

			_, err := branch.CommitStaged(branch.CommitRequest{
				Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-with-unstaged",
				Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
			})
			if err != nil {
				t.Fatal(err)
			}
			for path, want := range map[string]string{"metasystem/keep-a.go": wantA, "metasystem/keep-b.go": wantB} {
				got, readErr := os.ReadFile(filepath.Join(f.root, path))
				if readErr != nil || string(got) != want {
					t.Fatalf("unstaged %s = %q, %v; want %q", path, got, readErr, want)
				}
			}
			if head := git(t, f.root, "symbolic-ref", "HEAD"); head != "refs/heads/goal/goal-a" {
				t.Fatalf("HEAD = %s, want goal branch", head)
			}
		})
	}
}

func TestAmendRefusesBeforeOverwritingUnstagedTrackedEdit(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	prepareTrackedAmendFixture(t, f)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	readPath := "metasystem/records/reads/goal-a/" + unit + ".json"
	f.commit(t, readPath, "{}", "read\n\nGoal-Read: goal-a/u1 "+unit)
	write(t, f.root, readPath, "locally edited read")
	stage(t, f, "metasystem/code.go", "two")
	before := snapshotCheckout(t, f.root)

	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-overwrite-refusal",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.StaleCode {
		t.Fatalf("overwriting amend error = %v", err)
	}
	requireCheckoutUnchanged(t, f.root, before)
	if got, readErr := os.ReadFile(filepath.Join(f.root, readPath)); readErr != nil || string(got) != "locally edited read" {
		t.Fatalf("refusal changed unstaged read = %q, %v", got, readErr)
	}
}

func TestAmendInstallFailureRestoresCheckout(t *testing.T) {
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

	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-install-failure",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
		t.Fatalf("locked ref amend error = %v", err)
	}
	requireCheckoutUnchanged(t, f.root, before)
	if got, readErr := os.ReadFile(filepath.Join(f.root, "metasystem/keep-a.go")); readErr != nil || string(got) != "local a" {
		t.Fatalf("failed install changed unstaged file = %q, %v", got, readErr)
	}
}

func TestAmendInstallFailureRollsBackMovedCheckout(t *testing.T) {
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
	readBefore, readErr := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(readPath)))
	if readErr != nil {
		t.Fatal(readErr)
	}

	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-moved-install-failure",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
		t.Fatalf("locked moved amend error = %v", err)
	}
	requireCheckoutUnchanged(t, f.root, before)
	if got, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(readPath))); err != nil || string(got) != string(readBefore) {
		t.Fatalf("failed amend changed read record = %q, %v", got, err)
	}
}

func TestCommitRefusesNonHolder(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	stage(t, f, "metasystem/code.go", "one")
	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "non-holder", Kind: branch.Unit,
		CheckClaim: func() error { return errors.New("claim belongs to machine-b+lineage-b") },
	})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.NotHolderCode {
		t.Fatalf("non-holder error = %v", err)
	}
	if _, present, _ := localFixtureRef(f.root, "refs/heads/goal/goal-a"); present {
		t.Fatal("refused commit created the branch")
	}
}

func TestCommitRefusalsPreserveCheckout(t *testing.T) {
	t.Parallel()
	t.Run("class from another branch", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		commitUnit(t, f, "u1", "metasystem/code.go", "one")
		git(t, f.root, "switch", "--quiet", "-c", "other", f.base)
		stage(t, f, "metasystem/plans/wrong.md", "wrong")
		before := snapshotCheckout(t, f.root)
		_, err := branch.CommitStaged(branch.CommitRequest{
			Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u2", OpID: "class-refusal",
			Kind: branch.Unit, CheckClaim: claimAllowed,
		})
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.RangeCode {
			t.Fatalf("class refusal = %v", err)
		}
		requireCheckoutUnchanged(t, f.root, before)
	})

	t.Run("invalid range", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		invalid := git(t, f.root, "commit-tree", f.base+"^{tree}", "-p", f.base, "-m", "invalid")
		git(t, f.root, "update-ref", "refs/heads/goal/goal-a", invalid)
		stage(t, f, "metasystem/code.go", "one")
		before := snapshotCheckout(t, f.root)
		_, err := branch.CommitStaged(branch.CommitRequest{
			Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "range-refusal",
			Kind: branch.Unit, CheckClaim: claimAllowed,
		})
		var refusal *branch.RangeError
		if !errors.As(err, &refusal) {
			t.Fatalf("range refusal = %v", err)
		}
		requireCheckoutUnchanged(t, f.root, before)
	})
}

func TestAmendRechecksClaimBeforeBranchUpdate(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	old := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	stage(t, f, "metasystem/code.go", "two")
	checks := 0
	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "claim-recheck",
		Kind: branch.Unit, Amend: true, CheckClaim: func() error {
			checks++
			if checks > 1 {
				return errors.New("claim moved")
			}
			return nil
		},
	})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.NotHolderCode || checks != 2 {
		t.Fatalf("amend claim recheck = %v, checks=%d", err, checks)
	}
	if got := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a"); got != old {
		t.Fatalf("amend moved branch from %s to %s", old, got)
	}
}

func TestAmendDropsReadOfReplacedBuildAndReplaysLaterPlan(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	f.commit(t, "metasystem/records/reads/goal-a/"+unit+".json", "{}", "read\n\nGoal-Read: goal-a/u1 "+unit)
	f.commit(t, "metasystem/plans/later.md", "later", "later plan\n\nGoal-Plan: goal-a")
	stage(t, f, "metasystem/code.go", "two")
	tip, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-stale-read",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	commits, err := branch.ValidateRange(f.root, f.base, tip, "goal-a")
	if err != nil || len(commits) != 2 || commits[0].Kind != branch.Unit || commits[1].Kind != branch.Plan {
		t.Fatalf("rewritten suffix=%+v err=%v", commits, err)
	}
	if got := git(t, f.root, "show", tip+":metasystem/code.go"); got != "two" {
		t.Fatalf("amended bytes=%q", got)
	}
	if got := git(t, f.root, "show", tip+":metasystem/plans/later.md"); got != "later" {
		t.Fatalf("replayed plan=%q", got)
	}
}

func TestAmendDroppingReadStillChecksReplayedTree(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	f.commit(t, "metasystem/records/reads/goal-a/"+unit+".json", "{}", "read\n\nGoal-Read: goal-a/u1 "+unit)
	hooks := filepath.Join(f.root, ".fixture-hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := `#!/bin/sh
if test -n "$GOAL_TREE_HOOK" || test -e "$GIT_DIR/goal-tree-hook-ran"; then exit 0; fi
touch "$GIT_DIR/goal-tree-hook-ran"
printf 'injected\n' > metasystem/injected.go
git add metasystem/injected.go
GOAL_TREE_HOOK=1 git commit --amend --no-edit --quiet
`
	if err := testexec.WriteFile(filepath.Join(hooks, "post-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, f.root, "config", "core.hooksPath", hooks)
	stage(t, f, "metasystem/code.go", "two")
	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-tree-check",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.RangeCode || !strings.Contains(err.Error(), "replayed branch") {
		t.Fatalf("replayed-tree check = %v", err)
	}
}

func TestPlainCommitKeepsDirtyLedgerWithoutAdoption(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/memory/receipts.log", "seed\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "tracked ledger")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	write(t, f.root, "metasystem/memory/receipts.log", "dirty ledger\n")
	stage(t, f, "metasystem/code.go", "one")
	if _, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "plain-dirty-ledger",
		Kind: branch.Unit, CheckClaim: claimAllowed,
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(f.root, "metasystem/memory/receipts.log")); err != nil || string(got) != "dirty ledger\n" {
		t.Fatalf("dirty ledger = %q, %v", got, err)
	}
}

func TestAdoptionUntrackedCollisionRefusesStale(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	commitUnit(t, f, "u1", "metasystem/one.go", "one")
	if _, err := branch.Push(pushRequest(f, "collision-first")); err != nil {
		t.Fatal(err)
	}
	other := cloneBranchFixture(t, f)
	if _, err := branch.Push(pushRequest(other, "collision-adopt")); err != nil {
		t.Fatal(err)
	}
	commitUnit(t, other, "u2", "metasystem/collision.go", "remote")
	if _, err := branch.Push(pushRequest(other, "collision-remote")); err != nil {
		t.Fatal(err)
	}
	write(t, f.root, "metasystem/collision.go", "local untracked")
	stage(t, f, "metasystem/next.go", "next")
	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u3", OpID: "collision-local",
		Kind: branch.Unit, CheckClaim: claimAllowed,
	})
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.StaleCode || !strings.Contains(err.Error(), "Untracked working tree") {
		t.Fatalf("untracked collision = %v", err)
	}
}

func TestCommitAdoptsRemoteWithoutEndpointCheckout(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	remote := commitUnit(t, f, "u1", "metasystem/remote.go", "remote")
	if _, err := branch.Push(pushRequest(f, "seed-remote")); err != nil {
		t.Fatal(err)
	}
	other := cloneBranchFixture(t, f)
	other.commit(t, "local-only.txt", "local", "unrelated local commit")
	stage(t, other, "metasystem/next.go", "next")
	tip, err := branch.CommitStaged(branch.CommitRequest{
		Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u2", OpID: "adopt-away-from-endpoint",
		Kind: branch.Unit, CheckClaim: claimAllowed,
	})
	if err != nil || git(t, other.root, "rev-parse", tip+"^") != remote {
		t.Fatalf("adopted commit=%s err=%v", tip, err)
	}
}

func TestFailedCommitLeavesCheckoutAndRefsUnchanged(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	hooks := filepath.Join(f.root, ".fixture-hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\necho fixture hook refusal >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, f.root, "config", "core.hooksPath", hooks)
	stage(t, f, "metasystem/code.go", "one")
	before := snapshotCheckout(t, f.root)
	_, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "hook-refusal",
		Kind: branch.Unit, CheckClaim: claimAllowed,
	})
	if err == nil || !strings.Contains(err.Error(), "fixture hook refusal") {
		t.Fatalf("hook refusal=%v", err)
	}
	requireCheckoutUnchanged(t, f.root, before)
	if _, present, _ := localFixtureRef(f.root, "refs/heads/goal/goal-a"); present {
		t.Fatal("failed commit created the goal branch")
	}
}

func TestAmendReplacesWholeBuildList(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	stage(t, f, "metasystem/multi.go", "one")
	request := branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a",
		Units: []string{"5", "6", "7a", "7b"}, OpID: "multi-build", Kind: branch.Unit, CheckClaim: claimAllowed}
	old, err := branch.CommitStaged(request)
	if err != nil {
		t.Fatal(err)
	}
	stage(t, f, "metasystem/multi.go", "two")
	request.OpID, request.Amend = "multi-amend", true
	tip, err := branch.CommitStaged(request)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := branch.ValidateRange(f.root, f.base, tip, "goal-a")
	if err != nil || len(commits) != 1 || commits[0].ID == old || strings.Join(commits[0].Units, "+") != "5+6+7a+7b" {
		t.Fatalf("multi-unit replacement=%+v old=%s err=%v", commits, old, err)
	}
}

func localFixtureRef(repo, ref string) (string, bool, error) {
	out, err := execGit(repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", false, nil
	}
	return strings.TrimSpace(out), true, nil
}

func execGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
