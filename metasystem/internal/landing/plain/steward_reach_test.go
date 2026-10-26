package plain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func returnedThenLanded(t *testing.T) (*statusBed, Status, time.Time) {
	t.Helper()
	b := newStatusBed(t)
	now := time.Date(2026, 10, 25, 14, 31, 0, 0, time.UTC)
	b.lines("queue.jsonl",
		Line{Goal: "g", SHA: "old", At: "2026-10-25T00:23:00Z"},
		Line{Goal: "g", SHA: "old", Outcome: StateReturned, At: "2026-10-25T09:29:00Z", Reason: "returned"},
		Line{Goal: "g", SHA: "new", At: "2026-10-25T10:00:00Z"},
		Line{Goal: "waiting", SHA: "wait", At: now.Add(-75 * time.Minute).Format(time.RFC3339)})
	b.lines("pushes.jsonl", Pushed{Old: "base", Commit: "main", At: "2026-10-25T12:25:00Z"})
	return b, readStatus(b.home, b.record, b.view, healthSeams(now), b.git([]string{"new"}, map[string][]string{"base..main": {"new"}})), now
}

func TestLandingHealthReturnedThenLandedIsNotOverdue(t *testing.T) {
	t.Parallel()
	_, status, _ := returnedThenLanded(t)
	count := 0
	for _, g := range status.Health.Goals {
		if g.Goal == "g" {
			count++
			if g.Overdue {
				t.Fatalf("landed goal overdue: %+v", g)
			}
		}
	}
	if count != 1 {
		t.Fatalf("goal listed %d times: %+v", count, status.Health.Goals)
	}
	for _, row := range status.Health.Rows() {
		if row.Row == "overdue-goal" && row.Subject == "g" {
			t.Fatalf("stale return fired: %+v", row)
		}
	}
}

func TestLandingStewardReportSummaryAgreesWithSpeed(t *testing.T) {
	t.Parallel()
	b, status, now := returnedThenLanded(t)
	summary, err := StewardReport(b.install, b.checkout, status, nil, now, healthSeams(now), launch.Store{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "steward-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"oldest goal 75 min", "1 overdue goals", "0 overdue records"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q: %s", want, summary)
		}
	}
	if strings.Count(string(data), "Row overdue-goal:") != 1 || !strings.Contains(string(data), "waiting: 75 min") {
		t.Fatal(string(data))
	}
}

func TestLandingStewardReportNamesAFixGoalOnce(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	b.status.Health = &Health{Flakes: []goal.FlakeFact{{TrunkRedEntry: goal.TrunkRedEntry{FixGoal: "fix"}}, {TrunkRedEntry: goal.TrunkRedEntry{FixGoal: "fix"}}}}
	b.report(b.now)
	if n := strings.Count(b.text(), "Flake fix goal fix"); n != 1 {
		t.Fatalf("fix named %d times: %s", n, b.text())
	}
}

func TestLandingStewardReportCriticWindow(t *testing.T) {
	t.Parallel()
	for _, previous := range []string{"", "2026-10-24T19:00:00Z"} {
		t.Run(previous, func(t *testing.T) {
			t.Parallel()
			b := newReportBed(t)
			b.now = time.Date(2026, 10, 24, 22, 0, 0, 0, time.UTC)
			b.status.Queue = []Entry{{Goal: "g", State: StateLanded, At: "2026-10-24T21:27:49Z", LandedAt: "2026-10-24T21:45:00Z"}}
			if previous != "" {
				b.status.Queue = append(b.status.Queue, Entry{Goal: "g", State: StateLanded, LandedAt: previous})
			}
			for i, at := range []string{"2026-10-24T20:03:57Z", "2026-10-24T18:00:00Z"} {
				r := launch.Record{ID: fmt.Sprintf("read-%d", i), Kind: "read", Goal: "g", StartedAt: at, FinishedAt: "2026-10-24T20:14:40Z", State: launch.Completed, Measurement: launch.Measurement{UsageRead: true, InputTokens: int64(10 + i)}}
				if err := b.store.Create(r); err != nil {
					t.Fatal(err)
				}
			}
			b.report(b.now)
			want := "g critics: 10 tokens"
			if previous == "" {
				want = "g critics: 21 tokens"
			}
			if !strings.Contains(b.text(), want) {
				t.Fatalf("missing %q: %s", want, b.text())
			}
		})
	}
	b := newReportBed(t)
	b.status.Queue = []Entry{{Goal: "missing", State: StateLanded, At: b.now.Add(-20 * time.Minute).Format(time.RFC3339), LandedAt: b.now.Format(time.RFC3339)}}
	b.report(b.now)
	for _, want := range []string{"missing critics: unmeasured", b.store.Root, "window", "first landing", b.now.Local().Format("2006-01-02 15:04 MST")} {
		if !strings.Contains(b.text(), want) {
			t.Fatalf("missing %q: %s", want, b.text())
		}
	}
	if strings.Contains(b.text(), "*/record.json") {
		t.Fatal(b.text())
	}
	b = newReportBed(t)
	b.status.Queue = nil
	b.report(b.now)
	if !strings.Contains(b.text(), "no landing this hour") {
		t.Fatal(b.text())
	}
}

func TestLandingStewardReportDriftSincePreviousReport(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	b.now = time.Date(2026, 10, 25, 10, 31, 0, 0, time.UTC)
	b.report(b.now)
	for _, minutes := range []int{-41, 19} {
		e := state.Event{Kind: "drift", By: "keeper", At: b.now.Add(time.Duration(minutes) * time.Minute), Evidence: fmt.Sprintf("sample: before → %d", minutes)}
		b.linesAppend("events.jsonl", e)
	}
	b.report(b.now.Add(time.Hour))
	section, _, _ := strings.Cut(b.text(), "## Hour to "+b.now.Local().Format("2006-01-02 15:04 MST"))
	if strings.Contains(section, "before → -41") || !strings.Contains(section, "before → 19") {
		t.Fatal(section)
	}
}
