package contract

import (
	"os"
	"path/filepath"
	"testing"
)

// A path that does not exist yet, under a symlink pointing out of the
// repository, is outside the repository: the weak "resolve only if it
// exists" form kept it lexical and accepted it.
func TestRelUnderRepoRefusesANotYetExistingPathBehindALinkOut(t *testing.T) {
	t.Parallel()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, outside := filepath.Join(base, "repo"), filepath.Join(base, "outside")
	for _, dir := range []string{repo, outside} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(repo, "out")); err != nil {
		t.Fatal(err)
	}
	if rel, err := relUnderRepo(filepath.Join(repo, "out", "ledger.md"), repo); err == nil {
		t.Fatalf("a path behind a link out of the repository was accepted as %q", rel)
	}
	if rel, err := relUnderRepo(filepath.Join(repo, "inside", "ledger.md"), repo); err != nil || rel != "inside/ledger.md" {
		t.Fatalf("a not-yet-existing path inside the repository: %q %v", rel, err)
	}
}
