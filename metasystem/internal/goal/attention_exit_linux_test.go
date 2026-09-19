//go:build linux

package goal

import (
	"errors"
	"runtime"
	"testing"

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
