package steward

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// CacheTrimStateDir is <home>/.metasystem/cache-trim: each machine cache's
// flock, report and eviction candidates, beside the machine registry.
func CacheTrimStateDir() (string, error) {
	registryPath, err := registry.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(registryPath), "cache-trim"), nil
}

// machineCacheTrimmer is the runner's cycle step that trims the machine Go
// and staticcheck caches to their caps (disk-lifetimes A12). It reads the
// five disk settings from the checkout's metasystem.conf, runs under the
// trim budget, and ends at the next batch once the runner's stop file
// appears, so stopping the system never waits on a trim. It takes and
// holds no arbitration: only each cache's own nonblocking flock.
func machineCacheTrimmer(userCacheDir func() (string, error), stateDir string) func(context.Context, string, TickConfig) error {
	return func(ctx context.Context, top string, cfg TickConfig) error {
		stop := runnerStopPath(top)
		_, err := TrimMachineCaches(ctx, top, userCacheDir, stateDir, cfg.now(), func() time.Time { return cfg.now() }, func() bool {
			_, err := os.Stat(stop)
			return err == nil
		})
		return err
	}
}

// TrimMachineCaches runs one trim pass over the four machine caches with
// the settings of the checkout at top: the steward's step and the manual
// `disk clean --go-cache` alike. A nil userCacheDir is os.UserCacheDir; an
// empty stateDir is CacheTrimStateDir.
func TrimMachineCaches(ctx context.Context, top string, userCacheDir func() (string, error), stateDir string, now time.Time, clock func() time.Time, stopped func() bool) ([]gocache.TrimReport, error) {
	settings, err := config.CacheTrimSettings(filepath.Join(top, "metasystem.conf"))
	if err != nil {
		return nil, err
	}
	run := CacheTrimRun{Top: top, UserCacheDir: userCacheDir, StateDir: stateDir, Clock: clock, Stopped: stopped}
	return run.productionPass(settings)(ctx, nil, now, settings.Budget)
}

// CacheTrimPass is one trim pass over the named machine caches (nil is all
// four) at now, cut at budget.
type CacheTrimPass func(ctx context.Context, caches []string, now time.Time, budget time.Duration) ([]gocache.TrimReport, error)

// CacheTrimRun is a person's trim at a terminal: the checkout whose
// settings apply, the trimmer's seams (nil and "" are production), and the
// clock the person's budget is read on.
type CacheTrimRun struct {
	Top          string
	UserCacheDir func() (string, error)
	StateDir     string
	Clock        func() time.Time
	Stopped      func() bool
	// Pass replaces the trim pass (fixtures); nil is the steward's.
	Pass CacheTrimPass
}

func (run CacheTrimRun) productionPass(settings config.CacheTrim) CacheTrimPass {
	return func(ctx context.Context, caches []string, now time.Time, budget time.Duration) ([]gocache.TrimReport, error) {
		stateDir := run.StateDir
		if stateDir == "" {
			var err error
			if stateDir, err = CacheTrimStateDir(); err != nil {
				return nil, err
			}
		}
		ctx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		return gocache.TrimMachine(ctx, gocache.MachineTrim{
			UserCacheDir: run.UserCacheDir, StateDir: stateDir,
			EngineGoCapBytes: settings.EngineGoCapBytes, DelegateGoCapBytes: settings.DelegateGoCapBytes, StaticcheckCapBytes: settings.StaticcheckCapBytes,
			Keep: settings.Keep, Now: now, Clock: run.Clock, Stopped: run.Stopped, Only: caches,
		})
	}
}

// TrimMachineCachesForPerson is `metasystem disk clean` at a terminal: a
// person expects it to finish the job. It runs pass after pass of the
// steward's budget (disk.cache-trim-budget-sec), each continuing only the
// caches the last pass cut short, until every cache has finished or
// disk.cache-trim-person-budget-sec is spent; the last pass gets only what
// is left of it. progress, when set, is told each pass's reports. The
// result is each cache's latest report, in trim order.
func TrimMachineCachesForPerson(ctx context.Context, run CacheTrimRun, progress func(pass int, reports []gocache.TrimReport)) ([]gocache.TrimReport, error) {
	settings, err := config.CacheTrimSettings(filepath.Join(run.Top, "metasystem.conf"))
	if err != nil {
		return nil, err
	}
	clock := run.Clock
	if clock == nil {
		clock = time.Now
		run.Clock = clock
	}
	pass := run.Pass
	if pass == nil {
		pass = run.productionPass(settings)
	}
	started := clock()
	latest := map[string]gocache.TrimReport{}
	var caches []string
	for number := 1; ; number++ {
		now := clock()
		budget := min(settings.Budget, settings.PersonBudget-now.Sub(started))
		if budget <= 0 {
			break
		}
		reports, err := pass(ctx, caches, now, budget)
		for _, report := range reports {
			latest[report.Cache] = report
		}
		if err != nil {
			return ordered(latest), err
		}
		if progress != nil {
			progress(number, reports)
		}
		caches = nil
		for _, report := range reports {
			if report.EndedBy == "budget" {
				caches = append(caches, report.Cache)
			}
		}
		if len(caches) == 0 || ctx.Err() != nil {
			break
		}
	}
	return ordered(latest), nil
}

// ordered is the latest report of each cache, in trim order.
func ordered(latest map[string]gocache.TrimReport) []gocache.TrimReport {
	var reports []gocache.TrimReport
	for _, name := range gocache.MachineCaches() {
		if report, ok := latest[name]; ok {
			reports = append(reports, report)
		}
	}
	return reports
}
