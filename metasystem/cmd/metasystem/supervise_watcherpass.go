package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// runSuperviseWatcherPass runs one supervision census pass as a standalone
// writer: it claims the census-writer lock (refusing when a live writer
// already owns it), heartbeats, computes the fingerprint, scans the scope,
// publishes the verdict, warns when the scan exceeds its budget share, and
// releases the lock. The job-watching loop drives this once per interval.
func runSuperviseWatcherPass(args []string) int {
	flags := flag.NewFlagSet("supervise watcher-pass", flag.ContinueOnError)
	root := flags.String("root", "", "harness root (signatures and config)")
	scope := flags.String("scope", "", "checkout the census bounds to (defaults to --root)")
	supervisionDir := flags.String("supervision-dir", "", "census/heartbeat directory")
	heartbeat := flags.String("heartbeat", "", "watcher heartbeat file")
	tag := flags.String("tag", "", "watcher instance tag")
	interval := flags.Int("interval", 60, "observation interval seconds")
	capMin := flags.Int("cap-min", 0, "loaded watcher cap ceiling for the heartbeat attestation (defaults to the interval when unset)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *capMin < 1 {
		*capMin = *interval
	}
	if *root == "" || *supervisionDir == "" || *heartbeat == "" || *tag == "" {
		fmt.Fprintln(os.Stderr, "supervise watcher-pass: --root, --supervision-dir, --heartbeat, and --tag are required")
		return 2
	}
	censusScope := *scope
	if censusScope == "" {
		censusScope = *root
	}

	self := identity.Ref{Pid: int64(os.Getpid())}
	if exact, state, err := (identity.KernelProber{}).Probe(self.Pid); err == nil && state == identity.Alive {
		self.StartedAtSec = exact.StartedAt.Unix()
	}
	lock := &supervise.CensusWriterLock{Dir: *supervisionDir, Self: self, Tag: *tag, Prober: identity.KernelProber{}}
	if err := lock.Claim(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer lock.Release()

	_ = supervise.WriteHeartbeat(*heartbeat, "watcher", self, *tag, *interval, *capMin)

	intervalMS := *interval * 1000
	if override := os.Getenv("METASYSTEM_CENSUS_INTERVAL_MS"); override != "" {
		if n, err := strconv.Atoi(override); err == nil && n > 0 {
			intervalMS = n
		}
	}
	budgetPercent := 50
	if value, code, err := config.Get(config.GetParams{
		ConfPath: *root + "/metasystem.conf", Key: "census.max-interval-share-percent",
		Default: "50", DefaultSet: true,
	}); err == nil && code == 0 {
		if n, err := strconv.Atoi(value); err == nil && n >= 1 && n <= 100 {
			budgetPercent = n
		}
	}

	cfg := supervise.WatcherConfig{
		SupervisionDir: *supervisionDir,
		Interval:       *interval,
		IntervalMS:     intervalMS,
		BudgetPercent:  budgetPercent,
		Fingerprint:    func() (string, error) { return census.Fingerprint(*root, censusScope) },
		Census: func(fingerprint string, now time.Time) (census.Verdict, error) {
			if processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE"); processFile != "" {
				return census.RunFixtureCensus(*root, censusScope, processFile, fingerprint, *interval, now)
			}
			return census.RunProductionCensus(*root, censusScope, fingerprint, *interval, now)
		},
		Now:  func() time.Time { return time.Now().UTC() },
		Warn: func(message string) { fmt.Fprintln(os.Stderr, message) },
	}
	if err := cfg.WatcherPass(); err != nil {
		fmt.Fprintln(os.Stderr, "supervise watcher-pass:", err)
		return 1
	}
	_ = supervise.WriteHeartbeat(*heartbeat, "watcher", self, *tag, *interval, *capMin)
	return 0
}
