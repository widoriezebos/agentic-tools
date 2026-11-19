package durable

import (
	"errors"
	"os"
	"syscall"
)

func kernelSync(f *os.File) error { return kernelSyncWith(f, syscall.Fsync) }

func kernelSyncWith(f *os.File, fsync func(int) error) error {
	if f == nil {
		return os.ErrInvalid
	}
	conn, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var syncErr error
	err = conn.Control(func(fd uintptr) {
		for {
			syncErr = fsync(int(fd))
			if !errors.Is(syncErr, syscall.EINTR) {
				break
			}
		}
	})
	if err != nil {
		// Control can fail only while taking the file's descriptor reference.
		// File.Sync translates that closed-file error to os.ErrClosed without
		// issuing a syscall, including a Close racing with Control.
		return f.Sync()
	}
	if syncErr != nil {
		return &os.PathError{Op: "sync", Path: f.Name(), Err: syncErr}
	}
	return nil
}
