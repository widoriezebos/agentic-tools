package branch_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func preparedLanding(t *testing.T, fixture landFixture) (string, branch.LandResult) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "prepared")
	result, err := branch.PrepareLanding(landRequest(t, fixture, out))
	if err != nil {
		t.Fatal(err)
	}
	return out, result
}

func requirePublishCode(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("publish refusal=%v, want %s", err, code)
	}
}

func TestLandingPublicationGitAdapter(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		fixture := newLandFixture(t)
		out, prepared := preparedLanding(t, fixture)
		result, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
			GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed})
		if err != nil || result.Landing != prepared.Landing {
			t.Fatalf("land push=%+v err=%v", result, err)
		}
		if got := git(t, fixture.origin, "rev-parse", "refs/heads/main"); got != prepared.Landing {
			t.Fatalf("endpoint=%s want %s", got, prepared.Landing)
		}
		if refs := git(t, fixture.root, "ls-remote", "--refs", "origin", "refs/heads/landing/goal-a"); refs != "" {
			t.Fatalf("landing branch survived: %s", refs)
		}
		verified, err := branch.VerifyLandedSeries(fixture.root, prepared.Landing)
		if err != nil || len(verified) != 3 {
			t.Fatalf("verified=%+v err=%v", verified, err)
		}
		for _, item := range verified {
			if item.Actual != item.Expected {
				t.Fatalf("verification mismatch: %+v", item)
			}
			if identity := git(t, fixture.root, "show", "-s", "--format=%an <%ae>|%cn <%ce>", item.Commit); identity != "Wido Approver <wido@example.invalid>|Wido Approver <wido@example.invalid>" {
				t.Fatalf("landed identity=%s", identity)
			}
		}

		first := verified[0].Commit
		worktree := filepath.Join(t.TempDir(), "mutation")
		git(t, fixture.root, "worktree", "add", "--quiet", "--detach", worktree, first+"^")
		write(t, worktree, "metasystem/one.go", "mutated\n")
		git(t, worktree, "add", "metasystem/one.go")
		tree := git(t, worktree, "write-tree")
		message := git(t, fixture.root, "show", "-s", "--format=%B", first)
		messageFile := filepath.Join(t.TempDir(), "message")
		if err := os.WriteFile(messageFile, []byte(message), 0o644); err != nil {
			t.Fatal(err)
		}
		mutated := git(t, fixture.root, "commit-tree", tree, "-p", first+"^", "-F", messageFile)
		_, err = branch.VerifyLanded(fixture.root, mutated)
		requirePublishCode(t, err, branch.LandVerifyCode)
	})

	for _, test := range []struct {
		name, code, movedRef string
	}{
		{name: "endpoint_moved", code: branch.LandTrunkMovedCode, movedRef: "refs/heads/main"},
		{name: "landing_moved", code: branch.LandBranchMovedCode, movedRef: "refs/heads/landing/goal-a"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newLandFixture(t)
			out, prepared := preparedLanding(t, fixture)
			intruder := git(t, fixture.root, "commit-tree", fixture.base+"^{tree}", "-p", fixture.base, "-m", "intruder")
			req := branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
				GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed}
			req.Hooks.AfterRemoteRead = func() error {
				git(t, fixture.root, "push", "-q", "--force", "origin", intruder+":"+test.movedRef)
				return nil
			}
			_, err := branch.LandPush(req)
			requirePublishCode(t, err, test.code)
			endpoint := git(t, fixture.origin, "rev-parse", "refs/heads/main")
			landing := git(t, fixture.origin, "rev-parse", "refs/heads/landing/goal-a")
			if test.movedRef == "refs/heads/main" {
				if endpoint != intruder || landing != prepared.Landing {
					t.Fatalf("endpoint race refs=%s, %s", endpoint, landing)
				}
			} else if endpoint != prepared.Endpoint || landing != intruder {
				t.Fatalf("landing race refs=%s, %s", endpoint, landing)
			}
		})
	}
}
