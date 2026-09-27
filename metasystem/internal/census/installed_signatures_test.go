package census

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"path/filepath"
	"testing"
)

// TestInstalledAdapterSignaturesNarrowOnlyInFixtureMode: a fixture-mode
// root classifies against its configured runtimes (what fixture beds got by
// deleting unrelated adapter scripts from a scratch installation), the
// environment narrows it further, and neither applies to a production root.
func TestInstalledAdapterSignaturesNarrowOnlyInFixtureMode(t *testing.T) {
	t.Parallel()
	fixture, production := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(production, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	narrowed := func(name string) string {
		if name == FixtureSignatureRuntimesEnv {
			return "fake"
		}
		return ""
	}
	_, names, _, err := installedAdapterSignatures(fixture, narrowed)
	if err != nil || len(names) != 1 || names[0] != "fake" {
		t.Fatalf("fixture-mode narrowing = %v, %v", names, err)
	}
	_, names, _, err = installedAdapterSignatures(production, narrowed)
	if err != nil || len(names) != 4 {
		t.Fatalf("a production root honored the fixture narrowing: %v, %v", names, err)
	}
	// Unnarrowed, a fixture-mode root classifies against its configured
	// runtimes only; a production root against every declared runtime.
	_, names, _, err = installedAdapterSignatures(fixture, func(string) string { return "" })
	if err != nil || len(names) != 1 || names[0] != "fake" {
		t.Fatalf("fixture-mode universe = %v, %v", names, err)
	}
	_, names, texts, err := installedAdapterSignatures(production, func(string) string { return "" })
	if err != nil || len(names) != 4 || len(texts) != 4 || names[0] != "claude" || names[3] != "fake" {
		t.Fatalf("production universe = %v, %v", names, err)
	}
}

// TestExternalAdaptersNeverClassifyBeforeU6c: until the registry
// cross-check of U6c exists, the recognizers are built-ins only. A named
// external runtime is discovered and reported but its describe is never
// executed and it classifies nothing; an override leaves the built-in's
// signature (with devin acp excluded) in force.
func TestExternalAdaptersNeverClassifyBeforeU6c(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	marker := filepath.Join(root, "describe-ran")
	script := "#!/bin/sh\n: >'" + marker + "'\nprintf '{\"schemaVersion\":1,\"match\":[\".*\"]}\\n'\n"
	for _, name := range []string{"newagent", "devin"} {
		path := filepath.Join(root, "adapters", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("adapters.newagent.use=external\nadapters.devin.use=external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapters, _, err := ExternalAdapters(root)
	if err != nil || len(adapters) != 2 {
		t.Fatalf("discovery = %v, %v", adapters, err)
	}
	sigs, names, _, err := InstalledAdapterSignatures(root)
	if err != nil || len(names) != 4 {
		t.Fatalf("names = %v, %v", names, err)
	}
	if got := Runtime("/usr/local/bin/newagent --task x", sigs); got != "" {
		t.Fatalf("an external runtime classified a process as %q", got)
	}
	if got := Runtime("/usr/local/bin/devin acp", sigs); got != "" {
		t.Fatalf("the devin override replaced the built-in's signature (classified %q)", got)
	}
	if Runtime("/usr/local/bin/devin -p task", sigs) != "devin" {
		t.Fatal("the built-in devin signature is not in force")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a recognizer executed an external adapter's describe")
	}
}

// TestRefusedExternalIsAbsentToRecognizers: an executable the registry
// refuses (unnamed in the configuration, or group-writable) and a configured
// external runtime name never error the recognizers.
func TestRefusedExternalIsAbsentToRecognizers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, mode := range map[string]os.FileMode{"unnamed": 0o755, "loose": 0o775} {
		path := filepath.Join(root, "adapters", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude,loose\nadapters.loose.use=external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, refusals, err := ExternalAdapters(root)
	if err != nil || len(refusals) != 2 {
		t.Fatalf("refusals = %v, %v", refusals, err)
	}
	if _, names, _, err := InstalledAdapterSignatures(root); err != nil || len(names) != 4 {
		t.Fatalf("installed = %v, %v", names, err)
	}
	sigs, err := configuredSignatures(root)
	if err != nil || len(sigs) != 1 {
		t.Fatalf("configured = %d signatures, %v", len(sigs), err)
	}
}
