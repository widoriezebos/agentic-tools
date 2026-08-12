package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Proves the converted writer's bytes are what the canonical encoder
// produced: HTML left intact, two-space indent, one trailing newline.
func TestConvertedWriterBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.json")
	value := map[string]any{"html": "a<b>&c", "n": 1}
	if err := atomicWriteJSON(path, value); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	// The canonical encoder leaves HTML intact; MarshalIndent would write
	// the \u003c escapes instead, which is an on-disk byte change.
	for _, escape := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(text, escape) {
			t.Fatalf("HTML was escaped — on-disk bytes changed: %s", text)
		}
	}
	if !strings.Contains(text, "a<b>&c") {
		t.Fatalf("value not written verbatim: %s", text)
	}
	if !strings.HasSuffix(text, "}\n") {
		t.Fatalf("missing the trailing newline: %q", text)
	}
	if !strings.Contains(text, "\n  \"html\"") {
		t.Fatalf("indent changed: %s", text)
	}
}
