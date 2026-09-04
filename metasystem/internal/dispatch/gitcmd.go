package dispatch

import (
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// The one git invocation helper the dispatch decisions share.

// gitOutput runs a git subcommand in a directory and returns its trimmed
// stdout. Bounded: it runs inside the locked build-record path, where
// a hung git would block dispatch and arming checkout-wide.
func gitOutput(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout strings.Builder
	command.Stdout = &stdout
	command.Stderr = io.Discard
	limit := boundedexec.Timeout(filepath.Join(dir, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(command, limit, "git "+strings.Join(args, " ")); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// projectInstallPrefix mirrors conformance's project scope derivation: an
// adopted project at the Git toplevel has no prefix, while this repository's
// nested project has the "metasystem" prefix. Keep the derivation here because
// validate imports dispatch, so sharing the validate helper would create a
// package cycle.
func projectInstallPrefix(root string) (string, error) {
	command := exec.Command("git", "-C", root, "rev-parse", "--show-prefix")
	var stdout strings.Builder
	command.Stdout = &stdout
	command.Stderr = io.Discard
	limit := boundedexec.Timeout(filepath.Join(root, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(command, limit, "git rev-parse --show-prefix"); err != nil {
		return "", err
	}
	prefix := strings.TrimSuffix(stdout.String(), "\n")
	return strings.TrimSuffix(prefix, "/"), nil
}
