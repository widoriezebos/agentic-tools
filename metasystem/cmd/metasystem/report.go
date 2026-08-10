package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

// runReportStopBlock ports stop-block.py: print the stop-hook block decision,
// appending any caller detail given as the sole positional argument.
func runReportStopBlock(args []string) int {
	detail := ""
	if len(args) > 0 {
		detail = args[0]
	}
	encoded, _ := json.Marshal(report.StopBlock(detail))
	fmt.Println(string(encoded))
	return 0
}

// runReportOpenWork ports open-work.py: print STALE-PLAN and OPEN-WORK lines
// for a checkout's plans.
func runReportOpenWork(args []string) int {
	flags := flag.NewFlagSet("report open-work", flag.ContinueOnError)
	repo := flags.String("repo", "", "metasystem root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem report open-work --repo R")
		return 2
	}
	for _, line := range report.OpenWork(*repo) {
		fmt.Println(line)
	}
	return 0
}
