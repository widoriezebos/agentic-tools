package branch_test

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

type observingMirrorTransport struct {
	branch.GitPushTransport
	t                                *testing.T
	repo, origin, transport, ref     string
	destination, endpointTip, goalID string
	tip                              string
	fetched, pushed                  bool
}

func (m *observingMirrorTransport) Fetch(repo, remote, ref, destination string) error {
	m.t.Helper()
	if m.fetched || repo != m.repo || remote != m.origin || ref != m.ref || destination != m.destination {
		m.t.Fatalf("mirror fetch = (%q, %q, %q, %q)", repo, remote, ref, destination)
	}
	if err := m.GitPushTransport.Fetch(repo, remote, ref, destination); err != nil {
		return err
	}
	if got := git(m.t, repo, "rev-parse", destination); got != m.tip {
		m.t.Fatalf("fetched ref = %q, want %q", got, m.tip)
	}
	if _, err := branch.ValidateRange(repo, m.endpointTip, m.tip, m.goalID); err != nil {
		m.t.Fatalf("fetched range is not visible: %v", err)
	}
	m.fetched = true
	return nil
}

func (m *observingMirrorTransport) Push(repo, remote, ref, expected, tip string) (branch.CASOutcome, error) {
	m.t.Helper()
	if !m.fetched || m.pushed || repo != m.repo || remote != m.transport || ref != m.ref || expected != "" || tip != m.tip {
		m.t.Fatalf("mirror push = (%q, %q, %q, %q, %q)", repo, remote, ref, expected, tip)
	}
	if got := goalRef(m.t, repo, "refs/metasystem/goals/fetch/"); got != "" {
		m.t.Fatalf("temporary fetch ref remains before publication: %q", got)
	}
	m.pushed = true
	return m.GitPushTransport.Push(repo, remote, ref, expected, tip)
}

func TestMirrorGitAdapterTransfersAndCleansRef(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	mirrorClone := cloneBranchFixture(t, f)
	transport := filepath.Join(t.TempDir(), "transport.git")
	git(t, filepath.Dir(transport), "init", "-q", "--bare", transport)
	git(t, mirrorClone.root, "remote", "add", "transport", transport)
	tip := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "mirror-origin-one")); err != nil {
		t.Fatal(err)
	}
	if got := remoteGoalTip(t, f); got != tip {
		t.Fatalf("origin tip = %q, want %q", got, tip)
	}
	if _, err := execGit(mirrorClone.root, "cat-file", "-e", tip+"^{commit}"); err == nil {
		t.Fatalf("mirror clone already has origin tip %q", tip)
	}
	const ref = "refs/heads/goal/goal-a"
	const opID = "mirror-adapter"
	adapter := &observingMirrorTransport{t: t, repo: mirrorClone.root, origin: "origin", transport: "transport", ref: ref,
		destination: "refs/metasystem/goals/fetch/" + opID, endpointTip: f.base, goalID: "goal-a", tip: tip}
	result, err := branch.Mirror(branch.MirrorRequest{Repo: mirrorClone.root, Origin: "origin", Transport: "transport",
		EndpointTip: f.base, GoalID: "goal-a", OpID: opID, CheckClaim: claimAllowed, PushTransport: adapter})
	if err != nil || result != (branch.PushResult{State: "mirrored", Tip: tip}) || !adapter.fetched || !adapter.pushed {
		t.Fatalf("adapter mirror = %+v, err=%v, fetched=%t, pushed=%t", result, err, adapter.fetched, adapter.pushed)
	}
	if got := git(t, transport, "rev-parse", ref); got != tip {
		t.Fatalf("transport tip = %q, want %q", got, tip)
	}
	if got := goalRef(t, mirrorClone.root, "refs/metasystem/goals/fetch/"); got != "" {
		t.Fatalf("temporary fetch ref remains: %q", got)
	}
}

func TestMirrorDeleteReconcilesUnknownOutcome(t *testing.T) {
	t.Parallel()
	t.Run("sweep delete completed", func(t *testing.T) {
		t.Parallel()
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
