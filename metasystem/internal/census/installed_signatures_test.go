package census

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstalledAdapterSignaturesNarrowOnlyInFixtureMode: the fixture
// narrowing that replaced deleting adapter scripts from a scratch
// installation applies in a fixture-mode root and nowhere else.
func TestInstalledAdapterSignaturesNarrowOnlyInFixtureMode(t *testing.T) {
	fixture, production := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(production, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(FixtureSignatureRuntimesEnv, "fake")
	_, names, _, err := InstalledAdapterSignatures(fixture)
	if err != nil || len(names) != 1 || names[0] != "fake" {
		t.Fatalf("fixture-mode narrowing = %v, %v", names, err)
	}
	_, names, _, err = InstalledAdapterSignatures(production)
	if err != nil || len(names) != 4 {
		t.Fatalf("a production root honored the fixture narrowing: %v, %v", names, err)
	}
	t.Setenv(FixtureSignatureRuntimesEnv, "")
	_, names, texts, err := InstalledAdapterSignatures(fixture)
	if err != nil || len(names) != 4 || len(texts) != 4 || names[0] != "claude" || names[3] != "fake" {
		t.Fatalf("unnarrowed universe = %v, %v", names, err)
	}
}
