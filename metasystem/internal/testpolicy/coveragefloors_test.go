package testpolicy

import (
	"os"
	"path/filepath"
	"testing"
)

// The coverage floors are this repository's development test policy and
// live beside testing.json: one file per measured platform.
func TestCoverageFloorsLiveBesideTheTestingContract(t *testing.T) {
	t.Parallel()
	if got := CoverageFloorsFile("darwin"); got != "testing-coverage-floors.json" {
		t.Fatalf("darwin floors = %s", got)
	}
	if got := CoverageFloorsFile("linux"); got != "testing-coverage-floors-linux.json" {
		t.Fatalf("linux floors = %s", got)
	}
	for _, file := range CoverageFloorsFiles() {
		if _, err := os.Stat(filepath.Join("..", "..", file)); err != nil {
			t.Errorf("floors %s: %v", file, err)
		}
		if legacy := LegacyCoverageFloorsFile(file); legacy == "" || legacy == file {
			t.Errorf("floors %s has no legacy base path", file)
		}
	}
	if !ProtectedPolicyChange([]string{"metasystem/testing-coverage-floors-linux.json"}) || !ProtectedPolicyChange([]string{"testing-coverage-floors.json"}) {
		t.Fatal("a floors change does not plan deep")
	}
}
