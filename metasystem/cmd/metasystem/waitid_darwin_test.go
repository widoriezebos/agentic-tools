//go:build darwin

package main

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func waitExitedWithoutReaping(pid int) error {
	const processID = 1
	var info [128]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, processID, uintptr(pid),
		uintptr(unsafe.Pointer(&info[0])), uintptr(unix.WEXITED|unix.WNOWAIT), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
