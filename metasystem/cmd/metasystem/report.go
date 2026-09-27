package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
)

func runReportStopStatus(args []string) int {
	flags := flag.NewFlagSet("report stop-status", flag.ContinueOnError)
	id := flags.String("id", "", "exact short Stop report alias or legacy full id")
	root := pathFlag(flags, "root", "", "explicit metasystem installation")
	if flags.Parse(args) != nil {
		return 2
	}
	if *id == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem report stop-status --id ID [--root INSTALLATION]")
		return 2
	}
	if err := report.ValidateStopStatusID(*id); err != nil {
		fmt.Fprintln(os.Stderr, "report stop-status:", err)
		return 2
	}
	data, _, err := report.ReadStopStatus(*root, *id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "report stop-status:", err)
		return 1
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, "report stop-status:", err)
		return 1
	}
	return 0
}

func runReportStopResponse(args []string) int {
	flags := flag.NewFlagSet("report stop-response", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "explicit metasystem installation")
	payloadFile := flags.String("payload-file", "", "file containing one runtime Stop payload")
	runtime := flags.String("runtime", "", "expected report runtime")
	session := flags.String("session", "", "expected report session")
	jsonOutput := flags.Bool("json", false, "print the response record instead of the report")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *payloadFile == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem report stop-response --root INSTALLATION --payload-file FILE [--runtime R] [--session S] [--json]")
		return 2
	}
	resolvedRoot, err := report.StopStatusRoot(*root)
	if err != nil {
		return reportStopResponseError(err)
	}
	payload, err := os.ReadFile(*payloadFile)
	if err != nil {
		return reportStopResponseError(err)
	}
	resolved, err := stopreport.ResolveResponse(resolvedRoot, payload, *runtime, *session)
	if err != nil {
		return reportStopResponseError(err)
	}
	output := resolved.Report
	if *jsonOutput {
		output, err = json.Marshal(resolved.Response)
		if err != nil {
			return reportStopResponseError(err)
		}
		output = append(output, '\n')
	}
	if _, err := os.Stdout.Write(output); err != nil {
		return reportStopResponseError(err)
	}
	return 0
}
func reportStopResponseError(err error) int {
	message := strings.NewReplacer("\r", " ", "\n", " ").Replace(err.Error())
	fmt.Fprintln(os.Stderr, "report stop-response:", message)
	return 1
}

// runReportRunningWork relays `report running-work`: the turn-end active
// clause from report.RunningWorkClause — live
// jobs, missions running elsewhere, gate runs; nothing prints when idle.
func runReportRunningWork(args []string) int {
	flags := flag.NewFlagSet("report running-work", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
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
	repo := pathFlag(flags, "repo", "", "metasystem root")
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

// renderStopBlock renders one Stop refusal: an unrecorded block (bounded idle
// or the open-work form) with an optional system message, or, when the
// request names a record, the recorded refusal that owns the occurrence
// count. An open-work root first marks its open-work lines durably seen.
func renderStopBlock(request hooks.StopBlockRequest, now time.Time) (map[string]any, int, error) {
	class := request.Class
	if class == "" {
		class = string(report.StopClassSeatActionable)
	}
	if class != string(report.StopClassInfrastructure) && class != string(report.StopClassSeatActionable) {
		return nil, 2, fmt.Errorf("report stop-block: the class must be infrastructure or seat-actionable")
	}
	systemMessage := request.SystemMessage
	if request.OpenWorkRoot != "" {
		if warning := report.OpenWorkSeenWarning(request.OpenWorkRoot); warning != "" {
			if systemMessage != "" {
				systemMessage += "\n"
			}
			systemMessage += warning
		}
	}
	if request.ArmingResult != "" {
		if systemMessage != "" {
			systemMessage += "\n"
		}
		systemMessage += request.ArmingResult
	}
	systemMessage = report.BoundSystemMessage(systemMessage)
	if request.RefusalRecord != "" || request.Session != "" || request.Cause != "" || request.Remedy != "" {
		if request.RefusalRecord == "" || request.Session == "" || request.Cause == "" || request.Remedy == "" {
			return nil, 2, fmt.Errorf("report stop-block: the refusal record, session, cause, and remedy must be provided together")
		}
		block, err := report.StopRefusal(request.RefusalRecord, request.Session, request.Cause, request.Remedy, request.Detail, systemMessage, report.StopClass(class), now)
		if err != nil {
			return nil, 1, fmt.Errorf("report stop-block: %v", err)
		}
		return block, 0, nil
	}
	var block map[string]any
	if request.BoundedIdle {
		block = report.BoundedIdleStopBlock(request.Detail)
	} else {
		block = report.StopBlock(request.Detail)
	}
	if systemMessage != "" {
		block["systemMessage"] = systemMessage
	}
	return block, 0, nil
}
