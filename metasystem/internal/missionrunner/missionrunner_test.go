package missionrunner

import (
	"encoding/json"
	turnvocab "github.com/widoriezebos/agentic-tools/metasystem/internal/turn"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScaledWaitCompressesBelowOneSecond(t *testing.T) {
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "25")
	if w, err := ScaledWait(5); err != nil || w != 125*time.Millisecond {
		t.Fatalf("5s at scale 25 = %v %v; want 125ms", w, err)
	}
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "1")
	if w, err := ScaledWait(5); err != nil || w != 10*time.Millisecond {
		t.Fatalf("the 10ms floor holds: %v %v", w, err)
	}
}

func TestScaledSeconds(t *testing.T) {
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "")
	if got, err := ScaledSeconds(5); err != nil || got != 5 {
		t.Fatalf("default scale: got %d, %v", got, err)
	}
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "250")
	if got, err := ScaledSeconds(10); err != nil || got != 3 {
		t.Fatalf("scale 250: got %d, %v (want ceil(10*250/1000)=3)", got, err)
	}
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "100")
	if got, err := ScaledSeconds(5); err != nil || got != 1 {
		t.Fatalf("scale 100: got %d, %v (rounds up, floor one second)", got, err)
	}
	for _, bad := range []string{"0", "-5", "fast"} {
		t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", bad)
		if _, err := ScaledSeconds(5); err == nil {
			t.Fatalf("scale %q: want error", bad)
		}
	}
}

func TestInterval(t *testing.T) {
	t.Setenv("METASYSTEM_TEST_INTERVAL_MS", "")
	if got, err := Interval("METASYSTEM_TEST_INTERVAL_MS", 100); err != nil || got != 100*time.Millisecond {
		t.Fatalf("default: got %v, %v", got, err)
	}
	t.Setenv("METASYSTEM_TEST_INTERVAL_MS", "50")
	if got, err := Interval("METASYSTEM_TEST_INTERVAL_MS", 100); err != nil || got != 50*time.Millisecond {
		t.Fatalf("override: got %v, %v", got, err)
	}
	for _, bad := range []string{"0", "-1", "soon"} {
		t.Setenv("METASYSTEM_TEST_INTERVAL_MS", bad)
		if _, err := Interval("METASYSTEM_TEST_INTERVAL_MS", 100); err == nil {
			t.Fatalf("value %q: want error", bad)
		}
	}
}

func TestMissionLineage(t *testing.T) {
	// Known vectors pin the derivation: the lineage must stay recomputable
	// from the mission id alone, or successors stop renewing leases.
	if got := MissionLineage("demo"); got != "mission-2a97516c354b68848cdbd8f54a226a0a" {
		t.Fatalf("demo lineage: got %s", got)
	}
	if got := MissionLineage("bm-2"); got != "mission-e522644fbef1b9a3a88c12b75368a16f" {
		t.Fatalf("bm-2 lineage: got %s", got)
	}
	long := MissionLineage("a-mission-id-long-enough-that-plain-concatenation-would-overflow-the-lineage-bound-and-truncation-would-alias-a-shared-prefix")
	if len(long) != len("mission-")+32 {
		t.Fatalf("lineage length must be fixed, got %d", len(long))
	}
}

func TestAdjudicationTables(t *testing.T) {
	// An orchestrator may keep a stream where it is, or move an active one;
	// it may never unpark. Unparking belongs to the answered-ask path.
	for state, targets := range map[string][]string{
		"active":           {"active", "parked-reserved", "parked-stop-loss", "done"},
		"parked-reserved":  {"parked-reserved"},
		"parked-stop-loss": {"parked-stop-loss"},
		"done":             {"done"},
	} {
		if len(legalStreamTransitions[state]) != len(targets) {
			t.Fatalf("transitions from %s: got %v", state, legalStreamTransitions[state])
		}
		for _, target := range targets {
			if !legalStreamTransitions[state][target] {
				t.Fatalf("transition %s to %s must be legal", state, target)
			}
		}
	}
	if legalStreamTransitions["parked-reserved"]["active"] || legalStreamTransitions["done"]["active"] {
		t.Fatal("an orchestrator must not unpark or revive a stream")
	}
	// The ask-reason vocabulary moved to internal/turn (Phase 2.5 Unit B);
	// the engine's contract is that it asks that owner, and the owner's own
	// tests pin the set.
	for _, reason := range []string{"reserved-decision", "red-test", "merge-conflict", "host-failure"} {
		if !turnvocab.OrchestratorMayRaise(reason) {
			t.Fatalf("ask reason %s must be orchestrator-raisable", reason)
		}
	}
	if turnvocab.OrchestratorMayRaise("fence") || len(turnvocab.OrchestratorAskReasons()) != 4 {
		t.Fatalf("unexpected ask reasons: %v", turnvocab.OrchestratorAskReasons())
	}
	for _, status := range []string{"completed", "failed", "timeout", "cancelled"} {
		if !terminalJobStatuses[status] {
			t.Fatalf("job status %s must be terminal", status)
		}
	}
	if terminalJobStatuses["running"] || terminalJobStatuses[""] {
		t.Fatal("running and unknown statuses must not be terminal")
	}
}
