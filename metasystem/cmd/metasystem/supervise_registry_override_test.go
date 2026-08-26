package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSupervisionRegistryDefaultAndRunScopedOverride(t *testing.T) {
	t.Setenv(supervisionRegistryHomeEnv, "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := defaultRegistryPath()
	if err != nil || got != filepath.Join(home, ".metasystem", "armed-checkouts.jsonl") {
		t.Fatalf("default registry moved: %q %v", got, err)
	}
	originalHome := os.Getenv("HOME")
	override := filepath.Join(t.TempDir(), "registry-home")
	t.Setenv(supervisionRegistryHomeEnv, override)
	got, err = defaultRegistryPath()
	if err != nil || got != filepath.Join(override, ".metasystem", "armed-checkouts.jsonl") {
		t.Fatalf("override did not resolve at the UserHomeDir seam: %q %v", got, err)
	}
	if os.Getenv("HOME") != originalHome {
		t.Fatal("registry override changed process HOME")
	}
}

func TestSupervisionRegistryOverrideRefusesRelativeHome(t *testing.T) {
	t.Setenv(supervisionRegistryHomeEnv, "relative")
	if _, err := defaultRegistryPath(); err == nil {
		t.Fatal("relative registry home was accepted")
	}
}
