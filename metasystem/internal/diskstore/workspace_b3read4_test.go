package diskstore

// Witnesses of the fourth B3 read (the reader's probes; each failed on
// 5d7057808).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func r4Commit(r *realRepo, dir, msg string) string {
	r.git(dir, "commit", "-q", "--allow-empty", "-m", msg)
	return r.git(dir, "rev-parse", "HEAD")
}

// N1: a tracked file marked skip-worktree (or assume-unchanged), then
// edited: git status prints nothing, so the copy is "clean" and
// `git worktree remove --force` deletes the edit.
func TestB3Read4SkipWorktreeEditLost(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--skip-worktree", "--assume-unchanged"} {
		t.Run(flag, func(t *testing.T) {
			r := newRealRepo(t)
			ws := r.obtain("sw", r.base)
			r.git(ws.Record.Path, "update-index", flag, ".gitignore")
			edited := "metasystem/artifacts/\n*.env\n# three hours of edits\n"
			os.WriteFile(filepath.Join(ws.Record.Path, ".gitignore"), []byte(edited), 0o600)
			status := r.git(ws.Record.Path, "status", "--porcelain=v1", "-z", "--ignored=matching", "--untracked-files=all")
			out := r.release(ws.Record.ID)
			t.Logf("status %q outcome %+v", status, out)
			if data, err := os.ReadFile(filepath.Join(ws.Record.Path, ".gitignore")); err != nil || string(data) != edited {
				t.Fatalf("a %s edit (git status printed %q) was removed with the workspace: %+v", flag, status, out)
			}
		})
	}
}

// N2: a submodule inside a copy: ignored content inside the submodule is
// not listed by the superproject's status --ignored.
func TestB3Read4SubmoduleIgnoredContentLost(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	sub := filepath.Join(filepath.Dir(r.repo), "subsrc")
	os.MkdirAll(sub, 0o700)
	r.git(sub, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("*.cache\n"), 0o600)
	r.git(sub, "add", ".gitignore")
	r.git(sub, "commit", "-q", "-m", "sub base")
	r.git(r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "sub")
	r.git(r.repo, "commit", "-q", "-m", "add sub")
	head := r.git(r.repo, "rev-parse", "HEAD")
	ws := r.obtain("sm", head)
	r.git(ws.Record.Path, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init")
	precious := filepath.Join(ws.Record.Path, "sub", "results.cache")
	os.WriteFile(precious, []byte("hours of computed results"), 0o600)
	status := r.git(ws.Record.Path, "status", "--porcelain=v1", "-z", "--ignored=matching", "--untracked-files=all")
	out := r.release(ws.Record.ID)
	t.Logf("status %q outcome %+v", status, out)
	if _, err := os.Stat(precious); err != nil {
		t.Fatalf("ignored content inside a submodule of the copy removed (superproject status %q): %+v", status, out)
	}
}

// N3: a submodule commit that exists only in the copy's submodule, hidden
// by a committed `ignore = all` in .gitmodules: status is empty and the
// submodule's own git directory (inside the worktree's admin dir) goes.
func TestB3Read4SubmoduleCommitHiddenByIgnoreAllLost(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	sub := filepath.Join(filepath.Dir(r.repo), "subsrc")
	os.MkdirAll(sub, 0o700)
	r.git(sub, "init", "-q", "-b", "main")
	r.git(sub, "commit", "-q", "--allow-empty", "-m", "sub base")
	r.git(r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "sub")
	r.git(r.repo, "config", "-f", ".gitmodules", "submodule.sub.ignore", "all")
	r.git(r.repo, "add", ".gitmodules")
	r.git(r.repo, "commit", "-q", "-m", "add sub, ignore all")
	head := r.git(r.repo, "rev-parse", "HEAD")
	ws := r.obtain("sm2", head)
	r.git(ws.Record.Path, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init")
	subdir := filepath.Join(ws.Record.Path, "sub")
	r.git(subdir, "checkout", "-q", "-b", "work")
	unique := r4Commit(r, subdir, "submodule work only here")
	os.WriteFile(filepath.Join(subdir, "wip.go"), []byte("package x\n"), 0o600)
	status := r.git(ws.Record.Path, "status", "--porcelain=v1", "-z", "--ignored=matching", "--untracked-files=all")
	out := r.release(ws.Record.ID)
	t.Logf("status %q outcome %+v", status, out)
	_, err := realWorkspaceGit(context.Background(), sub, "cat-file", "-e", unique+"^{commit}")
	if _, statErr := os.Stat(filepath.Join(subdir, "wip.go")); statErr != nil && err != nil {
		t.Fatalf("submodule commit %s and untracked file removed; superproject status was %q: %+v", unique, status, out)
	}
}

// N5: a commit reachable only from the reflogs (agent ran reset --hard
// HEAD~1), the worktree HEAD reflog unreadable: the reflog read error is
// ignored and the release proceeds; the commit is then held by nothing.
func TestB3Read4UnreadableReflogCommitLost(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("rl", r.base)
	lost := r4Commit(r, ws.Record.Path, "work later discarded by reset")
	r.git(ws.Record.Path, "reset", "-q", "--hard", "HEAD~1")
	gitdir := strings.TrimSpace(r.git(ws.Record.Path, "rev-parse", "--git-dir"))
	common := strings.TrimSpace(r.git(r.repo, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(common) {
		common = filepath.Join(r.repo, common)
	}
	branchLog := filepath.Join(common, "logs", "refs", "heads", filepath.FromSlash(WorkspaceBranch(goalG, "rl")))
	for _, p := range []string{filepath.Join(gitdir, "logs", "HEAD"), branchLog} {
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatalf("chmod %s: %v", p, err)
		}
	}
	_, logErr := realWorkspaceGit(context.Background(), ws.Record.Path, "log", "-g", "--format=%H", "HEAD")
	out := r.release(ws.Record.ID)
	t.Logf("log -g err: %v; outcome %+v", logErr, out)
	if out.Done && !r.reachable(lost) {
		t.Fatalf("reflog unreadable (%v), release went ahead and commit %s is held by no ref", logErr, lost)
	}
}

// N7: a per-worktree ref (refs/worktree/*) holds a commit nothing else
// holds; worktree removal deletes the ref.
func TestB3Read4PerWorktreeRefCommitLost(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	ws := r.obtain("pw", r.base)
	kept := r4Commit(r, ws.Record.Path, "bisect/experiment kept under refs/worktree")
	r.git(ws.Record.Path, "update-ref", "refs/worktree/experiment", kept)
	r.git(ws.Record.Path, "reset", "-q", "--hard", r.base)
	r.git(ws.Record.Path, "reflog", "expire", "--expire=now", "--all")
	out := r.release(ws.Record.ID)
	t.Logf("outcome %+v", out)
	if out.Done && !r.reachable(kept) {
		t.Fatalf("commit %s held only by refs/worktree/experiment is held by nothing after release", kept)
	}
}
