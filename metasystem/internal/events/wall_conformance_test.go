package events

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The wall's witness events are KNOWN to the closed catalogue compiled into
// the engine: an unregistered wall event would be silently dropped exactly
// when its evidence mattered most.
func TestWallEventConformance(t *testing.T) {
	root := t.TempDir()

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
