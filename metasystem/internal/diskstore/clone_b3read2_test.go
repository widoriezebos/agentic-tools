package diskstore

// Witnesses of the second B3 read (the reader's probes, kept as they
// failed on 118b71a4f).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func simpleClone(t *testing.T) (*realRepo, string) {
	r := newRealRepo(t)
	clone := filepath.Join(filepath.Dir(r.repo), "clone")
	r.git(filepath.Dir(r.repo), "clone", "-q", r.repo, clone)
	r.git(clone, "config", "user.name", "t")
	r.git(clone, "config", "user.email", "t@example.com")
	return r, clone
}

func releaseCloneAt(t *testing.T, r *realRepo, path string) (CloneRelease, error) {
	return ReleaseClone(context.Background(), CloneReleaseRequest{Registry: r.registry, GitRoot: r.repo, Path: path,
		Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow})
}

// R2-C1: a clone's ignored content (a seat's metasystem/artifacts, a .env)
// is removed with no keep and no mention.
func TestB3Read2CloneIgnoredContentLost(t *testing.T) {
	t.Parallel()
	r, clone := simpleClone(t)
	os.MkdirAll(filepath.Join(clone, "metasystem", "artifacts", "evidence"), 0o700)
	os.WriteFile(filepath.Join(clone, "metasystem", "artifacts", "evidence", "receipt.log"), []byte(strings.Repeat("x", 4<<20)), 0o600)
	os.WriteFile(filepath.Join(clone, "local.env"), []byte("SECRET=1"), 0o600)
	out, err := releaseCloneAt(t, r, clone)
	t.Logf("outcome %+v err %v", out, err)
	if _, statErr := os.Stat(filepath.Join(clone, "metasystem", "artifacts", "evidence", "receipt.log")); statErr != nil {
		t.Fatalf("4 MiB ignored evidence in the clone removed; outcome reason %q", out.Reason)
	}
}

// R2-C2: a commit reachable only from HEAD's reflog (a reset --hard) is
// unreachable from every archive ref after the release.
func TestB3Read2CloneReflogOnlyCommitLost(t *testing.T) {
	t.Parallel()
	r, clone := simpleClone(t)
	r.git(clone, "commit", "-q", "--allow-empty", "-m", "work later reset away")
	lost := r.git(clone, "rev-parse", "HEAD")
	r.git(clone, "reset", "-q", "--hard", "HEAD~1")
	// The fixture's user configuration keeps the clone; a person's discard
	// releases it, and the reflog-only commit must be archived first.
	out, err := ReleaseClone(context.Background(), CloneReleaseRequest{Registry: r.registry, GitRoot: r.repo, Path: clone,
		Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow, Discard: &Discard{By: "t", At: testNow, Reason: "fixture"}})
	t.Logf("outcome %+v err %v", out, err)
	refs := r.git(r.repo, "for-each-ref", "--contains", lost)
	if strings.TrimSpace(refs) == "" && out.Done {
		t.Fatalf("reflog-only commit %s reachable from no ref after the clone release", lost)
	}
}

// R2-C3: an embedded repository committed as a gitlink without
// .gitmodules: submodule status fails, the inventory skips it silently, and
// its own commits go with the clone.
func TestB3Read2CloneEmbeddedRepoWithoutGitmodulesLost(t *testing.T) {
	t.Parallel()
	r, clone := simpleClone(t)
	inner := filepath.Join(clone, "inner")
	r.git(clone, "init", "-q", "-b", "main", inner)
	r.git(inner, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "inner unique")
	unique := r.git(inner, "rev-parse", "HEAD")
	r.git(clone, "add", "inner")
	r.git(clone, "commit", "-q", "-m", "embedded repo")
	out, err := releaseCloneAt(t, r, clone)
	t.Logf("outcome %+v err %v", out, err)
	if _, e := realWorkspaceGit(context.Background(), r.repo, "cat-file", "-e", unique+"^{commit}"); e != nil && out.Done {
		t.Fatalf("the embedded repo's unique commit %s is in no store after the clone release", unique)
	}
}

// R2-C4: a clone that is not beside the checkout (two levels away) is
// accepted: nothing checks "beside".
func TestB3Read2CloneNotBesideAccepted(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	far := filepath.Join(filepath.Dir(r.repo), "a", "b", "seat")
	os.MkdirAll(filepath.Dir(far), 0o700)
	r.git(filepath.Dir(r.repo), "clone", "-q", r.repo, far)
	out, err := releaseCloneAt(t, r, far)
	t.Logf("outcome %+v err %v", out, err)
	if out.Done {
		t.Fatalf("a clone that is not beside the checkout was released: %s", far)
	}
}
