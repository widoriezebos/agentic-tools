package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestReadStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		prepare    func(*testing.T, string) (*stateTestProber, Record, []byte)
		wantState  State
		wantRecord bool
		wantFile   bool
	}{
		{
			name: "running",
			prepare: func(t *testing.T, stateRoot string) (*stateTestProber, Record, []byte) {
				rec, exact := stateTestRecord(t, "running", stateRoot, 4101)
				data := stateTestWriteRecord(t, "running record", stateRoot, rec)
				stateTestHoldLock(t, "running lock", stateRoot)
				return stateTestProberFor(exact, identity.Alive), rec, data
			},
			wantState:  Running,
			wantRecord: true,
			wantFile:   true,
		},
		{
			name: "stopped",
			prepare: func(t *testing.T, stateRoot string) (*stateTestProber, Record, []byte) {
				stateTestCreateUnlockedLock(t, "stopped", stateRoot)
				return &stateTestProber{}, Record{}, nil
			},
			wantState: Stopped,
		},
		{
			name: "stale",
			prepare: func(t *testing.T, stateRoot string) (*stateTestProber, Record, []byte) {
				rec, exact := stateTestRecord(t, "stale", stateRoot, 4102)
				data := stateTestWriteRecord(t, "stale record", stateRoot, rec)
				stateTestCreateUnlockedLock(t, "stale lock", stateRoot)
				return stateTestProberFor(exact, identity.Dead), rec, data
			},
			wantState:  Stale,
			wantRecord: true,
		},
		{
			name: "uninspectable",
			prepare: func(t *testing.T, stateRoot string) (*stateTestProber, Record, []byte) {
				rec, exact := stateTestRecord(t, "uninspectable", stateRoot, 4103)
				data := stateTestWriteRecord(t, "uninspectable record", stateRoot, rec)
				stateTestHoldLock(t, "uninspectable lock", stateRoot)
				return stateTestProberFor(exact, identity.Unknown), rec, data
			},
			wantState:  Uninspectable,
			wantRecord: true,
			wantFile:   true,
		},
		{
			name: "unreadable",
			prepare: func(t *testing.T, stateRoot string) (*stateTestProber, Record, []byte) {
				data := []byte("not-json\n")
				stateTestWriteBytes(t, "unreadable record", stateRoot, data)
				stateTestHoldLock(t, "unreadable lock", stateRoot)
				return &stateTestProber{}, Record{}, data
			},
			wantState: Unreadable,
			wantFile:  true,
		},
		{
			name: "busy",
			prepare: func(t *testing.T, stateRoot string) (*stateTestProber, Record, []byte) {
				stateTestHoldLock(t, "busy", stateRoot)
				return &stateTestProber{}, Record{}, nil
			},
			wantState: Busy,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stateRoot := t.TempDir()
			prober, rec, original := test.prepare(t, stateRoot)
			status, err := Read(stateRoot, prober)

			testutil.Require(t, "read error", err, nil)
			testutil.Expect(t, "state", status.State, test.wantState)
			if test.wantRecord {
				testutil.Require(t, "status record present", status.Record != nil, true)
				testutil.Expect(t, "status record", *status.Record, rec)
			} else {
				testutil.Expect(t, "status record", status.Record, (*Record)(nil))
			}
			data, readErr := os.ReadFile(recordPath(stateRoot))
			if test.wantFile {
				testutil.Require(t, "record read error", readErr, nil)
				testutil.Expect(t, "record contents", data, original)
			} else {
				testutil.Expect(t, "record absent", errors.Is(readErr, os.ErrNotExist), true)
			}
		})
	}
}

func TestReadLosingLockProbeLeavesRecordAlone(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	rec, exact := stateTestRecord(t, "losing probe", stateRoot, 4201)
	original := stateTestWriteRecord(t, "losing probe record", stateRoot, rec)
	stateTestHoldLock(t, "losing probe lock", stateRoot)

	status, err := Read(stateRoot, stateTestProberFor(exact, identity.Dead))

	testutil.Require(t, "read error", err, nil)
	testutil.Expect(t, "state", status.State, Busy)
	data, readErr := os.ReadFile(recordPath(stateRoot))
	testutil.Require(t, "record read error", readErr, nil)
	testutil.Expect(t, "record contents", data, original)
}

func TestReadLeavesReplacementMadeUnderLockAlone(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	oldRecord, oldExact := stateTestRecord(t, "old record", stateRoot, 4301)
	stateTestWriteRecord(t, "old record", stateRoot, oldRecord)
	newRecord, _ := stateTestRecord(t, "replacement", stateRoot, 4302)
	var replacement []byte
	prober := &stateTestProber{results: []stateTestProbeResult{{
		exact: oldExact,
		state: identity.Dead,
		before: func() {
			stateTestHoldLock(t, "replacement lock", stateRoot)
			replacement = stateTestWriteRecord(t, "replacement record", stateRoot, newRecord)
		},
	}}}

	status, err := Read(stateRoot, prober)

	testutil.Require(t, "read error", err, nil)
	testutil.Expect(t, "state", status.State, Busy)
	data, readErr := os.ReadFile(recordPath(stateRoot))
	testutil.Require(t, "replacement read error", readErr, nil)
	testutil.Expect(t, "replacement contents", data, replacement)
}

func TestO1ReadAndStopDoNotCreateState(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		status, err := Read(stateRoot, &stateTestProber{})

		testutil.Require(t, "read error", err, nil)
		testutil.Expect(t, "read state", status.State, Stopped)
		_, statErr := os.Stat(Dir(stateRoot))
		testutil.Expect(t, "read state directory absent", errors.Is(statErr, os.ErrNotExist), true)
	})

	t.Run("stop", func(t *testing.T) {
		t.Parallel()

		stateRoot := t.TempDir()
		outcome, rec, err := Stop(stateRoot, StopOptions{Prober: &stateTestProber{}})

		testutil.Require(t, "stop error", err, nil)
		testutil.Expect(t, "stop outcome", outcome, StopOutcome(Stopped))
		testutil.Expect(t, "stop record", rec, (*Record)(nil))
		_, statErr := os.Stat(Dir(stateRoot))
		testutil.Expect(t, "stop state directory absent", errors.Is(statErr, os.ErrNotExist), true)
	})
}

func TestO2LateLockAcquisitionIsReleased(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	holder := stateTestHoldLock(t, "initial holder", stateRoot)
	f, won, done, err := waitLock(stateRoot, false, time.Second, stateTestImmediateAfter)

	testutil.Require(t, "bounded wait error", err, nil)
	testutil.Expect(t, "bounded wait won", won, false)
	testutil.Expect(t, "bounded wait file", f, (*os.File)(nil))
	holder.Release()
	<-done

	probe, probeWon, probeErr := probeLock(stateRoot)
	testutil.Require(t, "nonblocking probe error", probeErr, nil)
	testutil.Require(t, "nonblocking probe won", probeWon, true)
	releaseLock(probe)
}

type stateTestProbeResult struct {
	exact  identity.Exact
	state  identity.Liveness
	err    error
	before func()
}

type stateTestProber struct {
	mu      sync.Mutex
	results []stateTestProbeResult
	calls   []int64
}

func stateTestProberFor(exact identity.Exact, states ...identity.Liveness) *stateTestProber {
	results := make([]stateTestProbeResult, len(states))
	for index, state := range states {
		results[index] = stateTestProbeResult{exact: exact, state: state}
	}
	return &stateTestProber{results: results}
}

func (p *stateTestProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	p.mu.Lock()
	p.calls = append(p.calls, pid)
	index := len(p.calls) - 1
	if len(p.results) == 0 {
		p.mu.Unlock()
		return identity.Exact{}, identity.Unknown, errors.New("unexpected identity probe")
	}
	if index >= len(p.results) {
		index = len(p.results) - 1
	}
	result := p.results[index]
	p.mu.Unlock()
	if result.before != nil {
		result.before()
	}
	return result.exact, result.state, result.err
}

func stateTestRecord(t *testing.T, label, stateRoot string, pid int64) (Record, identity.Exact) {
	t.Helper()

	exact := stateTestExact(pid)
	process, err := identity.EncodeRef(exact.Ref())
	testutil.Require(t, label+" identity encode error", err, nil)
	return Record{
		SchemaVersion:    1,
		Process:          process,
		Address:          "127.0.0.1:0",
		Checkout:         "/work/checkout",
		Installation:     "/work/checkout/metasystem",
		StartedAt:        "2026-09-21T12:00:00Z",
		EngineBuild:      "test-build",
		ExecutableDigest: "sha256:test",
	}, exact
}

func stateTestExact(pid int64) identity.Exact {
	exact := identity.Exact{Pid: pid, StartedAt: time.Unix(1_700_000_000, 123_456_000)}
	if runtime.GOOS == "linux" {
		exact.StartTicks = 987654
		exact.BootID = "test-boot-id"
	}
	return exact
}

func stateTestWriteRecord(t *testing.T, label, stateRoot string, rec Record) []byte {
	t.Helper()

	data, err := json.Marshal(rec)
	testutil.Require(t, label+" record marshal error", err, nil)
	data = append(data, '\n')
	stateTestWriteBytes(t, label, stateRoot, data)
	return data
}

func stateTestWriteBytes(t *testing.T, label, stateRoot string, data []byte) {
	t.Helper()

	err := os.MkdirAll(Dir(stateRoot), 0o755)
	testutil.Require(t, label+" state directory error", err, nil)
	err = os.WriteFile(recordPath(stateRoot), data, 0o644)
	testutil.Require(t, label+" record write error", err, nil)
}

func stateTestCreateUnlockedLock(t *testing.T, label, stateRoot string) {
	t.Helper()

	err := os.MkdirAll(Dir(stateRoot), 0o755)
	testutil.Require(t, label+" state directory error", err, nil)
	f, err := os.OpenFile(lockPath(stateRoot), os.O_RDWR|os.O_CREATE, 0o644)
	testutil.Require(t, label+" lock create error", err, nil)
	testutil.Require(t, label+" lock close error", f.Close(), nil)
}

type stateTestLock struct {
	file *os.File
	once sync.Once
}

func stateTestHoldLock(t *testing.T, label, stateRoot string) *stateTestLock {
	t.Helper()

	err := os.MkdirAll(Dir(stateRoot), 0o755)
	testutil.Require(t, label+" state directory error", err, nil)
	f, err := os.OpenFile(lockPath(stateRoot), os.O_RDWR|os.O_CREATE, 0o644)
	testutil.Require(t, label+" lock open error", err, nil)
	testutil.Require(t, label+" lock acquisition error", unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB), nil)
	holder := &stateTestLock{file: f}
	t.Cleanup(holder.Release)
	return holder
}

func (l *stateTestLock) Release() {
	l.once.Do(func() {
		_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
		_ = l.file.Close()
	})
}

func stateTestImmediateAfter(time.Duration) <-chan time.Time {
	fired := make(chan time.Time, 1)
	fired <- time.Time{}
	return fired
}

func stateTestNeverAfter(time.Duration) <-chan time.Time {
	return make(chan time.Time)
}
