package launch

import (
	"fmt"
	"sort"
)

type KindReport struct {
	Kind           string `json:"kind"`
	Jobs           int    `json:"jobs"`
	Completed      int    `json:"completed"`
	Failed         int    `json:"failed"`
	Cancelled      int    `json:"cancelled"`
	Compactions    int    `json:"compactions"`
	CompactedJobs  int    `json:"compactedJobs"`
	PeaksAbove200K int    `json:"peaksAbove200K"`
}
type JobReport struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Compactions int    `json:"compactions"`
	PeakContext int64  `json:"peakContext"`
}
type SummaryReport struct {
	Kinds          []KindReport   `json:"kinds"`
	Refusals       map[string]int `json:"refusals"`
	Jobs           []JobReport    `json:"jobs"`
	BuildsOverCap  int            `json:"buildsOverCap"`
	CompactedReads int            `json:"compactedReads"`
}

func (m *Manager) Report(goal string) (SummaryReport, error) {
	records, err := m.Store.List()
	if err != nil {
		return SummaryReport{}, err
	}
	refusals, err := m.Store.Refusals()
	if err != nil {
		return SummaryReport{}, err
	}
	result := SummaryReport{Refusals: map[string]int{}}
	byKind := map[string]int{}
	for _, kind := range []string{"build", "design", "read", "critique"} {
		result.Kinds = append(result.Kinds, KindReport{Kind: kind})
		byKind[kind] = len(result.Kinds) - 1
	}
	for _, record := range records {
		if goal != "" && record.Goal != goal {
			continue
		}
		index, found := byKind[record.Kind]
		if !found {
			result.Kinds = append(result.Kinds, KindReport{Kind: record.Kind})
			index = len(result.Kinds) - 1
			byKind[record.Kind] = index
		}
		row := &result.Kinds[index]
		row.Jobs++
		switch record.State {
		case Completed:
			row.Completed++
		case Failed:
			row.Failed++
		case Cancelled:
			row.Cancelled++
		}
		row.Compactions += record.Measurement.Compactions
		if record.Measurement.Compactions > 0 {
			row.CompactedJobs++
		}
		if record.Measurement.PeakContext > 200000 {
			row.PeaksAbove200K++
		}
		if record.Measurement.Compactions > 0 || record.Measurement.CallsAbove200 > 0 {
			result.Jobs = append(result.Jobs, JobReport{record.ID, record.Kind, record.Measurement.Compactions, record.Measurement.PeakContext})
		}
		if record.Kind == "read" && record.Measurement.Compactions > 0 {
			result.CompactedReads++
		}
	}
	for _, refusal := range refusals {
		if goal != "" && refusal.Goal != goal {
			continue
		}
		result.Refusals[refusal.Code]++
		if refusal.Code == "LAUNCH_BUILD_OVERSIZE" {
			result.BuildsOverCap++
		}
	}
	return result, nil
}

func (report SummaryReport) Lines() []string {
	var lines []string
	for _, row := range report.Kinds {
		lines = append(lines, fmt.Sprintf("kind=%s jobs=%d completed=%d failed=%d cancelled=%d compactions=%d compacted-jobs=%d peaks-above-200k=%d", row.Kind, row.Jobs, row.Completed, row.Failed, row.Cancelled, row.Compactions, row.CompactedJobs, row.PeaksAbove200K))
	}
	var codes []string
	for code := range report.Refusals {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		lines = append(lines, fmt.Sprintf("refused code=%s count=%d", code, report.Refusals[code]))
	}
	for _, job := range report.Jobs {
		lines = append(lines, fmt.Sprintf("job id=%s kind=%s compactions=%d peak-context=%d", job.ID, job.Kind, job.Compactions, job.PeakContext))
	}
	return append(lines, fmt.Sprintf("builds-over-cap=%d compacted-reads=%d", report.BuildsOverCap, report.CompactedReads))
}
