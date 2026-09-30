//go:build darwin || linux

package textui

import "golang.org/x/sys/unix"

// TerminalWidth is the terminal's column count, 0 when the descriptor is
// not a terminal or does not say.
func TerminalWidth(fd uintptr) int {
	size, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return 0
	}
	return int(size.Col)
}
