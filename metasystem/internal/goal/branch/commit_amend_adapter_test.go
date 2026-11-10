package branch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// Git's working-directory-relative index paths and commit replay cannot be
// proved by the policy fixture.
func TestAmendAfterReadUsesRepositoryPathsGitAdapter(t *testing.T) {
	t.Setenv("GIT_AUTHOR_DATE", "2026-10-24T10:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-10-24T10:00:00Z")
	for _, layout := range []struct{ name, prefix string }{{"flat", ""}, {"nested", "metasystem"}} {
		for _, caller := range []string{"top", "installation"} {
			t.Run(layout.name+"/"+caller, func(t *testing.T) {
				f := newBranchFixture(t)
				installation := filepath.Join(f.root, layout.prefix)
				f.base = f.commit(t, filepath.Join(layout.prefix, "metasystem.conf"), "", "fixture installation")
				git(t, f.root, "push", "-q", "origin", "HEAD:main")
				publish := func(op string) {
					t.Helper()
					if _, err := branch.Push(pushRequest(f, op)); err != nil {
						t.Fatal(err)
					}
				}
				read := func(unit, commit string) string {
					t.Helper()
					record := "metasystem/records/misc/" + unit + "-read.md"
					write(t, f.root, record, "Read commit "+commit+" with unit digest "+mustUnitDigest(t, f.root, commit)+".\nVerdict: LAND\n")
					tip, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: installation, Remote: "origin", EndpointTip: f.base,
						GoalID: "goal-a", Unit: unit, OpID: "read-" + unit, ReaderRecord: record, CheckClaim: claimAllowed,
						GateRunID: "fixture-gate", GateTree: unitTree(t, f, commit)})
					if err != nil {
						t.Fatal(err)
					}
					publish("push-read-" + unit)
					return tip
				}
				code := filepath.ToSlash(filepath.Join(layout.prefix, "code.go"))
				unit := commitUnit(t, f, "u1", code, "one\n")
				publish("push-u1")
				obsolete := read("u1", unit)
				planPath := "metasystem/plans/later.md"
				plan := f.commit(t, planPath, "later", "later plan\n\nGoal-Plan: goal-a")
				other := commitUnit(t, f, "u2", filepath.ToSlash(filepath.Join(layout.prefix, "other.go")), "other\n")
				otherRead := read("u2", other)
				stage(t, f, code, "corrected\n")
				repo := f.root
				if caller == "installation" {
					repo = installation
				}
				t.Chdir(repo)
				tip, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: f.base,
					GoalID: "goal-a", Unit: "u1", OpID: "correct-u1", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
				if err != nil {
					t.Fatal(err)
				}
				commits, err := branch.ValidateRange(f.root, f.base, tip, "goal-a")
				if err != nil || len(commits) != 4 || commits[0].Kind != branch.Unit || commits[0].Unit != "u1" || commits[0].ID == unit ||
					commits[1].Kind != branch.Plan || commits[1].ID == plan || commits[2].Kind != branch.Unit || commits[2].Unit != "u2" || commits[3].Kind != branch.Read {
					t.Fatalf("corrected range=%+v err=%v", commits, err)
				}
				if got := git(t, f.root, "show", commits[0].ID+":"+code); got != "corrected" {
					t.Fatalf("corrected build content=%q", got)
				}
				if got := git(t, f.root, "show", tip+":"+planPath); got != "later" {
					t.Fatalf("replayed plan=%q", got)
				}
				for _, path := range []string{"metasystem/records/reads/goal-a/" + unit + ".json", "metasystem/records/misc/u1-read.md"} {
					git(t, f.root, "cat-file", "-e", obsolete+":"+path)
					if _, err := execGit(f.root, "cat-file", "-e", tip+":"+path); err == nil {
						t.Fatalf("obsolete read survived at %s", path)
					}
					if _, err := os.Stat(filepath.Join(f.root, path)); !os.IsNotExist(err) {
						t.Fatalf("obsolete read remains in checkout at %s: %v", path, err)
					}
				}
				for _, path := range []string{"metasystem/records/reads/goal-a/" + other + ".json", "metasystem/records/misc/u2-read.md"} {
					if got, want := git(t, f.root, "show", tip+":"+path), git(t, f.root, "show", otherRead+":"+path); got != want {
						t.Fatalf("another unit's read changed at %s", path)
					}
				}
				if tree, index := git(t, f.root, "rev-parse", tip+"^{tree}"), git(t, f.root, "write-tree"); tree != index {
					t.Fatalf("corrected tree=%s index=%s", tree, index)
				}
				if got := git(t, f.root, "status", "--porcelain"); got != "" {
					t.Fatalf("corrected checkout status=%q", got)
				}
			})
		}
	}
}
