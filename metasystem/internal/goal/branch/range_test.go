package branch_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

type branchFixture struct{ root, origin, base string }

func newBranchFixture(t *testing.T) *branchFixture {
	t.Helper()
	d := t.TempDir()
	origin, root := filepath.Join(d, "origin.git"), filepath.Join(d, "work")
	git(t, d, "init", "-q", "--bare", origin)
	git(t, origin, "symbolic-ref", "HEAD", "refs/heads/main")
	git(t, d, "clone", "-q", origin, root)
	git(t, root, "config", "user.name", "fixture")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	write(t, root, "base.txt", "base")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	git(t, root, "push", "-qu", "origin", "HEAD:main")
	return &branchFixture{root: root, origin: origin, base: git(t, root, "rev-parse", "HEAD")}
}

func (f *branchFixture) commit(t *testing.T, path, body, message string) string {
	t.Helper()
	write(t, f.root, path, body)
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", message)
	return git(t, f.root, "rev-parse", "HEAD")
}

func write(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestUnitDigestIgnoresDiffConfiguration(t *testing.T) {
	f := newBranchFixture(t)
	commit := f.commit(t, "metasystem/code.go", "a", "unit\n\nGoal-Unit: goal-a/u1")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "-q", f.origin, other)
	for _, row := range []struct {
		root   string
		values []string
	}{{f.root, []string{"false", "myers", "false", "false"}}, {other, []string{"true", "histogram", "true", "true"}}} {
		for i, key := range []string{"diff.noprefix", "diff.algorithm", "diff.renames", "diff.mnemonicPrefix"} {
			git(t, row.root, "config", key, row.values[i])
		}
	}
	a, err := branch.UnitDigest(f.root, commit)
	if err != nil {
		t.Fatal(err)
	}
	b, err := branch.UnitDigest(other, commit)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("raw digests differ: %s != %s", a, b)
	}
	if git(t, f.root, "diff", "--binary", "--full-index", commit+"^", commit) == git(t, other, "diff", "--binary", "--full-index", commit+"^", commit) {
		t.Fatal("patch digest witness did not differ")
	}
	changed := f.commit(t, "metasystem/code.go", "b", "unit 2\n\nGoal-Unit: goal-a/u2")
	c, err := branch.UnitDigest(f.root, changed)
	if err != nil || c == a {
		t.Fatalf("one-byte change: digest=%s err=%v", c, err)
	}
	if _, err := branch.UnitDigest(f.root, f.base); err == nil || !strings.Contains(err.Error(), "GOAL_BRANCH_RANGE") {
		t.Fatalf("root commit: %v", err)
	}
}

func TestValidateRangeRefusals(t *testing.T) {
	hexA, hexB := strings.Repeat("a", 40), strings.Repeat("b", 40)
	tests := []struct {
		name    string
		makeTip func(*branchFixture, *testing.T) string
	}{
		{"no trailer", func(f *branchFixture, t *testing.T) string { return f.commit(t, "metasystem/a.go", "x", "plain") }},
		{"two trailers", func(f *branchFixture, t *testing.T) string {
			return f.commit(t, "metasystem/a.go", "x", "two\n\nGoal-Unit: goal-a/u\nGoal-Plan: goal-a")
		}},
		{"unit whitespace", func(f *branchFixture, t *testing.T) string {
			return f.commit(t, "metasystem/a.go", "x", "bad\n\nGoal-Unit: goal-a/u\u00a0part")
		}},
		{"merge", func(f *branchFixture, t *testing.T) string {
			side := git(t, f.root, "commit-tree", f.base+"^{tree}", "-p", f.base, "-m", "side")
			return git(t, f.root, "commit-tree", f.base+"^{tree}", "-p", f.base, "-p", side, "-m", "merge\n\nGoal-Unit: goal-a/u")
		}},
		{"unit plan", func(f *branchFixture, t *testing.T) string {
			return f.commit(t, "metasystem/plans/x.md", "x", "unit\n\nGoal-Unit: goal-a/u")
		}},
		{"unit exclusion", func(f *branchFixture, t *testing.T) string {
			return f.commit(t, "metasystem/"+landing.WorkspaceExclusions()[0], "x", "unit\n\nGoal-Unit: goal-a/u")
		}},
		{"plan code", func(f *branchFixture, t *testing.T) string {
			return f.commit(t, "metasystem/a.go", "x", "plan\n\nGoal-Plan: goal-a")
		}},
		{"read twice", func(f *branchFixture, t *testing.T) string {
			write(t, f.root, "metasystem/records/reads/goal-a/"+hexA+".json", "{}")
			return f.commit(t, "metasystem/records/reads/goal-a/"+hexB+".json", "{}", "read\n\nGoal-Read: goal-a/u "+hexA)
		}},
		{"read mismatch", func(f *branchFixture, t *testing.T) string {
			return f.commit(t, "metasystem/records/reads/goal-a/"+hexB+".json", "{}", "read\n\nGoal-Read: goal-a/u "+hexA)
		}},
		{"other goal", func(f *branchFixture, t *testing.T) string {
			return f.commit(t, "metasystem/a.go", "x", "unit\n\nGoal-Unit: goal-b/u")
		}},
		{"not descendant", func(f *branchFixture, t *testing.T) string {
			return git(t, f.root, "commit-tree", f.base+"^{tree}", "-m", "root\n\nGoal-Unit: goal-a/u")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newBranchFixture(t)
			tip := test.makeTip(f, t)
			_, err := branch.ValidateRange(f.root, f.base, tip, "goal-a")
			var refusal *branch.RangeError
			if !errors.As(err, &refusal) || refusal.Code != "GOAL_BRANCH_RANGE" || !strings.Contains(err.Error(), tip) {
				t.Fatalf("tip %s: %v", tip, err)
			}
			if test.name == "not descendant" && !strings.Contains(refusal.Reason, "tip does not descend") {
				t.Fatalf("descendant refusal changed: %v", err)
			}
		})
	}
}

func TestValidateRangeCleanRangePasses(t *testing.T) {
	f := newBranchFixture(t)
	f.commit(t, "metasystem/plans/x.md", "plan", "plan\n\nGoal-Plan: goal-a")
	unit := f.commit(t, "metasystem/code.go", "unit", "unit\n\nGoal-Unit: goal-a/u1")
	tip := f.commit(t, "metasystem/records/reads/goal-a/"+unit+".json", "{}", "read\n\nGoal-Read: goal-a/u1 "+unit)
	commits, err := branch.ValidateRange(f.root, f.base, tip, "goal-a")
	if err != nil || len(commits) != 3 || commits[0].Kind != branch.Plan || commits[1].Kind != branch.Unit || commits[1].Unit != "u1" || commits[1].Digest == "" || commits[2].Kind != branch.Read || commits[2].Unit != "u1" {
		t.Fatalf("commits=%+v err=%v", commits, err)
	}
	empty, err := branch.ValidateRange(f.root, tip, tip, "goal-a")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}
