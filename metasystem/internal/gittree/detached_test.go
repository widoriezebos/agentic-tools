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
