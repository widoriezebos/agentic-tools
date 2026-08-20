package identity

import "golang.org/x/sys/unix"

// NODEV is the kernel's "no controlling terminal" device value.
const noControllingTerminal = 0xffffffff

func kernelTerminal(pid int64) (bool, bool) {
	kinfo, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil {
		return false, false
	}
	return uint32(kinfo.Eproc.Tdev) != noControllingTerminal, true
}
