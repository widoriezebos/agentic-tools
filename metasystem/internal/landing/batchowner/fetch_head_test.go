package batchowner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// TestLandingBaseTreeReadsItsOwnFetchNotTheSharedFetchHead runs the lane's
// base fetch under a git that empties FETCH_HEAD after every fetch, the way
// another session's fetch in the same checkout rewrites that one shared file
// between this fetch and its read.
func TestLandingBaseTreeReadsItsOwnFetchNotTheSharedFetchHead(t *testing.T) {
	scratch := t.TempDir()
	origin := filepath.Join(scratch, "origin.git")
	unsetGit(t, scratch, "init", "-q", "--bare", "-b", "main", origin)
	publisher := filepath.Join(scratch, "publisher")
	unsetGit(t, scratch, "clone", "-q", origin, publisher)
	unsetGit(t, publisher, "config", "user.name", "Fetch Fixture")
	unsetGit(t, publisher, "config", "user.email", "fetch@example.invalid")
	if err := os.WriteFile(filepath.Join(publisher, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unsetGit(t, publisher, "add", "base.txt")
	unsetGit(t, publisher, "commit", "-qm", "base")
	unsetGit(t, publisher, "push", "-q", "origin", "main")
	want := unsetGit(t, publisher, "rev-parse", "HEAD^{tree}")
	checkout := filepath.Join(scratch, "checkout")
	unsetGit(t, scratch, "clone", "-q", origin, checkout)

	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\n" + strconv.Quote(real) + " \"$@\"\nstatus=$?\ncase \" $* \" in *\" fetch \"*) : > " +
		strconv.Quote(filepath.Join(checkout, ".git", "FETCH_HEAD")) + " ;; esac\nexit $status\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := fetchLandingBaseTree(checkout)
	if err != nil {
		t.Fatalf("base fetch read the shared FETCH_HEAD: %v", err)
	}
	if got != want {
		t.Fatalf("base tree=%s, want %s", got, want)
	}
	if refs := unsetGit(t, checkout, "for-each-ref", "refs/metasystem/op/"); refs != "" {
		t.Fatalf("base fetch left its private ref: %s", refs)
	}
}
