package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
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

	expiry := make(chan time.Time, 1)
	expiry <- time.Time{}
	var waits []time.Duration
	_, err := gitOutputWithDeadline(dir, func(wait time.Duration) <-chan time.Time {
		waits = append(waits, wait)
		return expiry
	}, "rev-parse", "HEAD")
	if !errors.Is(err, boundedexec.ErrTimedOut) {
		t.Fatalf("a hung git must fail with ErrTimedOut: %v", err)
	}
	if !strings.Contains(err.Error(), "git rev-parse HEAD") {
		t.Fatalf("the failure does not name the git operation: %v", err)
	}
	if len(waits) != 2 || waits[0] != time.Second {
		t.Fatalf("deadline waits = %v, want two waits beginning with %s", waits, time.Second)
	}
}
