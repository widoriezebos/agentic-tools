//go:build darwin

package lease

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/unix"
)

// awaitUnreapedExit returns once the caller's child pid has exited and is a
// zombie, leaving it unreaped: waitid(P_PID, pid, WEXITED|WNOWAIT). x/sys/unix
// has no Waitid for darwin and wait4 there ignores WNOWAIT, so the call is
// issued directly (P_PID is 1; the siginfo_t buffer covers its 104 bytes).
func awaitUnreapedExit(pid int) error {
	var info [128]byte
	for {
		//lint:ignore SA1019 x/sys/unix has no libSystem waitid wrapper for darwin, and wait4 there ignores WNOWAIT and reaps
		_, _, errno := unix.Syscall6(unix.SYS_WAITID, 1, uintptr(pid),
			uintptr(unsafe.Pointer(&info[0])), unix.WEXITED|unix.WNOWAIT, 0, 0)
		// EINTR reissues the system call; it does not retry an assertion.
		if errors.Is(errno, unix.EINTR) {
			continue
		}
		if errno != 0 {
			return errno
		}
		return nil
	}
}
