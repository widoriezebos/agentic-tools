package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// The switch-on trial of 2026-09-30: seat m1e (a template checkout, its
// installation at checkout/metasystem) was at the helm when it joined a change
// to the lane. The owner held the batch whole for that seat, as designed, and
// recorded the yield in the landing seat's helm-yields.log; but landing status
// said the batch was collecting and "starts when the start rule says so", so
// a person waited 30 minutes for a start that could never come. The lane view
// must say what the owner decided: held for the seat at the helm, and the one
// act that releases it.
func TestLandingStatusNamesABatchHeldForASeatAtTheHelm(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	bed.alive = true
	checkout := realpath.Resolve(t.TempDir())
	installation := filepath.Join(checkout, "metasystem")
	for _, dir := range []string{filepath.Join(checkout, ".git"), installation} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	record := batch.Record{BatchID: "4gr18", State: batch.StateOpen,
		Units:   []batch.Unit{{GoalID: "change:533209e6d6c1", State: batch.UnitJoined, SeatRoot: installation, Claim: batch.Claim{Machine: "m1e"}}},
		History: []batch.HistoryEntry{{At: laneTestNow.Format("2006-01-02T15:04:05Z"), Verb: "open", To: batch.StateOpen}}}
	bed.records = []batch.Record{record}

	// Not at the helm: the batch collects as before.
	if view := bed.status(t); view.Batch == nil || view.Batch.State != lane.BatchCollecting {
		t.Fatalf("seat not at the helm: batch = %+v", view.Batch)
	}

	if _, err := helm.Write(installation, helm.Record{By: "wido", At: laneTestNow.Format("2006-01-02T15:04:05Z"), Reason: "coordinating"}); err != nil {
		t.Fatal(err)
	}
	view := bed.status(t)
	if view.Batch == nil || view.Batch.State != lane.BatchHeld {
		t.Fatalf("seat at the helm: batch = %+v, want state held", view.Batch)
	}
	for _, want := range []string{"m1e", "at the helm", "wido", "metasystem helm return"} {
		if !strings.Contains(view.Batch.Reason, want) {
			t.Errorf("reason %q lacks %q", view.Batch.Reason, want)
		}
	}
	code, stdout, stderr := bed.run(t, "landing", "status")
	if code != 0 || !strings.Contains(stdout, "held   batch 4gr18") || !strings.Contains(stdout, "metasystem helm return") {
		t.Fatalf("landing status = %d %q %q", code, stdout, stderr)
	}
	inv := &intentInvocation{owners: bed.owners()}
	if line := inv.statusLaneLine(); !strings.Contains(line, "held") || !strings.Contains(line, "helm return") {
		t.Fatalf("status lane line = %q", line)
	}
}
