package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
)

func absentProcessGroup(t *testing.T) int64 {
	t.Helper()
	for pgid := int64(999999); pgid < 1009999; pgid++ {
		if errors.Is(unix.Kill(int(-pgid), 0), unix.ESRCH) {
			return pgid
		}
	}
	t.Fatal("could not find an absent process group for the empty-scan test")
	return 0
}

func startTestProcessGroup(t *testing.T, command *exec.Cmd) int64 {
	t.Helper()
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	return int64(command.Process.Pid)
}

func waitForGroupOwnership(t *testing.T, pgid int64, tag string, want janitor.GroupOwnershipOutcome) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if janitor.GroupOwnership(pgid, tag) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process group %d did not reach ownership outcome %s", pgid, want)
}

func TestGroupOwnedEmptyScanExitsIndeterminate(t *testing.T) {
	pgid := absentProcessGroup(t)
	if code := runIdentityGroupOwned([]string{"--pgid", fmt.Sprint(pgid), "--tag", "metasystem-job-empty-scan"}); code != 3 {
		t.Fatalf("empty group scan exit=%d, want 3 (INDETERMINATE), never 1 (NOT-OWNED)", code)
	}
}

func TestGroupOwnedLiveNonOwnerExitsNotOwned(t *testing.T) {
	tag := fmt.Sprintf("metasystem-job-not-owned-%d", os.Getpid())
	pgid := startTestProcessGroup(t, exec.Command("sleep", "30"))
	waitForGroupOwnership(t, pgid, tag, janitor.GroupNotOwned)
	if code := runIdentityGroupOwned([]string{"--pgid", fmt.Sprint(pgid), "--tag", tag}); code != 1 {
		t.Fatalf("not-owned live group scan exit=%d, want 1 (NOT-OWNED)", code)
	}
}

func TestGroupOwnedRecordedProofMismatchExitsIndeterminate(t *testing.T) {
	tag := fmt.Sprintf("metasystem-job-record-mismatch-%d", os.Getpid())
	pgid := startTestProcessGroup(t, exec.Command("sh", "-c", "exit 0"))
	waitForGroupOwnership(t, pgid, tag, janitor.GroupIndeterminate)
	if err := unix.Kill(int(-pgid), 0); err != nil && err != unix.EPERM {
		t.Fatalf("zombie-backed process group %d is not signalable: %v", pgid, err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(root, "mismatched-record.json")
	if err := os.WriteFile(record, []byte(fmt.Sprintf(
		`{"runtime":"fake","instanceTag":"different-tag","pgid":%d}`, pgid)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	if code := runIdentityGroupOwned([]string{
		"--pgid", fmt.Sprint(pgid), "--tag", tag, "--root", root, "--record", record,
	}); code != 3 {
		t.Fatalf("recorded-proof mismatch exit=%d, want 3 (INDETERMINATE)", code)
	}
}
