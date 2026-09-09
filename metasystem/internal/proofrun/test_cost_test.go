package proofrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func retainedCostFixture(family, tree string, duration int64, counts RecurringLaunchCounts) RetainedCostReport {
	phases := make([]CostPhase, 0, len(requiredCostPhases))
	for _, name := range requiredCostPhases {
		phases = append(phases, CostPhase{Name: name, DurationMS: 1})
	}
	return RetainedCostReport{SchemaVersion: 1, Family: family, SourceTreeSHA256: tree,
		ConfigurationDigest: strings.Repeat("c", 64), CacheCondition: "warm-private", SelectedScenarios: append([]string(nil), requiredCostScenarios[family]...),
		Status: "passed", WallDurationMS: duration, Phases: phases, LaunchCounts: counts, FaultDetection: map[string]bool{"retained-boundary": true}}
}

func TestRetainedCostProducerBindsRealInputsAndCompleteFacts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(configuration, []byte("fixture=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	phases := map[string]time.Duration{}
	for _, name := range requiredCostPhases {
		phases[name] = time.Millisecond
	}
	started := time.Now().UTC()
	report, err := ProduceRetainedCostReport(CostMeasurement{Family: "adoption", SourceRoot: root,
		Configuration: configuration, CacheCondition: "warm-private", Status: "passed", StartedAt: started,
		FinishedAt: started.Add(20 * time.Millisecond), PhaseDurations: phases,
		LaunchCounts:   RecurringLaunchCounts{Build: 1, Wait: 2, FullCopiedRegistration: 1, CountsComplete: true},
		FaultDetection: map[string]bool{"copy-drift-source": true, "copy-drift-registration": true}})
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceTreeSHA256 == strings.Repeat("0", 64) || !validSHA256(report.SourceTreeSHA256) ||
		!validSHA256(report.ConfigurationDigest) || !equalStrings(report.SelectedScenarios, requiredCostScenarios["adoption"]) ||
		report.WallDurationMS != 20 || len(report.Phases) != len(requiredCostPhases) {
		t.Fatalf("produced report omitted bound measurement facts: %+v", report)
	}
	delete(phases, "publication")
	delete(phases, "verification-publication")
	if _, err := ProduceRetainedCostReport(CostMeasurement{Family: "adoption", SourceRoot: root,
		Configuration: configuration, CacheCondition: "warm-private", Status: "passed", StartedAt: started,
		FinishedAt: started.Add(time.Millisecond), PhaseDurations: phases,
		LaunchCounts: RecurringLaunchCounts{CountsComplete: true}, FaultDetection: map[string]bool{"fault": true}}); err == nil {
		t.Fatal("producer accepted incomplete phase accounting")
	}
}

func TestRetainedCostComparisonRequiresSameCompleteScenariosAndRealSavings(t *testing.T) {
	beforeCounts := RecurringLaunchCounts{Build: 4, Wait: 8, FullCopiedRegistration: 2, CountsComplete: true}
	afterCounts := RecurringLaunchCounts{Build: 2, Wait: 3, FullCopiedRegistration: 1, CountsComplete: true}
	comparison := CostComparison{
		Before: []RetainedCostReport{retainedCostFixture("dispatcher", strings.Repeat("a", 64), 600, beforeCounts), retainedCostFixture("adoption", strings.Repeat("a", 64), 400, beforeCounts)},
		After:  []RetainedCostReport{retainedCostFixture("dispatcher", strings.Repeat("b", 64), 390, afterCounts), retainedCostFixture("adoption", strings.Repeat("b", 64), 290, afterCounts)},
	}
	if verdict := EvaluateCostComparison(comparison); !verdict.Passed || verdict.ReductionPercent != 32 {
		t.Fatalf("valid retained comparison refused: %+v", verdict)
	}
	comparison.After[1].SelectedScenarios = comparison.After[1].SelectedScenarios[:3]
	comparison.After[0].FaultDetection["retained-boundary"] = false
	if verdict := EvaluateCostComparison(comparison); verdict.Passed || len(verdict.Problems) < 2 {
		t.Fatalf("incomplete scenarios or lost faults were accepted: %+v", verdict)
	}
}

func TestFixtureScenarioSelectionKeepsOrdinaryAndComparisonSets(t *testing.T) {
	all, err := FixtureScenarios("dispatcher", "all")
	if err != nil || len(all) != 10 || all[0] != "dispatch" || all[9] != "seat-refused" {
		t.Fatalf("ordinary dispatcher scenarios = %v, %v", all, err)
	}
	selected, err := FixtureScenarios("dispatcher", "comparison")
	if err != nil || !equalStrings(selected, []string{"adapter-selftest", "steward-continuation"}) {
		t.Fatalf("dispatcher comparison scenarios = %v, %v", selected, err)
	}
	if _, err := FixtureScenarios("dispatcher", "partial"); err == nil {
		t.Fatal("unowned partial fixture selection was accepted")
	}
}
