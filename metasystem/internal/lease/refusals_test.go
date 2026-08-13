package lease

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
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
	// cmd.Start returns after fork, BEFORE the child completes execve; in
	// that window the kernel reports an empty argv and the auth identity
	// (pid, start, command) is rightly unreadable. Under load — nested
	// gates inside full suites — the window stretches to test-visible
	// width, which was tonight's wandering nested-gate flake. Wait it out,
	// bounded.
	var s int64
	ok := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if s, ok = StartedAt(pid); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ok {
		t.Fatalf("child start unreadable after the exec window")
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

// lease-census-1 (the review): classification fails CLOSED. An unreadable
// state file or job record — anything short of genuine absence — refuses
// classification instead of silently yielding empty custody, which
// Classify would have escalated to HUMAN and RequireHolder would have
// passed through the write gate ungated.
func TestCustodyIdentitiesFailClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits cannot bite as root")
	}
	root := t.TempDir()
	supervision := filepath.Join(root, "artifacts", "agents", "supervision")
	os.MkdirAll(supervision, 0o755)

	// Absent state: supervision unarmed, classification proceeds.
	if _, _, err := custodyIdentities(root); err != nil {
		t.Fatalf("absent state must not refuse: %v", err)
	}
	// Unreadable state: refuses by name.
	statePath := filepath.Join(supervision, "state.json")
	os.WriteFile(statePath, []byte(`{"owner":null,"components":{}}`), 0o644)
	os.Chmod(statePath, 0o000)
	_, _, err := custodyIdentities(root)
	os.Chmod(statePath, 0o644)
	if err == nil || !strings.Contains(err.Error(), "supervision state is unreadable") {
		t.Fatalf("unreadable state did not refuse: %v", err)
	}

	// Corrupt job record: refuses like corrupt state.
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	os.WriteFile(filepath.Join(jobs, "j1.json"), []byte("{broken"), 0o644)
	if _, _, err := custodyIdentities(root); err == nil ||
		!strings.Contains(err.Error(), "corrupt or unidentified") {
		t.Fatalf("corrupt record did not refuse: %v", err)
	}
	os.Remove(filepath.Join(jobs, "j1.json"))

	// A record without a jobId is unidentified custody: refuses.
	os.WriteFile(filepath.Join(jobs, "j2.json"), []byte(`{"pid":1}`), 0o644)
	if _, _, err := custodyIdentities(root); err == nil {
		t.Fatal("an unidentified record did not refuse")
	}
	os.Remove(filepath.Join(jobs, "j2.json"))

	// An unreadable job record refuses by name.
	os.WriteFile(filepath.Join(jobs, "j3.json"), []byte(`{"jobId":"j3"}`), 0o644)
	os.Chmod(filepath.Join(jobs, "j3.json"), 0o000)
	_, _, err = custodyIdentities(root)
	os.Chmod(filepath.Join(jobs, "j3.json"), 0o644)
	if err == nil || !strings.Contains(err.Error(), "job record unreadable") {
		t.Fatalf("unreadable record did not refuse: %v", err)
	}
}

// lease-census-2 (the review): the sweep never certifies a generation it
// did not clear, and unprovable group ownership is never "provably not
// owned". Driven through the process-table seams.
func TestSweepFailsClosed(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	c := newClaimer(root)

	// Unparseable record: hard error, no stamp.
	os.WriteFile(filepath.Join(jobs, "bad.json"), []byte("{broken"), 0o644)
	if err := c.cleanupStaleJobs(5); err == nil ||
		!strings.Contains(err.Error(), "cannot parse job record") {
		t.Fatalf("corrupt record did not abort the sweep: %v", err)
	}
	os.Remove(filepath.Join(jobs, "bad.json"))

	// Missing claimEpoch: hard error.
	os.WriteFile(filepath.Join(jobs, "noepoch.json"), []byte(`{"jobId":"noepoch","status":"running"}`), 0o644)
	if err := c.cleanupStaleJobs(5); err == nil ||
		!strings.Contains(err.Error(), "missing or noninteger claimEpoch") {
		t.Fatalf("missing epoch did not abort: %v", err)
	}
	os.Remove(filepath.Join(jobs, "noepoch.json"))

	// Unknown status vocabulary: hard error.
	os.WriteFile(filepath.Join(jobs, "odd.json"), []byte(`{"jobId":"odd","claimEpoch":1,"status":"weird"}`), 0o644)
	if err := c.cleanupStaleJobs(5); err == nil ||
		!strings.Contains(err.Error(), "unknown status") {
		t.Fatalf("unknown status did not abort: %v", err)
	}
	os.Remove(filepath.Join(jobs, "odd.json"))

	// Unreadable record: hard error (permission bits; root disarms them).
	if os.Geteuid() != 0 {
		locked := filepath.Join(jobs, "locked.json")
		os.WriteFile(locked, []byte(`{"jobId":"locked","claimEpoch":1,"status":"running"}`), 0o644)
		os.Chmod(locked, 0o000)
		err := c.cleanupStaleJobs(5)
		os.Chmod(locked, 0o644)
		if err == nil || !strings.Contains(err.Error(), "cannot read job record") {
			t.Fatalf("unreadable record did not abort: %v", err)
		}
		os.Remove(locked)
	}
}

func TestGroupOwnsTagUnprovableRows(t *testing.T) {
	savedPids, savedPgid, savedCmd := sweepAllPids, sweepGetpgid, sweepProcessCommand
	defer func() { sweepAllPids, sweepGetpgid, sweepProcessCommand = savedPids, savedPgid, savedCmd }()

	// Process table unreadable: unprovable.
	sweepAllPids = func() ([]int64, error) { return nil, errors.New("table down") }
	if _, provable := groupOwnsTag(42, "t"); provable {
		t.Fatal("an unreadable table was ruled provable")
	}

	// A member whose pgid read fails with anything but ESRCH: unprovable.
	sweepAllPids = func() ([]int64, error) { return []int64{7}, nil }
	sweepGetpgid = func(pid int64) (int64, error) { return 0, errors.New("EIO") }
	if _, provable := groupOwnsTag(42, "t"); provable {
		t.Fatal("a failed pgid read was ruled provable")
	}

	// ESRCH is genuine absence: provable, not owned.
	sweepGetpgid = func(pid int64) (int64, error) { return 0, unix.ESRCH }
	if owned, provable := groupOwnsTag(42, "t"); !provable || owned {
		t.Fatalf("ESRCH must be provable absence: owned=%v provable=%v", owned, provable)
	}

	// A live member with unreadable identity: unprovable, never disproven.
	sweepGetpgid = func(pid int64) (int64, error) { return 42, nil }
	sweepProcessCommand = func(pid int64) (string, bool) { return "", false }
	if _, provable := groupOwnsTag(42, "t"); provable {
		t.Fatal("an unreadable member identity was ruled provable")
	}

	// A live tagged member: owned and provable.
	sweepProcessCommand = func(pid int64) (string, bool) { return "runner --tag t", true }
	if owned, provable := groupOwnsTag(42, "t"); !owned || !provable {
		t.Fatalf("a tagged member must prove ownership: owned=%v provable=%v", owned, provable)
	}
}
