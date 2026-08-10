package config

import (
	"os"
	"path/filepath"
	"testing"
)

const codexFilter = `{"cliVersionRange":{"min":"0.146.0","max":"0.146.x"},"keys":[
  {"path":"notice","reason":"bookkeeping","source":"KI-19"},
  {"path":"notice.model_migrations","reason":"bookkeeping","source":"KI-19"},
  {"path":"tui.model_availability_nux","reason":"bookkeeping","source":"KI-19"}]}`

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

func TestConfigIdentityExcludesFilteredKeys(t *testing.T) {
	dir := t.TempDir()
	filter := writeFile(t, dir, "filter.json", codexFilter)
	config := writeFile(t, dir, "config.toml", `model = "gpt-5.6-sol"

[notice]
hide_rate_limit_model_nudge = false

[notice.model_migrations]
"gpt-5.2" = "gpt-5.3-codex"

[tui.model_availability_nux]
"gpt-5.6-sol" = 1
`)
	id, err := BuildConfigIdentity("codex", "0.146.0", filter, []string{config})
	if err != nil {
		t.Fatal(err)
	}
	keys := id["configKeyHashes"].(map[string]any)
	if len(keys) != 1 {
		t.Fatalf("only the unfiltered key should remain: %v", keys)
	}
	if _, ok := keys["model"]; !ok {
		t.Fatalf("model must survive the filter: %v", keys)
	}
}

func TestConfigIdentityIgnoresBookkeepingChurn(t *testing.T) {
	dir := t.TempDir()
	filter := writeFile(t, dir, "filter.json", codexFilter)
	before := writeFile(t, dir, "before.toml", `model = "gpt-5.6-sol"
[notice]
hide_rate_limit_model_nudge = false
[notice.model_migrations]
"gpt-5.2" = "gpt-5.3-codex"
[tui.model_availability_nux]
"gpt-5.6-sol" = 1
`)
	after := writeFile(t, dir, "after.toml", `model = "gpt-5.6-sol"
[notice]
hide_rate_limit_model_nudge = true
[notice.model_migrations]
"gpt-5.2" = "gpt-5.6-sol"
[tui.model_availability_nux]
"gpt-5.6-sol" = 2
`)
	idA, _ := BuildConfigIdentity("codex", "0.146.0", filter, []string{before})
	idB, _ := BuildConfigIdentity("codex", "0.146.0", filter, []string{after})
	if configHashOf(t, idA) != configHashOf(t, idB) {
		t.Fatal("filtered bookkeeping changes must not move the config hash")
	}
}

func TestConfigIdentityChangeSensitive(t *testing.T) {
	dir := t.TempDir()
	filter := writeFile(t, dir, "filter.json", codexFilter)
	a := writeFile(t, dir, "a.toml", `model = "gpt-5.6-sol"`)
	b := writeFile(t, dir, "b.toml", `model = "gpt-5.6-terra"`)
	idA, _ := BuildConfigIdentity("codex", "0.146.0", filter, []string{a})
	idB, _ := BuildConfigIdentity("codex", "0.146.0", filter, []string{b})
	if configHashOf(t, idA) == configHashOf(t, idB) {
		t.Fatal("a behavior-changing key must move the config hash")
	}
}

func TestConfigIdentityStableAcrossEquivalentJSON(t *testing.T) {
	dir := t.TempDir()
	filter := writeFile(t, dir, "filter.json", `{"cliVersionRange":{"min":"1.0.0","max":"1.0.x"},"keys":[]}`)
	a := writeFile(t, dir, "a.json", `{"nested":{"beta":2,"alpha":1},"model":"same"}`)
	b := writeFile(t, dir, "b.json", "{\n  \"model\": \"same\",\n  \"nested\": {\"alpha\": 1, \"beta\": 2}\n}")
	idA, _ := BuildConfigIdentity("fixture", "1.0.0", filter, []string{a})
	idB, _ := BuildConfigIdentity("fixture", "1.0.0", filter, []string{b})
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

func TestConfigIdentityMalformedAndOutOfRangeFilterHashesFullMap(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.json", `{"model":"x","other":1}`)
	full := writeFile(t, dir, "full.json", `{"cliVersionRange":{"min":"1.0.0","max":"1.0.x"},"keys":[]}`)
	malformed := writeFile(t, dir, "bad.json", `{not json`)
	codexF := writeFile(t, dir, "codex.json", codexFilter)

	idFull, _ := BuildConfigIdentity("fixture", "1.0.0", full, []string{a})
	idMalformed, _ := BuildConfigIdentity("fixture", "1.0.0", malformed, []string{a})
	idOutOfRange, _ := BuildConfigIdentity("codex", "0.147.0", codexF, []string{a})

	if configHashOf(t, idMalformed) != configHashOf(t, idFull) {
		t.Fatal("a malformed filter must hash the full canonical map")
	}
	if configHashOf(t, idOutOfRange) != configHashOf(t, idFull) {
		t.Fatal("an out-of-range filter must hash the full canonical map")
	}
}

func TestVersionInRange(t *testing.T) {
	cases := []struct {
		version, min, max string
		want              bool
	}{
		{"0.146.0", "0.146.0", "0.146.x", true},
		{"0.146.5", "0.146.0", "0.146.x", true},
		{"0.147.0", "0.146.0", "0.146.x", false},
		{"0.145.9", "0.146.0", "0.146.x", false},
		{"1.2.3", "1.0.0", "2.0.0", true},
		{"2.0.1", "1.0.0", "2.0.0", false},
		{"2.1.0", "2.1.0", "2.1.x", true},
		{"not-a-version", "1.0.0", "2.0.0", false},
	}
	for _, c := range cases {
		if got := versionInRange(c.version, c.min, c.max); got != c.want {
			t.Errorf("versionInRange(%q,%q,%q) = %v, want %v", c.version, c.min, c.max, got, c.want)
		}
	}
}
