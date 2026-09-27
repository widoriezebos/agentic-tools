package testutil

import (
	"errors"

	"golang.org/x/sys/unix"
)

// waitFixtureBedOwnerExit blocks until process pid exits, on its pidfd. It
// returns at once when the process is already gone.
func waitFixtureBedOwnerExit(pid int) {
	descriptor, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return
	}
	defer unix.Close(descriptor)
	descriptors := []unix.PollFd{{Fd: int32(descriptor), Events: unix.POLLIN}}
	for {
		// EINTR reissues the system call.
		if _, err := unix.Poll(descriptors, -1); !errors.Is(err, unix.EINTR) {
			return
		}
	}
}
