package launch

import (
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
