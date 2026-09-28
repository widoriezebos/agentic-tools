package events

import (
	"encoding/json"
	"testing"
)

// The event catalogue is compiled into the engine: an installation without
// any registry file still drops an unregistered event and a wrong emitter.
func TestRegistryIsCompiledIn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if registryAllows(root, "lease", "not-a-registered-event") {
		t.Fatal("an unregistered event passed without a registry file")
	}
	var registry struct {
		Events map[string]struct {
			Emitters []string `json:"emitters"`
		} `json:"events"`
	}
	if err := json.Unmarshal(registrySource, &registry); err != nil || len(registry.Events) == 0 {
		t.Fatalf("the compiled-in registry is not a catalogue: %v", err)
	}
	for event, entry := range registry.Events {
		if len(entry.Emitters) == 0 {
			t.Errorf("event %s has no emitter", event)
			continue
		}
		if !registryAllows(root, entry.Emitters[0], event) {
			t.Errorf("registered event %s refused for its emitter %s", event, entry.Emitters[0])
		}
	}
}
