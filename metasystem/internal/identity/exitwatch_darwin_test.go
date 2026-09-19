//go:build darwin

package identity

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

type exitWatch struct {
	descriptor int
}

func armExitWatch(pid int) (*exitWatch, error) {
	descriptor, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("create kqueue: %w", err)
	}
	watch := &exitWatch{descriptor: descriptor}
	changes := []unix.Kevent_t{{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_ADD | unix.EV_ONESHOT,
		Fflags: unix.NOTE_EXIT,
	}}
	if _, err := keventNoInterrupt(descriptor, changes, nil); err != nil {
		watch.close()
		return nil, fmt.Errorf("register process exit: %w", err)
	}
	return watch, nil
}

func (watch *exitWatch) wait() error {
	defer watch.close()
	events := make([]unix.Kevent_t, 1)
	count, err := keventNoInterrupt(watch.descriptor, nil, events)
	if err != nil {
		return fmt.Errorf("wait for process exit: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("wait for process exit returned %d events", count)
	}
	return nil
}

func (watch *exitWatch) close() {
	if watch.descriptor < 0 {
		return
	}
	_ = unix.Close(watch.descriptor)
	watch.descriptor = -1
}

func keventNoInterrupt(descriptor int, changes, events []unix.Kevent_t) (int, error) {
	for {
		count, err := unix.Kevent(descriptor, changes, events, nil)
		// EINTR reissues the system call; it does not retry an assertion.
		if !errors.Is(err, unix.EINTR) {
			return count, err
		}
	}
}
