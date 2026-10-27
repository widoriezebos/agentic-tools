package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// TestWorktreeDiffDigestEqualsCommitPatchDigest holds finding 436: a unit
// round's worktree.diff, which a live read's DiffDigest binds, must be the
// same bytes as the commit patch a landed unit's PatchDigest binds, so a
// clean live read proves the committed subject of the same change.
func TestWorktreeDiffDigestEqualsCommitPatchDigest(t *testing.T) {
	t.Parallel()
	repo := initReadSubjectRepo(t)
	write := func(rel string, data []byte) {
		t.Helper()
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metasystem/moved.md", []byte("one\ntwo\nthree\nfour\nfive\nsix\n"))
	write("metasystem/gone.md", []byte("removed later\n"))
	gitReadSubject(t, repo, "add", "-A")
	gitReadSubject(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "second")
	base := gitReadSubject(t, repo, "rev-parse", "HEAD")

	write("metasystem/page.md", []byte("design two\n"))
	write("metasystem/added.go", []byte("package added\n"))
	write("metasystem/blob.bin", []byte{0, 1, 2, 3, 0xff, 0, 'x'})
	if err := os.Remove(filepath.Join(repo, "metasystem", "gone.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repo, "metasystem", "moved.md"), filepath.Join(repo, "metasystem", "renamed.md")); err != nil {
		t.Fatal(err)
	}

	diff, err := (&launch.UnitRunner{Git: launch.OSGitRunner{}}).WorktreeDiff(repo, base)
	if err != nil {
		t.Fatal(err)
	}
	gitReadSubject(t, repo, "add", "-A")
	gitReadSubject(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "change")
	commit := gitReadSubject(t, repo, "rev-parse", "HEAD")
	subject, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("commit subject present=%v err=%v", present, err)
	}
	sum := sha256.Sum256(diff)
	if live := hex.EncodeToString(sum[:]); live != subject.DiffDigest {
		patch, _ := gitReadSubjectFacts{}.CommitDiff(repo, commit+"^", commit)
		t.Fatalf("worktree.diff digest %s != commit patch digest %s\nworktree.diff:\n%s\ncommit patch:\n%s", live, subject.DiffDigest, diff, patch)
	}
}
