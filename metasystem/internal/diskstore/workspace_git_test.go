package diskstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// realWorkspaceGit runs the real git: the claim of the test below is git's
// own linked-worktree layout, which no stub can prove.
func realWorkspaceGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, &gitError{args: args, err: err, stderr: stderr.String()}
	}
	return stdout.Bytes(), nil
}

type gitError struct {
	args   []string
	err    error
	stderr string
}

func (e *gitError) Error() string {
	return strings.Join(e.args, " ") + ": " + e.err.Error() + ": " + e.stderr
}

// TestCopyWorkspaceRealGitIsALinkedWorktreeWhoseArchiveKeepsItsCommit is
// the real-git adapter claim of 3.6 and 3.7: a copy is a linked worktree of
// the checkout's common store with no private object store and no
// alternates line; a commit made in it and never landed survives its
// release, reachable from the archive ref, while the worktree and the
// branch are gone.
func TestCopyWorkspaceRealGitIsALinkedWorktreeWhoseArchiveKeepsItsCommit(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	ctx := context.Background()
	root := realDir(t)
	repo := filepath.Join(root, "repo")
	must := func(dir string, args ...string) string {
		t.Helper()
		out, err := realWorkspaceGit(ctx, dir, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	must(repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("metasystem/artifacts/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	must(repo, "add", ".gitignore")
	must(repo, "commit", "-q", "-m", "base")
	base := must(repo, "rev-parse", "HEAD")
	control := filepath.Join(repo, "metasystem")
	registry := CheckoutRegistry(control)
	workspace, err := ObtainWorkspace(ctx, WorkspaceRequest{Registry: registry, Control: control, GitRoot: repo, Owner: goalG, Name: "bed",
		CopyOf: base, Now: testNow, Entropy: rand.Reader, Git: realWorkspaceGit})
	if err != nil {
		t.Fatal(err)
	}
	gitdir := workspace.Record.Identity.Gitdir
	if !strings.HasPrefix(gitdir, filepath.Join(repo, ".git", "worktrees")) {
		t.Fatalf("the copy is a linked worktree of the common store: gitdir %s", gitdir)
	}
	if _, err := os.Stat(filepath.Join(gitdir, "objects")); !os.IsNotExist(err) {
		t.Fatalf("the copy has no private object store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		t.Fatalf("no alternates line names the copy: %v", err)
	}
	if status := must(repo, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
		t.Fatalf("the workspace leaves the checkout clean: %q", status)
	}
	if err := os.WriteFile(filepath.Join(workspace.Record.Path, "work.txt"), []byte("unique\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	must(workspace.Record.Path, "add", "work.txt")
	must(workspace.Record.Path, "commit", "-q", "-m", "unique work")
	unique := must(workspace.Record.Path, "rev-parse", "HEAD")
	outcome, err := ReleaseWorkspace(ctx, WorkspaceReleaseRequest{Registry: registry, GitRoot: repo, ID: workspace.Record.ID, Git: realWorkspaceGit,
		Census: &UseCensus{Taken: true}, By: "test", Now: testNow})
	if err != nil || !outcome.Done || outcome.Unique != 1 {
		t.Fatalf("release: %+v %v", outcome, err)
	}
	if archived := must(repo, "rev-parse", outcome.Archive); archived != unique {
		t.Fatalf("the archive ref holds the unique commit: %s != %s", archived, unique)
	}
	if _, err := os.Stat(workspace.Record.Path); !os.IsNotExist(err) {
		t.Fatalf("the worktree is gone: %v", err)
	}
	if branches := must(repo, "branch", "--list", "workspace/*"); branches != "" {
		t.Fatalf("the branch is gone: %q", branches)
	}
	if list := must(repo, "worktree", "list", "--porcelain"); strings.Contains(list, workspace.Record.Path) {
		t.Fatalf("git no longer lists the worktree: %s", list)
	}
}

// Unwrap exposes the process's exit status, as the production runner does.
func (e *gitError) Unwrap() error { return e.err }

// gitNotFound is a fake git's exit code 1, what rev-parse --verify -q
// answers for a ref that does not exist.
type gitNotFound struct{}

func (gitNotFound) Error() string { return "exit status 1" }
func (gitNotFound) ExitCode() int { return 1 }
