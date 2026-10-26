package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLandingStewardReportUnreadableFlakeGoalKeepsHour(t *testing.T) {
	t.Parallel()
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprint(malformed), func(t *testing.T) {
			t.Parallel()
			b := newReportBed(t)
			b.status.Health = &Health{Flakes: []goal.FlakeFact{{TrunkRedEntry: goal.TrunkRedEntry{FixGoal: "missing-fix"}}}}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "show" && strings.Contains(args[1], "missing-fix.md") {
					if malformed {
						return "invalid goal", nil
					}
					return "", errors.New("goal file missing at accepted ref")
				}
				return git(dir, args...)
			}
			b.report(b.now)
			for _, want := range []string{"missing-fix cannot be read", "### Quality", "### Speed", "### Tokens"} {
				if !strings.Contains(b.text(), want) {
					t.Fatalf("report missing %q: %s", want, b.text())
				}
			}
		})
	}
}

type reportBed struct {
	*statusBed
	status   Status
	acts     []Intervention
	store    launch.Store
	seams    ProveSeams
	now      time.Time
	launches []launch.Record
}

func newReportBed(t *testing.T) *reportBed {
	t.Helper()
	data, err := os.ReadFile("testdata/steward-report-hour.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Queue     []Entry
		Results   []Result
		Pushes    []Pushed
		Incidents []goal.TrunkRedEntry
		Acts      []Intervention
		Launches  []launch.Record
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	b := &reportBed{statusBed: newStatusBed(t), status: Status{Queue: fixture.Queue}, acts: fixture.Acts, store: launch.Store{Root: t.TempDir()}, seams: healthSeams(now), now: now, launches: fixture.Launches}
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return fixture.Incidents, nil }
	for _, r := range fixture.Results {
		b.linesAppend("results.jsonl", r)
	}
	for _, p := range fixture.Pushes {
		b.linesAppend("pushes.jsonl", p)
	}
	return b
}

func (b *reportBed) linesAppend(name string, value any) {
	b.t.Helper()
	if err := withLock(b.install, func() error { return appendLine(filepath.Join(Dir(b.install), name), value) }); err != nil {
		b.t.Fatal(err)
	}
}

func (b *reportBed) report(now time.Time) string {
	b.t.Helper()
	summary, err := StewardReport(b.install, b.checkout, b.status, b.acts, now, b.seams, b.store)
	if err != nil {
		b.t.Fatal(err)
	}
	return summary
}

func (b *reportBed) text() string {
	b.t.Helper()
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "steward-report.md"))
	if err != nil {
		b.t.Fatal(err)
	}
	return string(data)
}

func TestLandingStewardReportOncePerHour(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	first := b.report(b.now)
	text := b.text()
	b.unreadable("gates.jsonl")
	if got := b.report(b.now.Add(time.Hour - time.Nanosecond)); got != first || b.text() != text {
		t.Fatal("a tick before one hour read evidence or rewrote the report")
	}
	if err := os.Remove(filepath.Join(Dir(b.install), "gates.jsonl")); err != nil {
		t.Fatal(err)
	}
	second := b.report(b.now.Add(time.Hour))
	if second == first || !strings.HasSuffix(b.text(), text) || strings.Count(b.text(), "## Hour to ") != 2 {
		t.Fatal(b.text())
	}
	if !strings.Contains(second, "0 landed, 0 returned, 1 red") {
		t.Fatal(second)
	}
	if b.report(b.now.Add(time.Hour)) != second || strings.Count(b.text(), "## Hour to ") != 2 {
		t.Fatal("reported twice at the same clock")
	}
}

func TestLandingStewardReportQualitySpeedAndSummary(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	duration := 75.
	b.status.Health = &Health{Goals: []GoalHealth{{Goal: "waiting", Minutes: &duration, Overdue: true}}}
	summary := b.report(b.now)
	for _, want := range []string{"Red main-red: cause main", "TestMainFailure", "main-incident opened", "closed-incident closed", "quick: landed, 45 min; against 60 min", "slow: landed, 95 min; over 60 min", "returned: returned, 70 min", "Push push-quick", "Push push-slow", "Row overdue-goal", "Row unproven-incidents", "outcome proving main"} {
		if !strings.Contains(b.text(), want) {
			t.Fatalf("missing %q: %s", want, b.text())
		}
	}
	if strings.Contains(b.text(), "before-hour") || strings.Contains(b.text(), "future") || strings.Count(b.text(), "Red main-red:") != 1 {
		t.Fatal(b.text())
	}
	want := "hour to " + b.now.Local().Format("15:04") + ": 2 landed, 1 returned, 4 red (goal 1, main 1, flake 1, unknown 1), oldest goal 75 min, 1 acts, 1 overdue goals, 0 overdue records"
	if summary != want {
		t.Fatalf("got %s; want %s", summary, want)
	}
	status := readStatus(b.home, b.record, b.view, b.seams, b.git(nil, nil))
	if status.Health == nil || status.Health.Report != summary || !strings.Contains(status.Health.Line(), summary) || !strings.Contains(status.Health.Text(), "report: "+filepath.Join(Dir(b.install), "steward-report.md")) {
		t.Fatalf("health lost report: %+v", status)
	}
}

func TestLandingStewardReportTokensFromSessionRecords(t *testing.T) {
	t.Parallel()
	for _, measured := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "measured"}[measured], func(t *testing.T) {
			t.Parallel()
			b := newReportBed(t)
			if measured {
				for _, r := range b.launches {
					if r.WorkingDirectory == "lane" {
						r.WorkingDirectory = b.checkout
					}
					if string(r.AdapterData["fenceRoot"]) == `"lane-install"` {
						r.AdapterData["fenceRoot"], _ = json.Marshal(b.install)
					}
					if err := b.store.Create(r); err != nil {
						t.Fatal(err)
					}
				}
			}
			b.report(b.now)
			text := b.text()
			if measured {
				for _, want := range []string{"quick landing: 60 tokens", "slow landing: 60 tokens", "slow critics: 1019 tokens", "shared across goals"} {
					if !strings.Contains(text, want) {
						t.Fatalf("missing %q: %s", want, text)
					}
				}
				if strings.Count(text, "- slow critics:") != 1 || !strings.Contains(text, "session critic-session") || !strings.Contains(text, "session critic-second") {
					t.Fatal(text)
				}
				if strings.Contains(text, "session foreign-session") || strings.Contains(text, "session foreign-critic") || strings.Contains(text, "session critic-foreign-install") {
					t.Fatal("measured another checkout's session")
				}
			} else if !strings.Contains(text, "slow landing: unmeasured: no session record in "+b.store.Root) {
				t.Fatal(text)
			}
		})
	}
}

func TestLandingStewardReportUnmeasuredCountersAndFailures(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	r := b.launches[0]
	r.WorkingDirectory, r.Measurement.UsageRead = b.checkout, false
	if err := b.store.Create(r); err != nil {
		t.Fatal(err)
	}
	b.report(b.now)
	if !strings.Contains(b.text(), "landing-session/record.json measurement.usageRead") || strings.Contains(b.text(), "60 tokens") {
		t.Fatal(b.text())
	}
	before := b.text()
	if err := os.WriteFile(filepath.Join(Dir(b.install), "steward-report.json"), []byte(`{"at":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := StewardReport(b.install, b.checkout, b.status, b.acts, b.now.Add(time.Hour), b.seams, b.store); err == nil || b.text() != before {
		t.Fatal("damaged report state was silently overwritten")
	}
	if status := readStatus(b.home, b.record, b.view, b.seams, b.git(nil, nil)); status.Health != nil {
		t.Fatal("health hid damaged report state")
	}
}

func TestLandingStewardReportWriteFailureDoesNotSpendHour(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	b.unreadable("steward-report.md")
	if _, err := StewardReport(b.install, b.checkout, b.status, b.acts, b.now, b.seams, b.store); err == nil {
		t.Fatal("report write failure succeeded")
	}
	if _, err := os.Stat(filepath.Join(Dir(b.install), "steward-report.json")); !os.IsNotExist(err) {
		t.Fatalf("failed report spent the hour: %v", err)
	}
	if err := os.Remove(filepath.Join(Dir(b.install), "steward-report.md")); err != nil {
		t.Fatal(err)
	}
	b.report(b.now)
}

func TestLandingStewardReportConcurrentTicks(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			_, err := StewardReport(b.install, b.checkout, b.status, b.acts, b.now, b.seams, b.store)
			errors <- err
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Count(b.text(), "## Hour to ") != 1 {
		t.Fatal(b.text())
	}
}

func TestLandingStewardStepPublishesHourlyReport(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.staleQuestion()
	run := b.step(bedNow)
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "steward-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Row stale-question") || !strings.Contains(string(data), "outcome proving main") || !strings.Contains(run.Line, "hour to ") || !strings.Contains(run.Line, "1 acts") {
		t.Fatalf("run=%+v report=%s", run, data)
	}
}

func TestLandingStewardReportDelayedTickKeepsEvidence(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	b.report(b.now)
	b.linesAppend("results.jsonl", Result{Attempt: "gap-main-red", At: b.now.Add(45 * time.Minute).Format(time.RFC3339Nano), Result: Red, Cause: &Cause{Kind: "main"}})
	b.report(b.now.Add(2 * time.Hour))
	section, _, _ := strings.Cut(b.text(), "## Hour to "+b.now.Local().Format("2006-01-02 15:04 MST"))
	if !strings.Contains(section, "Red gap-main-red: cause main") || strings.Contains(section, "Red main-red:") {
		t.Fatal(section)
	}
}

func TestLandingStewardReportCriticsBelongToThisLanding(t *testing.T) {
	t.Parallel()
	b := newReportBed(t)
	b.status.Queue = append(b.status.Queue, Entry{Goal: "slow", State: StateLanded, At: "2026-10-12T07:00:00Z", LandedAt: "2026-10-12T10:50:00+02:00"})
	b.launches = append(b.launches, launch.Record{ID: "critic-at-previous-push", Kind: "read", Goal: "slow", WorkingDirectory: "/tmp/read-context", State: launch.Completed, StartedAt: "2026-10-12T08:50:00Z", Measurement: launch.Measurement{UsageRead: true, InputTokens: 999}})
	for _, r := range b.launches {
		if r.Kind != "read" {
			continue
		}
		if string(r.AdapterData["fenceRoot"]) == `"lane-install"` {
			r.AdapterData["fenceRoot"], _ = json.Marshal(b.install)
		}
		if r.ID == "critic-second" {
			r.Measurement.UsageRead = false
		}
		if err := b.store.Create(r); err != nil {
			t.Fatal(err)
		}
	}
	b.report(b.now.Add(-30 * time.Minute))
	b.report(b.now.Add(30 * time.Minute))
	text, _, _ := strings.Cut(b.text(), "## Hour to "+b.now.Add(-30*time.Minute).Local().Format("2006-01-02 15:04 MST"))
	if strings.Contains(text, "session critic-session") || strings.Contains(text, "999 tokens") || !strings.Contains(text, "slow critics: unmeasured:") || !strings.Contains(text, "critic-second/record.json measurement.usageRead") {
		t.Fatal(text)
	}
}
