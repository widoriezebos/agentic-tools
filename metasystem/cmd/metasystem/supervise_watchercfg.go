package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// watcherConfig is the ONE constructor of supervise.WatcherConfig: with
// two assemblers, the standing component and the one-shot watcher-pass
// would resolve the same tuning key through different precedence rules
// — a .local override honored by one census writer and ignored by the
// other. The deliberate resolution choice is the
// COMMITTED conf (config.ConfValue): a watcher is an armed component whose
// behavior the supervision fingerprint pins, so its tuning follows the
// committed state the fingerprint covers, never per-process overrides that
// would make two census writers disagree.
func watcherConfig(metasystemRoot, stateRoot, scope, supervisionDir string, intervalSec int) supervise.WatcherConfig {
	intervalMS := intervalSec * 1000
	if override := os.Getenv("METASYSTEM_CENSUS_INTERVAL_MS"); override != "" {
		if parsed, err := strconv.Atoi(override); err == nil && parsed > 0 {
			intervalMS = parsed
		}
	}
	budgetPercent := 50
	if parsed, err := strconv.Atoi(config.ConfValue(filepath.Join(metasystemRoot, "metasystem.conf"),
		"census.max-interval-share-percent", "50")); err == nil && parsed >= 1 && parsed <= 100 {
		budgetPercent = parsed
	}
	return supervise.WatcherConfig{
		SupervisionDir: supervisionDir,
		Interval:       intervalSec,
		IntervalMS:     intervalMS,
		BudgetPercent:  budgetPercent,
		// The metasystem root (where scripts, config, and the engine live)
		// and the repository scope (the git toplevel the census bounds to)
		// differ in a nested checkout; both travel from the armer.
		Fingerprint: func() (string, error) { return census.Fingerprint(metasystemRoot, scope) },
		Census: func(fingerprint string, now time.Time) (census.Verdict, error) {
			// The fixture enumeration override: a fake-runtime fixture feeds
			// the process table through a file (and the enumerator itself
			// refuses it outside metasystem.runtimes=fake).
			if processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE"); processFile != "" {
				return census.RunFixtureCensusAt(metasystemRoot, stateRoot, scope, processFile, fingerprint, intervalSec, now)
			}
			return census.RunProductionCensusAt(metasystemRoot, stateRoot, scope, fingerprint, intervalSec, now)
		},
		Now:  func() time.Time { return time.Now().UTC() },
		Warn: func(message string) { fmt.Fprintln(os.Stderr, message) },
	}
}
