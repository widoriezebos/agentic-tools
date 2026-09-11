package main

import (
	"os"
	"strings"
	"testing"
)

// The race gate arms a witness and runs inside a proof attempt, then runs
// this package's tests; the engine paths those tests exercise in-process read
// the same ambient controls (witness handoff, suite progress parent, proof
// attempt) and would take this package's tests for nested validations of
// the outer run. A test of this binary starts from a plain shell.
func TestMain(m *testing.M) {
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "METASYSTEM_GATE_WITNESS") || strings.HasPrefix(key, "METASYSTEM_SUITE_PROGRESS_") || strings.HasPrefix(key, "METASYSTEM_PROOF_") {
			os.Unsetenv(key)
		}
	}
	os.Exit(m.Run())
}
