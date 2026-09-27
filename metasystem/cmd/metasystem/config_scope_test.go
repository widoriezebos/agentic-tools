package main

import (
	"path/filepath"
	"testing"
)

// A configuration outside Git is validated against its own directory.
func TestConfigRepositoryScopeOutsideGitIsTheConfigurationDirectory(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	want, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got := configRepositoryScope(filepath.Join(directory, "metasystem.conf")); got != want {
		t.Fatalf("scope=%q want %q", got, want)
	}
}
