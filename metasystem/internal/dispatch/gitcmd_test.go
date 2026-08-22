package dispatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// gitOutput runs inside the LOCKED build-record path: a hung git there
// blocks dispatch and arming checkout-wide.
// The bound must release the caller, promptly and with an error.
func TestGitOutputBoundsAHangingGit(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nsleep 600\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metasystem.conf"), []byte("exec.local-timeout-sec=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	started := time.Now()
	_, err := gitOutput(dir, "rev-parse", "HEAD")
	if err == nil {
		t.Fatal("a hung git must fail, not answer")
	}
	if elapsed := time.Since(started); elapsed > 30*time.Second {
		t.Fatalf("the bound did not release the caller: %v", elapsed)
	}
}
