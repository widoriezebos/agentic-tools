package branch_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

type landFixture struct {
	*branchFixture
	units     []string
	tip       string
	projected string
	receipt   string
}

func newLandFixture(t *testing.T) landFixture {
	t.Helper()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/memory/receipts.log", "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "seed receipt register")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	git(t, f.root, "config", "user.name", "Ambient Seat")
	git(t, f.root, "config", "user.email", "ambient@example.invalid")
	git(t, f.root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")

	stage(t, f, "metasystem/plans/goal-a.md", "approved goal plan\n")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "land-plan", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	var units []string
	for index, item := range []struct{ unit, path, body string }{
		{"u1", "metasystem/one.go", "one\n"},
		{"u2", "metasystem/two.go", "two\n"},
		{"u3", "metasystem/three.go", "three\n"},
	} {
		commit := commitUnit(t, f, item.unit, item.path, item.body)
		units = append(units, commit)
		readUnit(t, f, item.unit, commit)
		if index == 1 {
			stage(t, f, "metasystem/plans/between.md", "folded into u3\n")
			if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
				GoalID: "goal-a", OpID: "land-between", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
				t.Fatal(err)
			}
		}
	}
	stage(t, f, "metasystem/plans/tail.md", "tail fold\n")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "land-tail", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(f, "land-push")); err != nil {
		t.Fatal(err)
	}
	tip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	projected, err := landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), git(t, f.root, "rev-parse", tip+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	writeLandingReceipt(t, receipt, projected, "attempt-deep")
	return landFixture{branchFixture: f, units: units, tip: tip, projected: projected, receipt: receipt}
}

func writeLandingReceipt(t *testing.T, path, tree, attempt string) {
	t.Helper()
	body := fmt.Sprintf(`{"schemaVersion":3,"tree":%q,"exitStatus":0,"time":"2026-09-17T10:00:00Z","proof":{"attemptId":%q}}`, tree, attempt)
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func landRequest(t *testing.T, f landFixture, out string) branch.LandRequest {
	t.Helper()
	return branch.LandRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: f.tip,
		GoalID: "goal-a", Out: out, TestReceipt: f.receipt, Last: true, LandingReady: true,
		GoalPage: "the whole goal is ready", ApprovedBy: "human:Wido", Seat: "seat-a", CheckClaim: claimAllowed}
}

func requireLandCode(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("refusal = %v, want %s", err, code)
	}
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists after refusal: %v", path, err)
	}
}
