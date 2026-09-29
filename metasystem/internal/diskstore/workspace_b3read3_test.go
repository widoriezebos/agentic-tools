package diskstore

// Witnesses of the third B3 read (the reader's probes that still apply
// after the Round B3-3 scope cut; each but the last failed on 8ddaa3618).

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R3-F6/C1 variant (workspace): 4 MiB of ignored content whose name has a
// space. Porcelain v1 quotes it; Measure is handed the quoted name, reads
// nothing, and the workspace is released with --force.
func TestB3Read3WorkspaceIgnoredQuotedNameLost(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	os.WriteFile(filepath.Join(r.repo, ".gitignore"), []byte("metasystem/artifacts/\n*.env\n*.bin\n"), 0o600)
	r.git(r.repo, "commit", "-qam", "ignore bins")
	head := r.git(r.repo, "rev-parse", "HEAD")
	ws := r.obtain("q", head)
	big := filepath.Join(ws.Record.Path, "model weights.bin")
	os.WriteFile(big, []byte(strings.Repeat("w", 4<<20)), 0o600)
	out := r.release(ws.Record.ID)
	t.Logf("outcome %+v", out)
	if _, err := os.Stat(big); err != nil {
		t.Fatalf("4 MiB ignored file with a space in its name removed with the workspace (limit 1 MiB); reason %q", out.Reason)
	}
}

// R3-W1 variant: a person's --discard release was interrupted after
// releasing+AuthorizedDiscard were written (the removal stopped). Work is
// then written into the workspace. A non-person release (a landing's
// release set, an agent's plain --release) without --discard finishes the
// person's discard and removes the new work unjudged.
func TestB3Read3NonPersonFinishesAPersonsDiscard(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("d", r.base)
	critical, err := r.registry.TryCritical(ws.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := critical.Record()
	rec.State = StateReleasing
	rec.AuthorizedDiscard = &Discard{By: "Wido", At: testNow, Reason: "scratch"}
	if err := critical.Write(rec); err != nil {
		t.Fatal(err)
	}
	critical.Release()
	work := filepath.Join(ws.Record.Path, "later-work.go")
	os.WriteFile(work, []byte("package x // written after the interrupted discard\n"), 0o600)
	set := ReleaseSet{Tip: r.base, Stores: []ReleaseEntry{{ID: ws.Record.ID, Path: ws.Record.Path, State: ReleasePending}}}
	RunReleaseSet(context.Background(), WorkspaceReleaseRequest{Registry: r.registry, GitRoot: r.repo, Git: realWorkspaceGit,
		Census: &UseCensus{Taken: true}, By: "the landing", Now: testNow}, &set)
	t.Logf("set %+v", set)
	if _, err := os.Stat(work); err != nil {
		t.Fatalf("the landing's (machine) release finished a person's discard and removed work written after it, unjudged")
	}
}

// R3-rule2: an interrupted copy creation is discarded by the next request
// with `git worktree prune`, which prunes EVERY missing worktree of the
// checkout: another worktree whose directory is only temporarily absent
// (an unmounted volume) loses its administrative entry, and a commit only
// its detached HEAD held is reachable from nothing.
func TestB3Read3DiscardPartialPrunesOtherWorktrees(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	other := filepath.Join(filepath.Dir(r.repo), "on-external-volume")
	r.git(r.repo, "worktree", "add", "-q", "--detach", other)
	r.git(other, "commit", "-q", "--allow-empty", "-m", "only the detached HEAD holds this")
	unique := r.git(other, "rev-parse", "HEAD")
	aside := other + "-unmounted"
	if err := os.Rename(other, aside); err != nil {
		t.Fatal(err)
	}
	record, err := r.registry.Register(Registration{Path: WorkspacePath(r.control, goalG, "bed"), Git: true, Class: WorkspaceClass, Owner: goalG,
		Checkout: r.control, Lifetime: LifetimeOwner, CapKind: CapTarget, Layout: LayoutCopy, Reservation: mustKey(t, goalG, "bed"), CopyOf: r.base}, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	writeReservation(t, r.registry, goalG, "bed", record.ID)
	if _, err := ObtainWorkspace(context.Background(), r.request("bed", r.base)); err != nil {
		t.Logf("obtain: %v", err)
	}
	listing := r.git(r.repo, "worktree", "list", "--porcelain")
	os.Rename(aside, other)
	if !strings.Contains(listing, "on-external-volume") {
		out, _ := realWorkspaceGit(context.Background(), other, "status")
		t.Fatalf("another worktree's admin entry was pruned by an unrelated request; commit %s is held by no ref or worktree now (git status in it: %q)", unique, strings.TrimSpace(string(out)))
	}
}

// R3-W1 variant: the verb's archive fails midway (the second archive ref
// cannot be written); later the SWEEPER retries (owner ended), after new
// uncommitted work and a new commit landed in the workspace. Expect: the
// new work keeps it; once clean, every tip (old and new) is archived
// before removal.
func TestB3Read3ArchiveFailsMidwaySweeperRetries(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("m", r.base)
	r.git(ws.Record.Path, "commit", "-q", "--allow-empty", "-m", "first")
	r.git(ws.Record.Path, "checkout", "-q", "--detach")
	r.git(ws.Record.Path, "commit", "-q", "--allow-empty", "-m", "detached head work")
	detached := r.git(ws.Record.Path, "rev-parse", "HEAD")
	updates := 0
	failing := func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "update-ref" {
			updates++
			if updates == 2 {
				return nil, os.ErrPermission
			}
		}
		return realWorkspaceGit(ctx, dir, args...)
	}
	out, err := ReleaseWorkspace(context.Background(), WorkspaceReleaseRequest{Registry: r.registry, GitRoot: r.repo, ID: ws.Record.ID, Git: failing,
		Census: &UseCensus{Taken: true}, By: "verb", Now: testNow})
	t.Logf("verb: %+v %v", out, err)
	os.WriteFile(filepath.Join(ws.Record.Path, "after.go"), []byte("package x\n"), 0o600)
	proof := WorkspaceProof{GitRoot: r.repo, Git: realWorkspaceGit, Now: testNow,
		Ended: func(Owner) (bool, bool, string) { return true, true, "ledger at x" }}
	critical, err := r.registry.TryCritical(ws.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v := proof.Observe(context.Background(), critical.Record()); v.Decision == Release {
		v = proof.Release(context.Background(), critical, &UseCensus{Taken: true})
		t.Logf("sweeper (dirty): %+v", v)
	}
	critical.Release()
	if _, err := os.Stat(filepath.Join(ws.Record.Path, "after.go")); err != nil {
		t.Fatalf("sweeper retry removed uncommitted work")
	}
	os.Remove(filepath.Join(ws.Record.Path, "after.go"))
	critical, _ = r.registry.TryCritical(ws.Record.ID)
	v := proof.Observe(context.Background(), critical.Record())
	if v.Decision == Release {
		v = proof.Release(context.Background(), critical, &UseCensus{Taken: true})
	}
	critical.Release()
	t.Logf("sweeper (clean): %+v", v)
	if !r.reachable(detached) {
		t.Fatalf("the detached HEAD commit %s is unreachable after the sweeper retry", detached)
	}
}
