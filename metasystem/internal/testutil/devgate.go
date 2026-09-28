package testutil

import (
	"os"
	"path/filepath"
)

// FixtureDevgateSource stands in for cmd/devgate in a fixture installation.
// The engine builds a tree through `go run ./cmd/devgate build ARGS` and the
// commit boundary re-proves it through `go run ./cmd/devgate static ARGS`; a
// fixture tree has no real engine to compile, so its build owner remains the
// fixture's own scripts/agents/go-build.sh and any other action runs the
// fixture's scripts/agents/devgate-ACTION.sh, each with the arguments after
// the action, passing output and exit status through.
const FixtureDevgateSource = `package main

import (
	"errors"
	"os"
	"os/exec"
)

func main() {
	args := []string{"scripts/agents/go-build.sh"}
	if len(os.Args) > 1 && os.Args[1] != "build" {
		args = []string{"scripts/agents/devgate-" + os.Args[1] + ".sh"}
	}
	if len(os.Args) > 2 {
		args = append(args, os.Args[2:]...)
	}
	command := exec.Command("bash", args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		os.Stderr.WriteString("fixture devgate: " + err.Error() + "\n")
		os.Exit(1)
	}
}
`

// FixtureDevgateModule is the go.mod a stand-in writes when the fixture has
// none: the engine module's line, so a caller that checks the module
// identity takes the Go path.
const FixtureDevgateModule = "module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.27\n"

// WriteFixtureDevgate plants the stand-in and, when the installation has
// none, the minimal go.mod.
func WriteFixtureDevgate(installationRoot string) error {
	dir := filepath.Join(installationRoot, "cmd", "devgate")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(FixtureDevgateSource), 0o644); err != nil {
		return err
	}
	module := filepath.Join(installationRoot, "go.mod")
	if _, err := os.Stat(module); os.IsNotExist(err) {
		return os.WriteFile(module, []byte(FixtureDevgateModule), 0o644)
	} else if err != nil {
		return err
	}
	return nil
}
