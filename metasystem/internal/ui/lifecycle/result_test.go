package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestStartResult(t *testing.T) {
	t.Parallel()

	t.Run("ready", func(t *testing.T) {
		t.Parallel()

		child := &fakeLaunchChild{line: "ready 127.0.0.1:49152", pid: 4201}
		result := StartResult(resultLaunchSpec(t), func(LaunchSpec) (Child, error) {
			return child, nil
		}, time.Second)

		testutil.Expect(t, "start result", result, Result{
			Lines: []string{"interface running at http://127.0.0.1:49152 (pid 4201)"},
			Code:  0,
		})
	})

	t.Run("failed line", func(t *testing.T) {
		t.Parallel()

		child := &fakeLaunchChild{line: "failed cannot listen on --listen or ui.listen"}
		result := StartResult(resultLaunchSpec(t), func(LaunchSpec) (Child, error) {
			return child, nil
		}, time.Second)

		testutil.Expect(t, "start failure result", result, Result{
			Lines: []string{"cannot listen on --listen or ui.listen"},
			Code:  1,
		})
	})

	t.Run("spawn failure", func(t *testing.T) {
		t.Parallel()

		result := StartResult(resultLaunchSpec(t), func(LaunchSpec) (Child, error) {
			return nil, errors.New("fork unavailable")
		}, time.Second)

		testutil.Expect(t, "spawn failure result", result, Result{
			Lines: []string{"cannot launch the interface server: fork unavailable"},
			Code:  1,
		})
	})

	t.Run("not ready", func(t *testing.T) {
		t.Parallel()

		spec := resultLaunchSpec(t)
		child := &fakeLaunchChild{readyErr: ErrReadyTimeout}
		result := StartResult(spec, func(LaunchSpec) (Child, error) {
			return child, nil
		}, time.Second)

		testutil.Expect(t, "not-ready result", result, Result{
			Lines: []string{"the interface server did not become ready; see " + spec.LogPath},
			Code:  1,
		})
	})
}

// resultLaunchSpec spawns into a checkout while the log, and so the state
// directory the launcher creates, lives under a state root of its own.
func resultLaunchSpec(t *testing.T) LaunchSpec {
	t.Helper()
	home := t.TempDir()
	return LaunchSpec{
		Dir:     filepath.Join(home, "checkout"),
		LogPath: filepath.Join(Dir(filepath.Join(home, "state")), "server.log"),
	}
}

func TestStatusResultStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state State
		line  func(string) string
	}{
		{"stopped", Stopped, func(string) string { return "interface not running" }},
		{"stale", Stale, func(string) string { return "interface not running (stale record from pid 4201 removed)" }},
		{"uninspectable", Uninspectable, func(string) string { return "cannot prove pid 4201 is the interface server; nothing was changed" }},
		{"unreadable", Unreadable, func(stateRoot string) string {
			return "a process holds the interface lock but " + recordPath(stateRoot) + " cannot be read; nothing was changed"
		}},
		{"busy", Busy, func(string) string { return "the interface is starting or stopping; try again" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stateRoot, prober := resultStateFixture(t, test.state)
			result := StatusResult(stateRoot, prober, func() (string, error) {
				return "sha256:current", nil
			})

			testutil.Expect(t, "status result", result, Result{
				Lines: []string{test.line(stateRoot)},
				Code:  1,
			})
		})
	}
}

func TestStatusResultDigestComparison(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		digest    func() (string, error)
		extraLine string
	}{
		{
			name:   "unchanged",
			digest: func() (string, error) { return "sha256:serving", nil },
		},
		{
			name:   "changed",
			digest: func() (string, error) { return "sha256:current", nil },
			extraLine: "the executable on disk differs from the one the interface is running; " +
				"to pick it up: metasystem ui restart",
		},
		{
			name:      "read error",
			digest:    func() (string, error) { return "", errors.New("permission denied") },
			extraLine: "cannot read the executable on disk to compare builds: permission denied",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stateRoot := t.TempDir()
			rec, exact := resultTestRecord(t, stateRoot)
			result := StatusResult(stateRoot, resultTestProber{exact: exact, state: identity.Alive}, test.digest)
			expectedLines := []string{
				"interface running at http://127.0.0.1:49152 (pid 4201, started 2026-09-21T12:34:56Z, build dev-test)",
			}
			if test.extraLine != "" {
				expectedLines = append(expectedLines, test.extraLine)
			}

			testutil.Expect(t, "record digest", rec.ExecutableDigest, "sha256:serving")
			testutil.Expect(t, "status result", result, Result{Lines: expectedLines, Code: 0})
		})
	}
}

func TestStopResult(t *testing.T) {
	t.Parallel()

	passive := []struct {
		name  string
		state State
		line  func(string) string
		code  int
	}{
		{"stopped", Stopped, func(string) string { return "interface not running" }, 0},
		{"stale", Stale, func(string) string { return "interface not running (stale record from pid 4201 removed)" }, 0},
		{"uninspectable", Uninspectable, func(string) string { return "cannot prove pid 4201 is the interface server; nothing was changed" }, 1},
		{"unreadable", Unreadable, func(stateRoot string) string {
			return "a process holds the interface lock but " + recordPath(stateRoot) + " cannot be read; nothing was changed"
		}, 1},
	}
	for _, test := range passive {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stateRoot, prober := resultStateFixture(t, test.state)
			result := StopResult(stateRoot, StopOptions{Prober: prober, Wait: 15 * time.Second})

			testutil.Expect(t, "stop result", result, Result{
				Lines: []string{test.line(stateRoot)},
				Code:  test.code,
			})
		})
	}

	t.Run("busy", func(t *testing.T) {
		t.Parallel()

		stateRoot, prober := resultStateFixture(t, Busy)
		result := StopResult(stateRoot, StopOptions{
			Prober: prober,
			Wait:   15 * time.Second,
			After:  resultImmediateAfter,
		})

		testutil.Expect(t, "busy stop result", result, Result{
			Lines: []string{"the interface is starting or stopping; try again"},
			Code:  1,
		})
	})

	t.Run("stopped now", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		_, exact := resultTestRecord(t, stateRoot)
		lock := resultTestLock(t, stateRoot, true)
		result := StopResult(stateRoot, StopOptions{
			Prober: resultTestProber{exact: exact, state: identity.Alive},
			Send: func(int, syscall.Signal) error {
				return unix.Flock(int(lock.Fd()), unix.LOCK_UN)
			},
			Wait:  15 * time.Second,
			After: resultNeverAfter,
		})

		testutil.Expect(t, "stopped-now result", result, Result{
			Lines: []string{"interface stopped"},
			Code:  0,
		})
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		_, exact := resultTestRecord(t, stateRoot)
		resultTestLock(t, stateRoot, true)
		result := StopResult(stateRoot, StopOptions{
			Prober: resultTestProber{exact: exact, state: identity.Alive},
			Send:   func(int, syscall.Signal) error { return nil },
			Wait:   15 * time.Second,
			After:  resultImmediateAfter,
		})

		testutil.Expect(t, "timeout result", result, Result{
			Lines: []string{"interface (pid 4201) did not stop within 15s; it was sent SIGTERM and left running"},
			Code:  1,
		})
	})
}

func TestRestartResult(t *testing.T) {
	t.Parallel()

	startSuccess := Result{Lines: []string{"interface running at http://127.0.0.1:49152 (pid 4301)"}, Code: 0}
	startFailure := Result{Lines: []string{"cannot launch the interface server: fork unavailable"}, Code: 1}

	t.Run("stopped suppresses stop line", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		starts := 0
		result := RestartResult(stateRoot, StopOptions{Prober: resultTestProber{state: identity.Dead}}, func() Result {
			starts++
			return startSuccess
		})

		testutil.Expect(t, "start calls", starts, 1)
		testutil.Expect(t, "restart result", result, startSuccess)
	})

	t.Run("stale line precedes start line", func(t *testing.T) {
		t.Parallel()

		stateRoot, prober := resultStateFixture(t, Stale)
		result := RestartResult(stateRoot, StopOptions{Prober: prober}, func() Result {
			return startSuccess
		})

		testutil.Expect(t, "restart result", result, Result{
			Lines: []string{
				"interface not running (stale record from pid 4201 removed)",
				"interface running at http://127.0.0.1:49152 (pid 4301)",
			},
			Code: 0,
		})
	})

	t.Run("stopped-now line precedes start line", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		_, exact := resultTestRecord(t, stateRoot)
		lock := resultTestLock(t, stateRoot, true)
		result := RestartResult(stateRoot, StopOptions{
			Prober: resultTestProber{exact: exact, state: identity.Alive},
			Send: func(int, syscall.Signal) error {
				return unix.Flock(int(lock.Fd()), unix.LOCK_UN)
			},
			After: resultNeverAfter,
		}, func() Result {
			return startSuccess
		})

		testutil.Expect(t, "restart result", result, Result{
			Lines: []string{
				"interface stopped",
				"interface running at http://127.0.0.1:49152 (pid 4301)",
			},
			Code: 0,
		})
	})

	t.Run("stop refusal does not start", func(t *testing.T) {
		t.Parallel()

		stateRoot, prober := resultStateFixture(t, Uninspectable)
		starts := 0
		result := RestartResult(stateRoot, StopOptions{Prober: prober}, func() Result {
			starts++
			return startSuccess
		})

		testutil.Expect(t, "start calls", starts, 0)
		testutil.Expect(t, "restart refusal", result, Result{
			Lines: []string{"cannot prove pid 4201 is the interface server; nothing was changed"},
			Code:  1,
		})
	})

	t.Run("stopped suppresses stop line before start failure", func(t *testing.T) {
		t.Parallel()

		result := RestartResult(t.TempDir(), StopOptions{Prober: resultTestProber{state: identity.Dead}}, func() Result {
			return startFailure
		})

		testutil.Expect(t, "restart failure", result, startFailure)
	})

	t.Run("stale line precedes start failure", func(t *testing.T) {
		t.Parallel()

		stateRoot, prober := resultStateFixture(t, Stale)
		result := RestartResult(stateRoot, StopOptions{Prober: prober}, func() Result {
			return startFailure
		})

		testutil.Expect(t, "restart failure", result, Result{
			Lines: []string{
				"interface not running (stale record from pid 4201 removed)",
				"cannot launch the interface server: fork unavailable",
			},
			Code: 1,
		})
	})
}

func TestServeFailure(t *testing.T) {
	t.Parallel()

	t.Run("unknown address names record", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		line := ServeFailure(stateRoot, &AlreadyRunningError{})

		testutil.Expect(t, "serve failure", line,
			"an interface server already runs for this checkout (address unknown: "+recordPath(stateRoot)+" is missing or unreadable)")
	})

	t.Run("known address passes through", func(t *testing.T) {
		t.Parallel()

		line := ServeFailure(t.TempDir(), &AlreadyRunningError{Address: "127.0.0.1:49152"})

		testutil.Expect(t, "serve failure", line,
			"an interface server already runs for this checkout at http://127.0.0.1:49152; stop it with: metasystem ui stop")
	})

	t.Run("other error passes through", func(t *testing.T) {
		t.Parallel()

		line := ServeFailure(t.TempDir(), errors.New("cannot read the serving executable: permission denied"))

		testutil.Expect(t, "serve failure", line, "cannot read the serving executable: permission denied")
	})
}

type resultTestProber struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p resultTestProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, p.err
}

func resultStateFixture(t *testing.T, state State) (string, identity.Prober) {
	t.Helper()
	stateRoot := t.TempDir()
	switch state {
	case Stopped:
		return stateRoot, resultTestProber{state: identity.Dead}
	case Stale:
		resultTestRecord(t, stateRoot)
		resultTestLock(t, stateRoot, false)
		return stateRoot, resultTestProber{state: identity.Dead}
	case Uninspectable:
		resultTestRecord(t, stateRoot)
		return stateRoot, resultTestProber{state: identity.Unknown, err: errors.New("inspection unavailable")}
	case Unreadable:
		resultTestLock(t, stateRoot, true)
		testutil.Require(t, "write unreadable record", os.WriteFile(recordPath(stateRoot), []byte("not json\n"), 0o644), nil)
		return stateRoot, resultTestProber{state: identity.Dead}
	case Busy:
		resultTestLock(t, stateRoot, true)
		return stateRoot, resultTestProber{state: identity.Dead}
	default:
		testutil.Require(t, "supported fixture state", state, Stopped)
		return "", nil
	}
}

func resultTestRecord(t *testing.T, stateRoot string) (Record, identity.Exact) {
	t.Helper()
	exact := identity.Exact{Pid: 4201, StartedAt: time.Unix(1_700_000_000, 123_456_000)}
	if runtime.GOOS == "linux" {
		exact.StartTicks = 987654
		exact.BootID = "result-test-boot"
	}
	process, err := identity.EncodeRef(exact.Ref())
	testutil.Require(t, "encode result process", err, nil)
	rec := Record{
		SchemaVersion:    1,
		Process:          process,
		Address:          "127.0.0.1:49152",
		Checkout:         "/work/checkout",
		Installation:     "/work/checkout/metasystem",
		StartedAt:        "2026-09-21T12:34:56Z",
		EngineBuild:      "dev-test",
		ExecutableDigest: "sha256:serving",
	}
	data, err := json.Marshal(rec)
	testutil.Require(t, "marshal result record", err, nil)
	testutil.Require(t, "create result state directory", os.MkdirAll(Dir(stateRoot), 0o755), nil)
	testutil.Require(t, "write result record", os.WriteFile(recordPath(stateRoot), append(data, '\n'), 0o644), nil)
	return rec, exact
}

func resultTestLock(t *testing.T, stateRoot string, held bool) *os.File {
	t.Helper()
	testutil.Require(t, "create lock directory", os.MkdirAll(Dir(stateRoot), 0o755), nil)
	lock, err := os.OpenFile(lockPath(stateRoot), os.O_CREATE|os.O_RDWR, 0o644)
	testutil.Require(t, "open result lock", err, nil)
	if held {
		testutil.Require(t, "hold result lock", unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB), nil)
	}
	t.Cleanup(func() {
		_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		_ = lock.Close()
	})
	return lock
}

func resultImmediateAfter(time.Duration) <-chan time.Time {
	fired := make(chan time.Time, 1)
	fired <- time.Time{}
	return fired
}

func resultNeverAfter(time.Duration) <-chan time.Time { return make(chan time.Time) }
