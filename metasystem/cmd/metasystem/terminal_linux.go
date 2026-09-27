package main

import "golang.org/x/sys/unix"

// isTerminal is the shell's [[ -t FD ]]: the descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}
