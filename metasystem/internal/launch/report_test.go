package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReportSinceFiltersRecordsAndRefusals(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	for _, record := range []Record{
		{ID: "before", Kind: "build", Goal: "goal-a", State: Completed, StartedAt: "2026-09-18T09:59:59Z"},
		{ID: "boundary", Kind: "build", Goal: "goal-a", State: Completed, StartedAt: "2026-09-18T10:00:00Z"},
		{ID: "after", Kind: "read", Goal: "goal-a", State: Completed, StartedAt: "2026-09-18T10:00:01Z"},
		{ID: "other-goal", Kind: "build", Goal: "goal-b", State: Completed, StartedAt: "2026-09-18T10:00:02Z"},
	} {
		if err := m.Store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	for _, refusal := range []Refusal{
		{Time: "2026-09-18T09:59:59Z", Code: "BEFORE", Goal: "goal-a"},
		{Time: "2026-09-18T10:00:00Z", Code: "BOUNDARY", Goal: "goal-a"},
		{Time: "2026-09-18T10:00:01Z", Code: "AFTER", Goal: "goal-a"},
		{Time: "2026-09-18T10:00:02Z", Code: "OTHER_GOAL", Goal: "goal-b"},
	} {
		if err := m.Store.AppendRefusal(refusal); err != nil {
			t.Fatal(err)
		}
	}

	since := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	report, err := m.Report("goal-a", since)
	if err != nil {
		t.Fatal(err)
	}
	if report.Kinds[0].Jobs != 1 || report.Kinds[2].Jobs != 1 {
		t.Fatalf("record boundary was not inclusive: %+v", report.Kinds)
	}
	if report.Refusals["BEFORE"] != 0 || report.Refusals["BOUNDARY"] != 1 || report.Refusals["AFTER"] != 1 || report.Refusals["OTHER_GOAL"] != 0 {
		t.Fatalf("refusal boundary was not inclusive: %+v", report.Refusals)
	}
}

func TestReportPerGoalCountsKindsRefusalsAndCompactions(t *testing.T) {
	m, _, _, _ := manager(t)
	first := seed(t, m, "reported-build", Completed)
	first.Goal, first.Measurement = "goal-a", Measurement{Compactions: 2, CallsAbove200: 1, PeakContext: 210000}
	if _, err := m.Store.Update(first.ID, func(record *Record) error {
		record.Goal, record.Measurement = first.Goal, first.Measurement
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	second := Record{ID: "reported-read", Kind: "read", Adapter: "claude-headless", Goal: "goal-a", WorkingDirectory: t.TempDir(), State: Failed, StartedAt: m.Now().Format("2006-01-02T15:04:05Z07:00"), Measurement: Measurement{Compactions: 1}}
	if err := m.Store.Create(second); err != nil {
		t.Fatal(err)
	}
	if err := m.Store.AppendRefusal(Refusal{Code: "LAUNCH_BUILD_OVERSIZE", Goal: "goal-a", Kind: "build"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Store.AppendRefusal(Refusal{Code: "LAUNCH_BUILD_UNSIZED", Goal: "other", Kind: "build"}); err != nil {
		t.Fatal(err)
	}
	report, err := m.Report("goal-a")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(report.Lines(), "\n")
	for _, wanted := range []string{"kind=build jobs=1 completed=1", "compactions=2 compacted-jobs=1 peaks-above-200k=1", "kind=read jobs=1 completed=0 failed=1", "refused code=LAUNCH_BUILD_OVERSIZE count=1", "job id=reported-build", "builds-over-cap=1 compacted-reads=1"} {
		if !strings.Contains(joined, wanted) {
			t.Errorf("missing %q in %s", wanted, joined)
		}
	}
}

func TestReportListsDesignAndReadUsageAgainstTheBaseline(t *testing.T) {
	m, _, _, _ := manager(t)
	notCounting := false
	design := Record{ID: "design-usage", Kind: "design", Adapter: "claude-headless", Goal: "accepted", State: Completed,
		StartedAt: "2026-09-18T11:00:00Z", Measurement: Measurement{Calls: 4, ToolCalls: 7, InputTokens: 200000,
			CacheReadTokens: 40000, CacheCreationTokens: 3000, OutputTokens: 237, PeakContext: 90000, Compactions: 1}}
	openDesign := Record{ID: "open-design", Kind: "design", Adapter: "claude-headless", Goal: "open", State: Completed,
		StartedAt: "2026-09-18T10:30:00Z", Measurement: Measurement{InputTokens: 80, CacheReadTokens: 15, OutputTokens: 5, PeakContext: 70}}
	read := Record{ID: "read-usage", Kind: "read", Adapter: "claude-headless", State: Completed, StartedAt: "2026-09-18T13:00:00Z",
		Measurement: reportReadMeasurement(t, "VERDICT: fix first (3 material findings)")}
	read.Measurement.Calls, read.Measurement.ToolCalls = 2, 3
	read.Measurement.InputTokens, read.Measurement.CacheReadTokens = 100, 200
	read.Measurement.CacheCreationTokens, read.Measurement.OutputTokens = 30, 40
	read.Measurement.PeakContext = 250
	otherRead := Record{ID: "read-other", Kind: "read", Adapter: "claude-headless", State: Completed, StartedAt: "2026-09-18T14:00:00Z",
		Measurement: reportReadMeasurement(t, "VERDICT: investigate")}
	landRead := Record{ID: "read-land", Kind: "read", Adapter: "claude-headless", State: Completed, StartedAt: "2026-09-18T15:00:00Z",
		Measurement: reportReadMeasurement(t, "VERDICT: land")}
	records := []Record{
		{ID: "accepted-critique-1", Kind: "critique", Adapter: "codex-exec", Goal: "accepted", State: Completed,
			StartedAt: "2026-09-18T10:00:00Z", Measurement: reportCritiqueMeasurement(t, 1, "VERDICT: revise")},
		openDesign,
		design,
		{ID: "open-critique-1", Kind: "critique", Adapter: "codex-exec", Goal: "open", State: Completed,
			StartedAt: "2026-09-18T11:30:00Z", VerdictCounts: &notCounting, Measurement: reportCritiqueMeasurement(t, 0, "VERDICT: land")},
		{ID: "accepted-critique-2", Kind: "critique", Adapter: "codex-exec", Goal: "accepted", State: Completed,
			StartedAt: "2026-09-18T12:00:00Z", Measurement: reportCritiqueMeasurement(t, 0, "VERDICT: land")},
		read,
		otherRead,
		landRead,
	}
	for _, record := range records {
		if err := m.Store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	report, err := m.Report("")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(report.Lines(), "\n")
	baseline := "baseline kind=design requests=28 tokens=2432374 peak-context=163000"
	wanted := []string{
		baseline,
		"job id=design-usage kind=design requests=4 tool-calls=7 tokens=243237 cache-read=40000 output=237 peak-context=90000 material=- verdict=- compactions=1 tokens-vs-baseline=0.10",
		"job id=read-usage kind=read requests=2 tool-calls=3 tokens=370 cache-read=200 output=40 peak-context=250 material=3 verdict=VERDICT: fix first (3 material findings) compactions=0",
		"job id=read-other kind=read requests=0 tool-calls=0 tokens=0 cache-read=0 output=0 peak-context=0 material=- verdict=VERDICT: investigate compactions=0",
		"job id=read-land kind=read requests=0 tool-calls=0 tokens=0 cache-read=0 output=0 peak-context=0 material=0 verdict=VERDICT: land compactions=0",
		"job id=accepted-critique-1 kind=critique requests=0 tool-calls=0 tokens=0 cache-read=0 output=0 peak-context=0 material=1 verdict=VERDICT: revise compactions=0",
		"job id=open-critique-1 kind=critique requests=0 tool-calls=0 tokens=0 cache-read=0 output=0 peak-context=0 material=0 verdict=VERDICT: land compactions=0",
		"round goal=accepted n=1 design=- critique=accepted-critique-1 design-tokens=- design-peak-context=- material=1 verdict=VERDICT: revise",
		"round goal=open n=1 design=open-design critique=open-critique-1 design-tokens=100 design-peak-context=70 material=0 verdict=VERDICT: land",
		"round goal=accepted n=2 design=design-usage critique=accepted-critique-2 design-tokens=243237 design-peak-context=90000 material=0 verdict=VERDICT: land",
		"rounds-to-acceptance goal=accepted rounds=2",
		"rounds-to-acceptance goal=open rounds=open",
	}
	for _, line := range wanted {
		if !strings.Contains(joined, line) {
			t.Errorf("report omitted %q:\n%s", line, joined)
		}
	}
	if strings.Index(joined, baseline) > strings.Index(joined, "job id=design-usage") {
		t.Fatalf("baseline follows the design row:\n%s", joined)
	}
	if strings.Index(joined, "job id=read-land") > strings.Index(joined, "round goal=accepted") {
		t.Fatalf("rounds do not follow all job rows:\n%s", joined)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var encoded struct {
		Jobs []map[string]any `json:"jobs"`
	}
	if err := json.Unmarshal(data, &encoded); err != nil {
		t.Fatal(err)
	}
	for _, job := range encoded.Jobs {
		if job["id"] == "design-usage" {
			if _, present := job["materialCount"]; present {
				t.Fatalf("design material count was serialized: %s", data)
			}
		}
	}
}

func reportReadMeasurement(t *testing.T, verdict string) Measurement {
	t.Helper()
	path := filepath.Join(t.TempDir(), "read.md")
	if err := os.WriteFile(path, []byte(verdict+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := Record{WorkingDirectory: t.TempDir(), AdapterData: map[string]json.RawMessage{}}
	setStrings(record.AdapterData, "declaredOutputs", []string{path})
	return Measurement{Verdict: readVerdict(record)}
}

func reportCritiqueMeasurement(t *testing.T, material int, verdict string) Measurement {
	t.Helper()
	worktree, state := t.TempDir(), t.TempDir()
	directory := filepath.Join(worktree, "metasystem", "artifacts", "reports")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	contents := strings.Repeat("material: yes\n", material) + verdict + "\n"
	if err := os.WriteFile(filepath.Join(directory, "round-critique-r1.md"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	measurement := Measurement{}
	if _, err := copyCritique(Record{WorkingDirectory: worktree, Tag: "round"}, state, &measurement); err != nil {
		t.Fatal(err)
	}
	return measurement
}
