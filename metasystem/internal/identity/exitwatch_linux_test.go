//go:build linux

package identity

import (
	"errors"

	"golang.org/x/sys/unix"
)

type exitWatch struct {
	pid int
}

func armExitWatch(pid int) (*exitWatch, error) {
	return &exitWatch{pid: pid}, nil
}

func (watch *exitWatch) wait() error {
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PID, watch.pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		// EINTR reissues the system call; it does not retry an assertion.
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

func (*exitWatch) close() {}
