package proofrun

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ErrMutationHeld is TryAcquireMutation's answer while another holder has
// the proof mutation lock.
var ErrMutationHeld = errors.New("the proof mutation lock is held")

// TryAcquireMutation takes the proof mutation lock without waiting: a caller
// with a budget (a runtime hook) must never queue behind a proof run.
func TryAcquireMutation(root string) (*MutationLock, error) {
	path := filepath.Join(root, "artifacts", "agents", "locks", "proof-runs.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrMutationHeld
		}
		return nil, fmt.Errorf("lock proof mutation for %s: %w", root, err)
	}
	return &MutationLock{file: file}, nil
}

// Handoff gives the locked file to a child process: the caller passes it
// (for example as an inherited descriptor) and then closes its own copy
// without unlocking, so the lock is held until the child's copy closes. The
// lock value is spent afterwards.
func (lock *MutationLock) Handoff() *os.File {
	if lock == nil {
		return nil
	}
	file := lock.file
	lock.file = nil
	return file
}
