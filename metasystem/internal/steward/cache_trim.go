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
	if stateDir == "" {
		if stateDir, err = CacheTrimStateDir(); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, settings.Budget)
	defer cancel()
	return gocache.TrimMachine(ctx, gocache.MachineTrim{
		UserCacheDir: userCacheDir, StateDir: stateDir,
		EngineGoCapBytes: settings.EngineGoCapBytes, DelegateGoCapBytes: settings.DelegateGoCapBytes, StaticcheckCapBytes: settings.StaticcheckCapBytes,
		Keep: settings.Keep, Now: now, Clock: clock, Stopped: stopped,
	})
}
