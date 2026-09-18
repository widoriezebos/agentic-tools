package branch_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

type recordingFetchTransport struct {
	branch.GitPushTransport
	destination *string
}

func (r recordingFetchTransport) Fetch(repo, remote, ref, destination string) error {
	*r.destination = destination
	return r.GitPushTransport.Fetch(repo, remote, ref, destination)
}

func mirrorRequest(f *branchFixture, transport string) branch.MirrorRequest {
	return branch.MirrorRequest{Repo: f.root, Origin: "origin", Transport: transport, EndpointTip: f.base,
		GoalID: "goal-a", OpID: "mirror-test", CheckClaim: claimAllowed}
}

func TestTransportMirrorAndLeasedDelete(t *testing.T) {
	f := newBranchFixture(t)
	transport := filepath.Join(t.TempDir(), "transport.git")
	git(t, filepath.Dir(transport), "init", "-q", "--bare", transport)
	git(t, f.root, "remote", "add", "transport", transport)
	commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "mirror-origin-one")); err != nil {
		t.Fatal(err)
	}
	first, err := branch.Mirror(mirrorRequest(f, "transport"))
	if err != nil || git(t, transport, "rev-parse", "refs/heads/goal/goal-a") != first.Tip {
		t.Fatalf("first mirror = %+v err=%v", first, err)
	}
	stage(t, f, "metasystem/code.go", "amended")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "u1", OpID: "mirror-amend", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(f, "mirror-origin-two")); err != nil {
		t.Fatal(err)
	}
	second, err := branch.Mirror(mirrorRequest(f, "transport"))
	if err != nil || second.Tip == first.Tip || git(t, transport, "rev-parse", "refs/heads/goal/goal-a") != second.Tip {
		t.Fatalf("amended mirror = %+v err=%v", second, err)
	}

	intruder := git(t, f.root, "commit-tree", f.base+"^{tree}", "-p", f.base, "-m", "intruder")
	moved := mirrorRequest(f, "transport")
	moved.Hooks.AfterTransportRead = func() error {
		git(t, f.root, "push", "-q", "--force", "transport", intruder+":refs/heads/goal/goal-a")
		return nil
	}
	_, err = branch.Mirror(moved)
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.LeaseMovedCode ||
		git(t, transport, "rev-parse", "refs/heads/goal/goal-a") != intruder {
		t.Fatalf("moved transport mirror = %v", err)
	}

	deleteExpected := intruder
	other := git(t, f.root, "commit-tree", f.base+"^{tree}", "-p", f.base, "-m", "other")
	err = branch.Delete(branch.DeleteRequest{Repo: f.root, Remote: "transport", GoalID: "goal-a", Expected: deleteExpected,
		CheckClaim: claimAllowed, AfterRemoteRead: func() error {
			git(t, f.root, "push", "-q", "--force", "transport", other+":refs/heads/goal/goal-a")
			return nil
		}})
	if !errors.As(err, &refusal) || refusal.Code != branch.LeaseMovedCode ||
		git(t, transport, "rev-parse", "refs/heads/goal/goal-a") != other {
		t.Fatalf("moved leased delete = %v", err)
	}
}

func TestMirrorDeleteReconcilesUnknownOutcome(t *testing.T) {
	t.Run("mirror operation id", func(t *testing.T) {
		f := newBranchFixture(t)
		transport := filepath.Join(t.TempDir(), "transport.git")
		git(t, filepath.Dir(transport), "init", "-q", "--bare", transport)
		git(t, f.root, "remote", "add", "transport", transport)
		commitUnit(t, f, "u1", "metasystem/code.go", "one")
		if _, err := branch.Push(pushRequest(f, "mirror-op-origin")); err != nil {
			t.Fatal(err)
		}
		destination := ""
		req := mirrorRequest(f, "transport")
		req.OpID = "mirror-op"
		req.PushTransport = recordingFetchTransport{destination: &destination}
		if _, err := branch.Mirror(req); err != nil || !strings.HasSuffix(destination, "/mirror-op") {
			t.Fatalf("mirror destination = %q, err=%v", destination, err)
		}
	})

	t.Run("delete completed", func(t *testing.T) {
		f := newBranchFixture(t)
		tip := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		git(t, f.root, "push", "-q", "origin", tip+":refs/heads/goal/goal-a")
		err := branch.Delete(branch.DeleteRequest{Repo: f.root, Remote: "origin", GoalID: "goal-a", Expected: tip,
			CheckClaim: claimAllowed, PushTransport: unknownAfterLanding{}})
		if err != nil || git(t, f.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a") != "" {
			t.Fatalf("completed unknown delete = %v", err)
		}
	})

	t.Run("delete not completed", func(t *testing.T) {
		f := newBranchFixture(t)
		tip := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		git(t, f.root, "push", "-q", "origin", tip+":refs/heads/goal/goal-a")
		err := branch.Delete(branch.DeleteRequest{Repo: f.root, Remote: "origin", GoalID: "goal-a", Expected: tip,
			CheckClaim: claimAllowed, PushTransport: unknownWithoutLanding{}})
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.PushUnknownCode {
			t.Fatalf("incomplete unknown delete = %v", err)
		}
	})

	t.Run("sweep delete completed", func(t *testing.T) {
		f := newBranchFixture(t)
		commitUnit(t, f, "u1", "metasystem/code.go", "one")
		if _, err := branch.Push(pushRequest(f, "unknown-sweep-origin")); err != nil {
			t.Fatal(err)
		}
		result, err := branch.Sweep(branch.SweepRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", Abandoned: true, CheckClaim: claimAllowed, PushTransport: unknownAfterLanding{}})
		if err != nil || !result.Deleted || git(t, f.root, "ls-remote", "--refs", "origin", "refs/heads/goal/goal-a") != "" {
			t.Fatalf("completed unknown sweep delete = %+v, %v", result, err)
		}
	})
}
