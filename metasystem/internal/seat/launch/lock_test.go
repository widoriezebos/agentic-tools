package launch

// One launch at a time on this host.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestASecondLaunchIsRefusedWithTheRunningOnesNicknameAndId(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), LockName)
	first, err := Take(path, "m1f", launchID)
	if err != nil {
		t.Fatalf("the first launch could not take the lock: %v", err)
	}
	defer func() { _ = first.Release() }()

	_, err = Take(path, "m1g", secondLaunch)
	refusal := refusalOf(t, err)
	if refusal.Code != CodeRunning {
		t.Fatalf("code = %s, want %s", refusal.Code, CodeRunning)
	}
	if !strings.Contains(refusal.Message, "m1f") || !strings.Contains(refusal.Message, launchID) {
		t.Fatalf("refusal = %q, want the running launch's nickname and id", refusal.Message)
	}
}

func TestTheLockIsGivenBackAndTakenAgain(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), LockName)
	first, err := Take(path, "m1f", launchID)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	second, err := Take(path, "m1g", secondLaunch)
	if err != nil {
		t.Fatalf("the lock was not given back: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
	// The holder's line is the new holder's, not the old one's.
	third, err := Take(path, "m1h", launchID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = third.Release() }()
	_, err = Take(path, "m1i", secondLaunch)
	if !strings.Contains(refusalOf(t, err).Message, "m1h") {
		t.Fatalf("refusal = %v, want the newest holder", err)
	}
}

func TestTheLockPathIsOneFileOnThisHost(t *testing.T) {
	t.Parallel()
	path, err := LockPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != LockName {
		t.Fatalf("lock path = %q", path)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("lock path %q is not absolute, so two checkouts could hold two locks", path)
	}
}

// A second process with another TMPDIR finds the same seat launch lock
// (Part B U1b-2, DL2-15): the lock is a shared host path, never one that
// follows TMPDIR, so a launcher whose child runs with a process-scratch
// TMPDIR still contends with every other launch on the host.
func TestTheLockPathIsTheSameWhateverTheTMPDIR(t *testing.T) {
	t.Parallel()
	if os.Getenv("SEAT_LOCK_PATH_HELPER") == "1" {
		path, err := LockPath()
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(3)
		}
		fmt.Printf("lock=%s\ntemp=%s\n", path, os.TempDir())
		return
	}
	mine, err := LockPath()
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(t.TempDir(), "nested-tmp")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestTheLockPathIsTheSameWhateverTheTMPDIR$", "-test.count=1")
	child.Env = append(os.Environ(), "SEAT_LOCK_PATH_HELPER=1", "TMPDIR="+nested)
	output, err := child.Output()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	var theirs, theirTemp string
	for _, line := range strings.Split(string(output), "\n") {
		if value, ok := strings.CutPrefix(line, "lock="); ok {
			theirs = value
		}
		if value, ok := strings.CutPrefix(line, "temp="); ok {
			theirTemp = value
		}
	}
	if theirTemp == "" || filepath.Clean(theirTemp) == filepath.Clean(os.TempDir()) {
		t.Fatalf("the helper ran with this process's TMPDIR %q; the witness proves nothing", theirTemp)
	}
	if theirs != mine {
		t.Fatalf("the seat launch lock differs: this process %s, a process with TMPDIR %s: %s", mine, theirTemp, theirs)
	}
}
