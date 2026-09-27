package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// SessionStart's rebuild owner returns while the build runs: the build is
// detached in its own session, named by the bootstrap fence and holding the
// proof mutation lock it was handed; a second start neither builds nor
// waits. The build is a fixture script that hands off through FIFOs, so the
// test never waits on elapsed time.
func TestStartEngineRebuildReturnsWhileTheBuildRunsDetached(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	fifo := func(name string) string {
		path := filepath.Join(t.TempDir(), name)
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	release, done, record := fifo("release"), fifo("done"), filepath.Join(t.TempDir(), "builds")
	t.Cleanup(func() {
		if file, err := os.OpenFile(release, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = file.Close()
		}
	})
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(installation, "scripts", "agents", "go-build.sh"), []byte("#!/usr/bin/env bash\n"+
		"printf '%s\\n' \"$$\" >>"+shellQuote(record)+"\n"+
		"read -r _ <"+shellQuote(release)+" || true\n"+
		"echo 'go-build: bin/metasystem @ fixture'\n"+
		"printf 'done\\n' >"+shellQuote(done)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	owners := hookOwners{diagnostics: io.Discard}
	if err := owners.StartEngineRebuild(installation); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(installation, filepath.FromSlash(hooks.BootstrapFencePath)))
	if err != nil {
		t.Fatal(err)
	}
	builder, err := strconv.Atoi(target)
	if err != nil || !hookProcessAlive(builder) {
		t.Fatalf("the fence names %q, not a live builder: %v", target, err)
	}
	if group, err := syscall.Getpgid(builder); err != nil || group != builder {
		t.Fatalf("the builder %d is not detached into its own session: group %d %v", builder, group, err)
	}
	if _, err := proofrun.TryAcquireMutation(installation); !errors.Is(err, proofrun.ErrMutationHeld) {
		t.Fatalf("the running build does not hold the proof mutation lock: %v", err)
	}
	if err := owners.StartEngineRebuild(installation); err != nil {
		t.Fatalf("a second start while the build runs = %v", err)
	}

	releaseFile, err := os.OpenFile(release, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = releaseFile.Close()
	doneFile, err := os.Open(done)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, doneFile)
	_ = doneFile.Close()
	// The lock is released when the build exits; waiting for it is waiting
	// for the build, never for a clock.
	lock, err := proofrun.AcquireMutation(installation)
	if err != nil {
		t.Fatal(err)
	}
	_ = lock.Release()
	builds, _ := os.ReadFile(record)
	log, _ := os.ReadFile(filepath.Join(installation, filepath.FromSlash(hooks.BootstrapLogPath)))
	if string(builds) != target+"\n" || !strings.Contains(string(log), "go-build: bin/metasystem @ fixture") {
		t.Fatalf("builds %q (fence %s) log %q", builds, target, log)
	}
	// The hook process exits right after starting the build, so init reaps
	// the builder; this test process outlives it and reaps it itself.
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(builder, &status, 0, nil); err != nil || status.ExitStatus() != 0 {
		t.Fatalf("reap the builder: %v status %d", err, status.ExitStatus())
	}
	if hooks.BootstrapFenceHeld(installation, hookProcessAlive) {
		t.Fatal("a finished build still holds the fence")
	}
}
