package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPeriodLawDefaultsAndExplicitWindows(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 34, 56, 0, time.UTC)
	period, err := resolvePeriod(Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := period.Start.Format(time.RFC3339), "2026-08-17T00:00:00Z"; got != want {
		t.Fatalf("automatic start = %s, want %s", got, want)
	}
	if got, want := period.End.Format(time.RFC3339), "2026-08-24T00:00:00Z"; got != want || !period.CalendarWeek || period.Week != "2026-W34" {
		t.Fatalf("automatic completed week = %+v", period)
	}

	custom, err := resolvePeriod(Options{Since: "2026-08-20T01:02:03Z", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if !custom.Start.Equal(time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)) || !custom.End.Equal(now) || custom.CalendarWeek {
		t.Fatalf("explicit open-ended override = %+v", custom)
	}
	if _, err := resolvePeriod(Options{GoalID: "goal-a", Since: "2026-08-01T00:00:00Z", Now: func() time.Time { return now }}); err == nil {
		t.Fatal("goal report accepted a week slice")
	}
	if _, err := resolvePeriod(Options{Since: "2026-08-24T00:00:00Z", PeriodEnd: "2026-08-24T00:00:00Z", Now: func() time.Time { return now }}); err == nil {
		t.Fatal("empty half-open window was accepted")
	}
}

func TestClosedReportFileInterface(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()

	custom, err := Report(Options{
		Root: f.root, Since: "2026-08-18T00:00:00Z", PeriodEnd: "2026-08-24T00:00:00Z",
		Now: func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCustom := filepath.Join(f.root, "artifacts", "agents", "metrics", "period-20260818T000000Z-20260824T000000Z.md")
	if custom.Target != wantCustom {
		t.Fatalf("custom target = %s, want %s", custom.Target, wantCustom)
	}
	if _, err := os.Stat(filepath.Join(f.root, "plans", "metrics", "machine-a", "2026-W34.md")); !os.IsNotExist(err) {
		t.Fatalf("custom window wrote a tracked report: %v", err)
	}

	weekly, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatal(err)
	}
	wantWeekly := filepath.Join(f.root, "plans", "metrics", "machine-a", "2026-W34.md")
	if weekly.Target != wantWeekly {
		t.Fatalf("weekly target = %s, want %s", weekly.Target, wantWeekly)
	}
	goalResult, err := Report(Options{Root: f.root, GoalID: "g1", PeriodEnd: "2026-08-24T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(f.root, "artifacts", "agents", "metrics", "goal-g1.md"); goalResult.Target != want {
		t.Fatalf("goal target = %s, want %s", goalResult.Target, want)
	}
}
