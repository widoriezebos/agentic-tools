package diskstore

// Witnesses of the fifth B3 read (the reader's probes; each failed on
// 416b57a61).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func r4Release(r *realRepo, id string, git WorkspaceGit, by string) WorkspaceRelease {
	out, err := ReleaseWorkspace(context.Background(), WorkspaceReleaseRequest{Registry: r.registry, GitRoot: r.repo, ID: id, Git: git,
		Census: &UseCensus{Taken: true}, By: by, Now: testNow})
	if err != nil {
		r.t.Fatalf("release: %v", err)
	}
	return out
}

// R5-1: the rev-parse classifier. Round B3-4: "rev-parse 'not found' (exit
// 1, nothing printed) is told apart from every other failure". The
// production runner (steward.ExecWorkspaceGit) returns nil output and an
// error "git ...: exit status 128: fatal: ..."; the classifier's
// strings.Contains(err, "exit status 1") also matches "exit status 128".
func TestB3Read5RevParseExit128ReadAsNotFound(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"128", "129"} {
		git := func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			return nil, errors.New("git " + strings.Join(args, " ") + ": exit status " + code + ": fatal: not a git repository")
		}
		_, found, err := revParseIn(context.Background(), git, "/nowhere", "HEAD")
		if err == nil {
			t.Fatalf("exit %s classified as not found (found=%v err=nil); the rule says only exit 1 is", code, found)
		}
	}
}

// R5-1b: the consequence when it fires. rev-parse HEAD in the worktree
// fails with exit 128 (injected; every other git call is real): the
// detached HEAD commit, whose reflog is expired, is never archived and the
// copy is released.
func TestB3Read5RevParseExit128HeadNotArchived(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("rp", r.base)
	r.git(ws.Record.Path, "checkout", "-q", "--detach")
	kept := r4Commit(r, ws.Record.Path, "detached work")
	r.git(ws.Record.Path, "reflog", "expire", "--expire=now", "--all")
	git := func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if dir == ws.Record.Path && len(args) > 0 && args[0] == "rev-parse" && args[len(args)-1] == "HEAD" {
			return nil, errors.New("git rev-parse --verify -q HEAD: exit status 128: fatal: injected")
		}
		return realWorkspaceGit(ctx, dir, args...)
	}
	out := r4Release(r, ws.Record.ID, git, "t")
	t.Logf("outcome %+v", out)
	if out.Done && !r.reachable(kept) {
		t.Fatalf("a rev-parse failure (exit 128) read as 'no HEAD'; release went ahead and %s is held by no ref", kept)
	}
}

// R5-2: a person's discard passes the reflog readability rule too (copyHidden
// runs only without a discard). The worktree HEAD reflog and branch reflog
// are unreadable; a commit reachable only from them is lost although the
// discard names only "uncommitted content".
func TestB3Read5DiscardPassesUnreadableReflog(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("dr", r.base)
	lost := r4Commit(r, ws.Record.Path, "committed work later reset away")
	r.git(ws.Record.Path, "reset", "-q", "--hard", "HEAD~1")
	gitdir := strings.TrimSpace(r.git(ws.Record.Path, "rev-parse", "--git-dir"))
	common := strings.TrimSpace(r.git(r.repo, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(common) {
		common = filepath.Join(r.repo, common)
	}
	branchLog := filepath.Join(common, "logs", "refs", "heads", filepath.FromSlash(WorkspaceBranch(goalG, "dr")))
	for _, p := range []string{filepath.Join(gitdir, "logs", "HEAD"), branchLog} {
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(p, 0o600)
	}
	out, err := ReleaseWorkspace(context.Background(), WorkspaceReleaseRequest{Registry: r.registry, GitRoot: r.repo, ID: ws.Record.ID, Git: realWorkspaceGit,
		Census: &UseCensus{Taken: true}, By: "person", Now: testNow, Discard: &Discard{By: "person", At: testNow, Reason: "scratch"}})
	t.Logf("outcome %+v err %v", out, err)
	if out.Done && !r.reachable(lost) {
		t.Fatalf("discard of uncommitted content also dropped committed work %s behind an unreadable reflog", lost)
	}
}

// R5-4: a submodule's git dir left in the worktree's gitdir after git rm
// (no .gitmodules entry, no gitlink): does git or the release keep it?
func TestB3Read5RemovedSubmoduleGitDirInWorktreeGitdir(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	sub := filepath.Join(filepath.Dir(r.repo), "subsrc5")
	os.MkdirAll(sub, 0o700)
	r.git(sub, "init", "-q", "-b", "main")
	r.git(sub, "commit", "-q", "--allow-empty", "-m", "s")
	ws := r.obtain("sm", r.base)
	r.git(ws.Record.Path, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "dep")
	r.git(ws.Record.Path, "commit", "-q", "-m", "add dep")
	unpushed := r4Commit(r, filepath.Join(ws.Record.Path, "dep"), "unpushed submodule work")
	r.git(ws.Record.Path, "rm", "-q", "-f", "dep")
	os.Remove(filepath.Join(ws.Record.Path, ".gitmodules"))
	r.git(ws.Record.Path, "add", "-A")
	r.git(ws.Record.Path, "commit", "-q", "-m", "drop dep")
	gitdir := strings.TrimSpace(r.git(ws.Record.Path, "rev-parse", "--git-dir"))
	modules := filepath.Join(gitdir, "modules", "dep")
	_, before := os.Stat(modules)
	out := r4Release(r, ws.Record.ID, realWorkspaceGit, "t")
	_, after := os.Stat(modules)
	t.Logf("modules before=%v after=%v outcome %+v", before, after, out)
	if before == nil && after != nil && out.Done {
		t.Fatalf("the removed submodule's git dir (commit %s) went with the worktree", unpushed)
	}
}

// R5-3, made a witness of F-5: a read-only directory in the copy is found
// by the pre-removal walk, which keeps the copy; no removal is started
// that stops halfway.
func TestB3Read5ReadOnlyDirectoryIsKeptBeforeRemoval(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("ro", r.base)
	dir := filepath.Join(ws.Record.Path, "vendor")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600)
	r.git(ws.Record.Path, "add", "vendor/a.txt")
	r.git(ws.Record.Path, "commit", "-q", "-m", "vendor")
	os.Chmod(dir, 0o500)
	defer os.Chmod(dir, 0o700)
	out := r4Release(r, ws.Record.ID, realWorkspaceGit, "t")
	if !out.Kept || strings.Contains(out.Reason, "removal stopped") {
		t.Fatalf("a read-only directory: %+v", out)
	}
	if _, err := os.Lstat(filepath.Join(ws.Record.Path, ".git")); err != nil {
		t.Fatalf("the removal started: %v", err)
	}
}
