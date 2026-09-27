package census

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// SignatureText is the registry's normalized declaration: strict
// line-oriented `match`/`exclude` lines with a trailing newline, the
// supervisor process excluded, and nothing for an undeclared runtime.
func TestSignatureText(t *testing.T) {
	text, err := SignatureText("fake")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
		t.Fatalf("declaration is not newline-terminated lines: %q", text)
	}
	if !strings.Contains(text, "match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)\n") {
		t.Fatalf("fake declaration lost its match line: %q", text)
	}
	if !strings.Contains(text, "delegate-supervisor") {
		t.Fatalf("fake declaration does not exclude the supervisor process: %q", text)
	}
	if _, err := SignatureText("no-such-runtime"); err == nil {
		t.Fatal("an undeclared runtime must have no signature")
	}
}

// S4-7 (supervision-fixtures.sh): every declared adapter runtime serves a
// non-empty, strictly line-oriented declaration that compiles.
func TestEveryRegistrySignatureIsWellFormed(t *testing.T) {
	t.Parallel()
	names := runtimes.WithAdapter()
	if len(names) < 4 {
		t.Fatalf("only %d adapter runtimes declared — the population went missing: %v", len(names), names)
	}
	for _, runtime := range names {
		text, err := SignatureText(runtime)
		if err != nil || text == "" {
			t.Fatalf("%s returned no signature: %v", runtime, err)
		}
		for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
			if !strings.HasPrefix(line, "match ") && !strings.HasPrefix(line, "exclude ") {
				t.Fatalf("malformed %s signature line: %q", runtime, line)
			}
		}
		if _, _, err := RuntimeSignature(runtime); err != nil {
			t.Fatalf("%s signature does not compile: %v", runtime, err)
		}
	}
}

// canonicalJSON emits compact, key-sorted JSON with no HTML escaping and no
// trailing newline.
func TestCanonicalJSON(t *testing.T) {
	got, err := canonicalJSON(map[string]any{"b": "x<y>&z", "a": 1})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":1,"b":"x<y>&z"}`
	if string(got) != want {
		t.Fatalf("canonical JSON = %q, want %q (sorted, compact, no HTML escaping)", got, want)
	}
}

func TestFingerprintMissingInputErrors(t *testing.T) {
	// A metasystem root missing the fixed supervision files errors loudly.
	if _, err := Fingerprint(t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("a root without the supervision files must error")
	}
}

func writeFingerprintRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range fingerprintFiles {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("content of "+rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.runtimes=fake\nwatch.interval-sec=60\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func mustFingerprint(t *testing.T, root string) string {
	t.Helper()
	fingerprint, err := Fingerprint(root, root)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

// Fingerprint happy path against a synthetic root: it hashes the fixed
// files, the registry signatures of the configured runtimes, and config,
// and is deterministic and sensitive to change.
func TestFingerprintDeterministicAndSensitive(t *testing.T) {
	root := writeFingerprintRoot(t)
	one := mustFingerprint(t, root)
	if two := mustFingerprint(t, root); one != two || len(one) != 64 {
		t.Fatalf("fingerprint not a stable 64-hex digest: %q %q", one, two)
	}
	// A change to a hashed file moves the fingerprint.
	os.WriteFile(filepath.Join(root, "scripts", "agents", "dispatch.sh"), []byte("changed\n"), 0o644)
	if changed := mustFingerprint(t, root); changed == one {
		t.Fatal("a code edit must change the fingerprint")
	}
}

// S4-3 (supervision-fixtures.sh, the adapters/fake.sh append): mutating a
// static identity owner invalidates the old verdict. The signature owner is
// now the engine binary that carries the runtime registry, so an engine
// change moves the fingerprint, and so does a change of the configured
// signature set; an adapter script, if one is left in a checkout, is no
// longer an input.
func TestFingerprintMovesWithEngineAndSignatureSetNotAdapterScripts(t *testing.T) {
	t.Parallel()
	root := writeFingerprintRoot(t)
	before := mustFingerprint(t, root)

	adapters := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(adapters, "fake.sh"), []byte("#!/bin/sh\n# signature fingerprint fixture\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := mustFingerprint(t, root); got != before {
		t.Fatal("an adapter script is no longer a fingerprint input")
	}

	engine := filepath.Join(root, "bin", "metasystem")
	file, err := os.OpenFile(engine, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	file.WriteString("\n# signature fingerprint fixture\n")
	file.Close()
	afterEngine := mustFingerprint(t, root)
	if afterEngine == before {
		t.Fatal("an engine change did not alter the fingerprint")
	}

	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.runtimes=fake,claude\nwatch.interval-sec=60\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mustFingerprint(t, root); got == afterEngine {
		t.Fatal("a signature-set change did not alter the fingerprint")
	}
}
