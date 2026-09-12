package gittree

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestDetachedWorktreeHeadArchivesCandidate(t *testing.T) {
	repository := t.TempDir()
	runDetachedGit(t, repository, "init", "-q", "-b", "main")
	runDetachedGit(t, repository, "config", "user.name", "fixture")
	runDetachedGit(t, repository, "config", "user.email", "fixture@example.invalid")
	writeDetachedFile(t, filepath.Join(repository, "metasystem", "value.txt"), "base\n")
	runDetachedGit(t, repository, "add", ".")
	runDetachedGit(t, repository, "commit", "-qm", "base")

	writeDetachedFile(t, filepath.Join(repository, "metasystem", "value.txt"), "candidate\n")
	runDetachedGit(t, repository, "add", ".")
	candidateTop := runDetachedGit(t, repository, "write-tree")
	candidate := runDetachedGit(t, repository, "rev-parse", candidateTop+":metasystem")
	detached, err := (Workspace{Dir: filepath.Join(repository, "metasystem")}).NewDetachedWorktree(candidate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := detached.Close(); err != nil {
			t.Error(err)
		}
	})

	if got, err := detached.Workspace().HeadTree(); err != nil || got != candidate {
		t.Fatalf("detached HEAD tree = %q, %v; want candidate %q", got, err, candidate)
	}
	assertDetachedArchiveValue(t, detached, "candidate\n")
}

func TestDetachedWorktreeWorkspaceMatchesPhysicalCWD(t *testing.T) {
	repository := t.TempDir()
	runDetachedGit(t, repository, "init", "-q", "-b", "main")
	runDetachedGit(t, repository, "config", "user.name", "fixture")
	runDetachedGit(t, repository, "config", "user.email", "fixture@example.invalid")
	writeDetachedFile(t, filepath.Join(repository, "metasystem", "value.txt"), "base\n")
	runDetachedGit(t, repository, "add", ".")
	runDetachedGit(t, repository, "commit", "-qm", "base")

	writeDetachedFile(t, filepath.Join(repository, "metasystem", "value.txt"), "candidate\n")
	runDetachedGit(t, repository, "add", ".")
	candidateTop := runDetachedGit(t, repository, "write-tree")
	candidate := runDetachedGit(t, repository, "rev-parse", candidateTop+":metasystem")

	temporaryRoot := t.TempDir()
	realTemporaryRoot := filepath.Join(temporaryRoot, "real")
	if err := os.Mkdir(realTemporaryRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkTemporaryRoot := filepath.Join(temporaryRoot, "symlink")
	if err := os.Symlink(realTemporaryRoot, symlinkTemporaryRoot); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", symlinkTemporaryRoot)

	detached, err := (Workspace{Dir: filepath.Join(repository, "metasystem")}).NewDetachedWorktree(candidate)
	if err != nil {
		t.Fatal(err)
	}
	cleanupPending := true
	t.Cleanup(func() {
		if cleanupPending {
			if err := detached.Close(); err != nil {
				t.Error(err)
			}
		}
	})

	workspace := detached.Workspace()
	canonicalRoot, err := filepath.EvalSymlinks(workspace.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Dir != canonicalRoot {
		t.Fatalf("workspace root = %q; want canonical path %q", workspace.Dir, canonicalRoot)
	}
	physicalCWD := exec.Command("pwd", "-P")
	physicalCWD.Dir = workspace.Dir
	physicalCWD.Env = ScrubbedEnviron()
	output, err := physicalCWD.CombinedOutput()
	if err != nil {
		t.Fatalf("read physical working directory: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != workspace.Dir {
		t.Fatalf("physical working directory = %q; want workspace root %q", got, workspace.Dir)
	}
	if got, err := workspace.HeadTree(); err != nil || got != candidate {
		t.Fatalf("detached HEAD tree = %q, %v; want candidate %q", got, err, candidate)
	}
	assertDetachedArchiveValue(t, detached, "candidate\n")

	parent := detached.parent
	if err := detached.Close(); err != nil {
		t.Fatal(err)
	}
	cleanupPending = false
	if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary worktree directory still exists after close: %v", err)
	}
}

func assertDetachedArchiveValue(t *testing.T, detached *DetachedWorktree, want string) {
	t.Helper()
	archive := exec.Command("git", "-C", detached.top, "archive", "HEAD:metasystem")
	archive.Env = ScrubbedEnviron()
	data, err := archive.Output()
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(bytes.NewReader(data))
	for {
		header, readErr := reader.Next()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		if header.Name != "value.txt" {
			continue
		}
		content, readErr := io.ReadAll(reader)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(content) != want {
			t.Fatalf("archive contains %q; want %q", content, want)
		}
		return
	}
	t.Fatal("candidate file missing from detached HEAD archive")
}

func writeDetachedFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runDetachedGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestDetachedWorktreesOpenConcurrentlyOnOneRepository(t *testing.T) {
	// Two proof groups of one receipt open their detached worktrees at the
	// same moment against the same repository. When every worktree carried
	// the basename "worktree", git named both admin entries alike and the
	// second add read the first's half-written commondir ("Undefined error:
	// 0", cadence run 18, 2026-09-12). Unique basenames keep the adds apart.
	repository := t.TempDir()
	runDetachedGit(t, repository, "init", "-q", "-b", "main")
	runDetachedGit(t, repository, "config", "user.name", "fixture")
	runDetachedGit(t, repository, "config", "user.email", "fixture@example.invalid")
	writeDetachedFile(t, filepath.Join(repository, "metasystem", "value.txt"), "base\n")
	runDetachedGit(t, repository, "add", ".")
	runDetachedGit(t, repository, "commit", "-qm", "base")
	writeDetachedFile(t, filepath.Join(repository, "metasystem", "value.txt"), "candidate\n")
	runDetachedGit(t, repository, "add", ".")
	candidate := runDetachedGit(t, repository, "rev-parse", runDetachedGit(t, repository, "write-tree")+":metasystem")

	const openers = 6
	results := make(chan error, openers)
	opened := make(chan *DetachedWorktree, openers)
	for i := 0; i < openers; i++ {
		go func() {
			detached, err := (Workspace{Dir: filepath.Join(repository, "metasystem")}).NewDetachedWorktree(candidate)
			if err == nil {
				opened <- detached
			}
			results <- err
		}()
	}
	for i := 0; i < openers; i++ {
		if err := <-results; err != nil {
			t.Errorf("concurrent detached worktree: %v", err)
		}
	}
	close(opened)
	tops := map[string]bool{}
	for detached := range opened {
		base := filepath.Base(detached.top)
		if tops[base] {
			t.Errorf("two detached worktrees share the basename %q", base)
		}
		tops[base] = true
		if got, err := detached.Workspace().HeadTree(); err != nil || got != candidate {
			t.Errorf("detached HEAD tree = %q, %v; want %q", got, err, candidate)
		}
		if err := detached.Close(); err != nil {
			t.Error(err)
		}
	}
}

func TestWorktreeAdministrationIsOneLockPerRepository(t *testing.T) {
	// The lock lives in the common git dir: the main checkout and a linked
	// worktree of it contend for the same one, and a non-blocking attempt
	// while it is held is refused instead of racing git's half-written
	// admin entry.
	repository := t.TempDir()
	runDetachedGit(t, repository, "init", "-q", "-b", "main")
	runDetachedGit(t, repository, "config", "user.name", "fixture")
	runDetachedGit(t, repository, "config", "user.email", "fixture@example.invalid")
	writeDetachedFile(t, filepath.Join(repository, "metasystem", "value.txt"), "base\n")
	runDetachedGit(t, repository, "add", ".")
	runDetachedGit(t, repository, "commit", "-qm", "base")
	linked := filepath.Join(t.TempDir(), "linked")
	runDetachedGit(t, repository, "worktree", "add", "-q", "--detach", linked, "HEAD")

	held, err := (Workspace{Dir: filepath.Join(repository, "metasystem")}).lockWorktreeAdmin(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Workspace{Dir: linked}).lockWorktreeAdmin(false); !errors.Is(err, unix.EWOULDBLOCK) {
		t.Fatalf("the linked worktree took the worktree administration while the main checkout held it: %v", err)
	}
	if _, err := (Workspace{Dir: repository}).lockWorktreeAdmin(false); !errors.Is(err, unix.EWOULDBLOCK) {
		t.Fatalf("a second engine on the main checkout took the held worktree administration: %v", err)
	}
	if err := held.release(); err != nil {
		t.Fatal(err)
	}
	second, err := (Workspace{Dir: linked}).lockWorktreeAdmin(false)
	if err != nil {
		t.Fatalf("the released worktree administration was not free: %v", err)
	}
	if err := second.release(); err != nil {
		t.Fatal(err)
	}
	if err := held.release(); err != nil {
		t.Fatalf("releasing twice is not idempotent: %v", err)
	}
}
