package lease

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// announceLiveChild spawns a child, announces it as a main (claiming the
// lease), and returns its pid and start.
func announceLiveChild(t *testing.T, root string) (pid, start int64) {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	pid = int64(cmd.Process.Pid)
	s, ok := StartedAt(pid)
	if !ok {
		t.Fatalf("child start unreadable")
	}
	if _, err := Announce(root, "child sess", pid, s, "tag", "fake", ""); err != nil {
		t.Fatalf("announce child: %v", err)
	}
	return pid, s
}

func TestRequireHolderRefusesNonHolderMain(t *testing.T) {
	root := t.TempDir()
	announceLiveChild(t, root) // the child holds the lease

	// We are also an authenticated main (announce ourselves), but a live
	// different holder keeps the lease, so our claim was refused and we are
	// OWNED-ELSEWHERE.
	self := int64(os.Getpid())
	if _, err := Announce(root, "my sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatalf("announce self: %v", err)
	}
	_, err := RequireHolder(root, self, nil)
	if err == nil || !strings.Contains(err.Error(), "OWNED-ELSEWHERE") {
		t.Fatalf("a non-holder main must be refused OWNED-ELSEWHERE, got %v", err)
	}
}

func TestRenewRefusesNonHolder(t *testing.T) {
	root := t.TempDir()
	announceLiveChild(t, root)
	self := int64(os.Getpid())
	if _, err := Announce(root, "my sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Renew(root, self); err == nil {
		t.Fatal("renew by a non-holder must be refused")
	}
}

func TestRunHeldRunsUngatedForHuman(t *testing.T) {
	root := t.TempDir() // empty: a fresh child has no recognised ancestry
	code, err := RunHeld(root, childOf(t), nil, []string{"/bin/sh", "-c", "exit 3"})
	if err != nil {
		t.Fatalf("human run-held errored: %v", err)
	}
	if code != 3 {
		t.Fatalf("human run-held should pass through the exit code, got %d", code)
	}
}

func TestRunHeldRefusesNonHolder(t *testing.T) {
	root := t.TempDir()
	announceLiveChild(t, root)
	self := int64(os.Getpid())
	if _, err := Announce(root, "my sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	// We are a MAIN but not the holder: run-held must refuse before running.
	code, err := RunHeld(root, self, nil, []string{"/bin/sh", "-c", "exit 0"})
	if err == nil || !strings.Contains(err.Error(), "OWNED-ELSEWHERE") {
		t.Fatalf("run-held should refuse a non-holder, got code=%d err=%v", code, err)
	}
}

func TestLoadLeaseAbsentAndInvalid(t *testing.T) {
	root := t.TempDir()
	if lease, err := loadLease(root, false); err != nil || lease != nil {
		t.Fatalf("absent lease, not required: want (nil,nil), got (%v,%v)", lease, err)
	}
	if _, err := loadLease(root, true); err == nil {
		t.Fatal("absent lease, required: must error")
	}
	// A structurally invalid lease is refused.
	writeJSON(t, leasePaths(root).Lease, `{"holderMainId":"","pid":0}`)
	if _, err := loadLease(root, false); err == nil {
		t.Fatal("a lease missing its holder identity must be refused")
	}
}

func TestExpectedEpochMismatchRefused(t *testing.T) {
	root := t.TempDir()
	pid, _ := announceLiveChild(t, root) // holder at epoch 1
	wrong := int64(2)
	if _, err := RequireHolder(root, pid, &wrong); err == nil || !strings.Contains(err.Error(), "claim epoch changed") {
		t.Fatalf("gating the holder on the wrong epoch must refuse, got %v", err)
	}
	// The right epoch passes.
	right := int64(1)
	if _, err := RequireHolder(root, pid, &right); err != nil {
		t.Fatalf("gating on the correct epoch should pass, got %v", err)
	}
}
