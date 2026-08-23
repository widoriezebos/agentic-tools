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
	}, TickConfig{}, time.Date(2026, 8, 23, 21, 0, 0, 0, time.UTC))
	if !strings.Contains(line, "working on narrator") || strings.Contains(line, "claimed goal:") {
		t.Fatalf("the sentence must translate the record, not quote it: %q", line)
	}
}

// The account is bounded: old ticks scroll away at the cap.
func TestNarrationCapsItsHistory(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < narrationCapLines+25; i++ {
		Narrate(root, TickResult{OpenWork: "observing", Decision: Decision{Action: ActNone}}, TickConfig{})
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
	Narrate(root, TickResult{OpenWork: "observing", Decision: Decision{Action: ActNone}}, TickConfig{})
}

// The account notices a stall building before the steward acts, and
// says nothing when work is fresh or the decision already speaks.
func TestNoticingsNameTheApproachingStall(t *testing.T) {
	mid := TickResult{
		OpenWork: "claimed goal: narrator",
		Decision: Decision{Action: ActNone},
		Evidence: Evidence{TicksSinceAdvance: 3},
	}
	notes := noticings(mid, TickConfig{StaleTicks: 5, MaxRevivals: 3})
	if len(notes) != 1 || !strings.Contains(notes[0].Line, "no visible progress for 3 checks") {
		t.Fatalf("an approaching stall must be noticed: %v", notes)
	}
	fresh := mid
	fresh.Evidence.TicksSinceAdvance = 1
	if n := noticings(fresh, TickConfig{StaleTicks: 5, MaxRevivals: 3}); len(n) != 0 {
		t.Fatalf("fresh work is not an anomaly: %v", n)
	}
	acting := mid
	acting.Decision = Decision{Action: ActNotify, Reason: "stalled"}
	if n := noticings(acting, TickConfig{StaleTicks: 5, MaxRevivals: 3}); len(n) != 0 {
		t.Fatalf("when the decision speaks, the noticing stays quiet: %v", n)
	}
}

// A building stall reaches the operator exactly once while it builds:
// the noticing key holds one pending slot, however many ticks notice.
func TestNoticingsReachTheHumanOncePerCondition(t *testing.T) {
	root := t.TempDir()
	items := []Noticing{{Key: "stall-approaching", Line: "noticing: test stall"}}
	ReachTheHuman(root, items)
	ReachTheHuman(root, items)
	pending, err := PendingNotifications(root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, p := range pending {
		if strings.Contains(p.Message, "test stall") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("one building condition holds one pending slot, got %d", count)
	}
}
