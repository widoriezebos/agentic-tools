package textui

import "golang.org/x/sys/unix"

// IsTerminal is the shell's [[ -t FD ]]: the descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}
