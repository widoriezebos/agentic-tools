package dispatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// sandbox creates a checkout root and returns it. Records land under
// artifacts/agents/jobs, locks under artifacts/agents/record-locks.
func sandbox(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// writeJSON writes value as a JSON file the lifecycle can read back.
func writeJSON(t *testing.T, path string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// readRecord loads a written job record for assertions.
func readRecord(t *testing.T, root, job string) map[string]any {
	t.Helper()
	_, recordPath, _ := paths(root, job)
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read record %s: %v", job, err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("parse record %s: %v", job, err)
	}
	return value
}

// createPending reserves a job and returns nothing; failures fail the test.
func createPending(t *testing.T, root, job string) {
	t.Helper()
	source := writeJSON(t, filepath.Join(t.TempDir(), "create.json"), map[string]any{
		"jobId": job, "status": "pending-setup", "phase": "setup",
		"error": nil, "mainId": "main-1", "claimEpoch": 5,
		"createdAt": "2026-08-10T00:00:00Z",
	})
	if err := RecordCreate(root, job, source); err != nil {
		t.Fatalf("create %s: %v", job, err)
	}
}

// setupPending completes the reservation, moving the record to pending.
func setupPending(t *testing.T, root, job string) {
	t.Helper()
	source := writeJSON(t, filepath.Join(t.TempDir(), "setup.json"), map[string]any{
		"jobId": job, "role": "implementer", "runtime": "fake", "round": 1,
		"parentJob": nil, "status": "pending", "phase": "handshake", "error": nil,
		"mainId": "main-1", "claimEpoch": 5, "sessionId": nil,
		"startedAt": "2026-08-10T00:00:01Z", "endedAt": nil,
	})
	if err := RecordSetup(root, job, source); err != nil {
		t.Fatalf("setup %s: %v", job, err)
	}
}

func wantCode(t *testing.T, err error, code int) {
	t.Helper()
	var op *OpError
	if !errors.As(err, &op) {
		t.Fatalf("expected an OpError with code %d, got %v", code, err)
	}
	if op.Code != code {
		t.Fatalf("expected code %d, got %d (%q)", code, op.Code, op.Message)
	}
}

func TestRecordCreateWritesReservation(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")

	record := readRecord(t, root, "job-a")
	if record["status"] != "pending-setup" {
		t.Fatalf("status = %v, want pending-setup", record["status"])
	}
	if record["jobId"] != "job-a" {
		t.Fatalf("jobId = %v", record["jobId"])
	}
}

func TestRecordCreateRefusesCollision(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")

	source := writeJSON(t, filepath.Join(t.TempDir(), "dup.json"), map[string]any{
		"jobId": "job-a", "status": "pending-setup",
	})
	wantCode(t, RecordCreate(root, "job-a", source), 1)
}

func TestRecordCreateRefusesWrongStatus(t *testing.T) {
	root := sandbox(t)
	source := writeJSON(t, filepath.Join(t.TempDir(), "bad.json"), map[string]any{
		"jobId": "job-a", "status": "pending",
	})
	wantCode(t, RecordCreate(root, "job-a", source), 1)
}

func TestRecordSetupCompletesReservation(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	record := readRecord(t, root, "job-a")
	if record["status"] != "pending" {
		t.Fatalf("status = %v, want pending", record["status"])
	}
}

func TestRecordSetupRefusesEpochMismatch(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")

	source := writeJSON(t, filepath.Join(t.TempDir(), "setup.json"), map[string]any{
		"jobId": "job-a", "status": "pending",
		"mainId": "main-1", "claimEpoch": 9, // reservation was 5
		"startedAt": "2026-08-10T00:00:01Z",
	})
	wantCode(t, RecordSetup(root, "job-a", source), 1)
}

func TestRecordCASAdvancesStatus(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	patch := writeJSON(t, filepath.Join(t.TempDir(), "run.json"), map[string]any{
		"sessionId": "sess-1", "phase": "running", "error": nil,
	})
	observed, err := RecordCAS(root, "job-a", "pending", "running", patch)
	if err != nil {
		t.Fatalf("cas pending->running: %v", err)
	}
	if observed != "" {
		t.Fatalf("unexpected observed on success: %q", observed)
	}
	record := readRecord(t, root, "job-a")
	if record["status"] != "running" {
		t.Fatalf("status = %v, want running", record["status"])
	}
	if record["sessionId"] != "sess-1" {
		t.Fatalf("sessionId = %v, want sess-1", record["sessionId"])
	}
}

func TestRecordCASRefusesStaleExpected(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	// The record is pending, but the caller expected running.
	patch := writeJSON(t, filepath.Join(t.TempDir(), "p.json"), map[string]any{"note": "x"})
	observed, err := RecordCAS(root, "job-a", "running", "completed", patch)
	wantCode(t, err, 3)
	if observed != "observed=pending" {
		t.Fatalf("observed = %q, want observed=pending", observed)
	}
	// The record must be untouched by a lost compare.
	record := readRecord(t, root, "job-a")
	if record["status"] != "pending" {
		t.Fatalf("status = %v, want pending (unchanged)", record["status"])
	}
}

func TestRecordCASRefusesStatusInPatch(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	patch := writeJSON(t, filepath.Join(t.TempDir(), "p.json"), map[string]any{"status": "running"})
	_, err := RecordCAS(root, "job-a", "pending", "running", patch)
	wantCode(t, err, 1)
}

func TestRecordCASRefusesImmutableField(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	patch := writeJSON(t, filepath.Join(t.TempDir(), "p.json"), map[string]any{"capMin": 999})
	_, err := RecordCAS(root, "job-a", "pending", "running", patch)
	wantCode(t, err, 1)
}

func TestRecordCASRefusesIllegalTransition(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	// pending -> completed is not a lawful edge.
	patch := writeJSON(t, filepath.Join(t.TempDir(), "p.json"), map[string]any{"note": "x"})
	_, err := RecordCAS(root, "job-a", "pending", "completed", patch)
	wantCode(t, err, 1)
}

// TestRecordCASReapStampsEndedAt exercises the reap path: a running job that a
// reaper judges lost is compare-and-swapped to a terminal status, which must
// stamp endedAt.
func TestRecordCASReapStampsEndedAt(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	run := writeJSON(t, filepath.Join(t.TempDir(), "run.json"), map[string]any{"sessionId": "s"})
	if _, err := RecordCAS(root, "job-a", "pending", "running", run); err != nil {
		t.Fatalf("cas to running: %v", err)
	}

	verdict := writeJSON(t, filepath.Join(t.TempDir(), "lost.json"), map[string]any{
		"error": "process-lost", "phase": "supervision",
	})
	observed, err := RecordCAS(root, "job-a", "running", "failed", verdict)
	if err != nil {
		t.Fatalf("reap cas running->failed: %v", err)
	}
	if observed != "" {
		t.Fatalf("unexpected observed: %q", observed)
	}
	record := readRecord(t, root, "job-a")
	if record["status"] != "failed" {
		t.Fatalf("status = %v, want failed", record["status"])
	}
	ended, ok := record["endedAt"].(string)
	if !ok || ended == "" {
		t.Fatalf("endedAt not stamped on terminal reap: %v", record["endedAt"])
	}
	if record["error"] != "process-lost" {
		t.Fatalf("error = %v, want process-lost", record["error"])
	}
}

// TestRecordCASTerminalMetadataFinal proves a finished record accepts only the
// closure/mirror metadata fields and refuses anything else.
func TestRecordCASTerminalMetadataFinal(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")
	run := writeJSON(t, filepath.Join(t.TempDir(), "run.json"), map[string]any{"sessionId": "s"})
	if _, err := RecordCAS(root, "job-a", "pending", "running", run); err != nil {
		t.Fatalf("to running: %v", err)
	}
	done := writeJSON(t, filepath.Join(t.TempDir(), "done.json"), map[string]any{"error": nil})
	if _, err := RecordCAS(root, "job-a", "running", "completed", done); err != nil {
		t.Fatalf("to completed: %v", err)
	}

	// A closure flag is allowed as a terminal metadata update.
	ok := writeJSON(t, filepath.Join(t.TempDir(), "close.json"), map[string]any{"chainClosed": true})
	if _, err := RecordCAS(root, "job-a", "completed", "completed", ok); err != nil {
		t.Fatalf("terminal closure update refused: %v", err)
	}
	// An arbitrary field is not.
	bad := writeJSON(t, filepath.Join(t.TempDir(), "bad.json"), map[string]any{"phase": "tampered"})
	_, err := RecordCAS(root, "job-a", "completed", "completed", bad)
	wantCode(t, err, 1)
}

func TestRecordProtocolErrorStampsAndIsIdempotent(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")

	if err := RecordProtocolError(root, "job-a", "pending", "missing return.json", ""); err != nil {
		t.Fatalf("protocol error stamp: %v", err)
	}
	record := readRecord(t, root, "job-a")
	if record["status"] != "failed" || record["error"] != "protocol_error" {
		t.Fatalf("record = %v, want failed/protocol_error", record)
	}
	pe, ok := record["protocolError"].(map[string]any)
	if !ok || pe["key"] == "" {
		t.Fatalf("protocolError not stamped: %v", record["protocolError"])
	}
	firstKey := pe["key"]

	// A second identical stamp is idempotent: no error, key unchanged.
	if err := RecordProtocolError(root, "job-a", "pending", "missing return.json", ""); err != nil {
		t.Fatalf("idempotent protocol error: %v", err)
	}
	again := readRecord(t, root, "job-a")
	if again["protocolError"].(map[string]any)["key"] != firstKey {
		t.Fatalf("idempotent stamp changed the key")
	}

	// A different violation on an already-failed record cannot re-stamp: the
	// status no longer matches the pending/running expectation.
	err := RecordProtocolError(root, "job-a", "pending", "a different violation", "")
	wantCode(t, err, 3)
}

func TestRecordCASRejectsInvalidJobID(t *testing.T) {
	root := sandbox(t)
	patch := writeJSON(t, filepath.Join(t.TempDir(), "p.json"), map[string]any{"note": "x"})
	_, err := RecordCAS(root, "Bad_Id", "pending", "running", patch)
	wantCode(t, err, 2)
}
