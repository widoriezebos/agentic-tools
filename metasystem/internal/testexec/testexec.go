// Package testexec writes test executables without exposing writable file
// descriptors to concurrent forks. Do not call WriteFile or Locked from a
// Locked callback: a fork waiting for ForkLock can block another reader and
// deadlock the nested call.
package testexec

import (
	"os"
	"syscall"
)

// WriteFile has the same contract as os.WriteFile and excludes concurrent forks.
func WriteFile(name string, data []byte, perm os.FileMode) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return os.WriteFile(name, data, perm)
}

// Locked runs write while excluding concurrent forks. The callback must finish
// writing and close its descriptor before returning.
func Locked(write func() error) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return write()
}
