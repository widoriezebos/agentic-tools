package fixtureauth

import (
	"strings"
	"testing"
	"time"
)

// GoalClock is the one clock choice the stop-proof cancellation and the
// census freshness check make (moved off the retired `job stop-proof-cancel`
// and `job census-fresh` command tests): a production root keeps the wall
// clock even when a frozen instant is in the environment; a fixture root
// receives the frozen instant, stable across calls, and says so; a malformed
// instant is refused by name.
func TestGoalClockFreezesOnlyAFixtureRoot(t *testing.T) {
	t.Setenv(fixtureEnv, "")
	frozen := time.Date(2026, 9, 19, 8, 0, 58, 0, time.UTC)
	t.Setenv(goalNowEnv, frozen.Format(time.RFC3339))
	wall := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	wallClock := func() time.Time { return wall }

	clock, fixture, err := GoalClock(fakeCheckout(t, "claude"), wallClock)
	if err != nil || fixture || !clock().Equal(wall) {
		t.Fatalf("production root: fixture=%v err=%v; want the wall clock", fixture, err)
	}

	clock, fixture, err = GoalClock(fakeCheckout(t, "fake"), wallClock)
	if err != nil || !fixture || !clock().Equal(frozen) || !clock().Equal(frozen) {
		t.Fatalf("fixture root: fixture=%v err=%v; want the frozen instant", fixture, err)
	}
	// GoalNow is one reading of the same clock, the wall clock in UTC.
	if now, err := GoalNow(fakeCheckout(t, "fake")); err != nil || !now.Equal(frozen) {
		t.Fatalf("fixture root: GoalNow=%v err=%v; want the frozen instant", now, err)
	}
	before := time.Now().UTC()
	if now, err := GoalNow(fakeCheckout(t, "claude")); err != nil || now.Location() != time.UTC || now.Before(before.Add(-time.Second)) {
		t.Fatalf("production root: GoalNow=%v err=%v; want the wall clock in UTC", now, err)
	}

	t.Setenv(goalNowEnv, "not-a-time")
	if _, _, err := GoalClock(fakeCheckout(t, "fake"), wallClock); err == nil || !strings.Contains(err.Error(), "METASYSTEM_GOAL_NOW must be an RFC3339 timestamp") {
		t.Fatalf("malformed fixture instant = %v", err)
	}
	if _, err := GoalNow(fakeCheckout(t, "fake")); err == nil || !strings.Contains(err.Error(), "METASYSTEM_GOAL_NOW must be an RFC3339 timestamp") {
		t.Fatalf("malformed fixture instant through GoalNow = %v", err)
	}
	if _, fixture, err := GoalClock(fakeCheckout(t, "claude"), wallClock); err != nil || fixture {
		t.Fatalf("production root read a malformed fixture instant: fixture=%v err=%v", fixture, err)
	}
}
