//go:build linux

package goal

import (
	"errors"

	"golang.org/x/sys/unix"
)

func transportMemberExited(pid int) (bool, error) {
	pidfd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			return true, nil
		}
		return false, err
	}
	defer unix.Close(pidfd)
	pollfds := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}
	if _, err := unix.Poll(pollfds, 0); err != nil {
		return false, err
	}
	return pollfds[0].Revents&unix.POLLIN != 0, nil
}
