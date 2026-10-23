package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
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
	enrolled := stateroottest.Installation(t, installation)
	fifo := func(name string) string {
		path := filepath.Join(t.TempDir(), name)
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	release, done, record := fifo("release"), fifo("done"), filepath.Join(t.TempDir(), "builds")
	releaseEnd := testenv.HoldFIFO(t, release)
	testenv.HoldFIFO(t, done)
	if err := os.MkdirAll(filepath.Join(installation, "cmd", "devgate"), 0o755); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(t.TempDir(), "build")
	if err := testexec.WriteFile(build, []byte("#!/usr/bin/env bash\n"+
		"printf '%s\\n' \"$$\" >>"+shellQuote(record)+"\n"+
		"read -r _ <"+shellQuote(release)+" || true\n"+
		"echo 'go-build: bin/metasystem @ fixture'\n"+
		"printf 'done\\n' >"+shellQuote(done)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// The bootstrap build is cmd/devgate's; the fixture stands in for the Go
	// toolchain through the owner's own injection point.
	owners := hookOwners{diagnostics: io.Discard, engineBuild: func() *exec.Cmd { return exec.Command(build) }}
	if err := owners.StartEngineRebuild(enrolled); err != nil {
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
	var status syscall.WaitStatus
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = syscall.Kill(-builder, syscall.SIGKILL)
			_, _ = syscall.Wait4(builder, &status, 0, nil)
		}
	})
	reap := func() (bool, string) {
		if !reaped {
			pid, err := syscall.Wait4(builder, &status, syscall.WNOHANG, nil)
			if err != nil {
				t.Fatalf("reap the builder: %v", err)
			}
			reaped = pid == builder
		}
		return !reaped, fmt.Sprintf("the build exited with status %d", status.ExitStatus())
	}
	giveUp := func() string {
		builds, _ := os.ReadFile(record)
		log, _ := os.ReadFile(filepath.Join(installation, filepath.FromSlash(hooks.BootstrapLogPath)))
		return fmt.Sprintf("builds %q; bootstrap log %q", builds, log)
	}
	if group, err := syscall.Getpgid(builder); err != nil || group != builder {
		t.Fatalf("the builder %d is not detached into its own session: group %d %v", builder, group, err)
	}
	if _, err := proofrun.TryAcquireMutation(installation); !errors.Is(err, proofrun.ErrMutationHeld) {
		t.Fatalf("the running build does not hold the proof mutation lock: %v", err)
	}
	if err := owners.StartEngineRebuild(enrolled); err != nil {
		t.Fatalf("a second start while the build runs = %v", err)
	}

	if _, err := releaseEnd.WriteString("go\n"); err != nil {
		t.Fatal(err)
	}
	if line := testenv.AwaitLine(t, "the fixture build to write done", done, reap, giveUp); line != "done" {
		t.Fatalf("done line = %q", line)
	}
	testenv.AwaitOr(t, "the fixture build to exit", func() bool { alive, _ := reap(); return !alive }, giveUp)
	if status.ExitStatus() != 0 {
		t.Fatalf("the build exited with status %d", status.ExitStatus())
	}
	// The lock is released when the build exits; waiting for it is waiting
	// for the build, never for a clock (the binary deadline bounds it).
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
	if hooks.BootstrapFenceHeld(enrolled, hookProcessAlive) {
		t.Fatal("a finished build still holds the fence")
	}
}

// An installation without cmd/devgate cannot rebuild its engine: the
// go-build.sh fallback is gone (verbs-object-action U7c), so the owner
// refuses, naming the bootstrap, instead of starting a build that cannot run.
func TestStartEngineRebuildRefusesAnInstallationWithoutDevgate(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	enrolled := stateroottest.Installation(t, installation)
	started := false
	owners := hookOwners{diagnostics: io.Discard, engineBuild: func() *exec.Cmd {
		started = true
		return exec.Command("true")
	}}
	err := owners.StartEngineRebuild(enrolled)
	if err == nil || !strings.Contains(err.Error(), "cmd/devgate") || started {
		t.Fatalf("rebuild without cmd/devgate = %v, started %t", err, started)
	}
	if hooks.BootstrapFenceHeld(enrolled, hookProcessAlive) {
		t.Fatal("a refused rebuild left the bootstrap fence held")
	}
}
