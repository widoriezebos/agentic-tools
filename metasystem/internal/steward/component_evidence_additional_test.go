package steward

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestPublicComponentAttemptBoundaryRejectsStaleAndInvalidCompletions(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 41, StartedAtSec: 100, StartTicks: 900, BootID: "boot-a"}
	attempt, err := BeginComponentAttempt(root, "steward-tick", 7, process, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteComponentAttempt(root, "steward-tick", 7, attempt.AttemptSeq, ComponentResult("BROKEN"), "FAILED", "evidence", now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "invalid result") {
		t.Fatalf("invalid completion result was accepted: %v", err)
	}
	if _, err := CompleteComponentAttempt(root, "steward-tick", 7, attempt.AttemptSeq, ComponentError, "", "evidence", now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "needs an outcome") {
		t.Fatalf("completion without an outcome was accepted: %v", err)
	}
	if _, err := CompleteComponentAttempt(root, "steward-tick", 8, attempt.AttemptSeq, ComponentError, "FAILED", "evidence", now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "attempt changed") {
		t.Fatalf("another generation completed this attempt: %v", err)
	}
	if _, err := CompleteComponentAttempt(root, "steward-tick", 7, attempt.AttemptSeq, ComponentError, "FAILED", "evidence", now.Add(-time.Second)); err == nil || !strings.Contains(err.Error(), "clock is earlier") {
		t.Fatalf("completion before its attempt was accepted: %v", err)
	}
	completed, err := CompleteComponentAttempt(root, "steward-tick", 7, attempt.AttemptSeq, ComponentError, "FAILED", "named failure", now.Add(time.Second))
	if err != nil || completed.Result != ComponentError || completed.Outcome != "FAILED" || completed.LastCompletion.IsZero() {
		t.Fatalf("valid failed completion was not recorded: record=%+v err=%v", completed, err)
	}
}

func TestHookPayloadFindsHealthLineInTextAndStructuredMessages(t *testing.T) {
	healthLine := "HEALTH unhealthy — repo-watcher=dead"
	if !hookPayloadContainsHealthLine("prefix "+healthLine+" suffix", healthLine) {
		t.Fatal("plain hook payload lost its health line")
	}
	payload := `{"ignored":"first"}
{"systemMessage":"prefix HEALTH unhealthy — repo-watcher=dead suffix"}`
	if !hookPayloadContainsHealthLine(payload, healthLine) {
		t.Fatal("structured hook payload lost its health line")
	}
	if hookPayloadContainsHealthLine(`{"systemMessage":"another line"}`, healthLine) {
		t.Fatal("unrelated hook payload claimed the health line")
	}
}

func TestComponentAttemptHistoryReplacesRetriesAndKeepsABoundedTail(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC)
	record := ComponentEvidence{Generation: 3, AttemptSeq: 1, LastAttempt: now}
	appendAttemptHistory(&record, now.Add(time.Second), ComponentError, "FAILED", "first")
	appendAttemptHistory(&record, now.Add(2*time.Second), ComponentOK, "EMITTED", "replacement")
	if len(record.AttemptHistory) != 1 || record.AttemptHistory[0].Result != ComponentOK {
		t.Fatalf("retry completion duplicated the same attempt: %+v", record.AttemptHistory)
	}
	for sequence := int64(2); sequence <= componentAttemptHistoryLimit+2; sequence++ {
		record.AttemptSeq = sequence
		record.LastAttempt = now.Add(time.Duration(sequence) * time.Second)
		appendAttemptHistory(&record, record.LastAttempt.Add(time.Second), ComponentError, "FAILED", "attempt")
	}
	if len(record.AttemptHistory) != componentAttemptHistoryLimit || record.AttemptHistory[0].AttemptSeq != 3 {
		t.Fatalf("attempt history did not retain the bounded newest tail: first=%d size=%d", record.AttemptHistory[0].AttemptSeq, len(record.AttemptHistory))
	}
}
