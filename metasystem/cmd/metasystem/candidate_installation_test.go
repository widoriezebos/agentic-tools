package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCandidateInstallationKeepsTheInstallationPrefix pins where the static
// gate runs in a scratch candidate: under the installation's own prefix, so a
// nested installation's gate finds its go.mod (an amended unit's publication
// ran it at the candidate's top and failed with "go.mod file not found").
func TestCandidateInstallationKeepsTheInstallationPrefix(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := t.TempDir()
	connectionGit(t, repo, "init", "-q", "-b", "main")
	nested := filepath.Join(repo, "metasystem")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(t.TempDir(), "worktree")
	for _, row := range []struct {
		name, installation, want string
	}{
		{"nested", nested, filepath.Join(candidate, "metasystem")},
		{"flat", repo, candidate},
	} {
		got, err := candidateInstallation(row.installation, candidate)
		if err != nil || got != row.want {
			t.Fatalf("%s: candidateInstallation = %q, %v; want %q", row.name, got, err, row.want)
		}
	}
}
