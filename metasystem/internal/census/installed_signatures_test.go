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

// TestInstalledSignaturesIncludeANamedExternalRuntime: the census and lease
// classification recognize an external runtime's processes through its
// describe (design 3.5), without a Go change.
func TestInstalledSignaturesIncludeANamedExternalRuntime(t *testing.T) {
	root := t.TempDir()
	adapter := filepath.Join(root, "adapters", "newagent")
	if err := os.MkdirAll(filepath.Dir(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '{\"schemaVersion\":1,\"match\":[\"^([^[:space:]]*/)?newagent([[:space:]]|$)\"]}\\n'\n"
	if err := os.WriteFile(adapter, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("adapters.newagent.use=external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sigs, names, _, err := InstalledAdapterSignatures(root)
	if err != nil || len(names) != 5 {
		t.Fatalf("names = %v, %v", names, err)
	}
	if Runtime("/usr/local/bin/newagent --task x", sigs) != "newagent" {
		t.Fatal("the external runtime's process was not recognized")
	}
}
