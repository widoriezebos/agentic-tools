//go:build linux

package lease

import "golang.org/x/sys/unix"

// awaitUnreapedExit returns once the caller's child pid has exited and is a
// zombie, leaving it unreaped: waitid(P_PID, pid, WEXITED|WNOWAIT).
func awaitUnreapedExit(pid int) error {
	var info unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		// EINTR reissues the system call; it does not retry an assertion.
		if err != unix.EINTR {
			return err
		}
	}
}
