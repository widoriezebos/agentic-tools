package main

import (
	"flag"
	"fmt"
	"io"
	"os"

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

func runMetricsReport(args []string) int {
	flags := flag.NewFlagSet("metrics report", flag.ContinueOnError)
	periodEnd := flags.String("period-end", "", "report instant and event-window end as ISO 8601; fractional seconds are truncated")
	since := flags.String("since", "", "event-window start as ISO 8601; fractional seconds are truncated")
	goalID := flags.String("goal", "", "write a whole-lifecycle report for one goal")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem metrics report [--period-end <iso8601>] [--since <iso8601>] [--goal <id>]")
		return 2
	}
	result, err := metrics.Report(metrics.Options{Root: ".", PeriodEnd: *periodEnd, Since: *since, GoalID: *goalID})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, path := range result.Paths {
		fmt.Println("metrics report written: " + path)
	}
	return 0
}
