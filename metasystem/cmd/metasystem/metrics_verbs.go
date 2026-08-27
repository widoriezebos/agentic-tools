package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
)

var generateMetricsReport = metrics.Report

func reportConcludedGoal(root, id string, warnings io.Writer) {
	result, err := generateMetricsReport(metrics.Options{Root: root, GoalID: id})
	if err == nil {
		return
	}
	target := result.Target
	if target == "" {
		target = metrics.GoalReportTarget(root, id)
	}
	fmt.Fprintf(warnings, "warning: goal %s concluded, but its metrics report could not be written to %s: %v\n", id, target, err)
}

func reportAfterConfirmedDone(code int, root, id string, warnings io.Writer) int {
	if code == 0 {
		reportConcludedGoal(root, id, warnings)
	}
	return code
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
