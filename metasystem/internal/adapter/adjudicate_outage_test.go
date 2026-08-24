package adapter

// The delegate launches are the second detection point of the
// provider-outage posture: a runtime CLI that dies on the provider's
// 529 feeds the shared outage mark, an ordinary crash does not, and a
// completed turn clears a standing mark. The verdicts themselves never
// change — the bookkeeping rides beside the state machine.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func adjudicateBed(t *testing.T) (AdjudicateParams, string) {
	t.Helper()
	root := t.TempDir()
	logPath := filepath.Join(root, "job.log")
	return AdjudicateParams{
		Stage:         "initial",
		Root:          root,
		Job:           "job-1",
		LogPath:       logPath,
		ViolationPath: filepath.Join(root, "protocol-violation.txt"),
		ReturnPath:    filepath.Join(root, "return.json"),
		MarkdownPath:  filepath.Join(root, "return.md"),
		CLIStatus:     1,
	}, logPath
}

// A non-zero CLI whose stderr carries the provider's 529 marks the
// outage; the verdict is the same runtime_error it always was.
func TestAdjudicateOverloadedCLIMarksTheOutage(t *testing.T) {
	p, logPath := adjudicateBed(t)
	if err := os.WriteFile(logPath, []byte(
		"API Error: 529 {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := AdjudicateTurn(p)
	if err != nil || verdict != "fail-pending runtime_error handshake" {
		t.Fatalf("the verdict must not change for an overload: %q %v", verdict, err)
	}
	mark, ok := outage.Read(p.Root)
	if !ok || mark.LastClass != "overloaded" || mark.Source != "delegate-adapter" {
		t.Fatalf("the overload must feed the mark: %+v ok=%v", mark, ok)
	}
}

// An ordinary CLI crash marks nothing: only the provider's own weather
// belongs in the outage record.
func TestAdjudicateOrdinaryCrashMarksNothing(t *testing.T) {
	p, logPath := adjudicateBed(t)
	if err := os.WriteFile(logPath, []byte("panic: nil pointer dereference\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AdjudicateTurn(p); err != nil {
		t.Fatal(err)
	}
	if _, ok := outage.Read(p.Root); ok {
		t.Fatal("an ordinary crash must not mark an outage")
	}
}

// A completed adjudication clears a standing mark: any provider
// success ends the outage.
func TestAdjudicateCompletedClearsTheMark(t *testing.T) {
	root := t.TempDir()
	if _, err := outage.Record(root, "overloaded", "529", "mission-runner", time.Now()); err != nil {
		t.Fatal(err)
	}
	verdict, err := AdjudicateTurn(AdjudicateParams{
		Stage:    "settle-result",
		Root:     root,
		SettleOK: true,
	})
	if err != nil || verdict != "finish completed null completed" {
		t.Fatalf("the settle verdict: %q %v", verdict, err)
	}
	if _, ok := outage.Read(root); ok {
		t.Fatal("a completed turn must clear the outage mark")
	}
}

// Provider success is a PROVEN conversation, not a zero exit: with the
// handshake correlated, a zero-status call clears the mark even when
// its reply fails validation; without the handshake, exit 0 alone
// proves nothing and the mark stands.
func TestAdjudicateProviderSuccessNeedsEvidence(t *testing.T) {
	p, _ := adjudicateBed(t)
	if _, err := outage.Record(p.Root, "overloaded", "529", "delegate-adapter", time.Now()); err != nil {
		t.Fatal(err)
	}
	p.CLIStatus = 0
	verdict, err := AdjudicateTurn(p)
	if err != nil || verdict != "fail-pending handshake_missing_session_id handshake" {
		t.Fatalf("the no-handshake verdict: %q %v", verdict, err)
	}
	if _, ok := outage.Read(p.Root); !ok {
		t.Fatal("exit 0 without a handshake proves nothing; the mark must stand")
	}
	p.HandshakeDone = true
	// The candidate is absent, so validation fails — but the correlated
	// handshake proves the provider answered.
	if _, err := AdjudicateTurn(p); err != nil {
		t.Fatal(err)
	}
	if _, ok := outage.Read(p.Root); ok {
		t.Fatal("a correlated conversation must clear the mark despite the task failing")
	}
}

// A zero-status CLI whose result is the provider's own error document
// records the outage instead of clearing it.
func TestAdjudicateErrorDocumentOnZeroExitRecords(t *testing.T) {
	p, _ := adjudicateBed(t)
	p.CLIStatus = 0
	p.HandshakeDone = true
	p.CandidatePath = filepath.Join(p.Root, "claude-result.json")
	if err := os.WriteFile(p.CandidatePath, []byte(
		`{"is_error":true,"result":"API Error: 529 Overloaded"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AdjudicateTurn(p); err != nil {
		t.Fatal(err)
	}
	mark, ok := outage.Read(p.Root)
	if !ok || mark.LastClass != "overloaded" {
		t.Fatalf("the provider's error document on a zero exit must record: %+v ok=%v", mark, ok)
	}
}

// The paid repair call is a detection point too: its stderr shares the
// same log, so a repair that dies on the provider's 529 feeds the mark.
func TestAdjudicateOverloadedRepairMarksTheOutage(t *testing.T) {
	p, logPath := adjudicateBed(t)
	if err := os.WriteFile(logPath, []byte("API Error: 529 Overloaded\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.Stage = "after-repair"
	p.CLIStatus = 0
	p.RepairRC = 1
	verdict, err := AdjudicateTurn(p)
	if err != nil || verdict != "protocol-error" {
		t.Fatalf("the failed-repair verdict: %q %v", verdict, err)
	}
	mark, ok := outage.Read(p.Root)
	if !ok || mark.Source != "delegate-adapter" {
		t.Fatalf("a repair call dying on overload must feed the mark: %+v ok=%v", mark, ok)
	}
}
