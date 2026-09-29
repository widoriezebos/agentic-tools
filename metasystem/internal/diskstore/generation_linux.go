package diskstore

import (
	"os"
	"syscall"
	"unsafe"
)

// fsIocGetversion is FS_IOC_GETVERSION, _IOR('v', 1, long).
const fsIocGetversion = 0x80087601

// fileGeneration is the open file's inode generation (FS_IOC_GETVERSION),
// or 0 when the file system does not report one.
func fileGeneration(file *os.File) uint64 {
	var generation [2]uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), fsIocGetversion, uintptr(unsafe.Pointer(&generation[0]))); errno != 0 {
		return 0
	}
	return uint64(generation[0])
}
