package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
)

func runReportStopPresent(args []string) int {
	flags := flag.NewFlagSet("report stop-present", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "resolved metasystem installation")
	input := flags.String("input-file", "", "Stop presentation input JSON")
	output := flags.String("output-file", "", "fresh Stop presentation result JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *input == "" || *output == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal report stop-present --root INSTALLATION --input-file FILE --output-file FILE")
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
	root := pathFlag(flags, "root", "", "resolved metasystem installation")
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
	completion := flags.String("completion-file", "", "presentation-only completion observation JSON")
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
		fmt.Fprintln(os.Stderr, "usage: metasystem internal report stop-input --root INSTALLATION --runtime RUNTIME --session SESSION --attempt HEX (--verdict-file FILE | --advisor) --output-file FILE [captured files]")
		return 2
	}
	err := report.ComposeStopPresentationInput(report.StopPresentationCollection{
		Root: *root, Runtime: *runtime, Session: *session, Attempt: *attempt, MainID: *mainID,
		Machine: *machine, Lineage: *lineage, ClaimEpoch: *claimEpoch, Advisor: *advisor,
		VerdictFile: *verdict, FactsFile: *facts, CompletionFile: *completion, HealthFile: *health,
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
	id := flags.String("id", "", "exact short Stop report alias or legacy full id")
	root := pathFlag(flags, "root", "", "explicit metasystem installation")
	if flags.Parse(args) != nil {
		return 2
	}
	if *id == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem session report --id ID [--root INSTALLATION]")
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
		fmt.Fprintln(os.Stderr, "usage: metasystem internal report stop-response --root INSTALLATION --payload-file FILE [--runtime R] [--session S] [--json]")
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

// runReportWatchJobs runs the background-job watcher (report.WatchJobs):
// `report watch-jobs --dir D [--dir D]... [--scope P] [--scope-field F]
// [--state FILE] [--stale-min N] [--cap-min N] [--interval SEC]
// [--start-verify-min N] [--baseline] [--once] [--census
// [--supervision-dir DIR] [--heartbeat FILE] [--instance-tag TAG]]
// [--root INSTALLATION]`. Thresholds resolve from flags, then environment,
// then the installation's metasystem.conf (watch.stale-min 20, watch.cap-min
// 180, watch.interval-sec 60).
func runReportWatchJobs(args []string) int {
	flags := flag.NewFlagSet("report watch-jobs", flag.ContinueOnError)
	var dirs []string
	flags.Func("dir", "job directory or glob pattern (repeatable)", func(value string) error {
		dirs = append(dirs, value)
		return nil
	})
	root := pathFlag(flags, "root", "", "installation whose configuration and census the watcher uses (default: this engine's)")
	scope := flags.String("scope", "", "only report jobs whose workspace field is this path or below")
	scopeField := flags.String("scope-field", "workspaceRoot", "record field naming the job's workspace")
	state := flags.String("state", "", "where already-reported jobs are remembered (default: derived from --dir and --scope)")
	staleMin := flags.String("stale-min", "", "minutes without a state change before STALE")
	capMin := flags.String("cap-min", "", "minutes without a state change before CAPPED")
	interval := flags.String("interval", "", "poll interval seconds")
	startVerifyMin := flags.Int64("start-verify-min", 5, "minutes a job may sit queued before NEVER-STARTED (0 disables)")
	baseline := flags.Bool("baseline", false, "record every current terminal job as reported and exit")
	once := flags.Bool("once", false, "make a single pass and exit")
	censusEnabled := flags.Bool("census", false, "run a supervision census over --scope before every pass")
	supervisionDir := flags.String("supervision-dir", "", "census and heartbeat directory (with --census)")
	heartbeatFile := flags.String("heartbeat", "", "watcher heartbeat file (with --census)")
	instanceTag := flags.String("instance-tag", "", "watcher instance tag (with --census)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || len(dirs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal report watch-jobs --dir DIR [--dir DIR]... [--scope PATH] [--state FILE] [--stale-min N] [--cap-min N] [--interval SEC] [--start-verify-min N] [--baseline] [--once] [--census]")
		return 2
	}
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	installation := *root
	if installation == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "watch-jobs:", err)
			return 1
		}
		installation = filepath.Dir(filepath.Dir(exe))
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	resolve := func(key, flagName, value, fallback string) (int64, bool) {
		resolved, code, err := config.Get(config.GetParams{Key: key, Flag: value, FlagSet: set[flagName],
			Default: fallback, DefaultSet: true, ConfPath: confPath})
		if err != nil || code != 0 {
			return 0, false
		}
		number, err := strconv.ParseInt(resolved, 10, 64)
		return number, err == nil
	}
	stale, okStale := resolve("watch.stale-min", "stale-min", *staleMin, "20")
	capped, okCap := resolve("watch.cap-min", "cap-min", *capMin, "180")
	intervalSec, okInterval := resolve("watch.interval-sec", "interval", *interval, "60")
	if !okStale || !okCap || !okInterval {
		fmt.Fprintln(os.Stderr, "watch-jobs: --stale-min, --cap-min and --interval must be integers")
		return 2
	}
	sleepEvery := time.Duration(intervalSec) * time.Second
	if override := os.Getenv("METASYSTEM_CENSUS_INTERVAL_MS"); override != "" {
		milliseconds, err := strconv.ParseInt(override, 10, 64)
		if err != nil || milliseconds < 1 {
			fmt.Fprintln(os.Stderr, "METASYSTEM_CENSUS_INTERVAL_MS must be a positive integer")
			return 2
		}
		sleepEvery = time.Duration(milliseconds) * time.Millisecond
	}
	watchScope := strings.TrimSuffix(*scope, "/")
	options := report.WatchJobsOptions{
		Dirs: dirs, Scope: watchScope, ScopeField: *scopeField, StateFile: *state, TempDir: os.TempDir(),
		StaleMin: stale, CapMin: capped, StartVerifyMin: *startVerifyMin, Interval: sleepEvery,
		Baseline: *baseline, Once: *once, Fingerprint: watchJobsFingerprint(),
		Now: time.Now, Out: os.Stdout, Err: os.Stderr,
	}
	if *censusEnabled {
		if watchScope == "" {
			fmt.Fprintln(os.Stderr, "--census requires --scope")
			return 2
		}
		top, err := stateroot.RepositoryTop(watchScope)
		if err != nil {
			fmt.Fprintln(os.Stderr, "--census scope is not a git repository")
			return 2
		}
		if watchScope, err = canonicalPath(top); err != nil {
			fmt.Fprintln(os.Stderr, "--census scope is not a git repository")
			return 2
		}
		options.Scope = watchScope
		dir := *supervisionDir
		if dir == "" {
			dir = filepath.Join(installation, "artifacts", "agents", "supervision")
		}
		heartbeat := *heartbeatFile
		if heartbeat == "" {
			heartbeat = filepath.Join(dir, "watcher.heartbeat.json")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "watch-jobs:", err)
			return 1
		}
		tag := *instanceTag
		if tag == "" {
			tag = fmt.Sprintf("watch-jobs-%d", os.Getpid())
		}
		maxBytes, _ := resolve("census.log-max-bytes", "", "", "1048576")
		options.CensusLog = filepath.Join(dir, "census.log")
		options.CensusLogMaxBytes = maxBytes
		options.Census = func() (string, error) {
			var output strings.Builder
			err := superviseWatcherPass(installation, watchScope, dir, heartbeat, tag, int(intervalSec), 0, &output)
			if err != nil {
				fmt.Fprintln(&output, err)
			}
			return output.String(), err
		}
	}
	options.Heartbeat = func() string {
		heartbeat, _ := deepestSuiteHeartbeat(options.Scope, time.Now())
		return heartbeat
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	options.Stop = func() bool { return ctx.Err() != nil }
	options.Sleep = func(wait time.Duration) {
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
	return report.WatchJobs(options)
}

// watchJobsFingerprint names the code a long-lived watcher is running: the
// first twelve hex digits of this engine's SHA-256.
func watchJobsFingerprint() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return "unknown"
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])[:12]
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
		fmt.Fprintln(os.Stderr, "usage: metasystem internal report open-work --repo R")
		return 2
	}
	for _, line := range report.OpenWork(*repo) {
		fmt.Println(line)
	}
	return 0
}
