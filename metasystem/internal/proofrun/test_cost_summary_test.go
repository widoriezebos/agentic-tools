package proofrun

import "testing"

func TestSummarizeTestResultCostSeparatesObservedNativeAndReusedWork(t *testing.T) {
	t.Parallel()
	result := TestResult{AttemptID: "attempt-1", CandidateTree: "tree-1",
		LaunchCounts: LaunchCounts{Build: 1, Test: 1, ReusedTest: 2, CountsComplete: true},
		Cost:         TestCost{ActualDurationMS: 1000, PreparationDurationMS: 25, ExecutionDurationMS: 900},
		Groups: []GroupResult{
			{ID: "build", Kind: "build", ExecutionIdentity: "build-identity", Status: "passed", NativeLaunched: true, DurationMS: 700},
			{ID: "unit", Kind: "unit", ExecutionIdentity: "unit-identity", Status: "failed", NativeLaunched: true, DurationMS: 100},
			{ID: "deep", Kind: "unit", ExecutionIdentity: "deep-identity", Status: "reused", ReuseAttempt: "prior-1", DurationMS: 800},
			{ID: "older", Kind: "unit", ExecutionIdentity: "older-identity", Status: "reused", ReuseAttempt: "prior-2"},
			{ID: "blocked", Kind: "unit", Status: "blocked"},
		}}
	report, err := SummarizeTestResultCost(result, 500)
	if err != nil {
		t.Fatal(err)
	}
	if report.NativeBuildGroups != 1 || report.NativeTestGroups != 1 || report.ReusedBuildGroups != 0 ||
		report.ReusedTestGroups != 2 || report.NativeExpensiveGroups != 1 || report.ReusedExpensiveGroups != 1 ||
		report.UnknownExpenseGroups != 1 || report.UnexecutedGroups != 1 {
		t.Fatalf("group cost counts misclassified: %+v", report)
	}
	if report.NativeObservedGroupDurationMS == nil || *report.NativeObservedGroupDurationMS != 800 ||
		report.ReusedSourceGroupDurationMS != nil || report.WaitDurationMS != nil || report.PublicationDurationMS != nil {
		t.Fatalf("unknown or overlapping durations were presented as measured wall time: %+v", report)
	}
	if report.MetadataPreparationDurationMS == nil || *report.MetadataPreparationDurationMS != 25 ||
		report.WorkerDurationMS == nil || *report.WorkerDurationMS != 900 {
		t.Fatalf("measured metadata and worker phases were lost: %+v", report)
	}
	if report.Groups[2].ExecutionIdentity != "deep-identity" || report.Groups[2].ReuseAttempt != "prior-1" ||
		report.Groups[2].ObservedDurationMS == nil || *report.Groups[2].ObservedDurationMS != 800 ||
		report.Groups[3].ObservedDurationMS != nil || report.Groups[3].Expensive != nil {
		t.Fatalf("source identity or missing duration lost: %+v", report.Groups)
	}
}

func TestSummarizeTestResultCostRequiresARealThresholdAndCompleteLaunchCounts(t *testing.T) {
	t.Parallel()
	result := TestResult{LaunchCounts: LaunchCounts{CountsComplete: true}}
	if _, err := SummarizeTestResultCost(result, 0); err == nil {
		t.Fatal("zero-cost threshold was accepted")
	}
	result.LaunchCounts.CountsComplete = false
	if _, err := SummarizeTestResultCost(result, 1); err == nil {
		t.Fatal("incomplete launch accounting was accepted")
	}
}

func TestSummarizeTestResultCostReportsMeasuredQueueWait(t *testing.T) {
	t.Parallel()
	result := TestResult{LaunchCounts: LaunchCounts{CountsComplete: true},
		Cost: TestCost{QueueDurationMS: 5000}}
	report, err := SummarizeTestResultCost(result, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if report.WaitDurationMS == nil || *report.WaitDurationMS != 5000 {
		t.Fatalf("measured queue wait was lost: %+v", report)
	}
	result.Cost.QueueDurationMS = 0
	report, err = SummarizeTestResultCost(result, 1000)
	if err != nil || report.WaitDurationMS != nil {
		t.Fatalf("unmeasured queue wait was invented: %+v, %v", report, err)
	}
}
