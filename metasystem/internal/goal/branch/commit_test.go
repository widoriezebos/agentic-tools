package branch_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
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
