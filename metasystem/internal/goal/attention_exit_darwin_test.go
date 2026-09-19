//go:build darwin

package goal

import (
	"errors"

	"golang.org/x/sys/unix"
)

func transportMemberExited(pid int) (bool, error) {
	kqueue, err := unix.Kqueue()
	if err != nil {
		return false, err
	}
	defer unix.Close(kqueue)
	change := unix.Kevent_t{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_ADD | unix.EV_ENABLE,
		Fflags: unix.NOTE_EXIT,
	}
	if _, err := unix.Kevent(kqueue, []unix.Kevent_t{change}, nil, nil); err != nil {
		if errors.Is(err, unix.ESRCH) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
