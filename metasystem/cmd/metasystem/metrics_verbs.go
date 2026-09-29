package main

import (
	"fmt"
	"io"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
)

var generateMetricsReport = metrics.Report

func reportAfterConfirmedDone(code int, root, id string, warnings io.Writer) int {
	return reportAfterConfirmedDoneWithReporter(code, root, id, warnings, generateMetricsReport)
}

func reportAfterConfirmedDoneWithReporter(code int, root, id string, warnings io.Writer, reporter func(metrics.Options) (metrics.Result, error)) int {
	if code == 0 {
		if err := concludedGoalMetrics(root, id, reporter); err != nil {
			fmt.Fprintf(warnings, "warning: %v\n", err)
		}
	}
	return code
}

// concludedGoalMetrics writes a concluded goal's metrics report. A failure is
// returned as a sentence naming the report's target; the conclusion itself
// has already landed.
func concludedGoalMetrics(root, id string, reporter func(metrics.Options) (metrics.Result, error)) error {
	if reporter == nil {
		reporter = generateMetricsReport
	}
	result, err := reporter(metrics.Options{Root: root, GoalID: id})
	if err == nil {
		return nil
	}
	target := result.Target
	if target == "" {
		target = metrics.GoalReportTarget(root, id)
	}
	return fmt.Errorf("goal %s concluded, but its metrics report could not be written to %s: %v", id, target, err)
}
