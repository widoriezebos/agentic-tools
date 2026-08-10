package census

import (
	"os"
	"path/filepath"
	"testing"
)

// SignatureText normalizes and validates adapter output.
func TestSignatureText(t *testing.T) {
	dir := t.TempDir()
	adapter := filepath.Join(dir, "good.sh")
	os.WriteFile(adapter, []byte("#!/bin/sh\nprintf 'match ^claude\\nexclude foo\\n'\n"), 0o755)
	text, err := SignatureText(adapter)
	if err != nil {
		t.Fatal(err)
	}
	if text != "match ^claude\nexclude foo\n" {
		t.Fatalf("unexpected normalization: %q", text)
	}

	bad := filepath.Join(dir, "bad.sh")
	os.WriteFile(bad, []byte("#!/bin/sh\nprintf '  match ^claude\\n'\n"), 0o755) // leading space
	if _, err := SignatureText(bad); err == nil {
		t.Fatal("a leading-space declaration must be rejected")
	}
	empty := filepath.Join(dir, "empty.sh")
	os.WriteFile(empty, []byte("#!/bin/sh\ntrue\n"), 0o755)
	if _, err := SignatureText(empty); err == nil {
		t.Fatal("an adapter emitting nothing must be rejected")
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

// Fingerprint happy path against a synthetic root: it hashes the fixed
// files, the adapter signature, and config, and is deterministic and
// sensitive to change.
func TestFingerprintDeterministicAndSensitive(t *testing.T) {
	root := t.TempDir()
	for _, rel := range fingerprintFiles {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("content of "+rel+"\n"), 0o644)
	}
	adapterDir := filepath.Join(root, "scripts", "agents", "adapters")
	os.MkdirAll(adapterDir, 0o755)
	os.WriteFile(filepath.Join(adapterDir, "fake.sh"),
		[]byte("#!/bin/sh\nprintf 'match ^metasystem-fake-agent\\n'\n"), 0o755)
	os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.runtimes=fake\nwatch.interval-sec=60\n"), 0o644)

	one, err := Fingerprint(root, root)
	if err != nil {
		t.Fatal(err)
	}
	two, err := Fingerprint(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if one != two || len(one) != 64 {
		t.Fatalf("fingerprint not a stable 64-hex digest: %q %q", one, two)
	}
	// A change to a hashed file moves the fingerprint.
	os.WriteFile(filepath.Join(root, "scripts", "agents", "dispatch.sh"), []byte("changed\n"), 0o644)
	changed, err := Fingerprint(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if changed == one {
		t.Fatal("a code edit must change the fingerprint")
	}
}
