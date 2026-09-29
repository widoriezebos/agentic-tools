package census

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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

// TestRecognizersClassifyANamedExternalRuntime: the census and lease
// classification recognize an external runtime's processes through its
// describe, cross-checked by the registry (design 3.5, R13); an override
// is one effective declaration that keeps the built-in's reserved
// exclusions, so `devin acp` stays unclaimed (VOA-31).
func TestRecognizersClassifyANamedExternalRuntime(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	scripts := map[string]string{
		"newagent": `{"schemaVersion":1,"name":"newagent","match":["^([^[:space:]]*/)?newagent([[:space:]]|$)"],"positive":"newagent -p task","lookalike":"newagent-helper serve"}`,
		"devin":    `{"schemaVersion":1,"name":"devin","match":["^([^[:space:]]*/)?devin([[:space:]]|$)"],"exclude":["^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)"]}`,
	}
	for name, describe := range scripts {
		path := filepath.Join(root, "adapters", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '%s\\n' '" + describe + "'\n"
		if err := testexec.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude,newagent\nadapters.newagent.use=external\nadapters.devin.use=external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sigs, names, _, err := InstalledAdapterSignatures(root)
	if err != nil || len(names) != 5 {
		t.Fatalf("names = %v, %v", names, err)
	}
	for argv, want := range map[string]string{
		"/opt/newagent/bin/newagent -p task --tag t": "newagent",
		"newagent-helper serve":                      "",
		"/usr/local/bin/devin -p task":               "devin",
		"/usr/local/bin/devin acp":                   "",
		"/usr/local/bin/metasystem delegate-supervisor newagent dispatch --root /r": "",
	} {
		if got := Runtime(argv, sigs); got != want {
			t.Errorf("Runtime(%q) = %q, want %q", argv, got, want)
		}
	}
	configured, err := configuredSignatures(root)
	if err != nil || len(configured) != 2 || Runtime("newagent -p x", configured) != "newagent" {
		t.Fatalf("configured = %d signatures, %v", len(configured), err)
	}
	// The fingerprint hashes what the recognizers use: the external's
	// executable and its effective signature (U6a read N-2).
	stageFingerprintInputs(t, root)
	before, err := Fingerprint(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "adapters", "newagent"), []byte("#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '%s\\n' '"+scripts["newagent"]+"'\n\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if after, err := Fingerprint(root, root); err != nil || after == before {
		t.Fatalf("a changed external adapter left the fingerprint %s (%v)", after, err)
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
	_, refusals, err := external.Discover(root)
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
	// The fingerprint skips the refused name as the recognizers do (U6a
	// read N-2), instead of failing on it.
	stageFingerprintInputs(t, root)
	if _, err := Fingerprint(root, root); err != nil {
		t.Fatalf("fingerprint with a refused external runtime selected: %v", err)
	}
}

func stageFingerprintInputs(t *testing.T, root string) {
	t.Helper()
	for _, rel := range FingerprintFiles() {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
