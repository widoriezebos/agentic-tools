package missionrunner

import "testing"

func TestTurnRecordLens(t *testing.T) {
	if record := TurnRecordOf(map[string]any{"runtime": "fake", "status": "running"}); record.Runtime() != "fake" {
		t.Fatal("typed read drifted")
	}
	if hostile := TurnRecordOf(map[string]any{"runtime": 42}); hostile.Runtime() != "" {
		t.Fatal("ill-typed runtime must read empty")
	}
}
