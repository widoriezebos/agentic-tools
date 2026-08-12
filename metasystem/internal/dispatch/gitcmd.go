package dispatch

import (
	"io"
	"os/exec"
	"strings"
)

// The one git invocation helper the dispatch decisions share.

// gitOutput runs a git subcommand in a directory and returns its trimmed
// stdout.
func gitOutput(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Stderr = io.Discard
	out, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
