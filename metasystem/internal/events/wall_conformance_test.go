package events

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The wall's witness events are KNOWN to the closed catalogue. The real
// registry is staged into the temp root so this proves catalogue
// membership, not just the absent-registry fallback: an unregistered
// wall event would be silently dropped exactly when its evidence
// mattered most.
func TestWallEventConformance(t *testing.T) {
	root := t.TempDir()
	source, err := os.ReadFile(repoRegistryPath(t))
	if err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(root, "scripts", "agents", "event-registry.json")
	if err := os.MkdirAll(filepath.Dir(registry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry, source, 0o644); err != nil {
		t.Fatal(err)
	}

	emitter := &Emitter{Component: "runner", Pid: 4242, PidStartedAt: 1000}
	rows := []struct {
		event  string
		fields map[string]string
	}{
		{"authorization-consumed", map[string]string{"missionId": "m1", "turnId": "t1", "authorizationDigest": "d1"}},
		{"authorization-refused", map[string]string{"missionId": "m1", "jobId": "j1", "error": "superseded", "authorizationDigest": "d1"}},
		{"wall-passed", map[string]string{"missionId": "m1", "turnId": "t1", "consumedCount": "2"}},
		{"taint-set", map[string]string{"missionId": "m1", "turnId": "t1", "taintId": "3", "error": "solo build"}},
		{"recovery-inspected", map[string]string{"missionId": "m1", "turnId": "t1", "verdict": "restorable"}},
	}
	for _, row := range rows {
		emitter.Emit(root, row.event, row.event+" witness", row.fields)
	}

	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "events.jsonl"))
	if err != nil {
		t.Fatalf("no recorder stream written: %v", err)
	}
	stream := string(data)
	for _, row := range rows {
		if !strings.Contains(stream, `"event":"`+row.event+`"`) {
			t.Fatalf("event %s was dropped by the catalogue:\n%s", row.event, stream)
		}
	}
}

// repoRegistryPath walks up from the package directory to the checkout
// root that carries the live registry.
func repoRegistryPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "scripts", "agents", "event-registry.json")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("event registry not found above the package directory")
		}
		dir = parent
	}
}
