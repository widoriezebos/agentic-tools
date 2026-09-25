package contract

import (
	"os"
	"path/filepath"
	"testing"
)

// newContractFiles keeps the validation inputs on disk without creating a Git repository.
func newContractFiles(t *testing.T, expectedRepositoryCalls int) (string, func(string) (string, error)) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	repo = resolvePath(repo)
	writeFileMode(t, filepath.Join(repo, "scripts", "gate.sh"),
		"#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\n'\n", 0o755)
	writeFileMode(t, filepath.Join(repo, "truth", "reference.txt"), "certified truth\n", 0o644)
	writeFileMode(t, filepath.Join(repo, "docs", "project-rules.md"), projectRules, 0o644)
	writeFileMode(t, filepath.Join(repo, "scripts", "agents", "arm-supervision.sh"),
		"#!/usr/bin/env bash\nprintf 'fixture-fingerprint\\n'\n", 0o755)
	contractPath := filepath.Join(repo, "plans", "mission-alpha.contract.md")
	writeFileMode(t, contractPath, sealableContract(), 0o644)
	resolvedPath := resolvePath(contractPath)
	if filepath.Dir(filepath.Dir(resolvedPath)) != repo {
		t.Fatalf("contract path %q is outside declared repository root %q", resolvedPath, repo)
	}
	calls := 0
	t.Cleanup(func() {
		if calls != expectedRepositoryCalls {
			t.Errorf("repository resolver called %d times, want %d", calls, expectedRepositoryCalls)
		}
	})
	return contractPath, func(path string) (string, error) {
		t.Helper()
		calls++
		if path != resolvedPath {
			t.Fatalf("repository resolver got path %q, want %q", path, resolvedPath)
		}
		return repo, nil
	}
}
