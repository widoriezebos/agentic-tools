package identity

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// NODEV is the kernel's "no controlling terminal" device value.
const noControllingTerminal = 0xffffffff

func kernelTerminal(pid int64) (bool, bool) {
	terminal, ok := kernelTerminalIdentity(pid)
	return terminal != "", ok
}

func kernelTerminalIdentity(pid int64) (string, bool) {
	kinfo, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil {
		return "", false
	}
	device := uint32(kinfo.Eproc.Tdev)
	if device == noControllingTerminal {
		return "", true
	}
	return fmt.Sprintf("darwin-tdev:%d", device), true
}
