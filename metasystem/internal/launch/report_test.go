package launch

import (
	"encoding/json"
	"strings"
	"testing"
)

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
	design := Record{ID: "design-usage", Kind: "design", Adapter: "claude-headless", State: Completed,
		Measurement: reportMeasurement(t, `{"calls":4,"toolCalls":7,"inputTokens":200000,"cacheReadTokens":40000,"cacheCreationTokens":3000,"outputTokens":237,"peakContext":90000,"materialCount":2,"verdict":"accepted","compactions":1}`)}
	read := Record{ID: "read-usage", Kind: "read", Adapter: "claude-headless", State: Completed,
		Measurement: reportMeasurement(t, `{"calls":2,"toolCalls":3,"inputTokens":100,"cacheReadTokens":200,"cacheCreationTokens":30,"outputTokens":40,"peakContext":250,"materialCount":1,"verdict":"revise"}`)}
	for _, record := range []Record{design, read} {
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
	designLine := "job id=design-usage kind=design requests=4 tool-calls=7 tokens=243237 cache-read=40000 output=237 peak-context=90000 material=2 verdict=accepted compactions=1 tokens-vs-baseline=0.10"
	readLine := "job id=read-usage kind=read requests=2 tool-calls=3 tokens=370 cache-read=200 output=40 peak-context=250 material=1 verdict=revise compactions=0"
	if !strings.Contains(joined, baseline) || !strings.Contains(joined, designLine) || !strings.Contains(joined, readLine) {
		t.Fatalf("report omitted usage rows:\n%s", joined)
	}
	if strings.Index(joined, baseline) > strings.Index(joined, designLine) {
		t.Fatalf("baseline follows the design row:\n%s", joined)
	}
}

func reportMeasurement(t *testing.T, data string) Measurement {
	t.Helper()
	var measurement Measurement
	if err := json.Unmarshal([]byte(data), &measurement); err != nil {
		t.Fatal(err)
	}
	return measurement
}
