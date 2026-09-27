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

// TestInstalledSignaturesIncludeANamedExternalRuntime: the census and lease
// classification recognize an external runtime's processes through its
// describe (design 3.5), without a Go change.
func TestInstalledSignaturesIncludeANamedExternalRuntime(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	adapter := filepath.Join(root, "adapters", "newagent")
	if err := os.MkdirAll(filepath.Dir(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '{\"schemaVersion\":1,\"match\":[\"^([^[:space:]]*/)?newagent([[:space:]]|$)\"]}\\n'\n"
	if err := testexec.WriteFile(adapter, []byte(script), 0o755); err != nil {
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

// TestOverrideKeepsTheBuiltInsReservedExclusions: VOA-31, one effective
// declaration per runtime name. An override of Devin whose describe drops the
// `devin acp` exclusion still does not classify that host helper as a
// delegate, while its own match still classifies the CLI.
func TestOverrideKeepsTheBuiltInsReservedExclusions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	adapter := filepath.Join(root, "adapters", "devin")
	if err := os.MkdirAll(filepath.Dir(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '{\"schemaVersion\":1,\"match\":[\"^([^[:space:]]*/)?devin([[:space:]]|$)\"]}\\n'\n"
	if err := testexec.WriteFile(adapter, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("adapters.devin.use=external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sig, text, err := RuntimeSignatureAt(root, "devin")
	if err != nil {
		t.Fatal(err)
	}
	sigs := []Signature{sig}
	if Runtime("/usr/local/bin/devin -p task", sigs) != "devin" {
		t.Fatalf("the override's match does not classify the CLI:\n%s", text)
	}
	if got := Runtime("/usr/local/bin/devin acp", sigs); got != "" {
		t.Fatalf("the override dropped the built-in's devin acp exclusion (classified %q):\n%s", got, text)
	}
}
