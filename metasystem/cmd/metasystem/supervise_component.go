package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// runSuperviseComponent runs one supervised component — the watcher or the
// reaper — launched by the owner. Every interval it rewrites its heartbeat (the
// owner reads liveness by heartbeat freshness) AND does its real work:
//
//   - watcher: runs the repository process census and publishes the verdict to
//     last-census.json, holding the census-writer lock for its whole life so at
//     most one census stream exists per supervision directory.
//   - reaper: sweeps the job records, applying the process-lost and budget-cap
//     terminal verdicts.
//
// A transient error in either job is logged and the loop continues — a
// component must not die on a bad scan or an unreadable record; the owner tears
// it down deliberately by signal, or replaces it when its heartbeat goes stale.
func runSuperviseComponent(args []string) int {
	flags := flag.NewFlagSet("supervise component", flag.ContinueOnError)
	component := flags.String("component", "", "watcher | reaper")
	repo := flags.String("repo", "", "checkout root the component operates on")
	tag := flags.String("tag", "", "component instance tag")
	heartbeat := flags.String("heartbeat", "", "heartbeat file path")
	intervalSec := flags.Int("interval", 60, "heartbeat/work interval seconds")
	// crashOnStart reproduces the pure crash-loop shape (D-2): a
	// component that dies on startup WITHOUT ever beating, so it never
	// reads Healthy and the breaker advances monotonically to N. A
	// component that beat even once would reset the breaker (the
	// four-fail-then-reset pattern), which is a different, non-terminal
	// shape. Fixture-only.
	crashOnStart := flags.Bool("crash-on-start", false, "exit immediately without beating (fixture-only)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *component == "" || *tag == "" || *heartbeat == "" {
		fmt.Fprintln(os.Stderr, "supervise component: --component, --tag, --heartbeat required")
		return 2
	}
	if *component == "watcher" || *component == "reaper" {
		if *repo == "" {
			fmt.Fprintln(os.Stderr, "supervise component: --repo is required for the "+*component)
			return 2
		}
	}
	if *crashOnStart {
		fmt.Fprintln(os.Stderr, "supervise component: crash-on-start (fixture)")
		return 1
	}

	// This process's own identity: the census-writer lock publishes it, and it
	// lets a heartbeat name the exact process it beats for.
	self := identity.Ref{Pid: int64(os.Getpid())}
	if exact, state, err := (identity.KernelProber{}).Probe(self.Pid); err == nil && state == identity.Alive {
		self.StartedAtSec = exact.StartedAt.Unix()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	beat := heartbeatWriter(*heartbeat, *component, self, *tag, *intervalSec)

	var work func()
	switch *component {
	case "watcher":
		release, pass, ok := setupWatcher(*repo, self, *tag, *intervalSec)
		if !ok {
			return 1
		}
		defer release()
		work = pass
	case "reaper":
		work = setupReaper(*repo)
	default:
		// An unknown component still beats, so a mislabelled owner launch is
		// observable rather than a silent no-op.
		work = func() {}
	}

	beat() // beat once immediately so the owner sees liveness fast
	work() // and produce a first verdict/sweep without waiting a full interval
	ticker := time.NewTicker(time.Duration(*intervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return 0
		case <-ticker.C:
			beat()
			work()
		}
	}
}

// heartbeatWriter returns a never-failing closure that rewrites the component's
// heartbeat with a fresh observedAtEpoch. A write error is swallowed so a full
// disk cannot crash a component (the stale heartbeat makes the owner replace it).
func heartbeatWriter(path, component string, self identity.Ref, tag string, intervalSec int) func() {
	return func() {
		_ = supervise.WriteHeartbeat(path, component, self, tag, intervalSec)
	}
}

// setupWatcher claims the census-writer lock and returns the release closure
// and the per-interval census pass. It returns ok=false (and logs) when a live
// writer already owns the lock — the owner then sees this watcher fail and, once
// the incumbent stops, relaunches one that can claim it.
func setupWatcher(repo string, self identity.Ref, tag string, intervalSec int) (release func(), pass func(), ok bool) {
	supervisionDir := supervise.SupervisionDir(repo)
	lock := &supervise.CensusWriterLock{
		Dir: supervisionDir, Self: self, Tag: tag, Prober: identity.KernelProber{},
	}
	if err := lock.Claim(); err != nil {
		fmt.Fprintln(os.Stderr, "supervise component watcher:", err)
		return nil, nil, false
	}

	intervalMS := intervalSec * 1000
	if override := os.Getenv("METASYSTEM_CENSUS_INTERVAL_MS"); override != "" {
		if parsed, err := strconv.Atoi(override); err == nil && parsed > 0 {
			intervalMS = parsed
		}
	}
	budgetPercent := 50
	if parsed, err := strconv.Atoi(config.ConfValue(filepath.Join(repo, "metasystem.conf"),
		"census.max-interval-share-percent", "50")); err == nil && parsed >= 1 && parsed <= 100 {
		budgetPercent = parsed
	}

	cfg := supervise.WatcherConfig{
		SupervisionDir: supervisionDir,
		Interval:       intervalSec,
		IntervalMS:     intervalMS,
		BudgetPercent:  budgetPercent,
		// The metasystem root and the repository scope are the same checkout for
		// a supervised component; the fingerprint and the live scan both key off
		// it.
		Fingerprint: func() (string, error) { return census.Fingerprint(repo, repo) },
		Census: func(fingerprint string, now time.Time) (census.Verdict, error) {
			return census.RunProductionCensus(repo, repo, fingerprint, intervalSec, now)
		},
		Now:  func() time.Time { return time.Now().UTC() },
		Warn: func(message string) { fmt.Fprintln(os.Stderr, message) },
	}
	pass = func() {
		if err := cfg.WatcherPass(); err != nil {
			fmt.Fprintln(os.Stderr, "supervise component watcher:", err)
		}
	}
	return lock.Release, pass, true
}

// setupReaper returns the per-interval job sweep, proving custody liveness
// against the live kernel.
func setupReaper(repo string) func() {
	cfg := supervise.ReaperConfig{
		JobsDir:   supervise.JobsDir(repo),
		Now:       func() time.Time { return time.Now().UTC() },
		Custodian: kernelCustodian,
		Emit:      func(line string) { fmt.Fprintln(os.Stderr, line) },
	}
	return func() {
		if err := cfg.ReaperPass(); err != nil {
			fmt.Fprintln(os.Stderr, "supervise component reaper:", err)
		}
	}
}

// kernelCustodian proves a job's custodian three-way against the live process
// table: it is the SAME custodian only if the pid is alive at its recorded
// start AND its command still carries the job's tag — a recycled pid, or a
// process no longer bearing the tag, is a stranger and reads as dead-to-us.
func kernelCustodian(pid, start int64, tag string) identity.Liveness {
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state == identity.Unknown {
		return identity.Unknown
	}
	if state == identity.Dead || exact.StartedAt.Unix() != start {
		return identity.Dead
	}
	if tag != "" && !strings.Contains(strings.Join(exact.Argv, " "), tag) {
		return identity.Dead
	}
	return identity.Alive
}
