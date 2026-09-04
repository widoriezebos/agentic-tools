package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

// runReportStopBlock prints the stop-hook block decision, appending any caller
// detail given as the sole positional argument.
func runReportStopBlock(args []string) int {
	flags := flag.NewFlagSet("report stop-block", flag.ContinueOnError)
	systemMessage := flags.String("system-message", "", "non-blocking display line carried beside the refusal")
	refusalRecord := flags.String("refusal-record", "", "per-session external stop-refusal record")
	session := flags.String("session", "", "session recorded for an external stop refusal")
	cause := flags.String("cause", "", "stable external stop-refusal cause")
	remedy := flags.String("remedy", "", "operator remedy for an external stop refusal")
	if flags.Parse(args) != nil {
		return 2
	}
	detail := ""
	if flags.NArg() > 0 {
		detail = flags.Arg(0)
	}
	var block map[string]any
	if *refusalRecord != "" || *session != "" || *cause != "" || *remedy != "" {
		if *refusalRecord == "" || *session == "" || *cause == "" || *remedy == "" {
			fmt.Fprintln(os.Stderr, "report stop-block: --refusal-record, --session, --cause, and --remedy must be provided together")
			return 2
		}
		var err error
		block, err = report.StopRefusal(*refusalRecord, *session, *cause, *remedy, detail, *systemMessage, time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "report stop-block: %v\n", err)
			return 1
		}
	} else {
		block = report.StopBlock(detail)
		if *systemMessage != "" {
			block["systemMessage"] = *systemMessage
		}
	}
	encoded, _ := json.Marshal(block)
	fmt.Println(string(encoded))
	return 0
}

// runReportRunningWork relays `report running-work`: the turn-end active
// clause from report.RunningWorkClause — live
// jobs, missions running elsewhere, gate runs; nothing prints when idle.
func runReportRunningWork(args []string) int {
	flags := flag.NewFlagSet("report running-work", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "report running-work: --repo is required")
		return 2
	}
	if clause := report.RunningWorkClause(*repo); clause != "" {
		fmt.Println(clause)
	}
	return 0
}

// runReportScanJobs relays `report scan-jobs`: one watcher classification
// pass from report.ScanJobs. Threshold misuse is a usage error;
// anything else exits 1.
func runReportScanJobs(args []string) int {
	flags := flag.NewFlagSet("report scan-jobs", flag.ContinueOnError)
	var dirs []string
	flags.Func("dir", "job directory or glob pattern (repeatable)", func(value string) error {
		dirs = append(dirs, value)
		return nil
	})
	state := flags.String("state", "", "seen-state file")
	running := flags.String("running", "", "running-set scratch file")
	scope := flags.String("scope", "", "scope root (optional)")
	scopeField := flags.String("scope-field", "workspaceRoot", "record field naming the job's workspace")
	staleMin := flags.Int64("stale-min", 0, "minutes before a live record is STALE")
	capMin := flags.Int64("cap-min", 0, "minutes before a job is CAPPED")
	startVerifyMin := flags.Int64("start-verify-min", 0, "minutes before a queued job is NEVER-STARTED")
	baseline := flags.Bool("baseline", false, "adopt history without reporting")
	if flags.Parse(args) != nil {
		return 2
	}
	if len(dirs) == 0 || *state == "" || *running == "" {
		fmt.Fprintln(os.Stderr, "report scan-jobs: --dir, --state, and --running are required")
		return 2
	}
	if *staleMin < 1 || *capMin < 1 || *startVerifyMin < 0 {
		fmt.Fprintln(os.Stderr, "report scan-jobs: --stale-min and --cap-min must be positive integers, --start-verify-min non-negative")
		return 2
	}
	err := report.ScanJobs(report.ScanJobsParams{
		Dirs: dirs, StateFile: *state, RunningFile: *running,
		Scope: *scope, ScopeField: *scopeField,
		StaleMin: *staleMin, CapMin: *capMin, StartVerifyMin: *startVerifyMin,
		Baseline: *baseline, Now: time.Now(),
	}, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runReportOpenWork prints STALE-PLAN and OPEN-WORK lines for a checkout's
// plans.
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
