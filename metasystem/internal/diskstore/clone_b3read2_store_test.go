package diskstore

// Witnesses of the second B3 read (the reader's probes, kept as they
// failed on 118b71a4f).

import (
	"context"
	"path/filepath"
	"testing"
)

// R2-C5: the checkout is a linked worktree whose common store lives in the
// clone named by --path: the archive is fetched into the clone's own store,
// then the clone (and the checkout's own worktree) is removed.
func TestB3Read2CloneHoldingTheCheckoutsCommonStore(t *testing.T) {
	t.Parallel()
	r, clone := simpleClone(t)
	r.git(clone, "commit", "-q", "--allow-empty", "-m", "unique in the clone")
	unique := r.git(clone, "rev-parse", "HEAD")
	wt := filepath.Join(filepath.Dir(r.repo), "checkout-wt")
	r.git(clone, "worktree", "add", "-q", "--detach", wt)
	out, err := ReleaseClone(context.Background(), CloneReleaseRequest{Registry: CheckoutRegistry(filepath.Join(wt, "metasystem")), GitRoot: wt, Path: clone,
		Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow})
	t.Logf("outcome %+v err %v", out, err)
	if _, e := realWorkspaceGit(context.Background(), r.repo, "cat-file", "-e", unique+"^{commit}"); e != nil && out.Done {
		t.Fatalf("the clone holding the checkout's common store was released; commit %s is in no surviving store", unique)
	}
}
