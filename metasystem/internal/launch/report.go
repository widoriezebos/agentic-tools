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
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Requests        int    `json:"requests"`
	ToolCalls       int    `json:"toolCalls"`
	Tokens          int64  `json:"tokens"`
	CacheReadTokens int64  `json:"cacheReadTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	PeakContext     int64  `json:"peakContext"`
	MaterialCount   int    `json:"materialCount"`
	Verdict         string `json:"verdict"`
	Compactions     int    `json:"compactions"`
}
type BaselineReport struct {
	Kind        string `json:"kind"`
	Requests    int64  `json:"requests"`
	Tokens      int64  `json:"tokens"`
	PeakContext int64  `json:"peakContext"`
}
type SummaryReport struct {
	Kinds          []KindReport   `json:"kinds"`
	Refusals       map[string]int `json:"refusals"`
	Jobs           []JobReport    `json:"jobs"`
	Baseline       BaselineReport `json:"baseline"`
	BuildsOverCap  int            `json:"buildsOverCap"`
	CompactedReads int            `json:"compactedReads"`
}

func (m *Manager) Report(goal string) (SummaryReport, error) {
	settings, err := m.resolvedSettings()
	if err != nil {
		return SummaryReport{}, err
	}
	records, err := m.Store.List()
	if err != nil {
		return SummaryReport{}, err
	}
	refusals, err := m.Store.Refusals()
	if err != nil {
		return SummaryReport{}, err
	}
	result := SummaryReport{Refusals: map[string]int{}, Baseline: BaselineReport{
		Kind: "design", Requests: settings.DesignBaselineRequests, Tokens: settings.DesignBaselineTokens,
		PeakContext: settings.DesignBaselinePeakTokens,
	}}
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
		if record.Measurement.Compactions > 0 || record.Measurement.CallsAbove200 > 0 ||
			record.State == Completed && (record.Kind == "design" || record.Kind == "read") {
			measurement := record.Measurement
			result.Jobs = append(result.Jobs, JobReport{
				ID: record.ID, Kind: record.Kind, Requests: measurement.Calls, ToolCalls: measurement.ToolCalls,
				Tokens: measurement.TotalTokens(), CacheReadTokens: measurement.CacheReadTokens,
				OutputTokens: measurement.OutputTokens, PeakContext: measurement.PeakContext,
				MaterialCount: measurement.MaterialCount, Verdict: measurement.Verdict,
				Compactions: measurement.Compactions,
			})
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
	lines = append(lines, fmt.Sprintf("baseline kind=%s requests=%d tokens=%d peak-context=%d", report.Baseline.Kind, report.Baseline.Requests, report.Baseline.Tokens, report.Baseline.PeakContext))
	for _, job := range report.Jobs {
		line := fmt.Sprintf("job id=%s kind=%s requests=%d tool-calls=%d tokens=%d cache-read=%d output=%d peak-context=%d material=%d verdict=%s compactions=%d",
			job.ID, job.Kind, job.Requests, job.ToolCalls, job.Tokens, job.CacheReadTokens, job.OutputTokens,
			job.PeakContext, job.MaterialCount, job.Verdict, job.Compactions)
		if job.Kind == "design" {
			line += fmt.Sprintf(" tokens-vs-baseline=%.2f", float64(job.Tokens)/float64(report.Baseline.Tokens))
		}
		lines = append(lines, line)
	}
	return append(lines, fmt.Sprintf("builds-over-cap=%d compacted-reads=%d", report.BuildsOverCap, report.CompactedReads))
}
