package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingProofStatusNamesPendingAndFailedAdmission(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"pending", "failed"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := proofPermissionBed(t)
			head := b.git(t, b.checkout, "rev-parse", "HEAD")
			tree := b.git(t, b.checkout, "rev-parse", "HEAD^{tree}")
			running := plain.Running{Attempt: "admission", Commit: head, Tree: tree, Since: b.now.Format("2006-01-02T15:04:05Z07:00"), Trunk: true,
				Admission: &plain.ExecutionAdmission{State: state}}
			b.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
			raw, err := json.Marshal(running)
			helmMust(t, err, os.WriteFile(filepath.Join(plain.Dir(b.installation), "running.json"), raw, 0600))
			code, text := b.plainVerbBed.run(t, "landing", "status", "--verbose")
			text = strings.Join(strings.Fields(text), " ")
			if code != 0 || !strings.Contains(text, state) || !strings.Contains(text, "metasystem landing prove --trunk") || strings.Contains(text, "proving tree ") {
				t.Fatalf("%s status: %d %s", state, code, text)
			}
			status := plain.ReadRunningProof(b.installation, b.owners.landing.plainProve)
			if status == nil || status.State != state {
				t.Fatalf("admission state: %+v", status)
			}
		})
	}
}

func TestLandingSelectionIgnoresLegacyClosedBarrenStop(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	stop := plain.Stop{Loop: "lane-return", Subject: "lane", Attempt: 2, Decision: "stop", Handoff: "ask lane", At: b.now.Add(-time.Hour).Format(time.RFC3339)}
	closed := stop
	closed.Decision, closed.At = "close", b.now.Format(time.RFC3339)
	selectionLines(t, filepath.Join(plain.Dir(b.lane), "stops.jsonl"), stop, closed)
	if code, text := b.run(t, b.lane, "landing", "run", "--goals", "a,b"); code != 0 {
		t.Fatalf("person selection: %d %s", code, text)
	}
	batch, err := plain.ReadBatch(b.lane)
	if err != nil || batch == nil || batch.Person == nil || batch.Person.BarrenStopAt != "" {
		t.Fatalf("closed barren stop entered selection: %+v %v", batch, err)
	}
}
