package lifecycle

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestStopOutcomes(t *testing.T) {
	t.Parallel()

	t.Run("stopped", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		outcome, rec, err := Stop(stateRoot, StopOptions{Prober: &stateTestProber{}})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "outcome", outcome, StopOutcome(Stopped))
		testutil.Expect(t, "record", rec, (*Record)(nil))
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		want, exact := stateTestRecord(t, "stale", stateRoot, 5101)
		stateTestWriteRecord(t, "stale record", stateRoot, want)
		stateTestCreateUnlockedLock(t, "stale lock", stateRoot)

		outcome, rec, err := Stop(stateRoot, StopOptions{Prober: stateTestProberFor(exact, identity.Dead)})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "outcome", outcome, StopOutcome(Stale))
		testutil.Require(t, "record present", rec != nil, true)
		testutil.Expect(t, "record", *rec, want)
		_, statErr := os.Stat(recordPath(stateRoot))
		testutil.Expect(t, "record removed", errors.Is(statErr, os.ErrNotExist), true)
	})

	t.Run("uninspectable", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		want, exact := stateTestRecord(t, "uninspectable", stateRoot, 5102)
		original := stateTestWriteRecord(t, "uninspectable record", stateRoot, want)
		stateTestHoldLock(t, "uninspectable lock", stateRoot)

		outcome, rec, err := Stop(stateRoot, StopOptions{Prober: stateTestProberFor(exact, identity.Unknown)})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "outcome", outcome, StopOutcome(Uninspectable))
		testutil.Require(t, "record present", rec != nil, true)
		testutil.Expect(t, "record", *rec, want)
		stateTestExpectRecord(t, "uninspectable", stateRoot, original)
	})

	t.Run("unreadable", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		original := []byte("{\"schemaVersion\":2}\n")
		stateTestWriteBytes(t, "unreadable record", stateRoot, original)
		stateTestHoldLock(t, "unreadable lock", stateRoot)

		outcome, rec, err := Stop(stateRoot, StopOptions{Prober: &stateTestProber{}})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "outcome", outcome, StopOutcome(Unreadable))
		testutil.Expect(t, "record", rec, (*Record)(nil))
		stateTestExpectRecord(t, "unreadable", stateRoot, original)
	})

	t.Run("busy", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		holder := stateTestHoldLock(t, "busy lock", stateRoot)
		outcome, rec, err := Stop(stateRoot, StopOptions{
			Prober: &stateTestProber{},
			Wait:   time.Second,
			After:  stateTestImmediateAfter,
		})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "outcome", outcome, StopOutcome(Busy))
		testutil.Expect(t, "record", rec, (*Record)(nil))
		holder.Release()
	})

	t.Run("stopped now", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		want, exact := stateTestRecord(t, "stopped now", stateRoot, 5103)
		stateTestWriteRecord(t, "stopped now record", stateRoot, want)
		holder := stateTestHoldLock(t, "stopped now lock", stateRoot)
		var signals []syscall.Signal
		send := func(pid int, signal syscall.Signal) error {
			signals = append(signals, signal)
			holder.Release()
			return nil
		}

		outcome, rec, err := Stop(stateRoot, StopOptions{
			Prober: stateTestProberFor(exact, identity.Alive),
			Send:   send,
			Wait:   time.Second,
			After:  stateTestNeverAfter,
		})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "outcome", outcome, StoppedNow)
		testutil.Require(t, "record present", rec != nil, true)
		testutil.Expect(t, "record", *rec, want)
		testutil.Expect(t, "signals, with no SIGKILL", signals, []syscall.Signal{syscall.SIGTERM})
		_, statErr := os.Stat(recordPath(stateRoot))
		testutil.Expect(t, "record removed", errors.Is(statErr, os.ErrNotExist), true)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		want, exact := stateTestRecord(t, "timeout", stateRoot, 5104)
		original := stateTestWriteRecord(t, "timeout record", stateRoot, want)
		stateTestHoldLock(t, "timeout lock", stateRoot)
		var signals []syscall.Signal

		outcome, rec, err := Stop(stateRoot, StopOptions{
			Prober: stateTestProberFor(exact, identity.Alive),
			Send: func(pid int, signal syscall.Signal) error {
				signals = append(signals, signal)
				return nil
			},
			Wait:  time.Second,
			After: stateTestImmediateAfter,
		})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "outcome", outcome, Timeout)
		testutil.Require(t, "record present", rec != nil, true)
		testutil.Expect(t, "record", *rec, want)
		testutil.Expect(t, "signals, with no SIGKILL", signals, []syscall.Signal{syscall.SIGTERM})
		stateTestExpectRecord(t, "timeout", stateRoot, original)
	})
}

func TestStopTimeoutWithDeadIdentityLeavesReplacementUntouched(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	rec, exact := stateTestRecord(t, "dead after timeout", stateRoot, 5201)
	original := stateTestWriteRecord(t, "dead after timeout record", stateRoot, rec)
	stateTestHoldLock(t, "dead after timeout lock", stateRoot)
	prober := stateTestProberFor(exact, identity.Alive, identity.Alive, identity.Dead)
	var signals []syscall.Signal

	outcome, returned, err := Stop(stateRoot, StopOptions{
		Prober: prober,
		Send: func(pid int, signal syscall.Signal) error {
			signals = append(signals, signal)
			return nil
		},
		Wait:  time.Second,
		After: stateTestImmediateAfter,
	})

	testutil.Require(t, "stop error", err, nil)
	testutil.Expect(t, "outcome", outcome, StoppedNow)
	testutil.Require(t, "record present", returned != nil, true)
	testutil.Expect(t, "record", *returned, rec)
	testutil.Expect(t, "signals, with no SIGKILL", signals, []syscall.Signal{syscall.SIGTERM})
	stateTestExpectRecord(t, "dead after timeout", stateRoot, original)
}

func TestStopHandlesIdentityGoneWithoutSignalling(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	want, exact := stateTestRecord(t, "identity gone", stateRoot, 5202)
	stateTestWriteRecord(t, "identity gone record", stateRoot, want)
	stateTestCreateUnlockedLock(t, "identity gone lock", stateRoot)
	prober := stateTestProberFor(exact, identity.Alive, identity.Dead)
	var signals []syscall.Signal

	outcome, rec, err := Stop(stateRoot, StopOptions{
		Prober: prober,
		Send: func(pid int, signal syscall.Signal) error {
			signals = append(signals, signal)
			return nil
		},
	})

	testutil.Require(t, "stop error", err, nil)
	testutil.Expect(t, "outcome", outcome, StopOutcome(Stale))
	testutil.Require(t, "record present", rec != nil, true)
	testutil.Expect(t, "record", *rec, want)
	testutil.Expect(t, "signals", signals, []syscall.Signal(nil))
	_, statErr := os.Stat(recordPath(stateRoot))
	testutil.Expect(t, "record removed", errors.Is(statErr, os.ErrNotExist), true)
}

func TestStopHandlesIdentityBecomingUnknownWithoutSignalling(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	want, exact := stateTestRecord(t, "identity unknown", stateRoot, 5203)
	original := stateTestWriteRecord(t, "identity unknown record", stateRoot, want)
	stateTestHoldLock(t, "identity unknown lock", stateRoot)
	prober := stateTestProberFor(exact, identity.Alive, identity.Unknown)
	var signals []syscall.Signal

	outcome, rec, err := Stop(stateRoot, StopOptions{
		Prober: prober,
		Send: func(pid int, signal syscall.Signal) error {
			signals = append(signals, signal)
			return nil
		},
	})

	testutil.Require(t, "stop error", err, nil)
	testutil.Expect(t, "outcome", outcome, StopOutcome(Uninspectable))
	testutil.Require(t, "record present", rec != nil, true)
	testutil.Expect(t, "record", *rec, want)
	testutil.Expect(t, "signals", signals, []syscall.Signal(nil))
	stateTestExpectRecord(t, "identity unknown", stateRoot, original)
}

func TestStopBusyThenRunningSignalsServer(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	holder := stateTestHoldLock(t, "starting lock", stateRoot)
	rec, exact := stateTestRecord(t, "published server", stateRoot, 5301)
	var publish sync.Once
	after := func(time.Duration) <-chan time.Time {
		publish.Do(func() {
			stateTestWriteRecord(t, "published server record", stateRoot, rec)
			holder.Release()
		})
		return stateTestNeverAfter(0)
	}
	var signals []syscall.Signal

	outcome, returned, err := Stop(stateRoot, StopOptions{
		Prober: stateTestProberFor(exact, identity.Alive),
		Send: func(pid int, signal syscall.Signal) error {
			signals = append(signals, signal)
			return nil
		},
		Wait:  time.Second,
		After: after,
	})

	testutil.Require(t, "stop error", err, nil)
	testutil.Expect(t, "outcome", outcome, StoppedNow)
	testutil.Require(t, "record present", returned != nil, true)
	testutil.Expect(t, "record", *returned, rec)
	testutil.Expect(t, "signals, with no SIGKILL", signals, []syscall.Signal{syscall.SIGTERM})
}

func TestStopReturnsBusyAfterSecondBusyEvaluation(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	holder := stateTestHoldLock(t, "busy twice lock", stateRoot)
	outcome, rec, err := Stop(stateRoot, StopOptions{
		Prober: &stateTestProber{},
		Wait:   time.Second,
		After:  stateTestImmediateAfter,
	})

	testutil.Require(t, "stop error", err, nil)
	testutil.Expect(t, "outcome", outcome, StopOutcome(Busy))
	testutil.Expect(t, "record", rec, (*Record)(nil))
	holder.Release()
}

func TestRestartStartsOnlyAfterStoppedOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prepare   func(*testing.T, string) StopOptions
		want      StopOutcome
		wantStart bool
	}{
		{
			name: "stopped",
			prepare: func(t *testing.T, stateRoot string) StopOptions {
				return StopOptions{Prober: &stateTestProber{}}
			},
			want:      StopOutcome(Stopped),
			wantStart: true,
		},
		{
			name: "stale",
			prepare: func(t *testing.T, stateRoot string) StopOptions {
				rec, exact := stateTestRecord(t, "stale", stateRoot, 5401)
				stateTestWriteRecord(t, "stale record", stateRoot, rec)
				stateTestCreateUnlockedLock(t, "stale lock", stateRoot)
				return StopOptions{Prober: stateTestProberFor(exact, identity.Dead)}
			},
			want:      StopOutcome(Stale),
			wantStart: true,
		},
		{
			name: "stopped now",
			prepare: func(t *testing.T, stateRoot string) StopOptions {
				rec, exact := stateTestRecord(t, "stopped now", stateRoot, 5402)
				stateTestWriteRecord(t, "stopped now record", stateRoot, rec)
				holder := stateTestHoldLock(t, "stopped now lock", stateRoot)
				return StopOptions{
					Prober: stateTestProberFor(exact, identity.Alive),
					Send: func(int, syscall.Signal) error {
						holder.Release()
						return nil
					},
					Wait:  time.Second,
					After: stateTestNeverAfter,
				}
			},
			want:      StoppedNow,
			wantStart: true,
		},
		{
			name: "uninspectable",
			prepare: func(t *testing.T, stateRoot string) StopOptions {
				rec, exact := stateTestRecord(t, "uninspectable", stateRoot, 5403)
				stateTestWriteRecord(t, "uninspectable record", stateRoot, rec)
				stateTestHoldLock(t, "uninspectable lock", stateRoot)
				return StopOptions{Prober: stateTestProberFor(exact, identity.Unknown)}
			},
			want: StopOutcome(Uninspectable),
		},
		{
			name: "unreadable",
			prepare: func(t *testing.T, stateRoot string) StopOptions {
				stateTestWriteBytes(t, "unreadable record", stateRoot, []byte("bad\n"))
				stateTestHoldLock(t, "unreadable lock", stateRoot)
				return StopOptions{Prober: &stateTestProber{}}
			},
			want: StopOutcome(Unreadable),
		},
		{
			name: "busy",
			prepare: func(t *testing.T, stateRoot string) StopOptions {
				stateTestHoldLock(t, "busy lock", stateRoot)
				return StopOptions{Prober: &stateTestProber{}, Wait: time.Second, After: stateTestImmediateAfter}
			},
			want: StopOutcome(Busy),
		},
		{
			name: "timeout",
			prepare: func(t *testing.T, stateRoot string) StopOptions {
				rec, exact := stateTestRecord(t, "timeout", stateRoot, 5404)
				stateTestWriteRecord(t, "timeout record", stateRoot, rec)
				stateTestHoldLock(t, "timeout lock", stateRoot)
				return StopOptions{
					Prober: stateTestProberFor(exact, identity.Alive),
					Send:   func(int, syscall.Signal) error { return nil },
					Wait:   time.Second,
					After:  stateTestImmediateAfter,
				}
			},
			want: Timeout,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stateRoot := t.TempDir()
			options := test.prepare(t, stateRoot)
			starts := 0
			outcome, _, err := Restart(stateRoot, options, func() error {
				starts++
				return nil
			})

			testutil.Require(t, "restart error", err, nil)
			testutil.Expect(t, "stop outcome", outcome, test.want)
			wantStarts := 0
			if test.wantStart {
				wantStarts = 1
			}
			testutil.Expect(t, "start calls", starts, wantStarts)
		})
	}
}

func stateTestExpectRecord(t *testing.T, label, stateRoot string, expected []byte) {
	t.Helper()

	data, err := os.ReadFile(recordPath(stateRoot))
	testutil.Require(t, label+" record read error", err, nil)
	testutil.Expect(t, label+" record contents", data, expected)
}
