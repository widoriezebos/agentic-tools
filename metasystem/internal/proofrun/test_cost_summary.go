package proofrun

import "fmt"

// TestCostSummary is a read-only projection of one retained test result. A
// reused group's duration describes its original producer, not time saved by
// this run. Nil durations mean the retained result did not measure that fact.
type TestCostSummary struct {
	SchemaVersion                 int                    `json:"schemaVersion"`
	AttemptID                     string                 `json:"attemptId"`
	CandidateTree                 string                 `json:"candidateTree"`
	ExpensiveThresholdMS          int64                  `json:"expensiveThresholdMs"`
	LaunchCounts                  LaunchCounts           `json:"launchCounts"`
	TotalDurationMS               *int64                 `json:"totalDurationMs"`
	MetadataPreparationDurationMS *int64                 `json:"metadataPreparationDurationMs"`
	WorkerDurationMS              *int64                 `json:"workerDurationMs"`
	PublicationDurationMS         *int64                 `json:"publicationDurationMs"`
	WaitDurationMS                *int64                 `json:"waitDurationMs"`
	NativeBuildGroups             int                    `json:"nativeBuildGroups"`
	NativeTestGroups              int                    `json:"nativeTestGroups"`
	ReusedBuildGroups             int                    `json:"reusedBuildGroups"`
	ReusedTestGroups              int                    `json:"reusedTestGroups"`
	NativeExpensiveGroups         int                    `json:"nativeExpensiveGroups"`
	ReusedExpensiveGroups         int                    `json:"reusedExpensiveGroups"`
	UnknownExpenseGroups          int                    `json:"unknownExpenseGroups"`
	UnexecutedGroups              int                    `json:"unexecutedGroups"`
	NativeObservedGroupDurationMS *int64                 `json:"nativeObservedGroupDurationMs"`
	ReusedSourceGroupDurationMS   *int64                 `json:"reusedSourceGroupDurationMs"`
	Groups                        []TestGroupCostSummary `json:"groups"`
}

type TestGroupCostSummary struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"`
	ExecutionIdentity  string `json:"executionIdentity,omitempty"`
	Status             string `json:"status"`
	NativeLaunched     bool   `json:"nativeLaunched"`
	ReuseAttempt       string `json:"reuseAttempt,omitempty"`
	ObservedDurationMS *int64 `json:"observedDurationMs"`
	Expensive          *bool  `json:"expensive"`
}

// SummarizeTestResultCost reports only observed work. Group durations may
// overlap under parallel execution, so their sum is never labeled wall time.
func SummarizeTestResultCost(result TestResult, expensiveMS int64) (TestCostSummary, error) {
	if expensiveMS <= 0 {
		return TestCostSummary{}, fmt.Errorf("expensive group threshold must be positive")
	}
	if !result.LaunchCounts.CountsComplete {
		return TestCostSummary{}, fmt.Errorf("test result launch counts are incomplete")
	}
	report := TestCostSummary{SchemaVersion: 1, AttemptID: result.AttemptID, CandidateTree: result.CandidateTree,
		ExpensiveThresholdMS: expensiveMS, LaunchCounts: result.LaunchCounts,
		TotalDurationMS:               positiveDuration(result.Cost.ActualDurationMS),
		MetadataPreparationDurationMS: positiveDuration(result.Cost.PreparationDurationMS),
		WorkerDurationMS:              positiveDuration(result.Cost.ExecutionDurationMS),
		PublicationDurationMS:         positiveDuration(result.Cost.PublicationDurationMS),
		WaitDurationMS:                positiveDuration(result.Cost.QueueDurationMS),
		Groups:                        make([]TestGroupCostSummary, 0, len(result.Groups))}
	var nativeDuration, reusedDuration int64
	var nativeComplete, reusedComplete = true, true
	for _, group := range result.Groups {
		row := TestGroupCostSummary{ID: group.ID, Kind: group.Kind, ExecutionIdentity: group.ExecutionIdentity,
			Status: group.Status, NativeLaunched: group.NativeLaunched, ReuseAttempt: group.ReuseAttempt,
			ObservedDurationMS: positiveDuration(group.DurationMS)}
		if group.NativeLaunched || group.Status == "reused" {
			if row.ObservedDurationMS == nil {
				report.UnknownExpenseGroups++
			} else {
				expensive := *row.ObservedDurationMS >= expensiveMS
				row.Expensive = &expensive
				if expensive {
					if group.NativeLaunched {
						report.NativeExpensiveGroups++
					} else {
						report.ReusedExpensiveGroups++
					}
				}
			}
			if group.NativeLaunched {
				if group.Kind == "build" {
					report.NativeBuildGroups++
				} else {
					report.NativeTestGroups++
				}
				if row.ObservedDurationMS == nil {
					nativeComplete = false
				} else {
					nativeDuration += *row.ObservedDurationMS
				}
			} else {
				if group.Kind == "build" {
					report.ReusedBuildGroups++
				} else {
					report.ReusedTestGroups++
				}
				if row.ObservedDurationMS == nil {
					reusedComplete = false
				} else {
					reusedDuration += *row.ObservedDurationMS
				}
			}
		} else {
			report.UnexecutedGroups++
		}
		report.Groups = append(report.Groups, row)
	}
	if nativeComplete && report.NativeBuildGroups+report.NativeTestGroups > 0 {
		report.NativeObservedGroupDurationMS = &nativeDuration
	}
	if reusedComplete && report.ReusedBuildGroups+report.ReusedTestGroups > 0 {
		report.ReusedSourceGroupDurationMS = &reusedDuration
	}
	return report, nil
}

func positiveDuration(duration int64) *int64 {
	if duration <= 0 {
		return nil
	}
	return &duration
}
