package steward

import (
	"os"
	"strings"
	"testing"
	"time"
)

// One tick, one sentence a person can read without the repository.
func TestNarrationLineSpeaksPlainly(t *testing.T) {
	root := t.TempDir()
	line := narrationLine(root, TickResult{
		OpenWork: "claimed goal: narrator",
		Decision: Decision{Action: ActNone},
	}, time.Date(2026, 8, 23, 21, 0, 0, 0, time.UTC))
	if !strings.Contains(line, "working on narrator") || strings.Contains(line, "claimed goal:") {
		t.Fatalf("the sentence must translate the record, not quote it: %q", line)
	}
}

// The account is bounded: old ticks scroll away at the cap.
func TestNarrationCapsItsHistory(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < narrationCapLines+25; i++ {
		Narrate(root, TickResult{OpenWork: "observing", Decision: Decision{Action: ActNone}})
	}
	data, err := os.ReadFile(NarrationPath(root))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != narrationCapLines {
		t.Fatalf("the cap must hold: %d lines", len(lines))
	}
}

// The storyteller never fails the shift: an unwritable home is silence,
// not an error.
func TestNarrationFailureIsSilent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/artifacts", []byte("a file where a directory must be"), 0o644); err != nil {
		t.Fatal(err)
	}
	Narrate(root, TickResult{OpenWork: "observing", Decision: Decision{Action: ActNone}})
}
