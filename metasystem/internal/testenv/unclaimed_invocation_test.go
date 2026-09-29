package testenv

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// unclaimedInvocationWitnessChild marks a child started by the witnesses
// below. If the refusal regresses, the child runs this package nested; the
// marker ends the witnesses' own copies at once, so a regression costs one
// nested run, never a chain of them.
const unclaimedInvocationWitnessChild = "METASYSTEM_TESTENV_UNCLAIMED_INVOCATION_WITNESS_CHILD"

func TestRefuseUnclaimedInvocationRefusesEveryLeadingPositionalArgument(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		args    []string
		refused bool
	}{
		{"watchdog evidence copy", []string{"pkg.test", "proof-run", "preserve", "--destination", "d", "--max-bytes", "1", "--source", "log"}, true},
		{"any other verb", []string{"pkg.test", "status"}, true},
		{"hook verb", []string{"metasystem", "hook", "stop"}, true},
		{"empty first argument", []string{"pkg.test", ""}, true},
		{"go test", []string{"pkg.test", "-test.paniconexit0", "-test.timeout=10m0s"}, false},
		{"test2json", []string{"pkg.test", "-test.v=test2json", "-test.paniconexit0"}, false},
		{"helper", []string{"pkg.test", "-test.run=^TestSupervisorProcessHelper$", "--", "setsid-child"}, false},
		{"helper flag", []string{"pkg.test", "--takeover-component-helper"}, false},
		{"no arguments", []string{"pkg.test"}, false},
		{"no argv", nil, false},
	} {
		var stderr bytes.Buffer
		code, refused := RefuseUnclaimedInvocation(test.args, &stderr)
		if refused != test.refused {
			t.Errorf("%s: refused=%t, want %t (stderr=%q)", test.name, refused, test.refused, stderr.String())
			continue
		}
		if refused && (code != 2 || !strings.Contains(stderr.String(), "no test entrypoint claims")) {
			t.Errorf("%s: code=%d stderr=%q, want exit 2 and the refusal", test.name, code, stderr.String())
		}
		if !refused && (code != 0 || stderr.Len() != 0) {
			t.Errorf("%s: code=%d stderr=%q for an invocation that is not refused", test.name, code, stderr.String())
		}
	}
}

// TestAnUnclaimedEngineVerbExitsTwoWithoutRunningThePackage drives this
// package's real test binary the way production code drives an engine it was
// handed os.Args[0] for: a verb first. testenv's TestMain claims no verb, so
// Main must refuse before it prepares anything or runs a test. The wait has
// no deadline: the refusal is immediate, and a nested package run ends in the
// package's own verdict, never in the refusal, so the assertion fails rather
// than hangs.
func TestAnUnclaimedEngineVerbExitsTwoWithoutRunningThePackage(t *testing.T) {
	t.Parallel()
	if os.Getenv(unclaimedInvocationWitnessChild) == "1" {
		return
	}
	for _, test := range []struct {
		name string
		env  []string
	}{
		{"Main", nil},
		// The setup fixture mode goes through MainWithSetup with a setup that
		// prints what it saw; the refusal comes before prepare and setup.
		{"MainWithSetup", []string{mainWithSetupFixtureEnv + "=pass"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			destination := filepath.Join(t.TempDir(), "evidence")
			command := exec.Command(os.Args[0], "proof-run", "preserve", "--destination", destination, "--max-bytes", "1", "--source", "log")
			command.Env = append(append(os.Environ(), unclaimedInvocationWitnessChild+"=1"), test.env...)
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			err := command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(output.String(), "no test entrypoint claims") {
				t.Fatalf("test binary as an engine: err=%v output=%q; want exit 2 and the refusal", err, output.String())
			}
			for _, ran := range []string{"PASS", "FAIL", "ran=true", "custodian=", "prepare test environment"} {
				if strings.Contains(output.String(), ran) {
					t.Fatalf("the test binary did work for an unclaimed engine verb (%q): %q", ran, output.String())
				}
			}
		})
	}
}
