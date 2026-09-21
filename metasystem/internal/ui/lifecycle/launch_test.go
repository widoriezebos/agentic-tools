package lifecycle

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestServeArgs(t *testing.T) {
	t.Parallel()

	observed := ServeArgs("/work/repository", "/opt/metasystem", "127.0.0.1:7878")
	expected := []string{
		"ui", "serve",
		"--repo", "/work/repository",
		"--metasystem-root", "/opt/metasystem",
		"--listen", "127.0.0.1:7878",
		"--ready-fd", "3",
	}
	testutil.Expect(t, "serve arguments", observed, expected)
}

func TestLaunchReadinessOutcomes(t *testing.T) {
	t.Parallel()

	const notReady = "not ready"
	tests := []struct {
		name         string
		line         string
		readyErr     error
		wantAddress  string
		wantPID      int
		wantError    string
		wantKilled   bool
		wantReleased bool
	}{
		{
			name:         "ready",
			line:         "ready 127.0.0.1:49152",
			wantAddress:  "127.0.0.1:49152",
			wantPID:      4201,
			wantReleased: true,
		},
		{
			name:      "failed",
			line:      "failed listen on --listen or ui.listen: address already in use",
			wantError: "listen on --listen or ui.listen: address already in use",
		},
		{
			name:       "timeout",
			readyErr:   ErrReadyTimeout,
			wantError:  notReady,
			wantKilled: true,
		},
		{
			name:       "end of file",
			readyErr:   io.EOF,
			wantError:  notReady,
			wantKilled: true,
		},
		{
			name:       "partial line",
			line:       "ready 127.0.0.1:49152",
			readyErr:   io.EOF,
			wantError:  notReady,
			wantKilled: true,
		},
		{
			name:       "unknown line",
			line:       "starting 127.0.0.1:49152",
			wantError:  notReady,
			wantKilled: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			child := &fakeLaunchChild{line: test.line, readyErr: test.readyErr, pid: 4201}
			spec := launchTestSpec(t)
			address, pid, err := Launch(spec, func(LaunchSpec) (Child, error) {
				return child, nil
			}, 3*time.Second)

			testutil.Expect(t, "address", address, test.wantAddress)
			testutil.Expect(t, "pid", pid, test.wantPID)
			testutil.Expect(t, "readiness wait", child.wait, 3*time.Second)
			testutil.Expect(t, "child killed", child.killed, test.wantKilled)
			testutil.Expect(t, "child released", child.released, test.wantReleased)
			if test.wantError == "" {
				testutil.Expect(t, "launch error", err, nil)
				return
			}
			testutil.Require(t, "launch returned error", err != nil, true)
			expectedError := test.wantError
			if expectedError == notReady {
				expectedError = "the interface server did not become ready; see " + spec.LogPath
			}
			testutil.Expect(t, "launch error", err.Error(), expectedError)
		})
	}
}

func TestLaunchUsesDefaultReadinessWait(t *testing.T) {
	t.Parallel()

	child := &fakeLaunchChild{line: "ready 127.0.0.1:49152", pid: 4201}
	_, _, err := Launch(launchTestSpec(t), func(LaunchSpec) (Child, error) {
		return child, nil
	}, 0)

	testutil.Require(t, "launch error", err, nil)
	testutil.Expect(t, "default readiness wait", child.wait, 10*time.Second)
}

func TestO1LaunchCreatesStateDirectoryBeforeSpawn(t *testing.T) {
	t.Parallel()

	spec := launchTestSpec(t)
	spawnErr := errors.New("fork unavailable")
	spawn := func(LaunchSpec) (Child, error) {
		info, err := os.Stat(filepath.Dir(spec.LogPath))
		testutil.Require(t, "state directory stat error", err, nil)
		testutil.Expect(t, "state path is a directory", info.IsDir(), true)
		return nil, spawnErr
	}

	address, pid, err := Launch(spec, spawn, time.Second)

	testutil.Expect(t, "address", address, "")
	testutil.Expect(t, "pid", pid, 0)
	testutil.Require(t, "launch returned error", err != nil, true)
	testutil.Expect(t, "cannot-launch error", err.Error(), "cannot launch the interface server: fork unavailable")
}

// launchTestSpec runs the child in a checkout while its log, and so the state
// directory the launcher creates, lives under a state root of its own.
func launchTestSpec(t *testing.T) LaunchSpec {
	t.Helper()

	home := t.TempDir()
	return LaunchSpec{
		Dir:     filepath.Join(home, "checkout"),
		LogPath: filepath.Join(Dir(filepath.Join(home, "state")), "server.log"),
	}
}

type fakeLaunchChild struct {
	line     string
	readyErr error
	pid      int
	wait     time.Duration
	killed   bool
	released bool
}

func (c *fakeLaunchChild) ReadyLine(wait time.Duration) (string, error) {
	c.wait = wait
	return c.line, c.readyErr
}

func (c *fakeLaunchChild) Pid() int { return c.pid }

func (c *fakeLaunchChild) Kill() error {
	c.killed = true
	return nil
}

func (c *fakeLaunchChild) Release() error {
	c.released = true
	c.pid = -1
	return nil
}
