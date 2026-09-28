package branch_test

import (
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
