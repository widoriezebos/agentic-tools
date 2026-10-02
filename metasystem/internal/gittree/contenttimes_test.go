package gittree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContentTimeIsDeterministicAndOld(t *testing.T) {
	t.Parallel()
	a, err := ContentTime("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ContentTime("0123456789abcdef0123456789abcdef01234567")
	c, _ := ContentTime("1123456789abcdef0123456789abcdef01234567")
	if !a.Equal(b) || a.Equal(c) {
		t.Fatalf("content times: same id %v/%v, other id %v", a, b, c)
	}
	// Go refuses a test input younger than two seconds; every content time
	// lies before 2020.
	if limit := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC); !a.Before(limit) || !c.Before(limit) || a.Before(contentTimeEpoch) {
		t.Fatalf("content times %v %v leave [%v, %v)", a, c, contentTimeEpoch, limit)
	}
	if _, err := ContentTime("not-hex"); err == nil {
		t.Fatal("a malformed id produced a time")
	}
}

// Two checkouts of one tree at different fixed paths, made at different
// moments, carry identical times for every entry; a changed file and its
// directories do not; the index agrees with the stamped tree.
func TestContentTimesMakeCheckoutsOfOneTreeStatIdentical(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git adapter integration needs git")
	}
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		command := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@invalid"}, args...)...)
		command.Env = ScrubbedEnviron()
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	writeDetachedFile(t, filepath.Join(repo, "pkg", "a.go"), "package pkg\n")
	writeDetachedFile(t, filepath.Join(repo, "pkg", "b.go"), "package pkg // b\n")
	if err := os.Symlink("a.go", filepath.Join(repo, "pkg", "link")); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "one")
	one := run("rev-parse", "HEAD^{tree}")
	writeDetachedFile(t, filepath.Join(repo, "pkg", "b.go"), "package pkg // B\n")
	run("add", "-A")
	run("commit", "-q", "-m", "two")
	two := run("rev-parse", "HEAD^{tree}")
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkout := func(slot, tree string) (*DetachedWorktree, string) {
		plan, err := Workspace{Dir: repo}.PlanDetachedWorktreeAt(filepath.Join(scratch, slot), "wt-"+slot)
		if err != nil {
			t.Fatal(err)
		}
		detached, err := plan.Create(tree)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = detached.Close() })
		if err := detached.ContentTimes(); err != nil {
			t.Fatal(err)
		}
		return detached, plan.Top
	}
	stat := func(path string) (time.Time, time.Time) {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		link, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.ModTime(), link.ModTime()
	}
	_, first := checkout("s0", one)
	_, second := checkout("s1", one)
	_, changed := checkout("s2", two)
	// Every content time lies in the fixed span after the epoch, decades
	// before any checkout's own time.
	contentTimeCeiling := contentTimeEpoch.Add(time.Duration(contentTimeSpan))
	for _, rel := range []string{"pkg/a.go", "pkg/b.go", "pkg/link", "pkg", "."} {
		a, al := stat(filepath.Join(first, rel))
		b, bl := stat(filepath.Join(second, rel))
		if !a.Equal(b) || !al.Equal(bl) || !a.Before(contentTimeCeiling) || a.Before(contentTimeEpoch) {
			t.Fatalf("%s: %v/%v vs %v/%v", rel, a, al, b, bl)
		}
	}
	for rel, same := range map[string]bool{"pkg/a.go": true, "pkg/b.go": false, "pkg": false, ".": false} {
		a, _ := stat(filepath.Join(first, rel))
		c, _ := stat(filepath.Join(changed, rel))
		if a.Equal(c) != same {
			t.Fatalf("%s: time equality %t across trees, want %t", rel, a.Equal(c), same)
		}
	}
	for _, top := range []string{first, changed} {
		command := exec.Command("git", "-C", top, "diff-index", "--quiet", "HEAD", "--")
		command.Env = ScrubbedEnviron()
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("stamped checkout reads dirty: %v %s", err, out)
		}
	}
	if _, err := (Workspace{Dir: repo}).PlanDetachedWorktreeAt(filepath.Join(scratch, "s0"), "wt-again"); err == nil {
		t.Fatal("a plan reused an existing parent")
	}
	if _, err := (Workspace{Dir: repo}).PlanDetachedWorktreeAt("relative", "wt"); err == nil {
		t.Fatal("a relative parent was accepted")
	}
}
