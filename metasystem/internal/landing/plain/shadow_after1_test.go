package plain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingShadowOnlyCorruptionIsFinding(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"landing-lane-resumed.json", "state.json", "events.jsonl", "resume-record-finding.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.incidents()
			if _, err := RefreshShadow(b.home, bedNow, "keeper", b.seams); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(Dir(b.install), name)
			if name == "landing-lane-resumed.json" {
				path = filepath.Join(lane.HostDir(b.home), name)
			}
			if err := os.WriteFile(path, []byte(`{not json`), 0600); err != nil {
				t.Fatal(err)
			}
			status := ObserveStatus(b.home, b.record, b.seams, nil)
			if len(status.Problems) != 0 || status.Health == nil || len(status.Findings) == 0 {
				t.Fatalf("shadow-only damage blocked status: %+v", status)
			}
			if name == "landing-lane-resumed.json" && (!ShadowOf(status, status.Raw).Lane.Since.Observed || status.Raw.Resume.At != "") {
				t.Fatal("corrupt resume supplied a transition time")
			}
			run := b.step(bedNow)
			if len(run.Interventions) != 1 || run.Interventions[0].Row != "unproven-incidents" || !strings.Contains(strings.Join(b.calls, ","), "trunk") {
				t.Fatalf("shadow damage stopped steward: %+v calls=%v", run, b.calls)
			}
			// The idle-agent finding is also an operational consumer of status Problems.
			since := bedNow.Add(-time.Hour).Format(time.RFC3339Nano)
			b.view.Owner = lane.OwnerView{State: lane.OwnerRunning, Since: &since}
			status = readStatus(b.home, b.record, b.view, shadowSeams(), b.git(nil, nil))
			if len(status.Problems) != 0 || status.AgentProgress == nil || status.AgentProgress.Finding == "" {
				t.Fatalf("shadow damage suppressed idle finding: %+v", status)
			}
		})
	}
}

func TestShadowBatchTimestampJSONUsesHyphens(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	stamp := bedNow.Format(time.RFC3339Nano)
	if err := writeBatch(b.install, &Batch{ID: "selection", Lane: b.record, Base: "main", State: BatchClosed, RunningAt: stamp, ClosedAt: stamp}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(batchPath(b.install))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"running-at", "closed-at"} {
		if string(record[name]) != `"`+stamp+`"` {
			t.Fatalf("missing persisted %s: %s", name, data)
		}
	}
	if record["running_at"] != nil || record["closed_at"] != nil {
		t.Fatalf("underscore timestamp persisted: %s", data)
	}
	batch, err := ReadBatch(b.install)
	if err != nil || batch.RunningAt != stamp || batch.ClosedAt != stamp {
		t.Fatalf("timestamp round trip: %+v %v", batch, err)
	}
}
