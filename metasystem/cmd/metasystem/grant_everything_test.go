package main

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func amsterdam(t *testing.T) *time.Location {
	t.Helper()
	zone, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Skipf("no tz database: %v", err)
	}
	return zone
}

// W1: the easy forms of a general grant's end, elapsed for --for, local for
// --until, one week at most, and a local deadline that does not exist or
// happens twice refused rather than shifted.
func TestGrantEndParsesTheEasyForms(t *testing.T) {
	t.Parallel()
	zone := amsterdam(t)
	now := time.Date(2026, 9, 30, 8, 51, 27, 0, zone)
	local := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, zone) }
	for _, c := range []struct {
		forValue, until string
		want            time.Time
	}{
		{"24h", "", local(2026, 10, 1, 8, 51)},
		{"8h", "", local(2026, 9, 30, 16, 51)},
		{"7d", "", local(2026, 10, 7, 8, 51)},
		{"1w", "", local(2026, 10, 7, 8, 51)},
		{"", "18:00", local(2026, 9, 30, 18, 0)},
		{"", "08:00", local(2026, 10, 1, 8, 0)},
		{"", "tomorrow", local(2026, 10, 2, 0, 0)},
		{"", "2026-10-06", local(2026, 10, 7, 0, 0)},
	} {
		got, err := parseGrantEnd(now, zone, c.forValue, c.until)
		if err != nil || !got.Equal(c.want) {
			t.Errorf("--for %q --until %q = %v %v, want %v", c.forValue, c.until, got, err, c.want)
		}
	}
	// Elapsed, not wall: a week across the autumn change is 168 hours.
	autumn := time.Date(2026, 10, 22, 12, 0, 0, 0, zone)
	if got, err := parseGrantEnd(autumn, zone, "1w", ""); err != nil || got.Sub(autumn) != 168*time.Hour {
		t.Errorf("1w across the change = %v %v", got, err)
	}
	for _, c := range []struct{ forValue, until, needle string }{
		{"", "", "--for 24h"},
		{"24h", "18:00", "one of"},
		{"8d", "", "latest"},
		{"169h", "", "latest"},
		{"", "2026-10-07", "latest"},
		{"0h", "", "--for 24h"},
		{"soon", "", "--for 24h"},
		{"", "25:00", "--until 18:00"},
		{"", "yesterday", "--until 18:00"},
		{"", "2026-09-29", "after now"},
	} {
		if _, err := parseGrantEnd(now, zone, c.forValue, c.until); err == nil || !strings.Contains(err.Error(), c.needle) {
			t.Errorf("--for %q --until %q = %v, want a refusal naming %q", c.forValue, c.until, err, c.needle)
		}
	}
	spring := time.Date(2027, 3, 27, 12, 0, 0, 0, zone)
	if _, err := parseGrantEnd(spring, zone, "", "02:30"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("02:30 on the spring change = %v, want does not exist", err)
	}
	fall := time.Date(2026, 10, 24, 12, 0, 0, 0, zone)
	if _, err := parseGrantEnd(fall, zone, "", "02:30"); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("02:30 on the autumn change = %v, want happens twice", err)
	}
}

// The examples a refusal prints are valid on any day: no fixed date.
func TestGrantEndExamplesHoldOnAnyDay(t *testing.T) {
	t.Parallel()
	if regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}`).MatchString(grantEndExamples) {
		t.Fatalf("the examples name a fixed date: %q", grantEndExamples)
	}
	for _, want := range []string{"--for 24h", "--until 18:00", "--until tomorrow", "--until YYYY-MM-DD"} {
		if !strings.Contains(grantEndExamples, want) {
			t.Errorf("the examples lack %q: %q", want, grantEndExamples)
		}
	}
}
