package goal

import (
	"os"
	"path/filepath"
	"testing"
)

// The default history probe must resolve HEAD at the checkout root's
// prefix, including when a tracked worktree file has been removed.
func TestGitAdapterAdoptionHeadTracking(t *testing.T) {
	t.Parallel()
	check := func(root string, want bool) {
		t.Helper()
		tracked, err := headTracksLedgerWithEnvironment(root, nil)
		if err != nil || tracked != want {
			t.Fatalf("root %q: HEAD tracks ledger=%v, err=%v; want %v", root, tracked, err, want)
		}
	}

	outside := t.TempDir()
	check(outside, false)

	// A checkout whose .git git cannot follow is a repository it cannot
	// read, never "no repository": the probe fails closed.
	unreadable := t.TempDir()
	writeFile(t, filepath.Join(unreadable, ".git"), "gitdir: "+filepath.Join(unreadable, "missing")+"\n")
	if tracked, err := headTracksLedgerWithEnvironment(unreadable, nil); err == nil || tracked {
		t.Fatalf("an unreadable checkout read as untracked: tracked=%v err=%v", tracked, err)
	}

	unborn := t.TempDir()
	gitOK(t, unborn, "init", "-q")
	check(unborn, false)

	repo := t.TempDir()
	gitOK(t, repo, "init", "-q")
	writeFile(t, filepath.Join(repo, "README.md"), "hi\n")
	gitOK(t, repo, "add", ".")
	gitOK(t, repo, "commit", "-qm", "no ledger")
	check(repo, false)

	writeFile(t, filepath.Join(repo, "plans", "goals.md"), goalFreeLedger)
	writeFile(t, filepath.Join(repo, "nested", "README.md"), "nested\n")
	gitOK(t, repo, "add", ".")
	gitOK(t, repo, "commit", "-qm", "toplevel ledger only")
	check(repo, true)
	nested := filepath.Join(repo, "nested")
	check(nested, false)

	writeFile(t, filepath.Join(nested, "plans", "goals.md"), goalFreeLedger)
	gitOK(t, repo, "add", ".")
	gitOK(t, repo, "commit", "-qm", "nested ledger")
	check(nested, true)
	if err := os.Remove(filepath.Join(nested, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
	check(nested, true)
}
