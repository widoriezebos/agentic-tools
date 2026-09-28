package testutil

import (
	"errors"

	"golang.org/x/sys/unix"
)

// waitFixtureBedOwnerExit blocks until process pid exits, on its kernel exit
// event. It returns at once when the process is already gone.
func waitFixtureBedOwnerExit(pid int) {
	queue, err := unix.Kqueue()
	if err != nil {
		return
	}
	defer unix.Close(queue)
	change := []unix.Kevent_t{{Ident: uint64(pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT}}
	events := make([]unix.Kevent_t, 1)
	for {
		// EINTR reissues the system call; the one-shot registration is idempotent.
		_, err := unix.Kevent(queue, change, events, nil)
		if !errors.Is(err, unix.EINTR) {
			return
		}
	}
}
