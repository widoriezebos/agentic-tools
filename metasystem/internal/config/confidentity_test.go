package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func configHashOf(t *testing.T, id map[string]any) string {
	t.Helper()
	h, _ := id["configHash"].(string)
	if h == "" {
		t.Fatalf("no configHash: %v", id)
	}
	return h
}

func TestConfigIdentityChangeSensitive(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.toml", `model = "gpt-5.6-sol"`)
	b := writeFile(t, dir, "b.toml", `model = "gpt-5.6-terra"`)
	idA, _ := BuildConfigIdentity("codex", "0.146.0", []string{a})
	idB, _ := BuildConfigIdentity("codex", "0.146.0", []string{b})
	if configHashOf(t, idA) == configHashOf(t, idB) {
		t.Fatal("a behavior-changing key must move the config hash")
	}
}

func TestConfigIdentityStableAcrossEquivalentJSON(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.json", `{"nested":{"beta":2,"alpha":1},"model":"same"}`)
	b := writeFile(t, dir, "b.json", "{\n  \"model\": \"same\",\n  \"nested\": {\"alpha\": 1, \"beta\": 2}\n}")
	idA, _ := BuildConfigIdentity("fixture", "1.0.0", []string{a})
	idB, _ := BuildConfigIdentity("fixture", "1.0.0", []string{b})
	if configHashOf(t, idA) != configHashOf(t, idB) {
		t.Fatal("equivalent objects must canonicalize to the same hash")
	}
	keys := idA["configKeyHashes"].(map[string]any)
	for _, want := range []string{"model", "nested.alpha", "nested.beta"} {
		if _, ok := keys[want]; !ok {
			t.Fatalf("missing flattened key %q: %v", want, keys)
		}
	}
}
