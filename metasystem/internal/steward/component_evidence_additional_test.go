package steward

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestComponentEvidenceHealthReadIsBoundedWhenWriterIsBusy(t *testing.T) {
	root := t.TempDir()
	lock, err := lockComponentEvidence(root, "supervision-hook", unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, _, err = loadComponentEvidenceForHealth(root, "supervision-hook")
	elapsed := time.Since(started)
	var busy *ComponentEvidenceBusyError
	if !errors.As(err, &busy) || busy.Component != "supervision-hook" {
		unlockComponentEvidence(lock)
		t.Fatalf("the contended health read did not return its typed busy error: %v", err)
	}
	if elapsed >= time.Second {
		unlockComponentEvidence(lock)
		t.Fatalf("the contended health read exceeded its one-second test bound: %s", elapsed)
	}
	role := checkHookFreshness(root, time.Now())
	if role.Status != HealthUnknown || role.Reason != "component evidence for supervision-hook is busy (a writer holds its lock)" {
		unlockComponentEvidence(lock)
		t.Fatalf("hook health did not surface the bounded-lock reason with its usual remedy: %+v", role)
	}
	unlockComponentEvidence(lock)
	role = checkHookFreshness(root, time.Now())
	if role.Status != HealthDead || role.Reason != "no hook turn generation is recorded" {
		t.Fatalf("an uncontended health read changed the missing-evidence behavior: %+v", role)
	}
}

func TestExpireHookAttemptCompletesAttemptWithElapsedHistory(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 51, StartedAtSec: 100, StartTicks: 901, BootID: "boot-expire"}
	attempt, err := BeginHookAttempt(root, process, "deadline-turn", now)
	if err != nil {
		t.Fatal(err)
	}
	record, err := ExpireHookAttempt(root, 57, now.Add(57*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if record.Generation != attempt.Generation || record.AttemptSeq != attempt.AttemptSeq ||
		record.Result != ComponentError || record.Outcome != "DEADLINE_EXPIRED" ||
		record.LastStopElapsedSec == nil || *record.LastStopElapsedSec != 57 {
		t.Fatalf("deadline expiry did not complete the exact attempt with its measurement: %+v", record)
	}
	if len(record.AttemptHistory) != 1 {
		t.Fatalf("deadline expiry did not append one terminal history entry: %+v", record.AttemptHistory)
	}
	history := record.AttemptHistory[0]
	if history.Generation != attempt.Generation || history.AttemptSeq != attempt.AttemptSeq ||
		history.Result != ComponentError || history.Outcome != "DEADLINE_EXPIRED" ||
		history.StopElapsedSec == nil || *history.StopElapsedSec != 57 {
		t.Fatalf("deadline expiry history did not retain the exact terminal fact: %+v", history)
	}
	next, err := BeginHookAttempt(root, process, "turn-after-deadline", now.Add(58*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if next.Generation == attempt.Generation || len(next.AttemptHistory) != 1 ||
		next.AttemptHistory[0].Outcome != "DEADLINE_EXPIRED" {
		t.Fatalf("the next turn did not start a new generation over the recorded expiry: %+v", next)
	}
}

func TestExpireHookAttemptRefusesCompletedRecordWithoutChangingIt(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 5, 0, 0, time.UTC)
	process := identity.Ref{Pid: 52, StartedAtSec: 101, StartTicks: 902, BootID: "boot-complete"}
	attempt, err := BeginHookAttempt(root, process, "completed-turn", now)
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"systemMessage":"HEALTH healthy"}`
	if _, err := CompleteHookAttempt(root, attempt.Generation, attempt.AttemptSeq,
		ComponentOK, "EMITTED", "HEALTH healthy", payload, nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	path := ComponentEvidencePath(root, "supervision-hook")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ExpireHookAttempt(root, 57, now.Add(57*time.Second))
	var conflict *HookExpireConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("completed hook expiry did not return its typed conflict: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("refused hook expiry changed the completed record\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

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

func TestHookCompletionStoresStopElapsedSeconds(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	attempt, err := BeginHookAttempt(root, identity.Ref{Pid: 42, StartedAtSec: 100}, "measured-stop", now)
	if err != nil {
		t.Fatal(err)
	}
	line := "HEALTH healthy — hook-freshness=alive"
	payload := `{"systemMessage":"HEALTH healthy — hook-freshness=alive"}`
	elapsed := int64(7)
	if _, err := CompleteHookAttempt(root, attempt.Generation, attempt.AttemptSeq,
		ComponentOK, "EMITTED", line, payload, &elapsed, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	record, err := loadComponentEvidence(ComponentEvidencePath(root, "supervision-hook"))
	if err != nil {
		t.Fatal(err)
	}
	if record.LastStopElapsedSec == nil || *record.LastStopElapsedSec != 7 || len(record.AttemptHistory) != 1 ||
		record.AttemptHistory[0].StopElapsedSec == nil || *record.AttemptHistory[0].StopElapsedSec != 7 {
		t.Fatalf("the seven-second Stop measurement was not stored on the record and its history: %+v", record)
	}
}

func TestHookCompletionStoresZeroStopElapsedSeconds(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 2, 0, 0, time.UTC)
	attempt, err := BeginHookAttempt(root, identity.Ref{Pid: 45, StartedAtSec: 103}, "same-second-stop", now)
	if err != nil {
		t.Fatal(err)
	}
	line := "HEALTH healthy — hook-freshness=alive"
	payload := `{"systemMessage":"HEALTH healthy — hook-freshness=alive"}`
	elapsed := int64(0)
	if _, err := CompleteHookAttempt(root, attempt.Generation, attempt.AttemptSeq,
		ComponentOK, "EMITTED", line, payload, &elapsed, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	path := ComponentEvidencePath(root, "supervision-hook")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"lastStopElapsedSec": 0`) || !strings.Contains(string(data), `"stopElapsedSec": 0`) {
		t.Fatalf("the measured zero-second Stop was omitted from its record or history: %s", data)
	}
	record, err := loadComponentEvidence(path)
	if err != nil {
		t.Fatal(err)
	}
	if record.LastStopElapsedSec == nil || *record.LastStopElapsedSec != 0 || len(record.AttemptHistory) != 1 ||
		record.AttemptHistory[0].StopElapsedSec == nil || *record.AttemptHistory[0].StopElapsedSec != 0 {
		t.Fatalf("the zero-second Stop measurement did not load from the record and its history: %+v", record)
	}
}

func TestHookCompletionRejectsNegativeStopElapsedSeconds(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 5, 0, 0, time.UTC)
	attempt, err := BeginHookAttempt(root, identity.Ref{Pid: 43, StartedAtSec: 101}, "negative-stop", now)
	if err != nil {
		t.Fatal(err)
	}
	line := "HEALTH healthy — hook-freshness=alive"
	payload := `{"systemMessage":"HEALTH healthy — hook-freshness=alive"}`
	elapsed := int64(-1)
	if _, err := CompleteHookAttempt(root, attempt.Generation, attempt.AttemptSeq,
		ComponentOK, "EMITTED", line, payload, &elapsed, now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("negative Stop elapsed seconds were accepted: %v", err)
	}
}

func TestHookCompletionWithoutStopElapsedSecondsOmitsAndLoadsTheField(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 10, 0, 0, time.UTC)
	attempt, err := BeginHookAttempt(root, identity.Ref{Pid: 44, StartedAtSec: 102}, "unmeasured-stop", now)
	if err != nil {
		t.Fatal(err)
	}
	line := "HEALTH healthy — hook-freshness=alive"
	payload := `{"systemMessage":"HEALTH healthy — hook-freshness=alive"}`
	if _, err := CompleteHookAttempt(root, attempt.Generation, attempt.AttemptSeq,
		ComponentOK, "EMITTED", line, payload, nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	path := ComponentEvidencePath(root, "supervision-hook")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "lastStopElapsedSec") || strings.Contains(string(data), "stopElapsedSec") {
		t.Fatalf("an unmeasured completion unexpectedly stored an optional Stop elapsed field: %s", data)
	}
	record, err := loadComponentEvidence(path)
	if err != nil {
		t.Fatalf("a record written without Stop elapsed seconds did not load: %v", err)
	}
	if record.LastStopElapsedSec != nil || len(record.AttemptHistory) != 1 || record.AttemptHistory[0].StopElapsedSec != nil {
		t.Fatalf("a record written without Stop elapsed seconds did not load nil measurements: %+v", record)
	}
}

func TestHookRetryClearsLastStopElapsedSeconds(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 15, 0, 0, time.UTC)
	process := identity.Ref{Pid: 46, StartedAtSec: 104}
	attempt, err := BeginHookAttempt(root, process, "retried-stop", now)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := int64(7)
	if _, err := CompleteHookAttempt(root, attempt.Generation, attempt.AttemptSeq,
		ComponentError, "EMISSION_FAILED", "", "write failed", &elapsed, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	retry, err := BeginHookAttempt(root, process, "retried-stop", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if retry.Result != ComponentIndeterminate || retry.Outcome != "ATTEMPTING" || retry.LastStopElapsedSec != nil {
		t.Fatalf("the same-key retry retained its failed predecessor's Stop measurement: %+v", retry)
	}
	if len(retry.AttemptHistory) != 1 || retry.AttemptHistory[0].StopElapsedSec == nil || *retry.AttemptHistory[0].StopElapsedSec != 7 {
		t.Fatalf("the retry did not retain the measured failed completion in history: %+v", retry.AttemptHistory)
	}
	data, err := os.ReadFile(ComponentEvidencePath(root, "supervision-hook"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"lastStopElapsedSec"`) {
		t.Fatalf("the attempting retry stored a stale last Stop measurement: %s", data)
	}
}

func TestInterruptedHookHistoryOmitsStopElapsedSeconds(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 10, 20, 0, 0, time.UTC)
	process := identity.Ref{Pid: 47, StartedAtSec: 105}
	interrupted, err := BeginHookAttempt(root, process, "interrupted-stop", now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := BeginHookAttempt(root, process, "next-stop", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(next.AttemptHistory) != 1 {
		t.Fatalf("the next turn did not retain exactly one interrupted attempt: %+v", next.AttemptHistory)
	}
	history := next.AttemptHistory[0]
	if history.Generation != interrupted.Generation || history.AttemptSeq != interrupted.AttemptSeq ||
		history.Outcome != "INTERRUPTED_BY_NEXT_TURN" || history.StopElapsedSec != nil {
		t.Fatalf("the interrupted attempt carried a Stop measurement or lost its terminal identity: %+v", history)
	}
	data, err := os.ReadFile(ComponentEvidencePath(root, "supervision-hook"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"stopElapsedSec"`) {
		t.Fatalf("the interrupted attempt serialized an unsupplied Stop measurement: %s", data)
	}
}

func TestComponentAttemptHistoryReplacesRetriesAndKeepsABoundedTail(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC)
	record := ComponentEvidence{Generation: 3, AttemptSeq: 1, LastAttempt: now}
	appendAttemptHistory(&record, now.Add(time.Second), ComponentError, "FAILED", "first", nil)
	appendAttemptHistory(&record, now.Add(2*time.Second), ComponentOK, "EMITTED", "replacement", nil)
	if len(record.AttemptHistory) != 1 || record.AttemptHistory[0].Result != ComponentOK {
		t.Fatalf("retry completion duplicated the same attempt: %+v", record.AttemptHistory)
	}
	for sequence := int64(2); sequence <= componentAttemptHistoryLimit+2; sequence++ {
		record.AttemptSeq = sequence
		record.LastAttempt = now.Add(time.Duration(sequence) * time.Second)
		appendAttemptHistory(&record, record.LastAttempt.Add(time.Second), ComponentError, "FAILED", "attempt", nil)
	}
	if len(record.AttemptHistory) != componentAttemptHistoryLimit || record.AttemptHistory[0].AttemptSeq != 3 {
		t.Fatalf("attempt history did not retain the bounded newest tail: first=%d size=%d", record.AttemptHistory[0].AttemptSeq, len(record.AttemptHistory))
	}
}
