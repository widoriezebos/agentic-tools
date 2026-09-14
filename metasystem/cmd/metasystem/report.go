package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

func runReportStopPresent(args []string) int {
	flags := flag.NewFlagSet("report stop-present", flag.ContinueOnError)
	root := flags.String("root", "", "resolved metasystem installation")
	input := flags.String("input-file", "", "Stop presentation input JSON")
	output := flags.String("output-file", "", "fresh Stop presentation result JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *input == "" || *output == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem report stop-present --root INSTALLATION --input-file FILE --output-file FILE")
		return 2
	}
	_, err := report.PresentStop(*root, *input, *output, time.Now())
	if err == nil {
		return 0
	}
	var validation report.StopPresentationValidationError
	if errors.As(err, &validation) {
		fmt.Fprintln(os.Stderr, "report stop-present:", err)
		return 2
	}
	fmt.Fprintln(os.Stderr, "report stop-present:", err)
	return 1
}

func runReportStopInput(args []string) int {
	flags := flag.NewFlagSet("report stop-input", flag.ContinueOnError)
	root := flags.String("root", "", "resolved metasystem installation")
	runtime := flags.String("runtime", "", "runtime name")
	session := flags.String("session", "", "runtime session")
	attempt := flags.String("attempt", "", "fresh 16-byte hexadecimal attempt")
	mainID := flags.String("main-id", "", "resolved main id")
	machine := flags.String("machine", "", "resolved machine")
	lineage := flags.String("lineage", "", "resolved lineage")
	claimEpoch := flags.Int64("claim-epoch", 0, "resolved holder claim epoch")
	advisor := flags.Bool("advisor", false, "compose the judgment-free read-only advisor allowance")
	verdict := flags.String("verdict-file", "", "retained turn verdict JSON")
	facts := flags.String("facts-file", "", "frozen judgment facts JSON")
	health := flags.String("health-file", "", "health preview JSON")
	digest := flags.String("digest-file", "", "pending digest bytes")
	digestPrefix := flags.String("digest-cursor-prefix", "", "pending digest cursor prefix")
	receipt := flags.String("receipt-file", "", "receipt result bytes")
	receiptStderr := flags.String("receipt-stderr-file", "", "receipt diagnostic bytes")
	receiptExit := flags.Int("receipt-exit", 0, "receipt command exit")
	arming := flags.String("arming-file", "", "arming result bytes")
	armingStderr := flags.String("arming-stderr-file", "", "arming diagnostic bytes")
	armingExit := flags.Int("arming-exit", 0, "arming command exit")
	notice := flags.String("notice-file", "", "collected non-control notices")
	failure := flags.String("failure-file", "", "collected unavailable diagnostics")
	output := flags.String("output-file", "", "fresh Stop presentation input JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *runtime == "" || *session == "" || *attempt == "" || *output == "" || *advisor == (*verdict != "") || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem report stop-input --root INSTALLATION --runtime RUNTIME --session SESSION --attempt HEX (--verdict-file FILE | --advisor) --output-file FILE [captured files]")
		return 2
	}
	err := report.ComposeStopPresentationInput(report.StopPresentationCollection{
		Root: *root, Runtime: *runtime, Session: *session, Attempt: *attempt, MainID: *mainID,
		Machine: *machine, Lineage: *lineage, ClaimEpoch: *claimEpoch, Advisor: *advisor,
		VerdictFile: *verdict, FactsFile: *facts, HealthFile: *health,
		DigestFile: *digest, DigestCursorPrefix: *digestPrefix,
		ReceiptFile: *receipt, ReceiptStderrFile: *receiptStderr, ReceiptExit: *receiptExit,
		ArmingFile: *arming, ArmingStderrFile: *armingStderr, ArmingExit: *armingExit,
		NoticeFile: *notice, FailureFile: *failure, OutputFile: *output,
	}, time.Now())
	if err == nil {
		return 0
	}
	var validation report.StopPresentationValidationError
	if errors.As(err, &validation) {
		fmt.Fprintln(os.Stderr, "report stop-input:", err)
		return 2
	}
	fmt.Fprintln(os.Stderr, "report stop-input:", err)
	return 1
}

func runReportStopStatus(args []string) int {
	flags := flag.NewFlagSet("report stop-status", flag.ContinueOnError)
	id := flags.String("id", "", "exact immutable Stop report id")
	root := flags.String("root", "", "explicit metasystem installation")
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

// runReportStopBlock prints the stop-hook block decision, leading with any caller
// detail given as the sole positional argument.
func runReportStopBlock(args []string) int {
	flags := flag.NewFlagSet("report stop-block", flag.ContinueOnError)
	systemMessage := flags.String("system-message", "", "non-blocking display line carried beside the refusal")
	refusalRecord := flags.String("refusal-record", "", "per-session external stop-refusal record")
	session := flags.String("session", "", "session recorded for an external stop refusal")
	cause := flags.String("cause", "", "stable external stop-refusal cause")
	remedy := flags.String("remedy", "", "operator remedy for an external stop refusal")
	armingResult := flags.String("arming-result", "", "failed supervision component lines and aggregate carried in the stop notice")
	class := flags.String("class", string(report.StopClassSeatActionable), "stop condition class: infrastructure or seat-actionable")
	boundedIdle := flags.Bool("bounded-idle", false, "render a counted idle-backlog refusal without the open-work block-once preface")
	openWorkRoot := flags.String("open-work-root", "", "checkout root whose open-work lines must be durably marked")
	if flags.Parse(args) != nil {
		return 2
	}
	if *class != string(report.StopClassInfrastructure) && *class != string(report.StopClassSeatActionable) {
		fmt.Fprintln(os.Stderr, "report stop-block: --class must be infrastructure or seat-actionable")
		return 2
	}
	if *openWorkRoot != "" {
		if warning := report.OpenWorkSeenWarning(*openWorkRoot); warning != "" {
			if *systemMessage != "" {
				*systemMessage += "\n"
			}
			*systemMessage += warning
		}
	}
	if *armingResult != "" {
		if *systemMessage != "" {
			*systemMessage += "\n"
		}
		*systemMessage += *armingResult
	}
	*systemMessage = report.BoundSystemMessage(*systemMessage)
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
		block, err = report.StopRefusal(*refusalRecord, *session, *cause, *remedy, detail, *systemMessage, report.StopClass(*class), time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "report stop-block: %v\n", err)
			return 1
		}
	} else {
		if *boundedIdle {
			block = report.BoundedIdleStopBlock(detail)
		} else {
			block = report.StopBlock(detail)
		}
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
