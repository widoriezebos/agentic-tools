package launch

// Discarding a stopped launch: the record is kept and marked, never removed,
// and the clone it made is never touched.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscardMarksAStoppedLaunchAndKeepsItsRecord(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	clone := filepath.Join(t.TempDir(), "agentic-tools-m1f")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(checkout, Record{Launch: launchID, Machine: "m1f", Destination: clone, Outcome: OutcomeFailed}); err != nil {
		t.Fatal(err)
	}
	discarded, err := Discard(checkout, launchID, recordClock)
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if discarded.DiscardedAt == nil || *discarded.DiscardedAt != "2026-09-25T12:00:00Z" {
		t.Fatalf("answered discardedAt = %v", discarded.DiscardedAt)
	}
	read, err := Load(checkout, launchID)
	if err != nil {
		t.Fatalf("the record is gone: %v", err)
	}
	if read.DiscardedAt == nil || *read.DiscardedAt != "2026-09-25T12:00:00Z" || read.Outcome != OutcomeFailed {
		t.Fatalf("record on disk = %+v", read)
	}
	if _, err := os.Stat(clone); err != nil {
		t.Fatalf("discard touched the clone: %v", err)
	}
}

func TestDiscardingADiscardedLaunchChangesNothingAndSucceeds(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	if err := Save(checkout, Record{Launch: launchID, Machine: "m1f", Outcome: OutcomeArmed}); err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(checkout, launchID, recordClock); err != nil {
		t.Fatal(err)
	}
	path, _ := Path(checkout, launchID)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Discard(checkout, launchID, recordClock.Add(time.Hour))
	if err != nil {
		t.Fatalf("second discard: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || again.DiscardedAt == nil || *again.DiscardedAt != "2026-09-25T12:00:00Z" {
		t.Fatalf("a second discard changed the record:\n%s\n%s", before, after)
	}
}

func TestDiscardRefusesALaunchStillRunningAndOneThatIsNotThere(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	for _, outcome := range []string{OutcomeStarting, OutcomeRunning} {
		if err := Save(checkout, Record{Launch: launchID, Machine: "m1f", Outcome: outcome}); err != nil {
			t.Fatal(err)
		}
		_, err := Discard(checkout, launchID, recordClock)
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != CodeDiscardRunning {
			t.Fatalf("%s: err = %v, want %s", outcome, err, CodeDiscardRunning)
		}
		read, _ := Load(checkout, launchID)
		if read.DiscardedAt != nil {
			t.Fatalf("%s: a refused discard wrote the record", outcome)
		}
	}
	_, err := Discard(checkout, secondLaunch, recordClock)
	var absent *Refusal
	if !errors.As(err, &absent) || absent.Code != CodeUnknown {
		t.Fatalf("absent: err = %v, want %s", err, CodeUnknown)
	}
	_, err = Discard(checkout, "../escape", recordClock)
	var invalid *Refusal
	if !errors.As(err, &invalid) || invalid.Code != CodeIDInvalid {
		t.Fatalf("invalid: err = %v, want %s", err, CodeIDInvalid)
	}
}

func TestPresentSaysWhetherTheDestinationIsStillOnDisk(t *testing.T) {
	t.Parallel()
	there := t.TempDir()
	if !Present(there) || Present(filepath.Join(there, "gone")) || Present("") {
		t.Fatal("Present misreads the disk")
	}
}
