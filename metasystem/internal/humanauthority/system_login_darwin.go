//go:build darwin

package humanauthority

const systemLoginProgram = "/usr/bin/login"

func isSystemLoginProgram(executable string) bool {
	// Only this System Integrity Protection-protected image may cross the
	// argument-read boundary: agent runtimes execute as the human, while this
	// setuid-root operating-system login process cannot be an agent runtime.
	return executable == systemLoginProgram
}
