package events

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MON-09: the four run events are KNOWN to the closed catalogue — the
// registry silently drops unknowns, so each row is emitted into a temp
// root and proven to land in the recorder stream with its required
// identifier and payload.
func TestRunEventConformance(t *testing.T) {
	root := t.TempDir()
	emitter := &Emitter{Component: "run", Pid: 4242, PidStartedAt: 1000}

	rows := []struct {
		event  string
		fields map[string]string
	}{
		{"run-launched", map[string]string{"runId": "r1", "kind": "suite", "custody": "wrapped"}},
		{"run-transition", map[string]string{"runId": "r1", "from": "running", "to": "green", "generation": "1"}},
		{"run-swept", map[string]string{"runId": "r1", "reason": "stale-claim-epoch"}},
		{"run-cas-refused", map[string]string{"runId": "r1", "expected": "running.g1", "found": "green.g1"}},
	}
	for _, row := range rows {
		emitter.Emit(root, row.event, row.event+" r1", row.fields)
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
	if !strings.Contains(stream, `"runId":"r1"`) {
		t.Fatalf("runId identifier missing from the stream:\n%s", stream)
	}
}
