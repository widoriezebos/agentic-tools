//go:build linux

package goal

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func transportMemberExited(pid int) (bool, error) {
	pidfd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			return true, nil
		}
		// Linux retains a reaped leader's PID as its live process group's ID.
		// pidfd_open returns EINVAL, while kill(pid, 0) distinguishes that case
		// from a live non-leader thread ID, which pidfd_open also rejects.
		if errors.Is(err, unix.EINVAL) && errors.Is(unix.Kill(pid, 0), unix.ESRCH) {
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

// awaitTransportMemberExit blocks until pid has exited. A process that was
// sent SIGKILL closes its files before the kernel marks it exited, so a
// reader that saw its pipe close must wait for the exit itself rather than
// probe once: the pidfd becomes readable exactly when the process exits.
func awaitTransportMemberExit(pid int) error {
	return awaitTransportMemberExitWithin(pid, transportExitBound)
}

func awaitTransportMemberExitWithin(pid int, bound time.Duration) error {
	pidfd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.EINVAL) && errors.Is(unix.Kill(pid, 0), unix.ESRCH) {
			return nil
		}
		return err
	}
	defer unix.Close(pidfd)
	deadline := time.Now().Add(bound)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("process %d did not exit within %s of being killed", pid, bound)
		}
		pollfds := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}
		n, err := unix.Poll(pollfds, int(remaining.Milliseconds())+1)
		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case err != nil:
			return err
		case n == 1 && pollfds[0].Revents&unix.POLLIN != 0:
			return nil
		}
	}
}

func TestTransportMemberExitedRefusesALiveThreadID(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	done := make(chan struct{}, 2)
	tids := make(chan int, 2)
	for range 2 {
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			tids <- unix.Gettid()
			<-release
			done <- struct{}{}
		}()
	}
	t.Cleanup(func() {
		close(release)
		<-done
		<-done
	})

	first, second := <-tids, <-tids
	tid := first
	if tid == unix.Getpid() {
		tid = second
	}
	if tid == unix.Getpid() {
		t.Fatalf("locked thread IDs %d and %d both equal process ID %d", first, second, unix.Getpid())
	}
	exited, err := transportMemberExited(tid)
	if err == nil || exited {
		t.Fatalf("live thread ID %d probe: exited=%t err=%v", tid, exited, err)
	}
}
