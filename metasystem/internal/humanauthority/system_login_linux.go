//go:build linux

package humanauthority

func isSystemLoginProgram(string) bool {
	// Linux has no system login program admitted across an unreadable argument
	// boundary; every process must provide readable arguments on this platform.
	return false
}
