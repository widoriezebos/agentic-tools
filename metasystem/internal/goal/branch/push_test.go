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

type unknownAfterLanding struct{ branch.GitPushTransport }

func (t unknownAfterLanding) Push(repo, remote, ref, expected, tip string) (branch.CASOutcome, error) {
	if _, err := t.GitPushTransport.Push(repo, remote, ref, expected, tip); err != nil {
		return branch.CASUnknown, err
	}
	return branch.CASUnknown, errors.New("connection ended before the result was read")
}
