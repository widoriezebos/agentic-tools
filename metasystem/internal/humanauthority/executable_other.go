//go:build !linux

package humanauthority

import "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"

func observeExecutable(pid int64) (string, bool, bool, string) {
	path, known := identity.ExecutablePath(pid)
	return path, known, false, ""
}
