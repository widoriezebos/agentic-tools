package branch_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestGoalLandingPublicationAndVerification(t *testing.T) {
	t.Parallel()
	t.Run("whole series", func(t *testing.T) {
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
		{name: "endpoint moved", code: branch.LandTrunkMovedCode, movedRef: "refs/heads/main"},
		{name: "landing moved", code: branch.LandBranchMovedCode, movedRef: "refs/heads/landing/goal-a"},
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
			if test.movedRef != "refs/heads/main" && git(t, fixture.origin, "rev-parse", "refs/heads/main") != prepared.Endpoint {
				t.Fatal("landing-tip race moved the endpoint")
			}
			if test.movedRef == "refs/heads/main" && !strings.Contains(git(t, fixture.root, "ls-remote", "--refs", "origin", "refs/heads/landing/goal-a"), prepared.Landing) {
				t.Fatal("endpoint race deleted the prepared landing branch")
			}
		})
	}
}

func TestLandPushRerunAfterPublishedCrashReportsLanded(t *testing.T) {
	t.Parallel()
	fixture := newLandFixture(t)
	out, prepared := preparedLanding(t, fixture)
	git(t, fixture.root, "push", "-q", "--atomic", "origin",
		prepared.Landing+":refs/heads/main", ":refs/heads/landing/goal-a")
	result, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
		GoalID: "goal-a", Prepared: out, CheckClaim: claimAllowed})
	if err != nil || result != (branch.PreparedLanding{Endpoint: prepared.Endpoint, Candidate: prepared.Candidate,
		Landing: prepared.Landing, Branch: prepared.Branch}) {
		t.Fatalf("published rerun = %+v, %v", result, err)
	}
}

func TestLandPushRechecksClaimBeforePush(t *testing.T) {
	t.Parallel()
	fixture := newLandFixture(t)
	out, prepared := preparedLanding(t, fixture)
	checks := 0
	_, err := branch.LandPush(branch.LandPushRequest{Repo: fixture.root, Remote: "origin", EndpointRef: "refs/heads/main",
		GoalID: "goal-a", Prepared: out, CheckClaim: func() error {
			checks++
			if checks == 2 {
				return errors.New("claim moved before push")
			}
			return nil
		}})
	requirePublishCode(t, err, branch.NotHolderCode)
	if checks != 2 || git(t, fixture.origin, "rev-parse", "refs/heads/main") != prepared.Endpoint {
		t.Fatalf("claim checks=%d endpoint moved", checks)
	}
}
