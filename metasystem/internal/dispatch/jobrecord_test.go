package dispatch

import (
	"encoding/json"
	"testing"
)

func TestJobRecordLens(t *testing.T) {
	record := JobRecordOf(map[string]any{
		"status": "running", "jobId": "j1", "role": "implementer",
		"parentJob": "root-1", "operationId": "reserve-j1", "goalId": "goal-a", "endedAt": nil, "error": nil,
		"round": json.Number("3"), "goalRevision": json.Number("7"), "capMin": json.Number("30"),
	})
	if record.Status() != "running" || record.JobID() != "j1" || record.Role() != "implementer" {
		t.Fatal("typed reads drifted")
	}
	if record.ParentJob() != "root-1" {
		t.Fatal("parent lost")
	}
	if record.GoalID() != "goal-a" {
		t.Fatal("goal provenance lost")
	}
	if record.OperationID() != "reserve-j1" {
		t.Fatal("reservation operation identity lost")
	}
	if revision, ok := record.GoalRevision(); !ok || revision != 7 {
		t.Fatalf("goal revision: %d %v", revision, ok)
	}
	if capMinutes, ok := record.CapMinutes(); !ok || capMinutes != 30 {
		t.Fatalf("cap minutes: %d %v", capMinutes, ok)
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
