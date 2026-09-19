package launch

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type KindReport struct {
	Kind           string `json:"kind"`
	Jobs           int    `json:"jobs"`
	Completed      int    `json:"completed"`
	Failed         int    `json:"failed"`
	Cancelled      int    `json:"cancelled"`
	Unmeasured     int    `json:"unmeasured"`
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
	MaterialCount   *int   `json:"materialCount,omitempty"`
	Verdict         string `json:"verdict"`
	Compactions     int    `json:"compactions"`
}
type BaselineReport struct {
	Kind        string `json:"kind"`
	Requests    int64  `json:"requests"`
	Tokens      int64  `json:"tokens"`
	PeakContext int64  `json:"peakContext"`
}
type RoundReport struct {
	Goal              string `json:"goal"`
	Number            int    `json:"number"`
	DesignID          string `json:"designId,omitempty"`
	CritiqueID        string `json:"critiqueId"`
	DesignTokens      *int64 `json:"designTokens,omitempty"`
	DesignPeakContext *int64 `json:"designPeakContext,omitempty"`
	MaterialCount     int    `json:"materialCount"`
	Verdict           string `json:"verdict"`
}
type RoundsToAcceptanceReport struct {
	Goal   string `json:"goal"`
	Rounds *int   `json:"rounds,omitempty"`
}
type SummaryReport struct {
	Kinds              []KindReport               `json:"kinds"`
	Refusals           map[string]int             `json:"refusals"`
	Jobs               []JobReport                `json:"jobs"`
	Rounds             []RoundReport              `json:"rounds"`
	RoundsToAcceptance []RoundsToAcceptanceReport `json:"roundsToAcceptance"`
	Baseline           BaselineReport             `json:"baseline"`
	BuildsOverCap      int                        `json:"buildsOverCap"`
	CompactedReads     int                        `json:"compactedReads"`
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
		if !record.Measured {
			row.Unmeasured++
		}
		row.Compactions += record.Measurement.Compactions
		if record.Measurement.Compactions > 0 {
			row.CompactedJobs++
		}
		if record.Measurement.PeakContext > 200000 {
			row.PeaksAbove200K++
		}
		if record.Measurement.Compactions > 0 || record.Measurement.CallsAbove200 > 0 ||
			record.State == Completed && (record.Kind == "design" || record.Kind == "read" || record.Kind == "critique") {
			measurement := record.Measurement
			result.Jobs = append(result.Jobs, JobReport{
				ID: record.ID, Kind: record.Kind, Requests: measurement.Calls, ToolCalls: measurement.ToolCalls,
				Tokens: measurement.TotalTokens(), CacheReadTokens: measurement.CacheReadTokens,
				OutputTokens: measurement.OutputTokens, PeakContext: measurement.PeakContext,
				MaterialCount: measuredMaterialCount(record), Verdict: measuredVerdict(record),
				Compactions: measurement.Compactions,
			})
		}
		if record.Kind == "read" && record.Measurement.Compactions > 0 {
			result.CompactedReads++
		}
	}
	result.addRounds(records, goal)
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
		lines = append(lines, fmt.Sprintf("kind=%s jobs=%d completed=%d failed=%d cancelled=%d unmeasured=%d compactions=%d compacted-jobs=%d peaks-above-200k=%d", row.Kind, row.Jobs, row.Completed, row.Failed, row.Cancelled, row.Unmeasured, row.Compactions, row.CompactedJobs, row.PeaksAbove200K))
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
		material := "-"
		if job.MaterialCount != nil {
			material = strconv.Itoa(*job.MaterialCount)
		}
		verdict := job.Verdict
		if verdict == "" {
			verdict = "-"
		}
		line := fmt.Sprintf("job id=%s kind=%s requests=%d tool-calls=%d tokens=%d cache-read=%d output=%d peak-context=%d material=%s verdict=%s compactions=%d",
			job.ID, job.Kind, job.Requests, job.ToolCalls, job.Tokens, job.CacheReadTokens, job.OutputTokens,
			job.PeakContext, material, verdict, job.Compactions)
		if job.Kind == "design" {
			line += fmt.Sprintf(" tokens-vs-baseline=%.2f", float64(job.Tokens)/float64(report.Baseline.Tokens))
		}
		lines = append(lines, line)
	}
	for _, round := range report.Rounds {
		design, tokens, peak := "-", "-", "-"
		if round.DesignID != "" {
			design = round.DesignID
		}
		if round.DesignTokens != nil {
			tokens = strconv.FormatInt(*round.DesignTokens, 10)
		}
		if round.DesignPeakContext != nil {
			peak = strconv.FormatInt(*round.DesignPeakContext, 10)
		}
		verdict := round.Verdict
		if verdict == "" {
			verdict = "-"
		}
		lines = append(lines, fmt.Sprintf("round goal=%s n=%d design=%s critique=%s design-tokens=%s design-peak-context=%s material=%d verdict=%s",
			round.Goal, round.Number, design, round.CritiqueID, tokens, peak, round.MaterialCount, verdict))
	}
	for _, summary := range report.RoundsToAcceptance {
		rounds := "open"
		if summary.Rounds != nil {
			rounds = strconv.Itoa(*summary.Rounds)
		}
		lines = append(lines, fmt.Sprintf("rounds-to-acceptance goal=%s rounds=%s", summary.Goal, rounds))
	}
	return append(lines, fmt.Sprintf("builds-over-cap=%d compacted-reads=%d", report.BuildsOverCap, report.CompactedReads))
}

func RecordLine(record Record, root string) string {
	exit := "-"
	if record.ExitCode != nil {
		exit = strconv.Itoa(*record.ExitCode)
	}
	verdict := strings.ReplaceAll(record.Measurement.Verdict, "\n", " ")
	page := fmt.Sprintf("page-lines=%d page-words=%d", record.Measurement.PageLines, record.Measurement.PageWords)
	if record.Measurement.PageMissing {
		page = "page=missing"
	}
	verdictState := ""
	if !record.VerdictIsCounting() {
		verdictState = " rerun-split"
	}
	return fmt.Sprintf("id=%s state=%s directory=%s exit=%s measured=%t result-lines=%d result-words=%d result-tail=%q calls=%d turns=%d compactions=%d peak-context=%d calls-above-200k=%d %s material=%d verdict=%q%s",
		record.ID, record.State, filepath.Join(root, record.ID), exit, record.Measured, record.Measurement.ResultLines, record.Measurement.ResultWords,
		record.Measurement.ResultTail, record.Measurement.Calls, record.Measurement.Turns, record.Measurement.Compactions, record.Measurement.PeakContext,
		record.Measurement.CallsAbove200, page, record.Measurement.MaterialCount, verdict, verdictState)
}

var readFixVerdict = regexp.MustCompile(`^VERDICT: fix first \(([0-9]+) material findings\)$`)

func measuredMaterialCount(record Record) *int {
	switch record.Kind {
	case "critique":
		if record.State != Completed {
			return nil
		}
		count := record.Measurement.MaterialCount
		return &count
	case "read":
		if record.Measurement.Verdict == "VERDICT: land" {
			count := 0
			return &count
		}
		match := readFixVerdict.FindStringSubmatch(record.Measurement.Verdict)
		if len(match) == 2 {
			count, err := strconv.Atoi(match[1])
			if err == nil {
				return &count
			}
		}
	}
	return nil
}

func measuredVerdict(record Record) string {
	if record.Kind == "design" {
		return ""
	}
	return record.Measurement.Verdict
}

func (report *SummaryReport) addRounds(records []Record, goalFilter string) {
	roundNumbers := map[string]int{}
	previousCritique := map[string]string{}
	acceptance := map[string]*int{}
	var goals []string
	seenGoal := map[string]bool{}
	for _, critique := range records {
		if critique.Kind != "critique" || critique.State != Completed ||
			(goalFilter != "" && critique.Goal != goalFilter) {
			continue
		}
		if !seenGoal[critique.Goal] {
			seenGoal[critique.Goal] = true
			goals = append(goals, critique.Goal)
		}
		roundNumbers[critique.Goal]++
		number := roundNumbers[critique.Goal]
		round := RoundReport{Goal: critique.Goal, Number: number, CritiqueID: critique.ID,
			MaterialCount: critique.Measurement.MaterialCount, Verdict: critique.Measurement.Verdict}
		var design *Record
		for index := range records {
			candidate := &records[index]
			if candidate.Kind != "design" || candidate.State != Completed || candidate.Goal != critique.Goal ||
				candidate.StartedAt >= critique.StartedAt ||
				(previousCritique[critique.Goal] != "" && candidate.StartedAt <= previousCritique[critique.Goal]) {
				continue
			}
			if design == nil || candidate.StartedAt > design.StartedAt {
				design = candidate
			}
		}
		if design != nil {
			round.DesignID = design.ID
			tokens := design.Measurement.TotalTokens()
			peak := design.Measurement.PeakContext
			round.DesignTokens, round.DesignPeakContext = &tokens, &peak
		}
		report.Rounds = append(report.Rounds, round)
		if _, accepted := acceptance[critique.Goal]; !accepted && critique.Measurement.MaterialCount == 0 &&
			verdictLine.MatchString(critique.Measurement.Verdict) &&
			(critique.VerdictCounts == nil || *critique.VerdictCounts) {
			acceptedRound := number
			acceptance[critique.Goal] = &acceptedRound
		}
		previousCritique[critique.Goal] = critique.StartedAt
	}
	for _, goal := range goals {
		report.RoundsToAcceptance = append(report.RoundsToAcceptance, RoundsToAcceptanceReport{
			Goal: goal, Rounds: acceptance[goal],
		})
	}
}
