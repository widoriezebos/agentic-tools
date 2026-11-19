package durable

import (
	"os"
	"syscall"
)

func SyncMemoryVolumeForTest(f *os.File, fstatfs func(int, *syscall.Statfs_t) error, lstat func(string) (os.FileInfo, error), full, kernel func(*os.File) error) error {
	return syncAtWith(process.current(true), f, full, kernel, func(f *os.File) bool {
		return memoryVolumeWith(f, fstatfs, lstat)
	})
}

func SyncOperationsForTest(f *os.File, full, kernel func(*os.File) error) error {
	return syncAt(process.current(true), f, full, kernel)
}
