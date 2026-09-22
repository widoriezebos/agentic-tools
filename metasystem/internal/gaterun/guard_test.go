package gaterun

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type guardHeldProcess struct {
	command *exec.Cmd
	release *os.File
	joined  bool
}

func startGuardHeldProcess(t *testing.T, label string) *guardHeldProcess {
	t.Helper()
	releaseReader, releaseWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("sh", "-c", "read -r _ <&3 || :", label)
	command.ExtraFiles = []*os.File{releaseReader}
	if err := command.Start(); err != nil {
		_ = releaseReader.Close()
		_ = releaseWriter.Close()
		t.Fatal(err)
	}
	_ = releaseReader.Close()
	process := &guardHeldProcess{command: command, release: releaseWriter}
	t.Cleanup(func() {
		if err := process.releaseAndJoin(); err != nil {
			t.Errorf("release held guard process %q: %v", label, err)
		}
	})
	return process
}

func (p *guardHeldProcess) releaseAndJoin() error {
	if p.joined {
		return nil
	}
	closeErr := p.release.Close()
	waitErr := p.command.Wait()
	p.joined = true
	return errors.Join(closeErr, waitErr)
}

func (p *guardHeldProcess) killAndJoin() (error, error) {
	if p.joined {
		return nil, nil
	}
	killErr := p.command.Process.Kill()
	_ = p.release.Close()
	waitErr := p.command.Wait()
	p.joined = true
	return killErr, waitErr
}

type guardAcquisitionAttempt struct {
	contender lock.Identity
	holder    lock.Identity
	release   chan struct{}
}

type guardAcquisitionResult struct {
	pid    int64
	result GuardResult
	notes  string
	err    error
}

type controlledGuardAcquirer struct {
	realAcquire func(string, lock.Identity, lock.Options) (*lock.Lock, error)
	observed    chan guardAcquisitionAttempt
	results     chan guardAcquisitionResult
	cancel      chan struct{}
	cancelOnce  sync.Once
	workers     sync.WaitGroup
}

func installControlledGuardAcquirer(t *testing.T) *controlledGuardAcquirer {
	t.Helper()
	harness := &controlledGuardAcquirer{
		realAcquire: executionGuardLockAcquire,
		observed:    make(chan guardAcquisitionAttempt),
		results:     make(chan guardAcquisitionResult, 2),
		cancel:      make(chan struct{}),
	}
	executionGuardLockAcquire = func(path string, contender lock.Identity, opts lock.Options) (*lock.Lock, error) {
		base := opts.Now()
		reads := 0
		oneShot := opts
		oneShot.Now = func() time.Time {
			reads++
			if reads == 1 {
				return base
			}
			return base.Add(opts.Wait + time.Nanosecond)
		}
		oneShot.Sleep = func(time.Duration) {}
		acquired, acquireErr := harness.realAcquire(path, contender, oneShot)
		var holderErr *lock.HolderError
		if !errors.As(acquireErr, &holderErr) {
			return acquired, acquireErr
		}
		attempt := guardAcquisitionAttempt{contender: contender, holder: holderErr.Holder, release: make(chan struct{})}
		select {
		case harness.observed <- attempt:
		case <-harness.cancel:
			return nil, errors.New("controlled guard acquisition cancelled")
		}
		select {
		case <-attempt.release:
			return nil, acquireErr
		case <-harness.cancel:
			return nil, errors.New("controlled guard acquisition cancelled")
		}
	}
	t.Cleanup(func() {
		harness.cancelOnce.Do(func() { close(harness.cancel) })
		harness.workers.Wait()
		executionGuardLockAcquire = harness.realAcquire
	})
	return harness
}

func (h *controlledGuardAcquirer) acquire(root string, pid int64, owner string, wait time.Duration) {
	h.workers.Add(1)
	go func() {
		defer h.workers.Done()
		var notes bytes.Buffer
		result, err := AcquireExecutionGuard(root, pid, owner, wait, time.Second, &notes)
		h.results <- guardAcquisitionResult{pid: pid, result: result, notes: notes.String(), err: err}
	}()
}

func requireBlockedGuardAttempt(t *testing.T, harness *controlledGuardAcquirer) guardAcquisitionAttempt {
	t.Helper()
	select {
	case attempt := <-harness.observed:
		return attempt
	case result := <-harness.results:
		t.Fatalf("guard acquisition returned before its blocked attempt was released: pid=%d result=%v err=%v", result.pid, result.result, result.err)
		return guardAcquisitionAttempt{}
	}
}

func TestExecutionGuardRegistersSpawnedMemberUntilLastRelease(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	result, err := AcquireExecutionGuard(root, self, "suite", time.Second, time.Second, &bytes.Buffer{})
	if err != nil || result != GuardAcquired {
		t.Fatalf("acquire: result=%v err=%v", result, err)
	}
	result, err = AcquireExecutionGuard(root, self, "nested validate", time.Second, time.Second, &bytes.Buffer{})
	if err != nil || result != GuardJoined {
		t.Fatalf("exact member join: result=%v err=%v", result, err)
	}

	successor := startGuardHeldProcess(t, "dispatch-supervisor")
	successorPid := int64(successor.command.Process.Pid)
	if err := RegisterSpawnedExecutionGuardMember(root, successorPid, "dispatch supervisor"); err != nil {
		t.Fatalf("spawn registration: %v", err)
	}
	if err := ReleaseExecutionGuard(root, self); err != nil {
		t.Fatalf("suite release: %v", err)
	}
	record, err := readExecutionGuardRecord(root)
	if err != nil || len(record.Members) != 1 || record.Members[0].Pid != successorPid {
		t.Fatalf("registered successor lifetime: %+v err=%v", record.Members, err)
	}

	realClock := clock
	frozen := time.Date(2026, time.September, 18, 13, 0, 0, 0, time.UTC)
	clock = func() time.Time { return frozen }
	t.Cleanup(func() { clock = realClock })
	harness := installControlledGuardAcquirer(t)
	harness.acquire(root, self, "foreign validation", time.Second)
	attempt := requireBlockedGuardAttempt(t, harness)
	if attempt.holder.Pid != successorPid || attempt.contender.Pid != self {
		t.Fatalf("foreign validation block = contender %d holder %d, want contender %d behind detached member %d", attempt.contender.Pid, attempt.holder.Pid, self, successorPid)
	}
	if err := ReleaseExecutionGuard(root, successorPid); err != nil {
		t.Fatalf("successor release: %v", err)
	}
	close(attempt.release)
	got := <-harness.results
	if got.err != nil || got.result != GuardAcquired {
		t.Fatalf("foreign validation after detached release: result=%v err=%v", got.result, got.err)
	}
	if err := ReleaseExecutionGuard(root, self); err != nil {
		t.Fatalf("foreign validation release: %v", err)
	}
	if _, err := readExecutionGuardRecord(root); !os.IsNotExist(err) {
		t.Fatalf("released foreign guard still exists: %v", err)
	}
	if err := successor.releaseAndJoin(); err != nil {
		t.Fatalf("detached successor release: %v", err)
	}
}

func TestExecutionGuardQueuesReportsExpiryAndCleansDeadHolder(t *testing.T) {
	root := t.TempDir()
	holderProcess := startGuardHeldProcess(t, "long-suite")
	holderPid := int64(holderProcess.command.Process.Pid)
	if result, err := AcquireExecutionGuard(root, holderPid, "long suite", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardAcquired {
		t.Fatalf("holder acquire: result=%v err=%v", result, err)
	}
	holder, err := readExecutionGuardRecord(root)
	if err != nil {
		t.Fatal(err)
	}

	realClock := clock
	realLockAcquire := executionGuardLockAcquire
	t.Cleanup(func() {
		clock = realClock
		executionGuardLockAcquire = realLockAcquire
	})
	fakeStart := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	fakeNow := fakeStart
	clockReads := 0
	var lockWaits []time.Duration
	clock = func() time.Time {
		clockReads++
		if clockReads > 12 {
			t.Fatalf("fake clock read %d times after lock waits %v; guard bypassed its injected lock wait", clockReads, lockWaits)
		}
		return fakeNow
	}
	executionGuardLockAcquire = func(path string, self lock.Identity, opts lock.Options) (*lock.Lock, error) {
		lockWaits = append(lockWaits, opts.Wait)
		if len(lockWaits) > 2 {
			t.Fatalf("injected lock wait called %d times; observed waits %v", len(lockWaits), lockWaits)
		}
		if opts.Now == nil || !opts.Now().Equal(fakeNow) {
			t.Fatalf("lock wait did not receive the guard clock at %s", fakeNow)
		}
		fakeNow = fakeNow.Add(opts.Wait)
		return nil, &lock.HolderError{Path: path, Holder: holder.identity(), State: lock.Alive}
	}

	var notes bytes.Buffer
	_, err = AcquireExecutionGuard(root, int64(os.Getpid()), "dispatch", 140*time.Millisecond, 30*time.Millisecond, &notes)
	if err == nil || !strings.Contains(err.Error(), "waiting for long suite") {
		t.Fatalf("expiry did not name its holder: %v", err)
	}
	if !strings.Contains(notes.String(), "waiting for long suite") {
		t.Fatalf("bounded wait emitted no progress note: %q", notes.String())
	}
	if len(lockWaits) != 2 || lockWaits[0] != 100*time.Millisecond || lockWaits[1] != 40*time.Millisecond {
		t.Fatalf("lock waits = %v; want 100ms then 40ms", lockWaits)
	}
	if elapsed := fakeNow.Sub(fakeStart); elapsed != 140*time.Millisecond {
		t.Fatalf("fake elapsed = %s; want 140ms", elapsed)
	}
	clock = realClock
	executionGuardLockAcquire = realLockAcquire

	frozen := fakeNow
	clock = func() time.Time { return frozen }
	harness := installControlledGuardAcquirer(t)
	contenders := make(map[int64]*guardHeldProcess, 2)
	owners := make(map[int64]string, 2)
	for _, owner := range []string{"dispatch one", "dispatch two"} {
		process := startGuardHeldProcess(t, owner)
		pid := int64(process.command.Process.Pid)
		contenders[pid] = process
		owners[pid] = owner
		harness.acquire(root, pid, owner, 5*time.Second)
	}
	blockedByLiveHolder := make(map[int64]guardAcquisitionAttempt, 2)
	for len(blockedByLiveHolder) < 2 {
		attempt := requireBlockedGuardAttempt(t, harness)
		if attempt.holder.Pid != holderPid {
			t.Fatalf("initial contender %d blocked behind pid %d, want live holder %d", attempt.contender.Pid, attempt.holder.Pid, holderPid)
		}
		if _, duplicate := blockedByLiveHolder[attempt.contender.Pid]; duplicate {
			t.Fatalf("contender %d reported two blocked attempts before controller release", attempt.contender.Pid)
		}
		blockedByLiveHolder[attempt.contender.Pid] = attempt
	}
	if killErr, waitErr := holderProcess.killAndJoin(); killErr != nil || waitErr == nil {
		t.Fatalf("make guard holder stale: kill=%v wait=%v", killErr, waitErr)
	}
	winnerPid := int64(0)
	for pid := range contenders {
		if winnerPid == 0 || owners[pid] < owners[winnerPid] {
			winnerPid = pid
		}
	}
	close(blockedByLiveHolder[winnerPid].release)
	first := <-harness.results
	if first.err != nil || first.result != GuardAcquired {
		t.Fatalf("first stale takeover: pid=%d result=%v err=%v", first.pid, first.result, first.err)
	}
	if first.pid != winnerPid {
		t.Fatalf("stale takeover winner = %d, want controller-released contender %d", first.pid, winnerPid)
	}
	loserPid := int64(0)
	for pid := range contenders {
		if pid != winnerPid {
			loserPid = pid
		}
	}
	close(blockedByLiveHolder[loserPid].release)
	loserAttempt := requireBlockedGuardAttempt(t, harness)
	if loserAttempt.contender.Pid != loserPid || loserAttempt.holder.Pid != winnerPid {
		t.Fatalf("loser block = contender %d holder %d, want contender %d behind new owner %d", loserAttempt.contender.Pid, loserAttempt.holder.Pid, loserPid, winnerPid)
	}
	current, err := readExecutionGuardRecord(root)
	if err != nil || current.Pid != winnerPid {
		t.Fatalf("second contender did not remain behind first owner %d: record=%+v err=%v", winnerPid, current, err)
	}
	if err := ReleaseExecutionGuard(root, winnerPid); err != nil {
		t.Fatal(err)
	}
	if err := contenders[winnerPid].releaseAndJoin(); err != nil {
		t.Fatalf("release first contender process: %v", err)
	}
	close(loserAttempt.release)
	second := <-harness.results
	if second.pid != loserPid || second.err != nil || second.result != GuardAcquired {
		t.Fatalf("second takeover after release: pid=%d result=%v err=%v", second.pid, second.result, second.err)
	}
	if strings.Count(first.notes+second.notes, "removed stale holder long suite") != 1 {
		t.Fatalf("stale cleanup note count != 1: first=%q second=%q", first.notes, second.notes)
	}
	if err := ReleaseExecutionGuard(root, loserPid); err != nil {
		t.Fatal(err)
	}
	if _, err := readExecutionGuardRecord(root); !os.IsNotExist(err) {
		t.Fatalf("released contender guard still exists: %v", err)
	}
	if err := contenders[loserPid].releaseAndJoin(); err != nil {
		t.Fatalf("release second contender process: %v", err)
	}
}

func TestExecutionGuardExemptsExactAncestorAndRejectsUnregisteredRelease(t *testing.T) {
	root := t.TempDir()
	parent := int64(os.Getppid())
	if result, err := AcquireExecutionGuard(root, parent, "parent suite", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardAcquired {
		t.Fatalf("parent acquire: result=%v err=%v", result, err)
	}
	if result, err := AcquireExecutionGuard(root, int64(os.Getpid()), "child dispatch", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardJoined {
		t.Fatalf("ancestor join: result=%v err=%v", result, err)
	}
	if err := ReleaseExecutionGuard(root, int64(os.Getpid())); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseExecutionGuard(root, int64(os.Getpid())); err == nil {
		t.Fatal("an unregistered caller released the guard")
	}
	if err := ReleaseExecutionGuard(root, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireExecutionGuard(root, int64(os.Getpid()), "", time.Second, time.Second, &bytes.Buffer{}); err == nil {
		t.Fatal("empty owner was accepted")
	}
	if _, err := AcquireExecutionGuard(root, int64(os.Getpid()), "owner", 0, time.Second, &bytes.Buffer{}); err == nil {
		t.Fatal("zero wait was accepted")
	}
	if _, err := (executionGuardCodec{}).Decode([]byte(`{"pid":1}`)); err == nil {
		t.Fatal("incomplete owner record was accepted")
	}
	if _, err := (executionGuardCodec{}).Decode([]byte(`not-json`)); err == nil {
		t.Fatal("malformed owner record was accepted")
	}
}
