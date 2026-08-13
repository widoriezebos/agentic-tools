package contract

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// gitTry treats a nonzero exit as an answer — but a git that never returns
// is a failure, bounded like every other external call (mission-contract-3).
func TestGitTryBoundsAHangingGit(t *testing.T) {
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
	if _, code := gitTry(dir, "status"); code != -1 {
		t.Fatalf("a hung git must answer as a failure, got code %d", code)
	}
	if elapsed := time.Since(started); elapsed > 30*time.Second {
		t.Fatalf("the bound did not release the caller: %v", elapsed)
	}
}
