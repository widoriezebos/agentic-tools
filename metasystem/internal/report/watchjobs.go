package report

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The background-job watcher (formerly scripts/watch-background-jobs.sh):
// the arm-once supervision contract of docs/orchestration.md. It reports
// every job that reaches a REPORTABLE state (DONE, STALE, CAPPED, VANISHED,
// NEVER-STARTED; the verdicts are ScanJobs's) and keeps watching. Each pass
// may first run a supervision census; the census log it keeps is bounded by
// census.log-max-bytes with one rotation. A MISSING state file auto-baselines
// on first run so arming never replays history; distinct --dir/--scope
// arguments get distinct default state files so projects never suppress each
// other's reports.

// WatchJobsOptions is one watcher run.
type WatchJobsOptions struct {
	Dirs           []string
	Scope          string // resolved scope root; empty watches every job
	ScopeField     string
	StateFile      string // empty: DefaultWatchJobsState(TempDir, Dirs, Scope)
	TempDir        string
	StaleMin       int64
	CapMin         int64
	StartVerifyMin int64
	Interval       time.Duration
	Baseline       bool // record every current terminal job as reported, then exit
	Once           bool // one pass, then exit
	Fingerprint    string

	// Census, when set, runs one supervision census pass before each scan
	// and returns its output; an error ends the run. CensusLog receives the
	// output, rotated to CensusLog.1 past CensusLogMaxBytes.
	Census            func() (string, error)
	CensusLog         string
	CensusLogMaxBytes int64
	// Heartbeat returns the deepest live suite heartbeat to prefix report
	// lines with; empty means none.
	Heartbeat func() string
	Now       func() time.Time
	Sleep     func(time.Duration)
	// Stop, when set, ends the watch loop before the next pass.
	Stop func() bool

	Out, Err io.Writer
}

// DefaultWatchJobsState is the state file for one watcher's arguments: a
// digest of every --dir and the scope, so distinct scopes never share state.
func DefaultWatchJobsState(tempDir string, dirs []string, scope string) string {
	digest := sha256.Sum256([]byte(strings.Join(dirs, "\n") + "\nscope=" + scope + "\n"))
	return filepath.Join(tempDir, "watch-jobs."+hex.EncodeToString(digest[:8])+".state")
}

// WatchJobs runs the watcher and returns its exit status: 0 after --once or
// --baseline, 1 when a pass fails, 2 for a usage error.
func WatchJobs(o WatchJobsOptions) int {
	if len(o.Dirs) == 0 {
		fmt.Fprintln(o.Err, "watch-jobs: --dir is required")
		return 2
	}
	if o.StaleMin < 1 || o.CapMin < 1 || o.StartVerifyMin < 0 || o.Interval <= 0 {
		fmt.Fprintln(o.Err, "watch-jobs: --stale-min, --cap-min and --interval must be positive integers, --start-verify-min non-negative")
		return 2
	}
	if o.StateFile == "" {
		o.StateFile = DefaultWatchJobsState(o.TempDir, o.Dirs, o.Scope)
	}
	if err := os.MkdirAll(filepath.Dir(o.StateFile), 0o755); err != nil {
		fmt.Fprintln(o.Err, "watch-jobs:", err)
		return 1
	}
	autoBaseline := false
	if _, err := os.Stat(o.StateFile); os.IsNotExist(err) {
		autoBaseline = true
	}
	handle, err := os.OpenFile(o.StateFile, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(o.Err, "watch-jobs:", err)
		return 1
	}
	handle.Close()
	scope := o.Scope
	if scope == "" {
		scope = "none"
	}
	fmt.Fprintf(o.Out, "ARMED watcher fp=%s dirs=%s scope=%s state=%s stale=%dm cap=%dm start-verify=%dm auto-baseline=%d\n",
		o.Fingerprint, strings.Join(o.Dirs, " "), scope, o.StateFile, o.StaleMin, o.CapMin, o.StartVerifyMin, boolDigit(autoBaseline))

	// The running set rides in a per-run scratch file so a restart resets
	// VANISHED tracking, as the in-process arrays of the shell watcher did.
	running, err := os.CreateTemp(o.TempDir, "watch-running.")
	if err != nil {
		fmt.Fprintln(o.Err, "watch-jobs:", err)
		return 1
	}
	running.Close()
	defer os.Remove(running.Name())

	pass := func(baseline bool) error {
		if o.Census != nil {
			output, censusErr := o.Census()
			logErr := appendCensusLog(o.CensusLog, output, o.CensusLogMaxBytes)
			if censusErr != nil {
				fmt.Fprint(o.Err, output)
				return censusErr
			}
			for _, line := range strings.Split(output, "\n") {
				if strings.Contains(line, "WARNING CENSUS-SLOW") {
					fmt.Fprintln(o.Err, line)
				}
			}
			if logErr != nil {
				return logErr
			}
		}
		var report strings.Builder
		if err := ScanJobs(ScanJobsParams{
			Dirs: o.Dirs, StateFile: o.StateFile, RunningFile: running.Name(),
			Scope: o.Scope, ScopeField: o.ScopeField,
			StaleMin: o.StaleMin, CapMin: o.CapMin, StartVerifyMin: o.StartVerifyMin,
			Baseline: baseline, Now: o.Now(),
		}, &report); err != nil {
			return err
		}
		heartbeat := ""
		if o.Scope != "" && o.Heartbeat != nil {
			heartbeat = o.Heartbeat()
		}
		lines := strings.TrimSuffix(report.String(), "\n")
		switch {
		case lines != "":
			for _, line := range strings.Split(lines, "\n") {
				if heartbeat != "" {
					line = heartbeat + " " + line
				}
				fmt.Fprintln(o.Out, line)
			}
		case heartbeat != "":
			fmt.Fprintln(o.Out, heartbeat)
		}
		return nil
	}
	fail := func(err error) int {
		fmt.Fprintln(o.Err, err)
		return 1
	}

	if o.Baseline {
		if err := pass(true); err != nil {
			return fail(err)
		}
		fmt.Fprintf(o.Err, "baseline recorded in %s\n", o.StateFile)
		return 0
	}
	if autoBaseline {
		// First run against a fresh state file adopts history instead of
		// flooding: arming precedes the first dispatch, so anything already
		// terminal predates this session's work.
		if err := pass(true); err != nil {
			return fail(err)
		}
		fmt.Fprintf(o.Err, "auto-baselined historical jobs into %s\n", o.StateFile)
	}
	for {
		if err := pass(false); err != nil {
			return fail(err)
		}
		if o.Once || (o.Stop != nil && o.Stop()) {
			return 0
		}
		o.Sleep(o.Interval)
	}
}

// appendCensusLog appends one census pass's output to the log, first
// rotating a non-empty log to log.1 when the append would exceed maxBytes.
func appendCensusLog(log, output string, maxBytes int64) error {
	if log == "" {
		return nil
	}
	if maxBytes < 1 {
		maxBytes = 1048576
	}
	if info, err := os.Stat(log); err == nil && info.Size() > 0 && info.Size()+int64(len(output)) > maxBytes {
		if err := os.Rename(log, log+".1"); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, output)
	return err
}

func boolDigit(value bool) int {
	if value {
		return 1
	}
	return 0
}
