package branch_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func pushRequest(f *branchFixture, opid string) branch.PushRequest {
	return branch.PushRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: opid, CheckClaim: claimAllowed,
	}
}

func remoteGoalTip(t *testing.T, f *branchFixture) string {
	t.Helper()
	fields := strings.Fields(git(t, f.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a"))
	if len(fields) != 2 {
		t.Fatalf("remote goal ref = %v", fields)
	}
	return fields[0]
}

func cloneBranchFixture(t *testing.T, f *branchFixture) *branchFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "clone")
	git(t, filepath.Dir(root), "clone", "-q", f.origin, root)
	git(t, root, "config", "user.name", "fixture")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	return &branchFixture{root: root, origin: f.origin, base: f.base}
}

func goalRef(t *testing.T, root, prefix string) string {
	t.Helper()
	return git(t, root, "for-each-ref", "--format=%(refname)", prefix)
}

func TestPushPublishesAmendAndSecondCloneAdopts(t *testing.T) {
	f := newBranchFixture(t)
	first := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	result, err := branch.Push(pushRequest(f, "push-first"))
	if err != nil || result.Tip != first || remoteGoalTip(t, f) != first {
		t.Fatalf("first push = %+v, %v", result, err)
	}
	stage(t, f, "metasystem/code.go", "two")
	replacement, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "commit-amend",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err = branch.Push(pushRequest(f, "push-amend"))
	if err != nil || result.Tip != replacement || remoteGoalTip(t, f) != replacement {
		t.Fatalf("amend push = %+v, %v", result, err)
	}
	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "-q", f.origin, other)
	request := branch.PushRequest{
		Repo: other, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "adopt", CheckClaim: claimAllowed,
	}
	result, err = branch.Push(request)
	if err != nil || result.State != "adopted" || result.Tip != replacement {
		t.Fatalf("adopt = %+v, %v", result, err)
	}
	if got := git(t, other, "rev-parse", "refs/heads/goal/goal-a"); got != replacement {
		t.Fatalf("adopted ref = %s, want %s", got, replacement)
	}
	if got := goalRef(t, other, "refs/metasystem/goals/fetch/"); got != "" {
		t.Fatalf("adoption left fetch ref %s", got)
	}
}

func TestAdoptRemoteTipRestoresHeadWhenTheRefUpdateFails(t *testing.T) {
	f := newBranchFixture(t)
	first := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "restore-first")); err != nil {
		t.Fatal(err)
	}
	other := cloneBranchFixture(t, f)
	if _, err := branch.Push(pushRequest(other, "restore-adopt")); err != nil {
		t.Fatal(err)
	}
	commitUnit(t, other, "u2", "metasystem/other.go", "two")
	if _, err := branch.Push(pushRequest(other, "restore-second")); err != nil {
		t.Fatal(err)
	}
	request := pushRequest(f, "restore-failed-update")
	request.Hooks.BeforeAdoptionRefMove = func() error {
		moved := git(t, f.root, "commit-tree", first+"^{tree}", "-p", first, "-m", "local move\n\nGoal-Plan: goal-a")
		git(t, f.root, "update-ref", "refs/heads/goal/goal-a", moved, first)
		return nil
	}
	if _, err := branch.Push(request); err == nil {
		t.Fatal("adoption with a failed ref transaction succeeded")
	}
	if got := git(t, f.root, "symbolic-ref", "-q", "HEAD"); got != "refs/heads/goal/goal-a" {
		t.Fatalf("HEAD stayed detached at %s", got)
	}
}

type unknownAfterLanding struct{ branch.GitPushTransport }

func (t unknownAfterLanding) Push(repo, remote, ref, expected, tip string) (branch.CASOutcome, error) {
	if _, err := t.GitPushTransport.Push(repo, remote, ref, expected, tip); err != nil {
		return branch.CASUnknown, err
	}
	return branch.CASUnknown, errors.New("connection ended before the result was read")
}

type unknownWithoutLanding struct{ branch.GitPushTransport }

func (unknownWithoutLanding) Push(string, string, string, string, string) (branch.CASOutcome, error) {
	return branch.CASUnknown, errors.New("connection ended before the remote accepted the push")
}

type fetchThenFail struct{ branch.GitPushTransport }

func (t fetchThenFail) Fetch(repo, remote, ref, destination string) error {
	if err := t.GitPushTransport.Fetch(repo, remote, ref, destination); err != nil {
		return err
	}
	return errors.New("fetch result was lost")
}

func TestPushReconcilesUnknownOutcomeAndPreparedTransaction(t *testing.T) {
	f := newBranchFixture(t)
	tip := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	request := pushRequest(f, "unknown")
	request.Transport = unknownAfterLanding{}
	result, err := branch.Push(request)
	if err != nil || result.Tip != tip || remoteGoalTip(t, f) != tip {
		t.Fatalf("unknown reconciliation = %+v, %v", result, err)
	}

	f = newBranchFixture(t)
	tip = commitUnit(t, f, "u1", "metasystem/code.go", "one")
	request = pushRequest(f, "crash-window")
	request.Hooks.AfterPush = func() error { return errors.New("simulated process death") }
	if _, err := branch.Push(request); err == nil || !strings.Contains(err.Error(), "simulated process death") {
		t.Fatalf("crash window = %v", err)
	}
	result, err = branch.Push(pushRequest(f, "recovery"))
	if err != nil || result.State != "reconciled" || result.Tip != tip {
		t.Fatalf("transaction recovery = %+v, %v", result, err)
	}

	f = newBranchFixture(t)
	tip = commitUnit(t, f, "u1", "metasystem/code.go", "one")
	request = pushRequest(f, "before-push-crash")
	request.Hooks.AfterRemoteRead = func() error { return errors.New("simulated crash before push") }
	if _, err := branch.Push(request); err == nil {
		t.Fatal("crash before push succeeded")
	}
	_, err = branch.Push(pushRequest(f, "report-not-landed"))
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.PushUnknownCode {
		t.Fatalf("unlanded recovery = %v", err)
	}
	if got := goalRef(t, f.root, "refs/metasystem/goals/txn/"); got != "" {
		t.Fatalf("unlanded recovery left transaction %s", got)
	}
	result, err = branch.Push(pushRequest(f, "retry-after-report"))
	if err != nil || result.Tip != tip || remoteGoalTip(t, f) != tip {
		t.Fatalf("retry after unlanded recovery = %+v, %v", result, err)
	}
}

func TestPushAndCommitAdoptDescendantRemoteAfterLandedCrash(t *testing.T) {
	for _, test := range []struct {
		name       string
		seedOrigin bool
	}{
		{name: "origin record absent"},
		{name: "origin record lags", seedOrigin: true},
	} {
		for _, nextAction := range []string{"push", "commit"} {
			t.Run(test.name+" then "+nextAction, func(t *testing.T) {
				f := newBranchFixture(t)
				if test.seedOrigin {
					commitUnit(t, f, "u0", "metasystem/seed.go", "seed")
					if _, err := branch.Push(pushRequest(f, "seed-origin")); err != nil {
						t.Fatal(err)
					}
				}
				landed := commitUnit(t, f, "u1", "metasystem/code.go", "one")
				request := pushRequest(f, "landed-crash")
				request.Hooks.AfterPush = func() error { return errors.New("process stopped after push") }
				if _, err := branch.Push(request); err == nil || remoteGoalTip(t, f) != landed {
					t.Fatalf("landed crash = %v, remote=%s", err, remoteGoalTip(t, f))
				}

				other := cloneBranchFixture(t, f)
				if result, err := branch.Push(pushRequest(other, "other-adopt")); err != nil || result.Tip != landed {
					t.Fatalf("other holder adoption = %+v, %v", result, err)
				}
				commitUnit(t, other, "u2", "metasystem/other.go", "two")
				if _, err := branch.Push(pushRequest(other, "other-push")); err != nil {
					t.Fatal(err)
				}
				remote := remoteGoalTip(t, f)

				result, err := branch.Push(pushRequest(f, "claim-returned"))
				if err != nil || result.State != "reconciled" || result.Tip != remote {
					t.Fatalf("reconcile descendant remote = %+v, %v; remote=%s", result, err, remote)
				}
				if got := git(t, f.root, "rev-parse", "refs/metasystem/goals/origin/goal-a"); got != remote {
					t.Fatalf("recorded origin tip = %s, want %s", got, remote)
				}
				if got := goalRef(t, f.root, "refs/metasystem/goals/txn/"); got != "" {
					t.Fatalf("reconciliation left transaction %s", got)
				}
				if nextAction == "push" {
					result, err := branch.Push(pushRequest(f, "adopt-after-reconcile"))
					if err != nil || result.State != "adopted" || result.Tip != remote {
						t.Fatalf("push after descendant reconciliation = %+v, %v", result, err)
					}
				}

				stage(t, f, "metasystem/next.go", "three")
				next, err := branch.CommitStaged(branch.CommitRequest{
					Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u3", OpID: "commit-after-reconcile",
					Kind: branch.Unit, CheckClaim: claimAllowed,
				})
				if err != nil || git(t, f.root, "rev-parse", next+"^") != remote {
					t.Fatalf("commit after descendant reconciliation = %s, %v", next, err)
				}
				if result, err := branch.Push(pushRequest(f, "push-after-reconcile")); err != nil || result.Tip != next || remoteGoalTip(t, f) != next {
					t.Fatalf("push after descendant reconciliation = %+v, %v", result, err)
				}
			})
		}
	}
}

func TestPushLeaseAndClaimMovementRefuseWithoutRetry(t *testing.T) {
	f := newBranchFixture(t)
	first := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "seed")); err != nil {
		t.Fatal(err)
	}
	stage(t, f, "metasystem/code.go", "two")
	local, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "commit-race",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := pushRequest(f, "lease-race")
	var competitor string
	request.Hooks.AfterRemoteRead = func() error {
		competitor = git(t, f.root, "commit-tree", first+"^{tree}", "-p", first, "-m", "competitor\n\nGoal-Plan: goal-a")
		git(t, f.root, "push", "-q", "--force", "origin", competitor+":refs/heads/goal/goal-a")
		return nil
	}
	_, err = branch.Push(request)
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.LeaseMovedCode || remoteGoalTip(t, f) != competitor || remoteGoalTip(t, f) == local {
		t.Fatalf("lease race error=%v remote=%s competitor=%s", err, remoteGoalTip(t, f), competitor)
	}
	if got := goalRef(t, f.root, "refs/metasystem/goals/txn/"); got != "" {
		t.Fatalf("lease refusal left transaction %s", got)
	}
	git(t, f.root, "read-tree", "--reset", "-u", competitor)
	git(t, f.root, "update-ref", "refs/heads/goal/goal-a", competitor, local)
	result, err := branch.Push(pushRequest(f, "after-lease-race"))
	if err != nil || result.State != "current" || result.Tip != competitor {
		t.Fatalf("push after lease reset = %+v, %v", result, err)
	}

	f = newBranchFixture(t)
	tip := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	held := true
	request = pushRequest(f, "claim-race")
	request.CheckClaim = func() error {
		if !held {
			return errors.New("claim moved to machine-b+lineage-b")
		}
		return nil
	}
	request.Hooks.AfterPush = func() error { held = false; return nil }
	_, err = branch.Push(request)
	if !errors.As(err, &refusal) || refusal.Code != branch.ClaimLostCode || remoteGoalTip(t, f) != tip {
		t.Fatalf("claim race error=%v remote=%s", err, remoteGoalTip(t, f))
	}
}

func TestPushUnknownNotLandedDoesNotWedgeLaterAmend(t *testing.T) {
	f := newBranchFixture(t)
	commitUnit(t, f, "u1", "metasystem/code.go", "one")
	request := pushRequest(f, "unknown-not-landed")
	request.Transport = unknownWithoutLanding{}
	_, err := branch.Push(request)
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.PushUnknownCode {
		t.Fatalf("unknown outcome = %v", err)
	}
	if got := goalRef(t, f.root, "refs/metasystem/goals/txn/"); got != "" {
		t.Fatalf("unknown outcome left transaction %s", got)
	}
	stage(t, f, "metasystem/code.go", "two")
	replacement, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "amend-after-unknown",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := branch.Push(pushRequest(f, "push-after-unknown"))
	if err != nil || result.Tip != replacement || remoteGoalTip(t, f) != replacement {
		t.Fatalf("push after unknown and amend = %+v, %v", result, err)
	}
}

func TestPushAdoptsRemoteAdvanceAndCommitUsesIt(t *testing.T) {
	f := newBranchFixture(t)
	first := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "push-first")); err != nil {
		t.Fatal(err)
	}
	otherRoot := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(otherRoot), "clone", "-q", f.origin, otherRoot)
	git(t, otherRoot, "config", "user.name", "fixture")
	git(t, otherRoot, "config", "user.email", "fixture@example.invalid")
	other := &branchFixture{root: otherRoot, origin: f.origin, base: f.base}
	if result, err := branch.Push(pushRequest(other, "adopt-first")); err != nil || result.Tip != first {
		t.Fatalf("second clone adoption = %+v, %v", result, err)
	}
	stage(t, other, "metasystem/code.go", "two")
	second, err := branch.CommitStaged(branch.CommitRequest{
		Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u1", OpID: "second-amend",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(other, "push-second")); err != nil {
		t.Fatal(err)
	}
	result, err := branch.Push(pushRequest(f, "adopt-second"))
	if err != nil || result.State != "adopted" || result.Tip != second || remoteGoalTip(t, f) != second {
		t.Fatalf("first clone adoption = %+v, %v remote=%s", result, err, remoteGoalTip(t, f))
	}
	stage(t, other, "metasystem/code.go", "three")
	third, err := branch.CommitStaged(branch.CommitRequest{
		Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u1", OpID: "third-amend",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(other, "push-third")); err != nil {
		t.Fatal(err)
	}
	stage(t, f, "metasystem/next.go", "next")
	next, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u2", OpID: "commit-after-advance",
		Kind: branch.Unit, CheckClaim: claimAllowed,
	})
	if err != nil || git(t, f.root, "rev-parse", next+"^") != third || remoteGoalTip(t, f) != third {
		t.Fatalf("commit after remote advance = %s, %v; remote=%s", next, err, remoteGoalTip(t, f))
	}
}

func TestPushAndCommitRefuseDivergedRemote(t *testing.T) {
	f := newBranchFixture(t)
	commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "seed")); err != nil {
		t.Fatal(err)
	}
	otherRoot := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(otherRoot), "clone", "-q", f.origin, otherRoot)
	git(t, otherRoot, "config", "user.name", "fixture")
	git(t, otherRoot, "config", "user.email", "fixture@example.invalid")
	other := &branchFixture{root: otherRoot, origin: f.origin, base: f.base}
	if _, err := branch.Push(pushRequest(other, "adopt")); err != nil {
		t.Fatal(err)
	}
	stage(t, f, "metasystem/code.go", "local")
	local, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "local-amend",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	stage(t, other, "metasystem/code.go", "remote")
	remote, err := branch.CommitStaged(branch.CommitRequest{
		Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u1", OpID: "remote-amend",
		Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(other, "remote-push")); err != nil {
		t.Fatal(err)
	}
	_, err = branch.Push(pushRequest(f, "stale-push"))
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.StaleCode || !strings.Contains(err.Error(), local) || !strings.Contains(err.Error(), remote) || remoteGoalTip(t, f) != remote {
		t.Fatalf("stale push = %v; remote=%s", err, remoteGoalTip(t, f))
	}
	stage(t, f, "metasystem/next.go", "next")
	_, err = branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u2", OpID: "stale-commit",
		Kind: branch.Unit, CheckClaim: claimAllowed,
	})
	if !errors.As(err, &refusal) || refusal.Code != branch.StaleCode || git(t, f.root, "rev-parse", "refs/heads/goal/goal-a") != local || remoteGoalTip(t, f) != remote {
		t.Fatalf("stale commit = %v; local=%s remote=%s", err, local, remoteGoalTip(t, f))
	}
}

func TestPushAdoptionValidatesRangeAndCleansFetchRef(t *testing.T) {
	f := newBranchFixture(t)
	invalid := git(t, f.root, "commit-tree", f.base+"^{tree}", "-p", f.base, "-m", "invalid")
	git(t, f.root, "push", "-q", "origin", invalid+":refs/heads/goal/goal-a")
	otherRoot := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(otherRoot), "clone", "-q", f.origin, otherRoot)
	other := &branchFixture{root: otherRoot, origin: f.origin, base: f.base}
	_, err := branch.Push(pushRequest(other, "invalid-adoption"))
	var refusal *branch.RangeError
	if !errors.As(err, &refusal) {
		t.Fatalf("invalid adoption = %v", err)
	}
	if _, present, _ := localFixtureRef(other.root, "refs/heads/goal/goal-a"); present {
		t.Fatal("invalid remote tip was adopted")
	}
	if got := goalRef(t, other.root, "refs/metasystem/goals/fetch/"); got != "" {
		t.Fatalf("invalid adoption left fetch ref %s", got)
	}
	request := pushRequest(other, "failed-fetch")
	request.Transport = fetchThenFail{}
	if _, err := branch.Push(request); err == nil || !strings.Contains(err.Error(), "fetch result was lost") {
		t.Fatalf("failed fetch = %v", err)
	}
	if got := goalRef(t, other.root, "refs/metasystem/goals/fetch/"); got != "" {
		t.Fatalf("failed fetch left fetch ref %s", got)
	}
}

func TestPushAdoptionKeepsUntrackedScratchFile(t *testing.T) {
	f := newBranchFixture(t)
	tip := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "publish-scratch")); err != nil {
		t.Fatal(err)
	}
	other := cloneBranchFixture(t, f)
	write(t, other.root, "notes/scratch.txt", "keep me")
	result, err := branch.Push(pushRequest(other, "adopt-with-scratch"))
	if err != nil || result.State != "adopted" || result.Tip != tip {
		t.Fatalf("adoption=%+v err=%v", result, err)
	}
	if got := git(t, other.root, "status", "--porcelain=v1", "--untracked-files=all"); got != "?? notes/scratch.txt" {
		t.Fatalf("scratch status=%q", got)
	}
}

func TestPushAdoptionCrashCannotStageAReversal(t *testing.T) {
	f := newBranchFixture(t)
	first := commitUnit(t, f, "u1", "metasystem/one.go", "one")
	if _, err := branch.Push(pushRequest(f, "publish-first")); err != nil {
		t.Fatal(err)
	}
	other := cloneBranchFixture(t, f)
	if _, err := branch.Push(pushRequest(other, "adopt-first")); err != nil {
		t.Fatal(err)
	}
	second := commitUnit(t, other, "u2", "metasystem/two.go", "two")
	if _, err := branch.Push(pushRequest(other, "publish-second")); err != nil {
		t.Fatal(err)
	}
	request := pushRequest(f, "crash-after-adoption-ref")
	request.Hooks.AfterAdoptionRefMove = func() error { return errors.New("simulated adoption crash") }
	if _, err := branch.Push(request); err == nil || !strings.Contains(err.Error(), "simulated adoption crash") {
		t.Fatalf("crash result=%v", err)
	}
	if got := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a"); got != second {
		t.Fatalf("goal ref=%s want=%s (first=%s)", got, second, first)
	}
	if got := git(t, f.root, "rev-parse", "HEAD"); got != second {
		t.Fatalf("detached checkout=%s want=%s", got, second)
	}
	if status := git(t, f.root, "status", "--porcelain=v1"); status != "" {
		t.Fatalf("adoption crash staged a reversal: %q", status)
	}
}
