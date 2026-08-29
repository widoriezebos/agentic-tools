package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestO13ProvenanceLifecycleGoalSurvivesTerminalAndFollowUp(t *testing.T) {
	root := sandbox(t)
	stage := t.TempDir()
	capFile := writeJSON(t, filepath.Join(stage, "cap.json"), map[string]any{
		"capMin": 5, "capDeadline": "2026-08-20T01:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	setup := filepath.Join(stage, "root-setup.json")
	if err := BuildSetup(setup, "root-job", "implementer", "", "main-1", "5", "goal-a", 2, capFile, "bed-m1"); err != nil {
		t.Fatal(err)
	}
	if err := RecordCreate(root, "root-job", setup); err != nil {
		t.Fatal(err)
	}
	full := writeJSON(t, filepath.Join(stage, "root-full.json"), map[string]any{
		"jobId": "root-job", "operationId": "root-job", "role": "implementer", "runtime": "fake", "round": 1,
		"mission": nil, "missionIncarnation": nil, "stream": nil, "reviews": nil,
		"goalId": "goal-a", "goalRevision": 2, "machineId": "bed-m1", "parentJob": nil, "status": "pending", "phase": "handshake",
		"error": nil, "mainId": "main-1", "claimEpoch": 5, "capMin": 5,
		"workspaceRoot": root, "baseSha": "base", "branch": "main",
		"permissions":    map[string]any{"requested": map[string]any{}},
		"requestedModel": "fake-model", "startedAt": "2026-08-20T00:00:00Z", "endedAt": nil,
	})
	if err := RecordSetup(root, "root-job", full); err != nil {
		t.Fatal(err)
	}
	toRunning := writeJSON(t, filepath.Join(stage, "running.json"), map[string]any{"sessionId": "session-a"})
	if _, err := RecordCAS(root, "root-job", "pending", "running", toRunning); err != nil {
		t.Fatal(err)
	}
	toDone := writeJSON(t, filepath.Join(stage, "done.json"), map[string]any{"error": nil})
	if _, err := RecordCAS(root, "root-job", "running", "completed", toDone); err != nil {
		t.Fatal(err)
	}
	if got := readRecord(t, root, "root-job")["goalId"]; got != "goal-a" {
		t.Fatalf("terminal root goalId = %v", got)
	}

	followFile := filepath.Join(stage, "follow.json")
	if err := BuildFollowRecord(BuildFollowRecordParams{
		Output: followFile, Parent: filepath.Join(root, "artifacts", "agents", "jobs", "root-job.json"),
		Job: "root-job-r2", Round: 2, ParentJob: "root-job", Fallbacks: "[]",
		ResumeMode: "fresh-context", CapResolution: capFile, Root: root,
		MainID: "main-1", ClaimEpoch: "5", GoalRevision: 2,
	}); err != nil {
		t.Fatal(err)
	}
	childSetup := filepath.Join(stage, "child-setup.json")
	if err := BuildSetup(childSetup, "root-job-r2", "implementer", "root-job", "main-1", "5", "goal-a", 2, capFile, "bed-m1"); err != nil {
		t.Fatal(err)
	}
	if err := RecordCreate(root, "root-job-r2", childSetup); err != nil {
		t.Fatal(err)
	}
	if err := RecordSetup(root, "root-job-r2", followFile); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordCAS(root, "root-job-r2", "pending", "running", toRunning); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordCAS(root, "root-job-r2", "running", "completed", toDone); err != nil {
		t.Fatal(err)
	}
	child := readRecord(t, root, "root-job-r2")
	if child["goalId"] != "goal-a" || child["goalRevision"].(float64) != 2 ||
		child["operationId"] != "root-job-r2" || child["status"] != "completed" {
		t.Fatalf("terminal follow-up lost goal provenance: %+v", child)
	}
	if _, err := os.Stat(followFile); err != nil {
		t.Fatal(err)
	}
}

func TestRecordSetupRefusesGoalReplacement(t *testing.T) {
	root := sandbox(t)
	stage := t.TempDir()
	capFile := writeJSON(t, filepath.Join(stage, "cap.json"), map[string]any{
		"capMin": 5, "capDeadline": "2026-08-20T01:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	setup := filepath.Join(stage, "setup.json")
	if err := BuildSetup(setup, "goal-bound", "implementer", "", "main-1", "5", "goal-a", 2, capFile, "bed-m1"); err != nil {
		t.Fatal(err)
	}
	if err := RecordCreate(root, "goal-bound", setup); err != nil {
		t.Fatal(err)
	}
	replacement := writeJSON(t, filepath.Join(stage, "replacement.json"), map[string]any{
		"jobId": "goal-bound", "operationId": "goal-bound", "status": "pending", "mainId": "main-1", "claimEpoch": 5,
		"goalId": "goal-b", "goalRevision": 2, "machineId": "bed-m1", "capMin": 5, "startedAt": "2026-08-20T00:00:00Z",
	})
	wantCode(t, RecordSetup(root, "goal-bound", replacement), 1)
	capReplacement := writeJSON(t, filepath.Join(stage, "cap-replacement.json"), map[string]any{
		"jobId": "goal-bound", "operationId": "goal-bound", "status": "pending", "mainId": "main-1", "claimEpoch": 5,
		"goalId": "goal-a", "goalRevision": 2, "machineId": "bed-m1", "capMin": 6, "startedAt": "2026-08-20T00:00:00Z",
	})
	wantCode(t, RecordSetup(root, "goal-bound", capReplacement), 1)
}
