package testenv

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// IsolateMachine runs a test in its own process and machine-state home.
// Enrollment is shared by every checkout on a machine, so parallel fixtures
// that enroll different terminals must use different machine homes.
func IsolateMachine(t *testing.T) bool {
	t.Helper()
	name := t.Name()
	owned := os.Getenv("METASYSTEM_TEST_MACHINE")
	if owned != "" && (owned == name || strings.HasPrefix(name, owned+"/")) {
		return false
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(name)+"$", "-test.count=1", "-test.timeout=30m", "-test.v")
	for _, entry := range WithoutInheritedControls(os.Environ()) {
		name, _, _ := strings.Cut(entry, "=")
		if name != supervisionRegistryHome && name != registryOwnerNonce && name != hostTempRootEnv && name != "METASYSTEM_FIXTURE_OWNER" {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "METASYSTEM_TEST_MACHINE="+name)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatalf("isolated machine: %v", err)
	}
	return true
}
