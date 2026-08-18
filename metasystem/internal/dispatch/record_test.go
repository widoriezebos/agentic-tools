package dispatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// The reader-visibility property, ported from record-protocol-fixtures.sh's
// concurrent python reader (script-fixtures-012/D43): while a protocol
// error is applied, no reader may ever observe status=failed WITHOUT its
// protocolError object — atomicfile's rename is what guarantees the two
// land together — and no .tmp residue may survive in the jobs directory.
func TestRecordProtocolErrorNeverExposesFailedWithoutViolation(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-vis")
	setupPending(t, root, "job-vis")
	recordPath := filepath.Join(root, "artifacts", "agents", "jobs", "job-vis.json")

	stop := make(chan struct{})
	torn := make(chan string, 1)
	var waiter sync.WaitGroup
	waiter.Add(1)
	go func() {
		defer waiter.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(recordPath)
			if err != nil {
				continue // mid-rename absence is not a torn state
			}
			var record map[string]any
			if json.Unmarshal(data, &record) != nil {
				select {
				case torn <- "reader saw unparseable record bytes":
				default:
				}
				return
			}
			if record["status"] == "failed" {
				if _, ok := record["protocolError"].(map[string]any); !ok {
					select {
					case torn <- "reader saw failed without protocolError":
					default:
					}
					return
				}
			}
		}
	}()
	for attempt := 0; attempt < 50; attempt++ {
		if err := RecordProtocolError(root, "job-vis", "pending", "missing return.json", ""); err != nil {
			t.Fatalf("protocol error stamp: %v", err)
		}
	}
	close(stop)
	waiter.Wait()
	select {
	case seen := <-torn:
		t.Fatal(seen)
	default:
	}
	residue, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.tmp*"))
	if len(residue) != 0 {
		t.Fatalf(".tmp residue survived: %v", residue)
	}
}

// RepairClaim's contract (D64): absent returnRepairs means zero; the
// claim wins once, atomically, only on a running record; a second claim
// and a non-running record are delegate-side losses (observed set); an
// unreadable record is a harness failure (error).
func TestRepairClaimWinsOnceAbsentMeansZero(t *testing.T) {
	root := t.TempDir()
	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-r.json"),
		map[string]any{"jobId": "job-r", "status": "running"})

	observed, err := RepairClaim(root, "job-r")
	if err != nil || observed != "" {
		t.Fatalf("first claim on an absent field must win: observed=%q err=%v", observed, err)
	}
	record, _ := readObject(filepath.Join(root, "artifacts/agents/jobs/job-r.json"))
	if isZeroNumber(record["returnRepairs"]) {
		t.Fatalf("won claim must stamp returnRepairs=1, got %v", record["returnRepairs"])
	}

	observed, err = RepairClaim(root, "job-r")
	if err == nil || observed != "observed=returnRepairs-claimed" {
		t.Fatalf("second claim must lose as already-claimed: observed=%q err=%v", observed, err)
	}

	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-c.json"),
		map[string]any{"jobId": "job-c", "status": "completed"})
	observed, err = RepairClaim(root, "job-c")
	if err == nil || observed != "observed=status=completed" {
		t.Fatalf("non-running record must lose with the status named: observed=%q err=%v", observed, err)
	}

	if _, err = RepairClaim(root, "job-absent"); err == nil {
		t.Fatal("an unreadable record is a harness failure, not a loss")
	}
}

func TestRepairClaimExplicitZeroWins(t *testing.T) {
	root := t.TempDir()
	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-z.json"),
		map[string]any{"jobId": "job-z", "status": "running", "returnRepairs": 0})
	if observed, err := RepairClaim(root, "job-z"); err != nil || observed != "" {
		t.Fatalf("explicit zero must win: observed=%q err=%v", observed, err)
	}
	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-one.json"),
		map[string]any{"jobId": "job-one", "status": "running", "returnRepairs": 1})
	if observed, err := RepairClaim(root, "job-one"); err == nil || observed != "observed=returnRepairs-claimed" {
		t.Fatalf("returnRepairs=1 must lose: observed=%q err=%v", observed, err)
	}
}

// Issue #10 (1): a FRESH chain root must not claim a later round in its
// name; the conventional -r1 root and resume records stay lawful.
func TestRecordCreateRefusesFreshLaterRoundNames(t *testing.T) {
	root := t.TempDir()
	write := func(job string, parent any) string {
		path := filepath.Join(root, job+".src.json")
		record := map[string]any{"jobId": job, "status": "pending-setup", "parentJob": parent}
		data, _ := json.Marshal(record)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if err := RecordCreate(root, "demo-c1-critique-r2", write("demo-c1-critique-r2", nil)); err == nil {
		t.Fatal("a fresh job named -r2 must refuse")
	} else if !strings.Contains(err.Error(), "must not claim round 2") {
		t.Fatalf("refusal must name the rule: %v", err)
	}
	if err := RecordCreate(root, "demo-c1-critique-r1", write("demo-c1-critique-r1", nil)); err != nil {
		t.Fatalf("the conventional -r1 chain root must stay lawful: %v", err)
	}
	if err := RecordCreate(root, "demo-c1-critique-r1-r2", write("demo-c1-critique-r1-r2", "demo-c1-critique-r1")); err != nil {
		t.Fatalf("a resume record may carry the round it is: %v", err)
	}
	// The boundary is numeric N >= 2 exactly (round 2): -r0 is odd but
	// claims no later round, so it stays lawful.
	if err := RecordCreate(root, "demo-c1-critique-r0", write("demo-c1-critique-r0", nil)); err != nil {
		t.Fatalf("-r0 claims no later round and must stay lawful: %v", err)
	}
	if err := RecordCreate(root, "demo-c1-critique-r12", write("demo-c1-critique-r12", nil)); err == nil {
		t.Fatal("a fresh job named -r12 must refuse")
	}
	// An overflow suffix fails CLOSED, never through (round 2).
	overflow := "demo-c1-critique-r9223372036854775808"
	if err := RecordCreate(root, overflow, write(overflow, nil)); err == nil {
		t.Fatal("an unparseable round suffix must refuse, not bypass")
	}
}
