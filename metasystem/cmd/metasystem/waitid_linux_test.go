//go:build linux

package main

import "golang.org/x/sys/unix"

func waitExitedWithoutReaping(pid int) error {
	var info unix.Siginfo
	return unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
}
