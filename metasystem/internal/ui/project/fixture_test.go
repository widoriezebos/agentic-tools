package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// readAt is the one instant every test reads at, so "read at" is a fact of the
// test rather than of the clock.
var readAt = time.Date(2026, 9, 21, 10, 11, 12, 0, time.UTC)

// The fixtures are two initialised Git checkouts under t.TempDir(): the
// template's self-hosted layout, whose installation is the nested metasystem
// directory beside development/metasystem-design.md, and an adopted
// installation at its application's root. The layout owner reads both of them
// the way it reads a real one, so nothing here is stubbed.
//
// Helpers fail with t.Fatalf rather than through testutil, whose labels must
// be unique within one test and which a helper called twice would repeat.

func selfHostedFixture(t *testing.T) Roots {
	t.Helper()
	checkout := gitRepository(t)
	plant(t, checkout, "development/metasystem-design.md", "# The design\n")
	installation := filepath.Join(checkout, "metasystem")
	plant(t, installation, "metasystem.conf", "")
	makeDirectory(t, filepath.Join(installation, "scripts", "agents"))
	return Roots{Checkout: checkout, Installation: installation, StateRoot: installation}
}

func adoptedFixture(t *testing.T) Roots {
	t.Helper()
	checkout := gitRepository(t)
	plant(t, checkout, "metasystem.conf", "")
	makeDirectory(t, filepath.Join(checkout, "scripts", "agents"))
	return Roots{Checkout: checkout, Installation: checkout, StateRoot: checkout}
}

func gitRepository(t *testing.T) string {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "application")
	if output, err := exec.Command("git", "init", "-q", "-b", "main", repository).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	canonical, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatalf("canonical repository: %v", err)
	}
	return canonical
}

// plant writes one file beneath a root, creating the directories above it, and
// answers its absolute path.
func plant(t *testing.T, root, relative, content string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	makeDirectory(t, filepath.Dir(full))
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("plant %s: %v", relative, err)
	}
	return full
}

func plantBytes(t *testing.T, root, relative string, content []byte) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	makeDirectory(t, filepath.Dir(full))
	if err := os.WriteFile(full, content, 0o644); err != nil {
		t.Fatalf("plant %s: %v", relative, err)
	}
	return full
}

func makeDirectory(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("make %s: %v", path, err)
	}
}

func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Fatalf("link %s -> %s: %v", name, target, err)
	}
}

func hardLink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Link(target, name); err != nil {
		t.Fatalf("hard link %s -> %s: %v", name, target, err)
	}
}

func rename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("rename %s -> %s: %v", from, to, err)
	}
}

// outside is a directory that is not beneath the checkout, which is where a
// test puts what the boundary must not serve.
func outside(t *testing.T) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "outside")
	makeDirectory(t, directory)
	return directory
}
