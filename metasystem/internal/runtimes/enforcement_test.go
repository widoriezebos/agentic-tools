package runtimes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Each runtime's hook settings source is compiled into the engine; its
// registration row names the engine source it came from.
func TestShippedEnforcementIsCompiledIn(t *testing.T) {
	t.Parallel()
	seen := 0
	for _, declaration := range All() {
		if declaration.ShippedEnforcementConfig == "" {
			if _, err := ShippedEnforcement(declaration.Name); err == nil {
				t.Errorf("%s ships no enforcement config but one resolved", declaration.Name)
			}
			continue
		}
		seen++
		data, err := ShippedEnforcement(declaration.Name)
		if err != nil {
			t.Fatalf("%s: %v", declaration.Name, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("%s enforcement is not JSON: %v", declaration.Name, err)
		}
		source := EnforcementSource(declaration.ShippedEnforcementConfig)
		onDisk, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(source)))
		if err != nil || string(onDisk) != string(data) {
			t.Fatalf("%s: the embedded config is not %s: %v", declaration.Name, source, err)
		}
		found := false
		for _, row := range RegistrationRows(declaration.Name) {
			if row.Operation == OpCopyFile || row.Operation == OpJSONStripKey {
				found = row.Source == source
			}
		}
		if !found {
			t.Errorf("%s's enforcement registration row does not name %s", declaration.Name, source)
		}
	}
	if seen != 3 {
		t.Fatalf("%d runtimes ship enforcement, want claude, codex and devin", seen)
	}
}
