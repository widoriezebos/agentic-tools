package realpath

import (
	"os"
	"path/filepath"
	"testing"
)

// linkedBed is a root holding a symlink "out" to a directory outside it.
func linkedBed(t *testing.T) (root, outside string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, outside = filepath.Join(base, "root"), filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	return root, outside
}

func TestResolveFollowsTheDeepestExistingAncestor(t *testing.T) {
	t.Parallel()
	root, outside := linkedBed(t)
	missing := filepath.Join(root, "out", "not-yet", "file")
	if got, want := Resolve(missing), filepath.Join(outside, "not-yet", "file"); got != want {
		t.Fatalf("Resolve(%s) = %s, want %s", missing, got, want)
	}
	if Within(Resolve(missing), root) {
		t.Fatal("a not-yet-existing path under a link out of root must resolve outside it")
	}
}

func TestResolveExistingResolvesOnlyAWholeExistingPath(t *testing.T) {
	t.Parallel()
	root, outside := linkedBed(t)
	if got := ResolveExisting(filepath.Join(root, "out")); got != outside {
		t.Fatalf("an existing link resolves: %s", got)
	}
	missing := filepath.Join(root, "out", "not-yet")
	if got := ResolveExisting(missing); got != missing {
		t.Fatalf("a missing path stays lexical: %s", got)
	}
}

func TestWithinComparesWholeSegments(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		path, root string
		want       bool
	}{
		{"/a/b", "/a/b", true}, {"/a/b/c", "/a/b", true}, {"/a/bc", "/a/b", false},
		{"/a", "/a/b", false}, {"/a/b/../c", "/a/b", false}, {"/x/..y", "/x", true},
	} {
		if got := Within(c.path, c.root); got != c.want {
			t.Fatalf("Within(%s, %s) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}

// TestCanonicalRefusesWhatDoesNotExist: an existing path through a link is
// its absolute, symlink-free form; a missing one is the resolution error,
// never a lexical guess.
func TestCanonicalRefusesWhatDoesNotExist(t *testing.T) {
	t.Parallel()
	root, outside := linkedBed(t)
	if got, err := Canonical(filepath.Join(root, "out")); err != nil || got != outside {
		t.Fatalf("Canonical of an existing link = %s, %v; want %s", got, err, outside)
	}
	if got, err := Canonical(filepath.Join(root, "out", "not-yet")); err == nil {
		t.Fatalf("Canonical of a missing path = %s, want its error", got)
	}
}

// Absolute is a path flag's form: empty stays empty, a relative path is
// joined to the working directory and cleaned, and no symlink is resolved.
func TestAbsoluteCleansWithoutResolving(t *testing.T) {
	t.Parallel()
	if got, err := Absolute(""); err != nil || got != "" {
		t.Fatalf("Absolute(\"\") = %q, %v", got, err)
	}
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Absolute("a/../b/./c/"); err != nil || got != filepath.Join(working, "b", "c") {
		t.Fatalf("Absolute(relative) = %q, %v", got, err)
	}
	if got, err := Absolute("/x/y/../z"); err != nil || got != "/x/z" {
		t.Fatalf("Absolute(absolute) = %q, %v", got, err)
	}
}
