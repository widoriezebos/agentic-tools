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
		checkout := t.TempDir()
		result := StartResult(LaunchSpec{
			Dir:     checkout,
			LogPath: filepath.Join(Dir(checkout), "server.log"),
		}, func(LaunchSpec) (Child, error) {
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
		checkout := t.TempDir()
		result := StartResult(LaunchSpec{
			Dir:     checkout,
			LogPath: filepath.Join(Dir(checkout), "server.log"),
		}, func(LaunchSpec) (Child, error) {
			return child, nil
		}, time.Second)

		testutil.Expect(t, "start failure result", result, Result{
			Lines: []string{"cannot listen on --listen or ui.listen"},
			Code:  1,
		})
	})

	t.Run("spawn failure", func(t *testing.T) {
		t.Parallel()

		checkout := t.TempDir()
		result := StartResult(LaunchSpec{Dir: checkout}, func(LaunchSpec) (Child, error) {
			return nil, errors.New("fork unavailable")
		}, time.Second)

		testutil.Expect(t, "spawn failure result", result, Result{
			Lines: []string{"cannot launch the interface server: fork unavailable"},
			Code:  1,
		})
	})

	t.Run("not ready", func(t *testing.T) {
		t.Parallel()

		checkout := t.TempDir()
		logPath := filepath.Join(Dir(checkout), "server.log")
		child := &fakeLaunchChild{readyErr: ErrReadyTimeout}
		result := StartResult(LaunchSpec{Dir: checkout, LogPath: logPath}, func(LaunchSpec) (Child, error) {
			return child, nil
		}, time.Second)

		testutil.Expect(t, "not-ready result", result, Result{
			Lines: []string{"the interface server did not become ready; see " + logPath},
			Code:  1,
		})
	})
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
		{"unreadable", Unreadable, func(checkout string) string {
			return "a process holds the interface lock but " + recordPath(checkout) + " cannot be read; nothing was changed"
		}},
		{"busy", Busy, func(string) string { return "the interface is starting or stopping; try again" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			checkout, prober := resultStateFixture(t, test.state)
			result := StatusResult(checkout, prober, func() (string, error) {
				return "sha256:current", nil
			})

			testutil.Expect(t, "status result", result, Result{
				Lines: []string{test.line(checkout)},
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

			checkout := t.TempDir()
			rec, exact := resultTestRecord(t, checkout)
			result := StatusResult(checkout, resultTestProber{exact: exact, state: identity.Alive}, test.digest)
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
		{"unreadable", Unreadable, func(checkout string) string {
			return "a process holds the interface lock but " + recordPath(checkout) + " cannot be read; nothing was changed"
		}, 1},
	}
	for _, test := range passive {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			checkout, prober := resultStateFixture(t, test.state)
			result := StopResult(checkout, StopOptions{Prober: prober, Wait: 15 * time.Second})

			testutil.Expect(t, "stop result", result, Result{
				Lines: []string{test.line(checkout)},
				Code:  test.code,
			})
		})
	}

	t.Run("busy", func(t *testing.T) {
		t.Parallel()

		checkout, prober := resultStateFixture(t, Busy)
		result := StopResult(checkout, StopOptions{
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

		checkout := t.TempDir()
		_, exact := resultTestRecord(t, checkout)
		lock := resultTestLock(t, checkout, true)
		result := StopResult(checkout, StopOptions{
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

		checkout := t.TempDir()
		_, exact := resultTestRecord(t, checkout)
		resultTestLock(t, checkout, true)
		result := StopResult(checkout, StopOptions{
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

		checkout := t.TempDir()
		starts := 0
		result := RestartResult(checkout, StopOptions{Prober: resultTestProber{state: identity.Dead}}, func() Result {
			starts++
			return startSuccess
		})

		testutil.Expect(t, "start calls", starts, 1)
		testutil.Expect(t, "restart result", result, startSuccess)
	})

	t.Run("stale line precedes start line", func(t *testing.T) {
		t.Parallel()

		checkout, prober := resultStateFixture(t, Stale)
		result := RestartResult(checkout, StopOptions{Prober: prober}, func() Result {
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

		checkout := t.TempDir()
		_, exact := resultTestRecord(t, checkout)
		lock := resultTestLock(t, checkout, true)
		result := RestartResult(checkout, StopOptions{
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

		checkout, prober := resultStateFixture(t, Uninspectable)
		starts := 0
		result := RestartResult(checkout, StopOptions{Prober: prober}, func() Result {
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

		checkout, prober := resultStateFixture(t, Stale)
		result := RestartResult(checkout, StopOptions{Prober: prober}, func() Result {
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

		checkout := t.TempDir()
		line := ServeFailure(checkout, &AlreadyRunningError{})

		testutil.Expect(t, "serve failure", line,
			"an interface server already runs for this checkout (address unknown: "+recordPath(checkout)+" is missing or unreadable)")
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
	checkout := t.TempDir()
	switch state {
	case Stopped:
		return checkout, resultTestProber{state: identity.Dead}
	case Stale:
		resultTestRecord(t, checkout)
		resultTestLock(t, checkout, false)
		return checkout, resultTestProber{state: identity.Dead}
	case Uninspectable:
		resultTestRecord(t, checkout)
		return checkout, resultTestProber{state: identity.Unknown, err: errors.New("inspection unavailable")}
	case Unreadable:
		resultTestLock(t, checkout, true)
		testutil.Require(t, "write unreadable record", os.WriteFile(recordPath(checkout), []byte("not json\n"), 0o644), nil)
		return checkout, resultTestProber{state: identity.Dead}
	case Busy:
		resultTestLock(t, checkout, true)
		return checkout, resultTestProber{state: identity.Dead}
	default:
		testutil.Require(t, "supported fixture state", state, Stopped)
		return "", nil
	}
}

func resultTestRecord(t *testing.T, checkout string) (Record, identity.Exact) {
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
		Checkout:         checkout,
		StartedAt:        "2026-09-21T12:34:56Z",
		EngineBuild:      "dev-test",
		ExecutableDigest: "sha256:serving",
	}
	data, err := json.Marshal(rec)
	testutil.Require(t, "marshal result record", err, nil)
	testutil.Require(t, "create result state directory", os.MkdirAll(Dir(checkout), 0o755), nil)
	testutil.Require(t, "write result record", os.WriteFile(recordPath(checkout), append(data, '\n'), 0o644), nil)
	return rec, exact
}

func resultTestLock(t *testing.T, checkout string, held bool) *os.File {
	t.Helper()
	testutil.Require(t, "create lock directory", os.MkdirAll(Dir(checkout), 0o755), nil)
	lock, err := os.OpenFile(lockPath(checkout), os.O_CREATE|os.O_RDWR, 0o644)
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
