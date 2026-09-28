package lock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileTakesAndReleasesAnExclusiveLock(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "held.lock")
	held, err := File(path, 0o600, Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the lock file is created with the named mode: %v %v", info, err)
	}
	_, err = File(path, 0o600, TryExclusive)
	var lockErr *LockError
	if !Busy(err) || !errors.As(err, &lockErr) {
		t.Fatalf("a second exclusive try must be busy and a LockError: %v", err)
	}
	if lockErr.Error() != lockErr.Err.Error() {
		t.Fatalf("a LockError reads as the flock error itself: %q", lockErr.Error())
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := File(path, 0o600, TryExclusive)
	if err != nil {
		t.Fatalf("after release the lock is free: %v", err)
	}
	_ = again.Release()
}

func TestFileSharesASharedLock(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shared.lock")
	first, err := File(path, 0o644, Shared)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	second, err := File(path, 0o644, TryShared)
	if err != nil {
		t.Fatalf("two shared holders coexist: %v", err)
	}
	defer second.Release()
	if _, err := File(path, 0o644, TryExclusive); !Busy(err) {
		t.Fatalf("an exclusive try under shared holders is busy: %v", err)
	}
}

func TestFileOpenFailureIsNotALockError(t *testing.T) {
	t.Parallel()
	_, err := File(filepath.Join(t.TempDir(), "missing", "x.lock"), 0o600, Exclusive)
	var lockErr *LockError
	if err == nil || errors.As(err, &lockErr) || Busy(err) {
		t.Fatalf("an open failure is the open's own error: %v", err)
	}
}
