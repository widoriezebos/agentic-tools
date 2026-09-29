package diskstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// The bound lock (3.12 "The bound lock and the one step"): every disposer
// of evidence serializes on one host-wide lock, ~/.metasystem/stores/
// .bound.flock, taken exclusively per item: the steward's pass and disk
// clean without waiting (held is pending), evidence dispose and evidence
// export blocking (bounded by one item's critical section). The engine's
// receipt appends and goal reopen and carry take it shared around their
// write, so a line or a reopen is either seen by a judgement or waits for
// the one item in flight.

// BoundLockName is the lock's file under the machine registry.
const BoundLockName = ".bound.flock"

// BoundLockPath is the host's bound lock under the home state root.
func BoundLockPath(homeStateRoot string) string {
	return filepath.Join(MachineRegistry(homeStateRoot).Dir, BoundLockName)
}

// ErrBoundHeld is a held bound lock taken without waiting.
var ErrBoundHeld = errors.New("the evidence bound lock is held: another disposal or pass is in its step")

// BoundLock is one acquisition of the bound lock.
type BoundLock struct{ file *os.File }

// TryBoundExclusive takes the lock LOCK_EX|LOCK_NB.
func TryBoundExclusive(path string) (*BoundLock, error) {
	return takeBound(path, unix.LOCK_EX|unix.LOCK_NB)
}

// BoundExclusive takes the lock LOCK_EX, waiting for the holder's one item.
func BoundExclusive(path string) (*BoundLock, error) { return takeBound(path, unix.LOCK_EX) }

// BoundShared takes the lock LOCK_SH, waiting for an item in flight.
func BoundShared(path string) (*BoundLock, error) { return takeBound(path, unix.LOCK_SH) }

func takeBound(path string, how int) (*BoundLock, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("the bound lock must be an absolute path, got %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := flockRetry(file, how); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrBoundHeld
		}
		return nil, err
	}
	return &BoundLock{file: file}, nil
}

// Release releases the lock; a nil lock releases nothing.
func (l *BoundLock) Release() {
	if l != nil && l.file != nil {
		_ = unlockAndClose(l.file)
		l.file = nil
	}
}
