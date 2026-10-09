package lifecycle

// A repeated start whose effect already holds (R-129-ui), and the real spawn
// the start command launches the server with.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestStartOnceAnswersARunningInterfaceAtTheAskedAddressWithoutStarting(t *testing.T) {
	t.Parallel()

	for _, listen := range []string{"127.0.0.1:49152", "127.0.0.1:0"} {
		t.Run(listen, func(t *testing.T) {
			t.Parallel()

			stateRoot := t.TempDir()
			_, exact := resultTestRecord(t, stateRoot)
			started := false
			result, repeat := StartOnce(stateRoot, resultTestProber{exact: exact, state: identity.Alive}, listen, func() Result {
				started = true
				return Result{Lines: []string{"started"}}
			})

			testutil.Expect(t, "repeat", repeat, true)
			testutil.Expect(t, "start called", started, false)
			testutil.Expect(t, "result", result, Result{
				Lines: []string{"the interface already runs at http://127.0.0.1:49152 (pid 4201 since 2026-09-21T12:34:56Z)"},
				Code:  0,
			})
		})
	}
}

func TestStartOnceStartsWhenTheRunningInterfaceIsNotThisStartsEffect(t *testing.T) {
	t.Parallel()

	for _, listen := range []string{"127.0.0.1:50000", "[::1]:0", "not-an-address", ""} {
		t.Run(listen, func(t *testing.T) {
			t.Parallel()

			stateRoot := t.TempDir()
			_, exact := resultTestRecord(t, stateRoot)
			started := 0
			result, repeat := StartOnce(stateRoot, resultTestProber{exact: exact, state: identity.Alive}, listen, func() Result {
				started++
				return Result{Lines: []string{"cannot listen: the interface already runs"}, Code: 1}
			})

			testutil.Expect(t, "repeat", repeat, false)
			testutil.Expect(t, "start calls", started, 1)
			testutil.Expect(t, "result", result, Result{Lines: []string{"cannot listen: the interface already runs"}, Code: 1})
		})
	}
}

func TestStartOnceStartsWhenNoInterfaceRuns(t *testing.T) {
	t.Parallel()

	for _, state := range []State{Stopped, Stale, Unreadable} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()

			stateRoot, prober := resultStateFixture(t, state)
			started := 0
			result, repeat := StartOnce(stateRoot, prober, "127.0.0.1:0", func() Result {
				started++
				return Result{Lines: []string{"interface running at http://127.0.0.1:49153 (pid 7)"}}
			})

			testutil.Expect(t, "repeat", repeat, false)
			testutil.Expect(t, "start calls", started, 1)
			testutil.Expect(t, "result", result, Result{Lines: []string{"interface running at http://127.0.0.1:49153 (pid 7)"}})
		})
	}
}

func TestExecutableDigestIsTheSha256OfTheRunningBinary(t *testing.T) {
	t.Parallel()

	first, err := ExecutableDigest()
	testutil.Require(t, "digest", err, nil)
	second, err := ExecutableDigest()
	testutil.Require(t, "second digest", err, nil)
	if !strings.HasPrefix(first, "sha256:") || len(first) != len("sha256:")+64 {
		t.Fatalf("digest = %q, want sha256: and 64 hex digits", first)
	}
	testutil.Expect(t, "stable digest", second, first)
}

// The ready line travels on the child's descriptor 3; a child that writes it
// is released, and its output lands in the log the spec names.
func TestExecSpawnReadsTheReadyLineTheChildWrites(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	script := filepath.Join(home, "server")
	testutil.Require(t, "write server", testexec.WriteFile(script, []byte("#!/bin/sh\necho serving \"$1\"\necho \"ready 127.0.0.1:49999\" >&3\n"), 0o755), nil)
	logPath := filepath.Join(home, "server.log")

	child, err := ExecSpawn(LaunchSpec{Executable: script, Args: []string{"--listen"}, Dir: home, LogPath: logPath})
	testutil.Require(t, "spawn", err, nil)
	if child.Pid() <= 0 {
		t.Fatalf("pid = %d", child.Pid())
	}
	line, err := child.ReadyLine(time.Minute)
	testutil.Require(t, "read ready line", err, nil)
	testutil.Expect(t, "ready line", line, "ready 127.0.0.1:49999")
	testutil.Require(t, "release", child.Release(), nil)
}

func TestExecSpawnExtraFilesFollowTheReadinessDescriptor(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	payload := filepath.Join(home, "address")
	testutil.Require(t, "write address", os.WriteFile(payload, []byte("127.0.0.1:49997\n"), 0o600), nil)
	file, err := os.Open(payload)
	testutil.Require(t, "open address", err, nil)
	defer file.Close()
	script := filepath.Join(home, "server")
	testutil.Require(t, "write server", testexec.WriteFile(script, []byte("#!/bin/sh\nIFS= read -r address <&4 || exit 1\nprintf 'ready %s\\n' \"$address\" >&3\n"), 0o755), nil)
	child, err := ExecSpawn(LaunchSpec{
		Executable: script, Dir: home, LogPath: filepath.Join(home, "server.log"), ExtraFiles: []*os.File{file},
	})
	testutil.Require(t, "spawn", err, nil)
	defer child.Kill()
	line, err := child.ReadyLine(WaitForReport)
	testutil.Require(t, "read ready line", err, nil)
	testutil.Expect(t, "ready line from extra descriptor", line, "ready 127.0.0.1:49997")
	_, err = child.(*execChild).process.Wait()
	testutil.Require(t, "wait for server exit", err, nil)
}

// A child that exits without a ready line ends the read rather than waiting
// out the whole wait, and Kill of it reports the process as already done.
func TestExecSpawnChildThatClosesWithoutReadyLine(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	script := filepath.Join(home, "server")
	testutil.Require(t, "write server", testexec.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755), nil)

	child, err := ExecSpawn(LaunchSpec{Executable: script, Dir: home, LogPath: filepath.Join(home, "server.log")})
	testutil.Require(t, "spawn", err, nil)
	_, err = child.ReadyLine(time.Minute)
	if err == nil || errors.Is(err, ErrReadyTimeout) {
		t.Fatalf("ready line error = %v, want end of input", err)
	}
	_ = child.Kill()
}

// WaitForReport sets no clock on the read: it ends on the child's line or on
// the child's exit, whatever the time it takes the child to get there.
func TestReadyLineWithoutABoundEndsOnTheReportOrTheExit(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	reporter := filepath.Join(home, "reporter")
	testutil.Require(t, "write reporter", testexec.WriteFile(reporter, []byte("#!/bin/sh\necho \"ready 127.0.0.1:49998\" >&3\n"), 0o755), nil)
	child, err := ExecSpawn(LaunchSpec{Executable: reporter, Dir: home, LogPath: filepath.Join(home, "reporter.log")})
	testutil.Require(t, "spawn reporter", err, nil)
	line, err := child.ReadyLine(WaitForReport)
	testutil.Require(t, "read the report with no bound", err, nil)
	testutil.Expect(t, "ready line", line, "ready 127.0.0.1:49998")
	testutil.Require(t, "release", child.Release(), nil)

	silent := filepath.Join(home, "silent")
	testutil.Require(t, "write silent", testexec.WriteFile(silent, []byte("#!/bin/sh\nexit 0\n"), 0o755), nil)
	child, err = ExecSpawn(LaunchSpec{Executable: silent, Dir: home, LogPath: filepath.Join(home, "silent.log")})
	testutil.Require(t, "spawn silent", err, nil)
	if _, err := child.ReadyLine(WaitForReport); err == nil || errors.Is(err, ErrReadyTimeout) {
		t.Fatalf("an exit with no report = %v, want end of input", err)
	}
	_ = child.Kill()
}

func TestExecSpawnRefusesWhatItCannotOpenOrStart(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	script := filepath.Join(home, "server")
	testutil.Require(t, "write server", testexec.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755), nil)

	if _, err := ExecSpawn(LaunchSpec{Executable: script, Dir: home, LogPath: filepath.Join(home, "absent", "server.log")}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unopenable log error = %v, want not exist", err)
	}
	logPath := filepath.Join(home, "server.log")
	if _, err := ExecSpawn(LaunchSpec{Executable: filepath.Join(home, "missing"), Dir: home, LogPath: logPath}); err == nil {
		t.Fatal("spawning a missing executable succeeded")
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("the log was not opened before the start was refused: %v", err)
	}
}
