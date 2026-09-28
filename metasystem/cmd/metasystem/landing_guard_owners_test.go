package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLandingGuardObservationOwnerNamesItsFailure ports the storage legs of
// scripts/agents/pre-commit-guard-fixtures.sh against the production
// observation owner: a file blocking the artifacts directory is a directory
// failure, a directory at the log path is a write failure, and a writable
// installation appends the line.
func TestLandingGuardObservationOwnerNamesItsFailure(t *testing.T) {
	t.Parallel()
	appendObservation := landingGuardOwners().AppendObservation
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "artifacts"), []byte("blocks observation directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendObservation(root, "line"); err == nil || err.Error() != "its observation directory could not be created" {
		t.Fatalf("blocked directory: %v", err)
	}

	root = t.TempDir()
	log := filepath.Join(root, "artifacts", "agents", "landing-observe.log")
	if err := os.MkdirAll(log, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appendObservation(root, "line"); err == nil || err.Error() != "its observation could not be written" {
		t.Fatalf("directory at the log path: %v", err)
	}

	root = t.TempDir()
	for _, line := range []string{"first", "second"} {
		if err := appendObservation(root, line); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-observe.log"))
	if err != nil || string(data) != "first\nsecond\n" {
		t.Fatalf("observation log %q: %v", data, err)
	}
}
