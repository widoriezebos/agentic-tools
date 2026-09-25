package launch

// One launch at a time on this host.

import (
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
	if filepath.Base(LockPath()) != LockName {
		t.Fatalf("lock path = %q", LockPath())
	}
	if !filepath.IsAbs(LockPath()) {
		t.Fatalf("lock path %q is not absolute, so two checkouts could hold two locks", LockPath())
	}
}
