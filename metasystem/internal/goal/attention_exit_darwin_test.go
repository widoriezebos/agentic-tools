//go:build darwin

package goal

import (
	"errors"
	"fmt"
	"time"

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

// awaitTransportMemberExit blocks until pid has exited. A process that was
// sent SIGKILL closes its files before the kernel marks it exited, so a
// reader that saw its pipe close must wait for the exit itself rather than
// probe once: the kqueue's NOTE_EXIT is delivered when the process exits,
// and a process already gone is refused with ESRCH.
func awaitTransportMemberExit(pid int) error {
	return awaitTransportMemberExitWithin(pid, transportExitBound)
}

func awaitTransportMemberExitWithin(pid int, bound time.Duration) error {
	kqueue, err := unix.Kqueue()
	if err != nil {
		return err
	}
	defer unix.Close(kqueue)
	change := unix.Kevent_t{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_ONESHOT,
		Fflags: unix.NOTE_EXIT,
	}
	// Registered with no event list, a refused registration is the call's
	// error; the wait then reads the exit event.
	if _, err := unix.Kevent(kqueue, []unix.Kevent_t{change}, nil, nil); err != nil {
		if errors.Is(err, unix.ESRCH) {
			return nil
		}
		return err
	}
	events := make([]unix.Kevent_t, 1)
	deadline := time.Now().Add(bound)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("process %d did not exit within %s of being killed", pid, bound)
		}
		timeout := unix.NsecToTimespec(remaining.Nanoseconds())
		n, err := unix.Kevent(kqueue, nil, events, &timeout)
		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case err != nil:
			return err
		case n == 1 && events[0].Fflags&unix.NOTE_EXIT != 0:
			return nil
		}
	}
}
