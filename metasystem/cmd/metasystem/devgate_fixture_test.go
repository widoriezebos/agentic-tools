package main

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureDevgateSource stands in for cmd/devgate in a fixture installation.
// The engine builds a tree through `go run ./cmd/devgate build ARGS`; a
// fixture tree has no real engine to compile, so its build owner remains the
// fixture's own scripts/agents/go-build.sh, which this program runs with the
// arguments after `build`, passing its output and exit status through.
const fixtureDevgateSource = `package main

import (
	"errors"
	"os"
	"os/exec"
)

func main() {
	args := []string{"scripts/agents/go-build.sh"}
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

// writeFixtureDevgate gives a fixture installation the build entry the engine
// runs, delegating to the fixture's own build script. It writes a minimal
// go.mod only when the fixture has none.
func writeFixtureDevgate(t *testing.T, installationRoot string) {
	t.Helper()
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "cmd", "devgate", "main.go"), []byte(fixtureDevgateSource), 0o644)
	module := filepath.Join(installationRoot, "go.mod")
	if _, err := os.Stat(module); os.IsNotExist(err) {
		writeTestingFixtureFile(t, module, []byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.27\n"), 0o644)
	} else if err != nil {
		t.Fatal(err)
	}
}
