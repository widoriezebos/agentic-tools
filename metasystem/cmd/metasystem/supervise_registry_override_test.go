package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

func TestSupervisionRegistryDefaultAndRunScopedOverride(t *testing.T) {
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.DefaultPath()
	if err != nil || got != filepath.Join(home, ".metasystem", "armed-checkouts.jsonl") {
		t.Fatalf("default registry moved: %q %v", got, err)
	}
	originalHome := os.Getenv("HOME")
	override := filepath.Join(t.TempDir(), "registry-home")
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", override)
	got, err = registry.DefaultPath()
	if err != nil || got != filepath.Join(override, ".metasystem", "armed-checkouts.jsonl") {
		t.Fatalf("override did not resolve at the UserHomeDir seam: %q %v", got, err)
	}
	if os.Getenv("HOME") != originalHome {
		t.Fatal("registry override changed process HOME")
	}
}

func TestSupervisionRegistryOverrideRefusesRelativeHome(t *testing.T) {
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", "relative")
	if _, err := registry.DefaultPath(); err == nil {
		t.Fatal("relative registry home was accepted")
	}
}
