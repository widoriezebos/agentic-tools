package contract

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// gitTry treats a nonzero exit as an answer — but a git that never returns
// is a failure, bounded like every other external call (mission-contract-3).
func TestGitTryBoundsAHangingGit(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nread held\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metasystem.conf"), []byte("exec.local-timeout-sec=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	reader, holder, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = holder.Close()
	})
	var gotBound boundedexec.Bound
	var waits []time.Duration
	var runErr error
	run := func(cmd *exec.Cmd, bound boundedexec.Bound, what string) error {
		gotBound = bound
		cmd.Stdin = reader
		expired := make(chan time.Time, 1)
		expired <- time.Time{}
		runErr = boundedexec.RunWithDeadline(cmd, bound, what, func(wait time.Duration) <-chan time.Time {
			waits = append(waits, wait)
			return expired
		})
		return runErr
	}
	output, code := gitTryWithRunner(dir, run, "status")
	if gotBound.Limit != time.Second || gotBound.Key != "exec.local-timeout-sec" {
		t.Fatalf("gitTry bound = %+v", gotBound)
	}
	if !errors.Is(runErr, boundedexec.ErrTimedOut) || !strings.Contains(runErr.Error(), "git status") {
		t.Fatalf("bounded owner result = %v", runErr)
	}
	if output != "" || code != -1 {
		t.Fatalf("a timed-out git must remain an empty failure answer, output=%q code=%d", output, code)
	}
	if len(waits) != 2 || waits[0] != time.Second || waits[1] != 5*time.Second {
		t.Fatalf("bounded owner waits = %v", waits)
	}
}
