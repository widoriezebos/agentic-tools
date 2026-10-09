package branch_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// Ordinary inverse application needs Git's merge behavior to prove that a
// correction and its original disappear while an unrelated edit survives.
func TestCommittedDropInverseGitAdapter(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/unit.txt", "base\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "tracked base")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	original := commitUnit(t, f, "original", "metasystem/unit.txt", "base\noriginal\n")
	correction := commitUnit(t, f, "correction", "metasystem/unit.txt", "base\noriginal\ncorrection\n")
	v := commitUnit(t, f, "V", "metasystem/other.txt", "unrelated\n")
	commit, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "original", OpID: "drop-original", Kind: branch.Drop, CheckClaim: claimAllowed, FrozenPatch: []byte{}, BeforeCommit: func(dir, parent, tree string) error {
		if parent != v {
			t.Fatalf("inverse parent %s", parent)
		}
		git(t, dir, "revert", "--no-commit", correction, original)
		body, err := os.ReadFile(filepath.Join(dir, "metasystem/unit.txt"))
		if err != nil || string(body) != "base\n" {
			t.Fatalf("inverse lost original bytes: %q %v", body, err)
		}
		keep, err := os.ReadFile(filepath.Join(dir, "metasystem/other.txt"))
		if err != nil || string(keep) != "unrelated\n" {
			t.Fatalf("inverse erased unrelated bytes: %q %v", keep, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := git(t, f.root, "rev-parse", commit+"^"); got != v {
		t.Fatalf("history rewritten: %s", got)
	}
	for _, id := range []string{original, correction, v} {
		git(t, f.root, "merge-base", "--is-ancestor", id, commit)
	}
	kind, err := branch.KindOf(f.root, commit, "goal-a")
	if err != nil || kind.Kind != branch.Drop || kind.Operation != "drop-original" {
		t.Fatalf("drop identity: %+v %v", kind, err)
	}
	status, err := branch.InspectStatus(f.root, f.base, commit, "goal-a")
	if err != nil || len(status.Units) != 3 || status.Prefix != 0 {
		t.Fatalf("pending drop waived original/unrelated reads: %+v %v", status, err)
	}
	if !slices.ContainsFunc(status.Units, func(u branch.UnitStatus) bool { return u.Unit == "V" && u.ReadState == "built" }) {
		t.Fatalf("unread V hidden: %+v", status)
	}
	// A fresh checkout sees the inverse but cannot fold it as another unit.
	fresh := filepath.Join(t.TempDir(), "fresh")
	git(t, f.root, "clone", "-q", f.root, fresh)
	git(t, fresh, "config", "goal.human.Wido", "Wido <wido@example.invalid>")
	_, err = branch.PrepareLanding(branch.LandRequest{Repo: fresh, Remote: "origin", EndpointTip: f.base, BranchTip: commit, GoalID: "goal-a", Last: true, LandingReady: true, ReadsWaived: true, CandidateOnly: true, ApprovedBy: "human:Wido", Seat: "seat", CheckClaim: claimAllowed})
	if err == nil || !strings.Contains(err.Error(), "pending drop") {
		t.Fatalf("pending inverse landed: %v", err)
	}
}
