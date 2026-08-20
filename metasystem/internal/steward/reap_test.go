package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func consumedIntentOnDisk(t *testing.T, root string, it Intent) {
	t.Helper()
	dir := consumedDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(it, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, it.Nonce+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func jobRecordOnDisk(t *testing.T, root, jobId, status, endedAt string) {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"status": status, "endedAt": endedAt}
	data, _ := json.Marshal(body)
	if err := os.WriteFile(filepath.Join(dir, jobId+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEndedContinuationReapsAndFreesTheGuard(t *testing.T) {
	root := t.TempDir()
	it := testIntent("rp-1")
	it.Notified, it.LaunchStamped = true, true
	consumedIntentOnDisk(t, root, it)
	jobRecordOnDisk(t, root, it.JobId, "completed", "2026-08-20T13:00:00Z")

	reports, err := ReapContinuations(root)
	if err != nil || len(reports) != 1 || !strings.Contains(reports[0].Outcome, "ended completed") {
		t.Fatalf("an ended job reaps with its outcome: %+v %v", reports, err)
	}
	if active, _ := ConsumedActive(root); len(active) != 0 {
		t.Fatalf("a reaped continuation frees the guard: %v", active)
	}
	if pending, _ := PendingNotifications(root); len(pending) != 1 {
		t.Fatalf("the close is told to the operator: %v", pending)
	}
	again, err := ReapContinuations(root)
	if err != nil || len(again) != 0 {
		t.Fatalf("reaping is idempotent: %+v %v", again, err)
	}
}

func TestRunningContinuationIsLeftAlone(t *testing.T) {
	root := t.TempDir()
	it := testIntent("rp-2")
	it.Notified, it.LaunchStamped = true, true
	consumedIntentOnDisk(t, root, it)
	jobRecordOnDisk(t, root, it.JobId, "running", "")
	reports, err := ReapContinuations(root)
	if err != nil || len(reports) != 0 {
		t.Fatalf("a running job is not reaped: %+v %v", reports, err)
	}
	if active, _ := ConsumedActive(root); len(active) != 1 {
		t.Fatal("the guard stays while it runs")
	}
}

func TestUnstampedConsumedReconcilesAsNotifiedUnknown(t *testing.T) {
	root := t.TempDir()
	it := testIntent("rp-3")
	it.Notified = true // consumed after delivery, crashed before stamp
	consumedIntentOnDisk(t, root, it)
	reports, err := ReapContinuations(root)
	if err != nil || len(reports) != 1 || !strings.Contains(reports[0].Outcome, "outcome unknown") {
		t.Fatalf("the crash boundary reconciles visibly: %+v %v", reports, err)
	}
}

func TestStampedWithoutRecordIsAnIncidentNotAGuess(t *testing.T) {
	root := t.TempDir()
	it := testIntent("rp-4")
	it.Notified, it.LaunchStamped = true, true
	consumedIntentOnDisk(t, root, it)
	reports, err := ReapContinuations(root)
	if err != nil || len(reports) != 1 || !strings.Contains(reports[0].Outcome, "no job record") {
		t.Fatalf("a traceless launch is named, not guessed at: %+v %v", reports, err)
	}
}

func TestReapClosesTheChainAndValidatesTheReturn(t *testing.T) {
	root := t.TempDir()
	it := testIntent("rp-5")
	it.Notified, it.LaunchStamped = true, true
	consumedIntentOnDisk(t, root, it)
	jobRecordOnDisk(t, root, it.JobId, "completed", "2026-08-20T15:30:00Z")
	// A return exists but the standing checker will find the record
	// incomplete (no chain fields): the outcome names the protocol
	// error instead of blessing the bytes.
	dir := filepath.Join(root, "artifacts", "agents", it.JobId, "rounds", "1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "return.json"), []byte(`{"jobId":"job-1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reports, err := ReapContinuations(root)
	if err != nil || len(reports) != 1 || !strings.Contains(reports[0].Outcome, "PROTOCOL-ERROR") {
		t.Fatalf("an invalid return is named a protocol error: %+v %v", reports, err)
	}
	// The chain is closed regardless: no permanently open chain.
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", it.JobId+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if closed, _ := record["chainClosed"].(bool); !closed {
		t.Fatalf("the reaper closes the chain it reaps: %+v", record)
	}
}
