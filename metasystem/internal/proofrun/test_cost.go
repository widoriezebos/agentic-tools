package proofrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

const RetainedCostReportSchemaVersion = 1

var requiredCostPhases = []string{
	"planning-discovery", "input-hashing", "candidate-preparation", "setup", "build", "test", "verification-publication",
}

var requiredCostScenarios = map[string][]string{
	"dispatcher": {"adapter-selftest", "steward-continuation"},
	"adoption":   {"filled-target-delivery", "copied-registration-setup", "copied-registration-positive", "copy-drift-source", "copy-drift-registration"},
}

var fixtureScenarioCatalog = map[string][]string{
	"dispatcher": {"dispatch", "mission-runner", "adapter-selftest", "steward-continuation", "brain-delegate-refuses", "brain-cancel-close-reap-refuse", "brain-breach-stop-exempt", "brain-absent-node-proceeds", "brain-fence-helper-fails", "seat-refused"},
}

// FixtureScenarios returns the ordinary complete scenario catalog or the
// accepted recurring-cost selection. The Go owner keeps comparison calls from
// silently dropping or adding a scenario while the shell remains the real
// process-owning parent.
func FixtureScenarios(family, selection string) ([]string, error) {
	var selected []string
	switch selection {
	case "all":
		selected = fixtureScenarioCatalog[family]
	case "comparison":
		selected = requiredCostScenarios[family]
	default:
		return nil, fmt.Errorf("unknown fixture selection %q", selection)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("unknown fixture family %q", family)
	}
	return append([]string(nil), selected...), nil
}

type CostPhase struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"durationMs"`
}

type RecurringLaunchCounts struct {
	Build                  int  `json:"build"`
	Wait                   int  `json:"wait"`
	FullCopiedRegistration int  `json:"fullCopiedRegistration"`
	CountsComplete         bool `json:"countsComplete"`
}

type RetainedCostReport struct {
	SchemaVersion       int                   `json:"schemaVersion"`
	Family              string                `json:"family"`
	SourceTreeSHA256    string                `json:"sourceTreeSha256"`
	ConfigurationDigest string                `json:"configurationDigest"`
	CacheCondition      string                `json:"cacheCondition"`
	SelectedScenarios   []string              `json:"selectedScenarios"`
	Status              string                `json:"status"`
	WallDurationMS      int64                 `json:"wallDurationMs"`
	Phases              []CostPhase           `json:"phases"`
	LaunchCounts        RecurringLaunchCounts `json:"launchCounts"`
	FaultDetection      map[string]bool       `json:"faultDetection"`
}

type CostComparison struct {
	Before []RetainedCostReport `json:"before"`
	After  []RetainedCostReport `json:"after"`
}

type CostComparisonVerdict struct {
	Passed           bool     `json:"passed"`
	BeforeDurationMS int64    `json:"beforeDurationMs"`
	AfterDurationMS  int64    `json:"afterDurationMs"`
	ReductionPercent float64  `json:"reductionPercent"`
	Problems         []string `json:"problems"`
}

// CostMeasurement contains facts captured by the real process-owning fixture
// parent. The producer, rather than shell JSON assembly, binds those facts to
// the full source/configuration inputs and the Go-owned scenario selection.
type CostMeasurement struct {
	Family         string
	SourceRoot     string
	Configuration  string
	CacheCondition string
	Status         string
	StartedAt      time.Time
	FinishedAt     time.Time
	PhaseDurations map[string]time.Duration
	LaunchCounts   RecurringLaunchCounts
	FaultDetection map[string]bool
}

// ProduceRetainedCostReport constructs one evaluator-ready report from a real
// whole-command measurement. It computes source and configuration identities
// itself and refuses missing phase, count, scenario, or retained-fault facts.
func ProduceRetainedCostReport(measurement CostMeasurement) (RetainedCostReport, error) {
	selected, err := FixtureScenarios(measurement.Family, "comparison")
	if err != nil {
		return RetainedCostReport{}, err
	}
	if measurement.SourceRoot == "" || measurement.Configuration == "" || measurement.CacheCondition == "" ||
		measurement.Status == "" || measurement.StartedAt.IsZero() || !measurement.FinishedAt.After(measurement.StartedAt) {
		return RetainedCostReport{}, fmt.Errorf("cost measurement is missing source, configuration, cache, status, or wall-clock facts")
	}
	sourceDigest, err := FullDigest(measurement.SourceRoot)
	if err != nil {
		return RetainedCostReport{}, fmt.Errorf("digest measured source: %w", err)
	}
	configuration, err := os.ReadFile(measurement.Configuration)
	if err != nil {
		return RetainedCostReport{}, fmt.Errorf("read measured configuration: %w", err)
	}
	configurationDigest := fmt.Sprintf("%x", sha256.Sum256(configuration))
	phases := make([]CostPhase, 0, len(requiredCostPhases))
	for _, name := range requiredCostPhases {
		duration, present := measurement.PhaseDurations[name]
		if !present || duration < 0 {
			return RetainedCostReport{}, fmt.Errorf("cost measurement is missing valid phase %s", name)
		}
		phases = append(phases, CostPhase{Name: name, DurationMS: duration.Milliseconds()})
	}
	if !measurement.LaunchCounts.CountsComplete {
		return RetainedCostReport{}, fmt.Errorf("cost measurement launch counts are incomplete")
	}
	if len(measurement.FaultDetection) == 0 {
		return RetainedCostReport{}, fmt.Errorf("cost measurement has no retained fault results")
	}
	for fault, detected := range measurement.FaultDetection {
		if fault == "" || !detected {
			return RetainedCostReport{}, fmt.Errorf("cost measurement has an absent or undetected retained fault")
		}
	}
	return RetainedCostReport{SchemaVersion: RetainedCostReportSchemaVersion, Family: measurement.Family,
		SourceTreeSHA256: sourceDigest, ConfigurationDigest: configurationDigest, CacheCondition: measurement.CacheCondition,
		SelectedScenarios: selected, Status: measurement.Status, WallDurationMS: measurement.FinishedAt.Sub(measurement.StartedAt).Milliseconds(),
		Phases: phases, LaunchCounts: measurement.LaunchCounts, FaultDetection: cloneFaultDetection(measurement.FaultDetection)}, nil
}

func cloneFaultDetection(values map[string]bool) map[string]bool {
	result := make(map[string]bool, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func ReadCostComparison(path string) (CostComparison, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CostComparison{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var comparison CostComparison
	if err := decoder.Decode(&comparison); err != nil {
		return CostComparison{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return CostComparison{}, fmt.Errorf("cost comparison has trailing JSON")
	}
	return comparison, nil
}

func EvaluateCostComparison(comparison CostComparison) CostComparisonVerdict {
	verdict := CostComparisonVerdict{}
	before, beforeProblems := costReportsByFamily(comparison.Before, "before")
	after, afterProblems := costReportsByFamily(comparison.After, "after")
	verdict.Problems = append(verdict.Problems, beforeProblems...)
	verdict.Problems = append(verdict.Problems, afterProblems...)
	aggregateBefore, aggregateAfter := RecurringLaunchCounts{}, RecurringLaunchCounts{}
	for _, family := range []string{"dispatcher", "adoption"} {
		left, leftOK := before[family]
		right, rightOK := after[family]
		if !leftOK || !rightOK {
			verdict.Problems = append(verdict.Problems, "comparison requires one before and after report for "+family)
			continue
		}
		verdict.BeforeDurationMS += left.WallDurationMS
		verdict.AfterDurationMS += right.WallDurationMS
		aggregateBefore.Build += left.LaunchCounts.Build
		aggregateBefore.Wait += left.LaunchCounts.Wait
		aggregateBefore.FullCopiedRegistration += left.LaunchCounts.FullCopiedRegistration
		aggregateAfter.Build += right.LaunchCounts.Build
		aggregateAfter.Wait += right.LaunchCounts.Wait
		aggregateAfter.FullCopiedRegistration += right.LaunchCounts.FullCopiedRegistration
		if left.ConfigurationDigest != right.ConfigurationDigest || left.CacheCondition != right.CacheCondition ||
			!equalStrings(left.SelectedScenarios, right.SelectedScenarios) {
			verdict.Problems = append(verdict.Problems, family+" before/after inputs are not comparable")
		}
		if left.SourceTreeSHA256 == right.SourceTreeSHA256 {
			verdict.Problems = append(verdict.Problems, family+" before/after reports name the same source tree")
		}
		if right.WallDurationMS*100 > left.WallDurationMS*110 {
			verdict.Problems = append(verdict.Problems, family+" became more than ten percent slower")
		}
		if !equalFaults(left.FaultDetection, right.FaultDetection) {
			verdict.Problems = append(verdict.Problems, family+" fault detection changed")
		}
	}
	if verdict.BeforeDurationMS > 0 {
		verdict.ReductionPercent = float64(verdict.BeforeDurationMS-verdict.AfterDurationMS) * 100 / float64(verdict.BeforeDurationMS)
	}
	if verdict.BeforeDurationMS == 0 || verdict.AfterDurationMS*100 > verdict.BeforeDurationMS*70 {
		verdict.Problems = append(verdict.Problems, "combined recurring wall time did not fall by at least thirty percent")
	}
	if aggregateAfter.Build >= aggregateBefore.Build || aggregateAfter.Wait >= aggregateBefore.Wait ||
		aggregateAfter.FullCopiedRegistration >= aggregateBefore.FullCopiedRegistration {
		verdict.Problems = append(verdict.Problems, "build, wait, and full copied-registration launch counts did not each fall")
	}
	sort.Strings(verdict.Problems)
	verdict.Passed = len(verdict.Problems) == 0
	return verdict
}

func costReportsByFamily(reports []RetainedCostReport, side string) (map[string]RetainedCostReport, []string) {
	result := map[string]RetainedCostReport{}
	var problems []string
	for _, report := range reports {
		if report.SchemaVersion != RetainedCostReportSchemaVersion || requiredCostScenarios[report.Family] == nil || result[report.Family].Family != "" {
			problems = append(problems, side+" cost report has an unknown or duplicate family")
			continue
		}
		result[report.Family] = report
		if report.Status != "passed" || report.WallDurationMS <= 0 || !validSHA256(report.SourceTreeSHA256) ||
			!validSHA256(report.ConfigurationDigest) || report.CacheCondition == "" || !report.LaunchCounts.CountsComplete {
			problems = append(problems, side+" "+report.Family+" report is incomplete or not passed")
		}
		if !equalStrings(report.SelectedScenarios, requiredCostScenarios[report.Family]) {
			problems = append(problems, side+" "+report.Family+" report does not contain the exact selected scenarios")
		}
		phaseSeen := map[string]bool{}
		for _, phase := range report.Phases {
			if phase.Name == "" || phase.DurationMS < 0 || phaseSeen[phase.Name] {
				problems = append(problems, side+" "+report.Family+" phase accounting is invalid")
			}
			phaseSeen[phase.Name] = true
		}
		for _, phase := range requiredCostPhases {
			if !phaseSeen[phase] {
				problems = append(problems, side+" "+report.Family+" is missing phase "+phase)
			}
		}
		if len(report.FaultDetection) == 0 {
			problems = append(problems, side+" "+report.Family+" has no retained fault-detection map")
		}
		for fault, detected := range report.FaultDetection {
			if fault == "" || !detected {
				problems = append(problems, side+" "+report.Family+" has an undetected retained fault")
			}
		}
	}
	return result, problems
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalFaults(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
