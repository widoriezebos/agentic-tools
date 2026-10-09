package main

import (
	"testing"
	"time"
)

// fixtureDeadlineRemaining uses the binary or parent deadline, if either exists.
// Zero leaves an event-driven fixture without a wall-clock cap.
func fixtureDeadlineRemaining(t *testing.T, parents ...time.Time) time.Duration {
	t.Helper()
	deadline, ok := t.Deadline()
	for _, parent := range parents {
		if !parent.IsZero() && (!ok || parent.Before(deadline)) {
			deadline, ok = parent, true
		}
	}
	if !ok {
		return 0
	}
	now := time.Now()
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		t.Fatal("the test binary's deadline has expired")
	}
	return remaining
}

func TestFixtureDeadlineRemainingAllowsNoBinaryDeadline(t *testing.T) {
	t.Parallel()
	remaining := fixtureDeadlineRemaining(t)
	if _, ok := t.Deadline(); !ok {
		if remaining != 0 {
			t.Fatalf("a fixture without a binary deadline must have no cap: %s", remaining)
		}
	} else if remaining <= 0 {
		t.Fatalf("a binary deadline must remain a positive backstop: %s", remaining)
	}
}

func TestFixtureDeadlineRemainingHonorsParentDeadline(t *testing.T) {
	t.Parallel()
	now := time.Now()
	parent := now.Add(time.Hour)
	remaining := fixtureDeadlineRemaining(t, parent)
	if remaining <= 0 || remaining > time.Hour {
		t.Fatalf("the parent deadline must bound the fixture: %s", remaining)
	}
	if binary, ok := t.Deadline(); ok && binary.Before(parent) && remaining > binary.Sub(now) {
		t.Fatalf("the earlier binary deadline must still bound the fixture: %s", remaining)
	}
}
