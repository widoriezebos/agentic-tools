package gaterun

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

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

	successor := exec.Command("sleep", "60")
	if err := successor.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = successor.Process.Kill()
		_, _ = successor.Process.Wait()
	}()
	successorPid := int64(successor.Process.Pid)
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
	if err := ReleaseExecutionGuard(root, successorPid); err != nil {
		t.Fatalf("successor release: %v", err)
	}
	if _, err := readExecutionGuardRecord(root); !os.IsNotExist(err) {
		t.Fatalf("released guard still exists: %v", err)
	}
}

func TestExecutionGuardQueuesReportsExpiryAndCleansDeadHolder(t *testing.T) {
	root := t.TempDir()
	holderProcess := exec.Command("sleep", "60")
	if err := holderProcess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = holderProcess.Process.Kill()
		_, _ = holderProcess.Process.Wait()
	}()
	holderPid := int64(holderProcess.Process.Pid)
	if result, err := AcquireExecutionGuard(root, holderPid, "long suite", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardAcquired {
		t.Fatalf("holder acquire: result=%v err=%v", result, err)
	}
	holder, err := readExecutionGuardRecord(root)
	if err != nil {
		t.Fatal(err)
	}

	realClock := clock
	realLockAcquire := executionGuardLockAcquire
	defer func() {
		clock = realClock
		executionGuardLockAcquire = realLockAcquire
	}()
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

	if err := holderProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_, _ = holderProcess.Process.Wait()
	notes.Reset()
	result, err := AcquireExecutionGuard(root, int64(os.Getpid()), "dispatch", time.Second, time.Second, &notes)
	if err != nil || result != GuardAcquired {
		t.Fatalf("stale takeover: result=%v err=%v", result, err)
	}
	if !strings.Contains(notes.String(), "removed stale holder long suite") {
		t.Fatalf("stale cleanup was silent: %q", notes.String())
	}
	if err := ReleaseExecutionGuard(root, int64(os.Getpid())); err != nil {
		t.Fatal(err)
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
