package branch_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
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
	t.Parallel()
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
