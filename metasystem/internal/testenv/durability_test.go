package testenv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
)

const durabilityOptedFixtureEnv = "TESTENV_DURABILITY_OPTED_FIXTURE"

func durabilityOptedFixtureSetup() error {
	fmt.Println("setup ran")
	return nil
}

// The harness lowers its own test process to kernel after prepare.
func TestMainLowersTestProcessDurability(t *testing.T) {
	t.Parallel()
	if got := durable.Current(); got != "kernel" {
		t.Fatalf("test process level = %q, want kernel", got)
	}
}

// A refused opt-in stops the binary before setup and tests, and cleans up.
func TestMainWithSetupStopsOnRefusedDurabilityOptIn(t *testing.T) {
	t.Parallel()
	scratch := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestMainLowersTestProcessDurability$", "-test.count=1", "-test.v")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != "TMPDIR" && name != durabilityOptedFixtureEnv {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "TMPDIR="+scratch, durabilityOptedFixtureEnv+"=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("refused opt-in exit = %v, want status 2\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "prepare test durability:") {
		t.Fatalf("refused opt-in not reported:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "setup ran") || strings.Contains(stdout.String(), "=== RUN") {
		t.Fatalf("setup or tests ran after a refused opt-in:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "tmpdir-after="+scratch+"\n") {
		t.Fatalf("refused opt-in did not clean up the prepared namespace:\n%s", stdout.String())
	}
}
