package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func runReportStopStatus(args []string) int {
	flags := flag.NewFlagSet("report stop-status", flag.ContinueOnError)
	id := flags.String("id", "", "exact short Stop report alias or legacy full id")
	root := pathFlag(flags, "root", "", "explicit metasystem installation")
	if flags.Parse(args) != nil {
		return 2
	}
	if *id == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem session status --id ID [--root INSTALLATION]")
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

// runReportWatchJobs runs the background-job watcher (report.WatchJobs):
// `report watch-jobs --dir D [--dir D]... [--scope P] [--scope-field F]
// [--state FILE] [--stale-min N] [--cap-min N] [--interval SEC]
// [--start-verify-min N] [--baseline] [--once] [--census
// [--supervision-dir DIR] [--heartbeat FILE] [--instance-tag TAG]]
// [--root INSTALLATION]`. Thresholds resolve from flags, then environment,
// then the installation's metasystem.conf (watch.stale-min 20, watch.cap-min
// 180, watch.interval-sec 60).
func runReportWatchJobs(args []string) int {
	return runReportWatchJobsTo(args, os.Stdout, os.Stderr)
}

// runReportWatchJobsTo is the watcher verb with its output streams supplied.
func runReportWatchJobsTo(args []string, stdout, stderr io.Writer) int {
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
		fmt.Fprintln(stderr, "usage: metasystem internal report watch-jobs --dir DIR [--dir DIR]... [--scope PATH] [--state FILE] [--stale-min N] [--cap-min N] [--interval SEC] [--start-verify-min N] [--baseline] [--once] [--census]")
		return 2
	}
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	installation := *root
	if installation == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(stderr, "watch-jobs:", err)
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
		fmt.Fprintln(stderr, "watch-jobs: --stale-min, --cap-min and --interval must be integers")
		return 2
	}
	sleepEvery := time.Duration(intervalSec) * time.Second
	if override := os.Getenv("METASYSTEM_CENSUS_INTERVAL_MS"); override != "" {
		milliseconds, err := strconv.ParseInt(override, 10, 64)
		if err != nil || milliseconds < 1 {
			fmt.Fprintln(stderr, "METASYSTEM_CENSUS_INTERVAL_MS must be a positive integer")
			return 2
		}
		sleepEvery = time.Duration(milliseconds) * time.Millisecond
	}
	watchScope := strings.TrimSuffix(*scope, "/")
	options := report.WatchJobsOptions{
		Dirs: dirs, Scope: watchScope, ScopeField: *scopeField, StateFile: *state, TempDir: os.TempDir(),
		StaleMin: stale, CapMin: capped, StartVerifyMin: *startVerifyMin, Interval: sleepEvery,
		Baseline: *baseline, Once: *once, Fingerprint: watchJobsFingerprint(),
		Now: time.Now, Out: stdout, Err: stderr,
	}
	if *censusEnabled {
		if watchScope == "" {
			fmt.Fprintln(stderr, "--census requires --scope")
			return 2
		}
		top, err := stateroot.RepositoryTop(watchScope)
		if err != nil {
			fmt.Fprintln(stderr, "--census scope is not a git repository")
			return 2
		}
		if watchScope, err = canonicalPath(top); err != nil {
			fmt.Fprintln(stderr, "--census scope is not a git repository")
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
			fmt.Fprintln(stderr, "watch-jobs:", err)
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
