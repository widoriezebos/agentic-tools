package goal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLaneBudgetRecordsKeepRepeatedAndCompletedStays(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	dir := filepath.Join(install, "artifacts", "agents", "landing")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	queue := `{"goal":"bounded","sha":"a","at":"2026-10-11T07:00:00Z"}
{"goal":"bounded","sha":"a","at":"2026-10-11T08:00:00Z","delivered":"repeat"}
{"goal":"bounded","sha":"a","at":"2026-10-11T10:00:00Z","outcome":"returned"}
{"goal":"bounded","sha":"a","at":"2026-10-11T11:00:00Z"}
{"goal":"bounded","sha":"b","at":"2026-10-11T12:00:00Z"}
`
	if err := os.WriteFile(filepath.Join(dir, "queue.jsonl"), []byte(queue), 0644); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	read := func(now time.Time, minutes float64, paused bool) {
		t.Helper()
		record, err := ReadLaneBudget(install, "bounded", start, now)
		if err != nil || record.PausedMinutes != minutes || record.PausedAt.IsZero() == paused {
			t.Fatalf("clock at %s: %+v %v", now, record, err)
		}
		if paused && now.Equal(start.Add(5*time.Hour)) && !record.PausedAt.Equal(start.Add(3*time.Hour)) {
			t.Fatalf("replacement commit restarted the continuous stay: %+v", record)
		}
	}
	read(start.Add(time.Hour), 60, true)
	read(start.Add(2*time.Hour), 120, false)
	read(start.Add(5*time.Hour), 240, true)
	if err := os.WriteFile(filepath.Join(dir, "pushes.jsonl"), []byte(`{"at":"2026-10-11T13:00:00Z","batch-members":[{"goal":"bounded","sha":"b"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	read(start.Add(6*time.Hour), 240, false)
}

func TestLaneBudgetWaitingOutcomeKeepsStayOpen(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	dir := filepath.Join(install, "artifacts", "agents", "landing")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	queue := `{"goal":"bounded","sha":"a","at":"2026-10-11T08:00:00Z"}
{"goal":"bounded","sha":"a","at":"2026-10-11T09:00:00Z","outcome":"waiting","held":true}
`
	if err := os.WriteFile(filepath.Join(dir, "queue.jsonl"), []byte(queue), 0644); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	record, err := ReadLaneBudget(install, "bounded", start, start.Add(4*time.Hour))
	if err != nil || record.PausedMinutes != 240 || !record.PausedAt.Equal(start) {
		t.Fatalf("waiting ended stay: %+v %v", record, err)
	}
}

func TestLaneBudgetTornLinesDoNotHideLaterRecords(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	dir := filepath.Join(install, "artifacts", "agents", "landing")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	queue := `{"goal":"bounded","sha":"a","at":"2026-10-11T08:00:00Z"}
{interrupted
{"goal":"bounded","sha":"a","at":"2026-10-11T10:00:00Z","outcome":"returned"}
{torn`
	if err := os.WriteFile(filepath.Join(dir, "queue.jsonl"), []byte(queue), 0644); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	for _, id := range []string{"bounded", "unrelated"} {
		record, err := ReadLaneBudget(install, id, start, start.Add(4*time.Hour))
		want := float64(0)
		if id == "bounded" {
			want = 120
		}
		if err != nil || record.PausedMinutes != want || !record.PausedAt.IsZero() {
			t.Fatalf("torn record poisoned %s: %+v %v", id, record, err)
		}
	}
}

func TestLaneBudgetOwnMalformedStayFailsOnlyItsGoal(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	dir := filepath.Join(install, "artifacts", "agents", "landing")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "queue.jsonl"), []byte(`{"goal":"bounded","sha":"a","at":"bad"}`), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	if _, err := ReadLaneBudget(install, "bounded", now.Add(-4*time.Hour), now); err == nil {
		t.Fatal("own unreadable clock was accepted")
	}
	if _, err := ReadLaneBudget(install, "unrelated", now.Add(-4*time.Hour), now); err != nil {
		t.Fatalf("another goal inherited error: %v", err)
	}
}
