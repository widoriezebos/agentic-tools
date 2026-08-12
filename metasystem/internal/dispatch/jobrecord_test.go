package dispatch

import (
	"encoding/json"
	"testing"
)

func TestJobRecordLens(t *testing.T) {
	record := JobRecordOf(map[string]any{
		"status": "running", "jobId": "j1", "role": "implementer",
		"parentJob": "root-1", "endedAt": nil, "error": nil,
		"round": json.Number("3"),
	})
	if record.Status() != "running" || record.JobID() != "j1" || record.Role() != "implementer" {
		t.Fatal("typed reads drifted")
	}
	if record.ParentJob() != "root-1" {
		t.Fatal("parent lost")
	}
	if record.EndedAt() != "" || record.ErrorText() != "" {
		t.Fatal("null must read as the zero value")
	}
	if round, ok := record.Round(); !ok || round != 3 {
		t.Fatalf("round: %d %v", round, ok)
	}
	// Ill-typed fields read as zero values, never panic — the cast contract.
	hostile := JobRecordOf(map[string]any{"status": 42, "round": "three"})
	if hostile.Status() != "" {
		t.Fatal("an ill-typed status must read empty")
	}
	if _, ok := hostile.Round(); ok {
		t.Fatal("an ill-typed round must read not-ok")
	}
	// The lens shares the map: a CAS-style patch is visible immediately.
	raw := record.Raw()
	raw["status"] = "completed"
	if record.Status() != "completed" {
		t.Fatal("the lens does not share the document")
	}
}
