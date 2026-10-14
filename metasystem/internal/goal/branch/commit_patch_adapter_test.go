package branch_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// Git's three-tree index merge and apply path resolution are the adapter contract.
func TestFrozenPatchPreservesIndexGitAdapter(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	original := commitUnit(t, f, "declaration", "metasystem/metasystem.conf", "proof.cheap=false\n")
	commitUnit(t, f, "later", "metasystem/code.go", "unrelated later code\n")
	stage(t, f, "metasystem/records/unrelated.md", "person's staged record\n")
	staged := git(t, f.root, "diff", "--cached", "--binary")
	patch := []byte("--- a/metasystem/metasystem.conf\n+++ b/metasystem/metasystem.conf\n@@ -1 +1 @@\n-proof.cheap=false\n+proof.cheap=true\n")
	req := branch.CommitRequest{Repo: filepath.Join(f.root, "metasystem"), Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "inverse-act", OpID: "inverse-act", Kind: branch.Unit, FrozenPatch: patch, CheckClaim: claimAllowed}
	parent := git(t, f.root, "rev-parse", "HEAD")
	commit, err := branch.CommitFrozenPatch(req)
	if err != nil {
		t.Fatal(err)
	}
	if git(t, f.root, "rev-parse", commit+"^") != parent || git(t, f.root, "show", commit+":metasystem/metasystem.conf") != "proof.cheap=true" || git(t, f.root, "show", commit+":metasystem/code.go") != "unrelated later code" {
		t.Fatal("inverse did not append the exact delta")
	}
	if git(t, f.root, "show", original+":metasystem/metasystem.conf") != "proof.cheap=false" {
		t.Fatal("original history changed")
	}
	if git(t, f.root, "diff", "--cached", "--binary") != staged {
		t.Fatal("unrelated staging changed")
	}
	replay, err := branch.CommitFrozenPatch(req)
	if err != nil || replay != commit || git(t, f.root, "rev-parse", "HEAD") != commit {
		t.Fatalf("replay %s %v", replay, err)
	}
}

func TestFrozenPatchRefusesRemoteAheadGitAdapter(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	local := commitUnit(t, f, "declaration", "metasystem/metasystem.conf", "proof.cheap=false\n")
	git(t, f.root, "push", "-q", "origin", "HEAD:goal/goal-a")
	other := cloneBranchFixture(t, f)
	git(t, other.root, "switch", "-q", "-c", "goal/goal-a", "origin/goal/goal-a")
	remote := commitUnit(t, other, "later", "metasystem/code.go", "remote code\n")
	git(t, other.root, "push", "-q", "origin", "HEAD:goal/goal-a")
	stage(t, f, "metasystem/records/unrelated.md", "person's staged record\n")
	staged := git(t, f.root, "diff", "--cached", "--binary")
	req := branch.CommitRequest{Repo: filepath.Join(f.root, "metasystem"), Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "inverse-act", OpID: "inverse-act", Kind: branch.Unit, FrozenPatch: []byte("--- a/metasystem/metasystem.conf\n+++ b/metasystem/metasystem.conf\n@@ -1 +1 @@\n-proof.cheap=false\n+proof.cheap=true\n"), CheckClaim: claimAllowed}
	if _, err := branch.CommitFrozenPatch(req); err == nil || !strings.Contains(err.Error(), "bring the local branch current") {
		t.Fatalf("remote-ahead inverse: %v", err)
	}
	if git(t, f.root, "rev-parse", "HEAD") != local || remoteGoalTip(t, f) != remote || git(t, f.root, "diff", "--cached", "--binary") != staged {
		t.Fatal("refused inverse changed the branch or index")
	}
	// Follow the remedy without consuming the unrelated staged record.
	git(t, f.root, "fetch", "-q", "origin", "goal/goal-a")
	git(t, f.root, "merge", "--ff-only", "-q", "FETCH_HEAD")
	if _, err := branch.CommitFrozenPatch(req); err != nil {
		t.Fatalf("inverse after bringing the branch current: %v", err)
	}
	if git(t, f.root, "show", "HEAD:metasystem/code.go") != "remote code" || git(t, f.root, "diff", "--cached", "--binary") != staged {
		t.Fatal("inverse undid remote code or unrelated staging")
	}
}

func TestFrozenPatchKeepsCommitChecksGitAdapter(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	parent := commitUnit(t, f, "declaration", "metasystem/metasystem.conf", "proof.cheap=false\n")
	gateErr := errors.New("candidate check failed")
	calls := 0
	req := branch.CommitRequest{Repo: filepath.Join(f.root, "metasystem"), Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "inverse-act", OpID: "inverse-act", Kind: branch.Unit, FrozenPatch: []byte("--- a/metasystem/metasystem.conf\n+++ b/metasystem/metasystem.conf\n@@ -1 +1 @@\n-proof.cheap=false\n+proof.cheap=true\n"), CheckClaim: claimAllowed,
		BeforeCommit: func(dir, before, tree string) error {
			calls++
			if before != parent || git(t, dir, "write-tree") != tree || git(t, dir, "show", ":metasystem/metasystem.conf") != "proof.cheap=true" {
				t.Fatal("checks did not see the inverse candidate")
			}
			return gateErr
		}}
	if _, err := branch.CommitFrozenPatch(req); !errors.Is(err, gateErr) || calls != 1 || git(t, f.root, "rev-parse", "HEAD") != parent {
		t.Fatalf("inverse bypassed commit checks: calls=%d err=%v", calls, err)
	}
}

func TestFrozenPatchRefusesCodeGitAdapter(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	parent := commitUnit(t, f, "code", "metasystem/code.go", "original code\n")
	req := branch.CommitRequest{Repo: filepath.Join(f.root, "metasystem"), Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "inverse-act", OpID: "inverse-act", Kind: branch.Unit, FrozenPatch: []byte("--- a/metasystem/code.go\n+++ b/metasystem/code.go\n@@ -1 +1 @@\n-original code\n+replacement code\n"), CheckClaim: claimAllowed}
	if _, err := branch.CommitFrozenPatch(req); err == nil || !strings.Contains(err.Error(), "only metasystem.conf") {
		t.Fatalf("code patch accepted: %v", err)
	}
	if git(t, f.root, "rev-parse", "HEAD") != parent || git(t, f.root, "status", "--porcelain") != "" {
		t.Fatal("refused patch changed the checkout")
	}
}
