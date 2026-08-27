package gaterun

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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
	holderPid := int64(holderProcess.Process.Pid)
	if result, err := AcquireExecutionGuard(root, holderPid, "long suite", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardAcquired {
		t.Fatalf("holder acquire: result=%v err=%v", result, err)
	}

	var notes bytes.Buffer
	_, err := AcquireExecutionGuard(root, int64(os.Getpid()), "dispatch", 140*time.Millisecond, 30*time.Millisecond, &notes)
	if err == nil || !strings.Contains(err.Error(), "waiting for long suite") {
		t.Fatalf("expiry did not name its holder: %v", err)
	}
	if !strings.Contains(notes.String(), "waiting for long suite") {
		t.Fatalf("bounded wait emitted no progress note: %q", notes.String())
	}

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
