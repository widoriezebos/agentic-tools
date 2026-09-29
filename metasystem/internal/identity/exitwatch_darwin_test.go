//go:build darwin

package identity

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// darwinPPID is idtype_t P_PID from <sys/wait.h>; darwinSiginfoSize covers
// its siginfo_t (104 bytes). x/sys/unix has no Waitid for darwin, so the
// watch issues the system call itself.
const (
	darwinPPID        = 1
	darwinSiginfoSize = 128
)

type exitWatch struct {
	pid int
}

func armExitWatch(pid int) (*exitWatch, error) {
	return &exitWatch{pid: pid}, nil
}

// wait returns once the caller's child is waitable, a zombie, and leaves it
// unreaped: waitid(P_PID, pid, WEXITED|WNOWAIT), as on Linux. A kqueue
// NOTE_EXIT was earlier than that: XNU posts it while the process is still
// exiting, before it is a zombie, so a probe right after it read
// exiting=true zombie=false under load.
func (watch *exitWatch) wait() error {
	var info [darwinSiginfoSize]byte
	for {
		//lint:ignore SA1019 x/sys/unix has no libSystem waitid wrapper for darwin, and wait4 there ignores WNOWAIT and reaps
		_, _, errno := unix.Syscall6(unix.SYS_WAITID, darwinPPID, uintptr(watch.pid),
			uintptr(unsafe.Pointer(&info[0])), unix.WEXITED|unix.WNOWAIT, 0, 0)
		// EINTR reissues the system call; it does not retry an assertion.
		if errors.Is(errno, unix.EINTR) {
			continue
		}
		if errno != 0 {
			return fmt.Errorf("wait for process exit: %w", errno)
		}
		return nil
	}
}

func (*exitWatch) close() {}

func keventNoInterrupt(descriptor int, changes, events []unix.Kevent_t) (int, error) {
	for {
		count, err := unix.Kevent(descriptor, changes, events, nil)
		// EINTR reissues the system call; it does not retry an assertion.
		if !errors.Is(err, unix.EINTR) {
			return count, err
		}
	}
}
