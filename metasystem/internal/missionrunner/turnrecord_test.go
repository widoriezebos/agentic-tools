package missionrunner

import (
	"encoding/json"
	"testing"
)

func TestTurnRecordLens(t *testing.T) {
	record := TurnRecordOf(map[string]any{
		"runtime": "fake", "status": "running", "outcome": nil,
		"hostSession": "sess-1", "instanceTag": "metasystem-host-t1",
		"turnCapMin": json.Number("30"),
	})
	if record.Runtime() != "fake" || record.Status() != "running" {
		t.Fatal("typed reads drifted")
	}
	if record.Outcome() != "" {
		t.Fatal("null outcome must read empty")
	}
	if record.HostSession() != "sess-1" || record.InstanceTag() != "metasystem-host-t1" {
		t.Fatal("session fields drifted")
	}
	if cap, ok := record.TurnCapMin(); !ok || cap != 30 {
		t.Fatalf("cap: %d %v", cap, ok)
	}
	hostile := TurnRecordOf(map[string]any{"runtime": 42, "turnCapMin": "thirty"})
	if hostile.Runtime() != "" {
		t.Fatal("ill-typed runtime must read empty")
	}
	if _, ok := hostile.TurnCapMin(); ok {
		t.Fatal("ill-typed cap must read not-ok")
	}
	// Shared document: launch-time patches stay visible through the lens.
	raw := record.Raw()
	raw["status"] = "completed"
	if record.Status() != "completed" {
		t.Fatal("the lens does not share the document")
	}
}
