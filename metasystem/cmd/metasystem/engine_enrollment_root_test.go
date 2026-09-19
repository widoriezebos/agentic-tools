package main

import (
	"path/filepath"
	"testing"
)

// A linked worktree borrows the enrollment of the same installation path in
// its main checkout, so a nested installation keeps its subdirectory; a main
// checkout has nothing to borrow.
func TestLinkedWorktreeEnrollmentKeepsTheInstallationSubdirectory(t *testing.T) {
	t.Parallel()
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(top, "metasystem")
	writeTestingFixtureFile(t, filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644)
	testingFixtureGit(t, top, "init", "-q")
	testingFixtureGit(t, top, "add", ".")
	testingFixtureGit(t, top, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-q", "-m", "base")
	if got, linked := linkedEnrollmentRoot(installation); linked {
		t.Fatalf("main checkout borrowed enrollment from %q", got)
	}

	worktree := filepath.Join(t.TempDir(), "tree")
	testingFixtureGit(t, top, "worktree", "add", "-q", "--detach", worktree, "HEAD")
	got, linked := linkedEnrollmentRoot(filepath.Join(worktree, "metasystem"))
	if !linked || got != installation {
		t.Fatalf("linked worktree enrollment root = %q linked=%v, want the main checkout installation %q", got, linked, installation)
	}
}
