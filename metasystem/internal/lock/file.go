package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// Mode is the flock(2) operation File takes.
type Mode int

const (
	// Exclusive waits for the exclusive lock.
	Exclusive Mode = iota
	// Shared waits for a shared lock.
	Shared
	// TryExclusive takes the exclusive lock or fails Busy at once.
	TryExclusive
	// TryShared takes a shared lock or fails Busy at once.
	TryShared
)

func (m Mode) operation() int {
	switch m {
	case Shared:
		return unix.LOCK_SH
	case TryExclusive:
		return unix.LOCK_EX | unix.LOCK_NB
	case TryShared:
		return unix.LOCK_SH | unix.LOCK_NB
	default:
		return unix.LOCK_EX
	}
}

// FileLock is an flock(2) held on an open lock file. It is the plain
// advisory file lock beside this package's born-owning directory lock: the
// kernel drops it when the holder dies, so it needs no owner record.
type FileLock struct {
	file *os.File
}

// LockError is a failure of the flock itself, as opposed to opening the lock
// file. It reads exactly as the flock error, so wrapping it keeps a caller's
// wording, and callers tell the two failures apart with errors.As.
type LockError struct {
	Path string
	Err  error
}

func (e *LockError) Error() string { return e.Err.Error() }
func (e *LockError) Unwrap() error { return e.Err }

// File opens path read-write (created with perm when absent) and takes mode
// on it. An open failure is returned as os.OpenFile returned it; a flock
// failure closes the file and returns a *LockError. A blocking mode retries
// an interrupted flock (EINTR).
func File(path string, perm os.FileMode, mode Mode) (*FileLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(file.Fd()), mode.operation())
		if err != unix.EINTR || mode == TryExclusive || mode == TryShared {
			break
		}
	}
	if err != nil {
		_ = file.Close()
		return nil, &LockError{Path: path, Err: err}
	}
	return &FileLock{file: file}, nil
}

// File is the open lock file, for callers that keep the descriptor.
func (l *FileLock) File() *os.File { return l.file }

// Release drops the lock and closes the file.
func (l *FileLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}

// Busy reports whether err is a Try mode finding the lock held.
func Busy(err error) bool {
	return errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)
}
