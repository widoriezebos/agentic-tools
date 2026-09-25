package launch

// The record on disk: what is written, what is listed, and what happens to a
// launch whose process died.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var recordClock = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

const secondLaunch = "01M3BQ0000000000000000000A"

func TestARecordIsWrittenAndReadBackWhole(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	written := Record{
		Launch: launchID, Machine: "m1f", Destination: "/w/agentic-tools-m1f",
		ClonedCommit: headCommit, BuiltStamp: headCommit,
		Process: Process{PID: 4242, StartedAt: 1764000000},
		Outcome: OutcomeRunning, ReviewBy: reviewBy,
		Created: Created{Destination: true},
		Steps:   []Step{{Step: StepClone, Outcome: StepDone, At: "2026-09-25T12:00:00Z"}},
		Next:    Next{Session: "cd /w/agentic-tools-m1f && claude"},
	}
	if err := Save(checkout, written); err != nil {
		t.Fatalf("save: %v", err)
	}
	read, err := Load(checkout, launchID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if read.SchemaVersion != SchemaVersion || read.Machine != "m1f" || read.Process.PID != 4242 ||
		len(read.Steps) != 1 || read.Next.Session != written.Next.Session {
		t.Fatalf("record = %+v", read)
	}
}

func TestListIsEveryRecordNewestFirstAndSkipsWhatItCannotRead(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	for _, id := range []string{launchID, secondLaunch} {
		if err := Save(checkout, Record{Launch: id, Machine: "m" + id[:2], Outcome: OutcomeDone}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(Dir(checkout), "01M3BQ0000000000000000000B.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(checkout), "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := List(checkout)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 2 || records[0].Launch != launchID || records[1].Launch != secondLaunch {
		t.Fatalf("records = %+v, want the two readable ones, newest first", records)
	}
}

func TestAHostThatHasLaunchedNothingListsNothingAndDoesNotFail(t *testing.T) {
	t.Parallel()
	records, err := List(t.TempDir())
	if err != nil || len(records) != 0 {
		t.Fatalf("list = %v, %v", records, err)
	}
}

func TestARunningRecordWhoseProcessDiedIsMarkedFailed(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	if err := Save(checkout, Record{
		Launch: launchID, Machine: "m1f", Outcome: OutcomeRunning,
		Process: Process{PID: 4242, StartedAt: 1764000000},
		Steps: []Step{
			{Step: StepClone, Outcome: StepDone},
			{Step: StepEngine, Outcome: StepDone},
		},
	}); err != nil {
		t.Fatal(err)
	}
	records, err := Reconcile(checkout, func(Process) bool { return false }, recordClock)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(records) != 1 || records[0].Outcome != OutcomeFailed {
		t.Fatalf("records = %+v, want one failed launch", records)
	}
	step, found := records[0].StepOf(StepEngine)
	if !found || step.Outcome != StepFailed || step.Words != "the launch process died after engine" {
		t.Fatalf("step = %+v, want the death recorded against the step it was in", step)
	}
	if records[0].EndedAt == nil || *records[0].EndedAt != "2026-09-25T12:00:00Z" {
		t.Fatalf("endedAt = %v", records[0].EndedAt)
	}
	// The rewrite reaches disk, so the next reader and the page agree.
	again, err := Load(checkout, launchID)
	if err != nil || again.Outcome != OutcomeFailed {
		t.Fatalf("on disk = %+v, %v", again, err)
	}
}

func TestALiveLaunchAndAFinishedOneAreLeftAlone(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	if err := Save(checkout, Record{Launch: launchID, Outcome: OutcomeRunning, Process: Process{PID: 7, StartedAt: 3}}); err != nil {
		t.Fatal(err)
	}
	if err := Save(checkout, Record{Launch: secondLaunch, Outcome: OutcomeArmed}); err != nil {
		t.Fatal(err)
	}
	records, err := Reconcile(checkout, func(process Process) bool { return process.PID == 7 }, recordClock)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if records[0].Outcome != OutcomeRunning {
		t.Fatalf("a live launch was marked %q", records[0].Outcome)
	}
	if records[1].Outcome != OutcomeArmed {
		t.Fatalf("a finished launch was marked %q", records[1].Outcome)
	}
}

func TestARecordIsNamedByALaunchIdAndNothingElse(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	for _, id := range []string{"../escape", "short", "", strings.Repeat("a", 26)} {
		if _, err := Path(checkout, id); err == nil {
			t.Fatalf("%q was accepted as a launch id", id)
		}
	}
	path, err := Path(checkout, launchID)
	if err != nil || path != filepath.Join(Dir(checkout), launchID+".json") {
		t.Fatalf("path = %q, %v", path, err)
	}
}
