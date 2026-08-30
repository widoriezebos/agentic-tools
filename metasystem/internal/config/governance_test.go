package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCorrelationPolicyActivatesOnlyWithTheOneWordChoice(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
		ok    bool
	}{
		{value: "", want: "", ok: true},
		{value: "A", want: "A", ok: true},
		{value: "B", want: "B", ok: true},
		{value: "C", want: "C", ok: true},
		{value: "auto", ok: false},
	} {
		t.Run("choice-"+test.value, func(t *testing.T) {
			root := t.TempDir()
			content := []byte("metasystem.governance.correlation-policy=" + test.value + "\n")
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), content, 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := CorrelationPolicy(root)
			if test.ok && (err != nil || got != test.want) {
				t.Fatalf("choice %q did not activate exactly: got=%q err=%v", test.value, got, err)
			}
			if !test.ok && err == nil {
				t.Fatalf("unknown choice %q activated as %q", test.value, got)
			}
		})
	}
}
