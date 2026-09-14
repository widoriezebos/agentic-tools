package steward

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// ContextReport is the measured result for one half-open seven-day window.
type ContextReport struct {
	WeekStart           time.Time
	WeekEnd             time.Time
	Cohort              map[string]int
	Samples             int
	P95                 int64
	Max                 int64
	Compactions         map[string]int
	Resets              int
	Handoffs            int
	ReferenceMismatches []string
	DuplicateSamples    int
	DuplicateMarkers    int
	UnassignableSamples int
	UnassignableMarkers int
	Pass                bool
	Failures            []string
}

type contextCallExport struct {
	Runtime      string    `json:"runtime"`
	Session      string    `json:"session"`
	InvocationID string    `json:"invocationId"`
	PromptTokens int64     `json:"promptTokens"`
	Source       string    `json:"source"`
	CursorOffset int64     `json:"cursorOffset"`
	At           time.Time `json:"at"`
}

// ContextEvidenceRetiredError reports that a requested report would require
// raw call evidence which has already crossed the retained-history boundary.
type ContextEvidenceRetiredError struct {
	WeekStart     time.Time
	RetainedSince time.Time
}

func (e *ContextEvidenceRetiredError) Error() string {
	return fmt.Sprintf("CONTEXT_EVIDENCE_RETIRED requested=%s retained-since=%s", e.WeekStart.Format("2006-01-02"), e.RetainedSince.Format("2006-01-02"))
}

var (
	lookupContextReportRuntime = runtimes.Lookup
	readContextCallEvidence    = usage.ReadCallEvidence
)

const contextReportCoverage = "This cohort covers distinct recorded per-call samples. Runtimes with per-invocation usage, including Devin and ACP outcomes, are outside it. Calls the harness did not record are outside it. Claude fallback identity uses a physical line and timestamp. Compaction sources are Claude compact_boundary and Codex compacted. A reset is a later registered session for the same runtime and process identity. This report does not prove an independent inventory of all provider calls."

// WriteContextReport reads and validates all evidence before publishing the
// calls export and then the digest-bound Markdown report.
func WriteContextReport(stateRoot string, weekStart, now time.Time) (
	callsPath, reportPath string, report ContextReport, err error,
) {
	weekStart, err = contextReportWeekStart(weekStart)
	if err != nil {
		return "", "", ContextReport{}, err
	}
	weekEnd := weekStart.AddDate(0, 0, 7)
	weekName := weekStart.Format("2006-01-02")
	directory := filepath.Join(stateRoot, "artifacts", "reports", "coordinator-context", weekName)
	callsPath = filepath.Join(directory, "calls.jsonl")
	reportPath = filepath.Join(directory, "report.md")

	evidence, err := readContextCallEvidence(stateRoot)
	if err != nil {
		return callsPath, reportPath, ContextReport{}, err
	}
	if weekStart.Before(evidence.RetainedSince) {
		return callsPath, reportPath, ContextReport{}, &ContextEvidenceRetiredError{WeekStart: weekStart, RetainedSince: evidence.RetainedSince}
	}
	sessions := evidence.Sessions
	registrations := evidence.Registrations
	samples := evidence.Samples
	markers := evidence.Markers

	normalizedSamples, normalizedMarkers, duplicateSamples, duplicateMarkers, err :=
		normalizeContextCalls(samples, markers, weekStart, weekEnd)
	if err != nil {
		return callsPath, reportPath, ContextReport{}, err
	}
	sourceGaps := contextReportSourceGaps(samples)
	report, retained, coverageGaps := buildContextReport(
		sessions, registrations, normalizedSamples, normalizedMarkers,
		duplicateSamples, duplicateMarkers, weekStart, weekEnd)
	coverageGaps = mergeContextCoverageGaps(coverageGaps, sourceGaps)

	handoffs, err := contextReportHandoffs(stateRoot, weekStart, weekEnd)
	if err != nil {
		return callsPath, reportPath, ContextReport{}, err
	}
	report.Handoffs = handoffs
	referenceMismatches, referenceGaps, err := contextReportReferenceMismatches(stateRoot, weekStart, weekEnd)
	if err != nil {
		return callsPath, reportPath, ContextReport{}, err
	}
	report.ReferenceMismatches = referenceMismatches
	coverageGaps = append(coverageGaps, referenceGaps...)
	finishContextReport(&report, coverageGaps)

	callsText, err := renderContextCalls(retained)
	if err != nil {
		return callsPath, reportPath, ContextReport{}, err
	}
	digest := sha256.Sum256([]byte(callsText))
	digestText := hex.EncodeToString(digest[:])
	reportText := renderContextReport(report, coverageGaps, digestText, now.UTC())

	if durable, writeErr := atomicfile.WriteText(callsPath, callsText, stateRoot); writeErr != nil {
		return callsPath, reportPath, ContextReport{}, fmt.Errorf("cannot publish context calls %s: %w", callsPath, writeErr)
	} else if !durable {
		return callsPath, reportPath, ContextReport{}, fmt.Errorf("context calls %s were published with durability unknown", callsPath)
	}
	if durable, writeErr := atomicfile.WriteText(reportPath, reportText, stateRoot); writeErr != nil {
		return callsPath, reportPath, ContextReport{}, fmt.Errorf("cannot publish context report %s: %w", reportPath, writeErr)
	} else if !durable {
		return callsPath, reportPath, ContextReport{}, fmt.Errorf("context report %s was published with durability unknown", reportPath)
	}
	return callsPath, reportPath, report, nil
}

func contextReportWeekStart(value time.Time) (time.Time, error) {
	utc := value.UTC()
	midnight := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	if value.IsZero() || !value.Equal(midnight) {
		return time.Time{}, fmt.Errorf("context report week start must be UTC midnight")
	}
	return midnight, nil
}

type providerCallKey struct {
	runtime, session, invocation string
}

type fallbackCallKey struct {
	runtime, session, invocation, at string
	ordinal                          int64
}

type markerIdentity struct {
	runtime, session, kind, at, detail string
	ordinal                            int64
}

// normalizeContextCalls collapses only replay copies with the identity rules
// owned by the report. Undated evidence remains distinct and visible.
func normalizeContextCalls(samples []usage.CallSample, markers []usage.Marker,
	weekStart, weekEnd time.Time,
) ([]usage.CallSample, []usage.Marker, int, int, error) {
	var normalizedSamples []usage.CallSample
	providerCalls := map[providerCallKey]int{}
	fallbackCalls := map[fallbackCallKey]int{}
	duplicateSamples := 0
	for _, sample := range samples {
		if sample.At.IsZero() || sample.InvocationID == "" {
			normalizedSamples = append(normalizedSamples, sample)
			continue
		}
		if isClaudeFallbackIdentity(sample) {
			key := fallbackCallKey{
				runtime: sample.Runtime, session: sample.Session, invocation: sample.InvocationID,
				ordinal: sample.Ordinal, at: contextTimestampKey(sample.At),
			}
			if first, found := fallbackCalls[key]; found {
				if !equalCallTokens(normalizedSamples[first], sample) {
					return nil, nil, 0, 0, fmt.Errorf("conflicting fallback call identity %s/%s/%s ordinal=%d at=%s",
						sample.Runtime, sample.Session, sample.InvocationID, sample.Ordinal, key.at)
				}
				if inContextWeek(sample.At, weekStart, weekEnd) {
					duplicateSamples++
				}
				continue
			}
			fallbackCalls[key] = len(normalizedSamples)
			normalizedSamples = append(normalizedSamples, sample)
			continue
		}
		key := providerCallKey{runtime: sample.Runtime, session: sample.Session, invocation: sample.InvocationID}
		if first, found := providerCalls[key]; found {
			representative := normalizedSamples[first]
			if !representative.At.Equal(sample.At) || !equalCallTokens(representative, sample) {
				return nil, nil, 0, 0, fmt.Errorf("conflicting provider call identity %s/%s/%s", sample.Runtime, sample.Session, sample.InvocationID)
			}
			if inContextWeek(sample.At, weekStart, weekEnd) {
				duplicateSamples++
			}
			continue
		}
		providerCalls[key] = len(normalizedSamples)
		normalizedSamples = append(normalizedSamples, sample)
	}

	var normalizedMarkers []usage.Marker
	seenMarkers := map[markerIdentity]bool{}
	duplicateMarkers := 0
	for _, marker := range markers {
		if marker.At.IsZero() || marker.Kind == "" {
			normalizedMarkers = append(normalizedMarkers, marker)
			continue
		}
		key := markerIdentity{
			runtime: marker.Runtime, session: marker.Session, kind: marker.Kind,
			at: contextTimestampKey(marker.At), ordinal: marker.Ordinal, detail: marker.Detail,
		}
		if seenMarkers[key] {
			if inContextWeek(marker.At, weekStart, weekEnd) {
				duplicateMarkers++
			}
			continue
		}
		seenMarkers[key] = true
		normalizedMarkers = append(normalizedMarkers, marker)
	}
	return normalizedSamples, normalizedMarkers, duplicateSamples, duplicateMarkers, nil
}

func isClaudeFallbackIdentity(sample usage.CallSample) bool {
	if sample.Runtime != "claude" || !strings.HasPrefix(sample.InvocationID, "line:") {
		return false
	}
	digits := strings.TrimPrefix(sample.InvocationID, "line:")
	if digits == "" {
		return false
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func equalCallTokens(left, right usage.CallSample) bool {
	return left.PromptTokens == right.PromptTokens &&
		left.InputTokens == right.InputTokens &&
		left.CacheCreation == right.CacheCreation &&
		left.CacheRead == right.CacheRead
}

func contextTimestampKey(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func inContextWeek(value, weekStart, weekEnd time.Time) bool {
	return !value.Before(weekStart) && value.Before(weekEnd)
}

func buildContextReport(sessions []usage.CallSession, registrations []usage.CallRegistration,
	samples []usage.CallSample, markers []usage.Marker, duplicateSamples, duplicateMarkers int,
	weekStart, weekEnd time.Time,
) (ContextReport, []usage.CallSample, []string) {
	report := ContextReport{
		WeekStart: weekStart, WeekEnd: weekEnd, Cohort: map[string]int{},
		Compactions: map[string]int{}, DuplicateSamples: duplicateSamples, DuplicateMarkers: duplicateMarkers,
	}
	coverage := map[string]bool{}
	var retained []usage.CallSample
	for _, sample := range samples {
		if sample.At.IsZero() || sample.InvocationID == "" {
			report.UnassignableSamples++
			continue
		}
		declaration, registered := lookupContextReportRuntime(sample.Runtime)
		if !registered {
			coverage[fmt.Sprintf("runtime %s is not registered", sample.Runtime)] = true
			continue
		}
		if declaration.ContextSample != string(usage.PerCall) {
			coverage[fmt.Sprintf("runtime %s recorded unexpected per-call samples despite capability %s", sample.Runtime, declaration.ContextSample)] = true
			continue
		}
		if !expectedContextSampleSource(sample.Runtime, sample.Source) {
			coverage[fmt.Sprintf("runtime %s session %s has unexpected sample source %q", sample.Runtime, sample.Session, sample.Source)] = true
			continue
		}
		if !inContextWeek(sample.At, weekStart, weekEnd) {
			continue
		}
		retained = append(retained, sample)
		key := contextSessionKey(sample.Runtime, sample.Session)
		report.Cohort[key]++
		report.Compactions[sample.Runtime] += 0
	}
	for _, marker := range markers {
		if marker.At.IsZero() || marker.Kind == "" {
			report.UnassignableMarkers++
			continue
		}
		declaration, registered := lookupContextReportRuntime(marker.Runtime)
		if !registered {
			coverage[fmt.Sprintf("runtime %s is not registered", marker.Runtime)] = true
			continue
		}
		if declaration.ContextSample != string(usage.PerCall) {
			coverage[fmt.Sprintf("runtime %s recorded unexpected per-call markers despite capability %s", marker.Runtime, declaration.ContextSample)] = true
			continue
		}
		if marker.Kind != "compaction" {
			coverage[fmt.Sprintf("runtime %s session %s has unexpected marker kind %q", marker.Runtime, marker.Session, marker.Kind)] = true
			continue
		}
		if inContextWeek(marker.At, weekStart, weekEnd) {
			report.Compactions[marker.Runtime]++
		}
	}
	sortContextSamples(retained)
	report.Samples = len(retained)
	if len(retained) > 0 {
		values := make([]int64, len(retained))
		for index, sample := range retained {
			values[index] = sample.PromptTokens
			if sample.PromptTokens > report.Max {
				report.Max = sample.PromptTokens
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		report.P95 = values[(95*len(values)+99)/100-1]
	}

	registrations = distinctContextRegistrations(registrations)
	report.Resets = contextReportResets(registrations, weekStart, weekEnd)
	registeredPairs := map[string]bool{}
	for _, registration := range registrations {
		key := contextSessionKey(registration.Runtime, registration.Session)
		registeredPairs[key] = true
		if !inContextWeek(registration.FirstSeen, weekStart, weekEnd) {
			continue
		}
		declaration, registered := lookupContextReportRuntime(registration.Runtime)
		if !registered {
			coverage[fmt.Sprintf("registered runtime %s is not in the runtime registry", registration.Runtime)] = true
			continue
		}
		if declaration.ContextSample == string(usage.PerCall) && report.Cohort[key] == 0 {
			coverage[fmt.Sprintf("registered per-call session %s has no dated sample in the week", key)] = true
		}
	}
	for _, session := range sessions {
		declaration, registered := lookupContextReportRuntime(session.Runtime)
		if !registered || declaration.ContextSample != string(usage.PerCall) {
			continue
		}
		key := contextSessionKey(session.Runtime, session.Session)
		if !registeredPairs[key] {
			coverage[fmt.Sprintf("reset coverage is unavailable for %s: no registered process evidence", key)] = true
		}
	}
	coverageGaps := make([]string, 0, len(coverage))
	for gap := range coverage {
		coverageGaps = append(coverageGaps, gap)
	}
	sort.Strings(coverageGaps)
	return report, retained, coverageGaps
}

func expectedContextSampleSource(runtime, source string) bool {
	want := map[string]string{"claude": "claude-transcript", "codex": "codex-rollout"}[runtime]
	return want != "" && (source == want || strings.HasPrefix(source, want+"; "))
}

// contextReportSourceGaps observes every committed row before replay
// normalization. Source is deliberately absent from provider-call identity,
// but an invalid source must not disappear behind a valid representative.
func contextReportSourceGaps(samples []usage.CallSample) []string {
	gaps := map[string]bool{}
	for _, sample := range samples {
		declaration, registered := lookupContextReportRuntime(sample.Runtime)
		if registered && declaration.ContextSample == string(usage.PerCall) &&
			!expectedContextSampleSource(sample.Runtime, sample.Source) {
			gaps[fmt.Sprintf("runtime %s session %s has unexpected sample source %q", sample.Runtime, sample.Session, sample.Source)] = true
		}
	}
	return sortedContextKeys(gaps)
}

func mergeContextCoverageGaps(groups ...[]string) []string {
	merged := map[string]bool{}
	for _, group := range groups {
		for _, gap := range group {
			merged[gap] = true
		}
	}
	return sortedContextKeys(merged)
}

func contextSessionKey(runtime, session string) string { return runtime + "/" + session }

func sortContextSamples(samples []usage.CallSample) {
	sort.SliceStable(samples, func(i, j int) bool {
		left, right := samples[i], samples[j]
		if !left.At.Equal(right.At) {
			return left.At.Before(right.At)
		}
		if left.Runtime != right.Runtime {
			return left.Runtime < right.Runtime
		}
		if left.Session != right.Session {
			return left.Session < right.Session
		}
		if left.InvocationID != right.InvocationID {
			return left.InvocationID < right.InvocationID
		}
		return left.Ordinal < right.Ordinal
	})
}

func distinctContextRegistrations(rows []usage.CallRegistration) []usage.CallRegistration {
	seen := map[string]bool{}
	var distinct []usage.CallRegistration
	for _, row := range rows {
		key := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%s", row.Runtime, row.Session, row.PID, row.PIDStartedAt, contextTimestampKey(row.FirstSeen))
		if !seen[key] {
			seen[key] = true
			distinct = append(distinct, row)
		}
	}
	return distinct
}

func contextReportResets(registrations []usage.CallRegistration, weekStart, weekEnd time.Time) int {
	type processKey struct {
		runtime         string
		pid, pidStarted int64
	}
	groups := map[processKey]map[string]time.Time{}
	for _, row := range registrations {
		key := processKey{runtime: row.Runtime, pid: row.PID, pidStarted: row.PIDStartedAt}
		if groups[key] == nil {
			groups[key] = map[string]time.Time{}
		}
		first, found := groups[key][row.Session]
		if !found || row.FirstSeen.Before(first) {
			groups[key][row.Session] = row.FirstSeen
		}
	}
	resets := 0
	for _, sessions := range groups {
		type firstSession struct {
			name string
			at   time.Time
		}
		ordered := make([]firstSession, 0, len(sessions))
		for name, at := range sessions {
			ordered = append(ordered, firstSession{name: name, at: at})
		}
		sort.Slice(ordered, func(i, j int) bool {
			if !ordered[i].at.Equal(ordered[j].at) {
				return ordered[i].at.Before(ordered[j].at)
			}
			return ordered[i].name < ordered[j].name
		})
		for index := 1; index < len(ordered); index++ {
			if inContextWeek(ordered[index].at, weekStart, weekEnd) {
				resets++
			}
		}
	}
	return resets
}

func contextReportHandoffs(stateRoot string, weekStart, weekEnd time.Time) (int, error) {
	directory := filepath.Join(stateRoot, "artifacts", "agents", "steward", "consumed")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("cannot list consumed context intents %s: %w", directory, err)
	}
	count := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), ".tmp.json") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return 0, fmt.Errorf("cannot inspect consumed context intent %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return 0, fmt.Errorf("consumed context intent is not a regular file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("cannot read consumed context intent %s: %w", path, err)
		}
		var intent Intent
		if err := json.Unmarshal(data, &intent); err != nil {
			return 0, fmt.Errorf("consumed context intent %s is malformed: %w", path, err)
		}
		if intent.Reason == "seatHandoff" && intent.LaunchStamped && inContextWeek(info.ModTime(), weekStart, weekEnd) {
			count++
		}
	}
	return count, nil
}

func contextReportReferenceMismatches(stateRoot string, weekStart, weekEnd time.Time) ([]string, []string, error) {
	directory := filepath.Join(stateRoot, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("cannot list context report job records %s: %w", directory, err)
	}
	var mismatches []string
	var gaps []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), ".tmp.json") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		record, err := dispatch.ReadRecordObject(path)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot read context report job record %s: %w", path, err)
		}
		job := dispatch.JobRecordOf(record)
		if job.Status() != "failed" ||
			(!strings.HasPrefix(job.ErrorText(), "reference_mismatch:") && !strings.HasPrefix(job.RefusalClass(), "reference_mismatch:")) {
			continue
		}
		jobID := job.JobID()
		if jobID == "" {
			jobID = strings.TrimSuffix(entry.Name(), ".json")
		}
		endedAt, parseErr := time.Parse(time.RFC3339Nano, job.EndedAt())
		if parseErr != nil || endedAt.IsZero() {
			gaps = append(gaps, fmt.Sprintf("reference mismatch job %s has no valid terminal time", jobID))
			continue
		}
		if inContextWeek(endedAt, weekStart, weekEnd) {
			mismatches = append(mismatches, jobID)
		}
	}
	sort.Strings(mismatches)
	sort.Strings(gaps)
	return mismatches, gaps, nil
}

func finishContextReport(report *ContextReport, coverageGaps []string) {
	var failures []string
	if report.Samples == 0 {
		failures = append(failures, "the distinct per-call cohort is empty")
	}
	if report.Samples > 0 && report.P95 >= ContextBoundTokens {
		failures = append(failures, fmt.Sprintf("p95 prompt tokens %d is not below %d", report.P95, ContextBoundTokens))
	}
	if report.Max > ContextCeilingTokens {
		failures = append(failures, fmt.Sprintf("maximum prompt tokens %d is over %d", report.Max, ContextCeilingTokens))
	}
	compactions := 0
	for _, count := range report.Compactions {
		compactions += count
	}
	if compactions > 0 {
		failures = append(failures, fmt.Sprintf("%d compaction marker(s) were recorded", compactions))
	}
	if report.Resets > 0 {
		failures = append(failures, fmt.Sprintf("%d process reset(s) were recorded", report.Resets))
	}
	if len(report.ReferenceMismatches) > 0 {
		failures = append(failures, "reference mismatch jobs were recorded: "+strings.Join(report.ReferenceMismatches, ", "))
	}
	if report.UnassignableSamples > 0 || report.UnassignableMarkers > 0 {
		failures = append(failures, fmt.Sprintf("unassignable evidence includes %d sample(s) and %d marker(s)", report.UnassignableSamples, report.UnassignableMarkers))
	}
	for _, gap := range coverageGaps {
		failures = append(failures, "coverage gap: "+gap)
	}
	report.Failures = failures
	report.Pass = len(failures) == 0
}

func renderContextCalls(samples []usage.CallSample) (string, error) {
	var output bytes.Buffer
	for _, sample := range samples {
		encoded, err := json.Marshal(contextCallExport{
			Runtime: sample.Runtime, Session: sample.Session, InvocationID: sample.InvocationID,
			PromptTokens: sample.PromptTokens, Source: sample.Source,
			CursorOffset: sample.Ordinal, At: sample.At,
		})
		if err != nil {
			return "", fmt.Errorf("cannot render context call %s/%s/%s: %w", sample.Runtime, sample.Session, sample.InvocationID, err)
		}
		output.Write(encoded)
		output.WriteByte('\n')
	}
	return output.String(), nil
}

func renderContextReport(report ContextReport, coverageGaps []string, callsDigest string, generatedAt time.Time) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# Coordinator context week %s\n\n", report.WeekStart.Format("2006-01-02"))
	fmt.Fprintf(&output, "Generated at: %s\n\n", generatedAt.Format(time.RFC3339Nano))
	fmt.Fprintf(&output, "Window: [%s, %s) at UTC midnight\n\n", report.WeekStart.Format(time.RFC3339), report.WeekEnd.Format(time.RFC3339))
	verdict := "FAIL"
	if report.Pass {
		verdict = "PASS"
	}
	fmt.Fprintf(&output, "Verdict: **%s**\n\n", verdict)
	output.WriteString("## Cohort\n\n")
	output.WriteString("| Runtime/session | Distinct samples |\n| --- | ---: |\n")
	cohortKeys := sortedContextKeys(report.Cohort)
	if len(cohortKeys) == 0 {
		output.WriteString("| none | 0 |\n")
	} else {
		for _, key := range cohortKeys {
			fmt.Fprintf(&output, "| %s | %d |\n", contextReportMarkdownText(key), report.Cohort[key])
		}
	}
	fmt.Fprintf(&output, "\nDistinct samples: %d  \nP95 prompt tokens: %d  \nMaximum prompt tokens: %d\n\n", report.Samples, report.P95, report.Max)
	output.WriteString("`cursorOffset` in `calls.jsonl` is the source ordinal recorded on the sample, not a cursor byte offset.\n\n")

	output.WriteString("## Operational signals\n\n")
	output.WriteString("| Runtime | Compactions |\n| --- | ---: |\n")
	compactionKeys := sortedContextKeys(report.Compactions)
	if len(compactionKeys) == 0 {
		output.WriteString("| none | 0 |\n")
	} else {
		for _, runtime := range compactionKeys {
			fmt.Fprintf(&output, "| %s | %d |\n", contextReportMarkdownText(runtime), report.Compactions[runtime])
		}
	}
	fmt.Fprintf(&output, "\nResets: %d  \nHandoffs: %d\n\n", report.Resets, report.Handoffs)
	output.WriteString("A reset is a later distinct registered session for the same runtime, PID, and PID start time; sessions are ordered by first-seen time and then session id.\n\n")
	output.WriteString("Handoffs use the retained consumed record's modification time as a proxy, not a reconstructed dispatch timestamp.\n\n")
	if len(report.ReferenceMismatches) == 0 {
		output.WriteString("Reference mismatch job ids: none\n\n")
	} else {
		escaped := make([]string, len(report.ReferenceMismatches))
		for index, id := range report.ReferenceMismatches {
			escaped[index] = contextReportMarkdownText(id)
		}
		output.WriteString("Reference mismatch job ids: " + strings.Join(escaped, ", ") + "\n\n")
	}

	output.WriteString("## Evidence normalization and coverage\n\n")
	fmt.Fprintf(&output, "Omitted duplicate samples: %d  \nOmitted duplicate markers: %d  \nUnassignable samples: %d  \nUnassignable markers: %d\n\n",
		report.DuplicateSamples, report.DuplicateMarkers, report.UnassignableSamples, report.UnassignableMarkers)
	output.WriteString("Provider call identity uses runtime, session, and invocation id. Claude `line:<decimal>` fallback identity also uses its physical ordinal and timestamp. Source annotations do not create provider calls.\n\n")
	output.WriteString("Coverage gaps:\n\n")
	if len(coverageGaps) == 0 {
		output.WriteString("- none\n\n")
	} else {
		for _, gap := range coverageGaps {
			output.WriteString("- " + contextReportMarkdownText(gap) + "\n")
		}
		output.WriteByte('\n')
	}
	output.WriteString(contextReportCoverage + "\n\n")
	fmt.Fprintf(&output, "Calls file SHA-256: `%s`\n\n", callsDigest)
	output.WriteString("## Verdict clauses\n\n")
	if report.Pass {
		output.WriteString("- PASS: the cohort is nonempty; p95 is strictly below 150000; maximum is at most 200000; compactions, resets, and reference mismatches are zero; coverage and assignment are complete.\n")
	} else {
		for _, failure := range report.Failures {
			output.WriteString("- FAIL: " + contextReportMarkdownText(failure) + "\n")
		}
	}
	return output.String()
}

func contextReportMarkdownText(value string) string {
	return strings.NewReplacer(
		"\\", "\\\\", "\r", " ", "\n", " ", "|", "\\|", "`", "\\`",
		"*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "\\<", ">", "\\>",
	).Replace(value)
}

func sortedContextKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
