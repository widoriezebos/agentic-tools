package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLandingKeeperTickRetiresStopsWhilePaused(t *testing.T) {
	t.Parallel()
	b := shadowCorrectionVerbBed(t)
	r, _, err := lane.Read(b.home)
	helmMust(t, err)
	_, err = lane.SetPause(b.home, "Wido", laneTestNow)
	helmMust(t, err)
	writeQuestionFixture(t, filepath.Join(plain.Dir(r.Install), "queue.jsonl"), plain.Line{Goal: "landed", SHA: "sha", At: laneTestNow.Add(-time.Hour).Format(time.RFC3339Nano)})
	writeQuestionFixture(t, filepath.Join(plain.Dir(r.Install), "stops.jsonl"), plain.Stop{Loop: "lane-proof", Subject: "landed", Decision: "stop", At: laneTestNow.Add(-time.Hour).Format(time.RFC3339Nano)})
	b.plainProve.Policy = func(key string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "auto"}, nil }
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
	agent := newTestLandingAgent(func(a *landingAgent) {
		a.manager = func() *launch.Manager { return manager }
		a.now = func() time.Time { return laneTestNow }
		a.proofEffects = b.plainProve
		a.machine = func(string) (string, error) { return "fixture", nil }
	})
	keeper := newLandingAgentKeeper(r.Install, b.home, agent)
	keeper.Observe = nil
	keeper.BarrenStop = nil
	keeper.Step()
	open, err := plain.OpenStops(r.Install)
	if err != nil || len(open) != 0 {
		finding, _ := os.ReadFile(filepath.Join(plain.Dir(r.Install), "shadow-refresh.json"))
		t.Fatalf("paused tick left stops: %+v %v refresh=%s", open, err, finding)
	}
	events, _, err := state.ReadEvents(plain.Dir(r.Install))
	helmMust(t, err)
	count := 0
	for _, event := range events {
		if event.Kind == "loop-stop" && event.By == "keeper" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("tick retirement events=%d: %+v", count, events)
	}
}

func TestLandingStopRetainsTheEnrolledPerson(t *testing.T) {
	t.Parallel()
	b, _ := batchVerbFixture(t, "sealed")
	code, out := b.run(t, b.seat, "landing", "stop", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	pause, present := lane.ReadPause(b.home)
	if !present || pause.By != "Wido" {
		t.Fatalf("stop author: %+v present=%v", pause, present)
	}
	data, err := os.ReadFile(filepath.Join(plain.Dir(b.lane), "stop-person.json"))
	if err != nil {
		t.Fatal(err)
	}
	var person plain.ActProvenance
	if err := json.Unmarshal(data, &person); err != nil {
		t.Fatal(err)
	}
	if person.Person != "Wido" || person.Kind != "stop" || person.Root != b.seat || person.Destination != b.record || person.CheckedAt.IsZero() {
		t.Fatalf("person's act: %+v", person)
	}
}

func TestLandingHealthRulesAreTheLifecycleTable(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	b.cwd = b.checkout
	code, out := b.run(t, "landing", "health", "--rules")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, row := range state.Lifecycle {
		if strings.Count(out, row.Kind+": "+row.Retires) != 1 {
			t.Fatalf("missing or repeated rule %s: %s", row.Kind, out)
		}
	}
	lines := 0
	for _, line := range strings.Split(out, "\n") {
		for _, row := range state.Lifecycle {
			if strings.Contains(line, row.Kind+": ") {
				lines++
			}
		}
	}
	if lines != len(state.Lifecycle) {
		t.Fatalf("rules=%d; table=%d", lines, len(state.Lifecycle))
	}
}

func TestLandingStopAfterFailedRegenerationRecordsStopAndShowsFailure(t *testing.T) {
	t.Parallel()
	b, _ := batchVerbFixture(t, "sealed")
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "regenerating.json"), plain.RunningRegeneration{Goal: "A", Since: laneTestNow.Format(time.RFC3339Nano), Launch: &plain.LaunchRecord{What: "regeneration", By: "Wido", At: laneTestNow.Format(time.RFC3339Nano), Error: "compiler unavailable"}})
	b.owners.landing.stopRegeneration = plain.StopRegeneration
	for range 2 {
		code, out := b.run(t, b.seat, "landing", "stop", "--json")
		if code != 0 || !strings.Contains(out, "failed regeneration is closed") {
			t.Fatalf("stop: %d %s", code, out)
		}
	}
	pause, present := lane.ReadPause(b.home)
	if !present || pause.By != "Wido" {
		t.Fatalf("stop not recorded: %+v", pause)
	}
	saved, err := plain.ReadRunningRegeneration(b.lane, plain.ProveSeams{})
	if err != nil || saved == nil || saved.State != "launch-failed" || saved.Launch.Error != "compiler unavailable" {
		t.Fatalf("failure lost: %+v %v", saved, err)
	}
	code, out := b.run(t, b.seat, "landing", "status")
	if code != 0 || !strings.Contains(out, "compiler unavailable") || strings.Contains(out, "regeneration process is undecidable") {
		t.Fatalf("failure status: %d %s", code, out)
	}
}
