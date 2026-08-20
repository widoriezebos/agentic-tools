package steward

// The worker universe beyond runtime-shaped processes: mission
// runners, monitored runs, and live gates keep pid-bearing records
// even though their command lines match no runtime signature. A
// death proof must see them, so the census supplement probes every
// recorded pid directly — a live one blocks revival, an unreadable
// or unprovable one blocks the proof itself.

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// recordStores are the pid-bearing record locations the supplement
// scans, relative to the repository root.
var recordStores = []string{
	"artifacts/agents/runs",
	"artifacts/agents/supervision/gate-runs",
	"artifacts/agents/mains",
}

// The supervision directory itself stays OUT: the watcher is
// infrastructure that ticks whether or not anyone works — counting
// it as a worker would block every death proof on every armed
// repository, the same churn-versus-work confusion the evidence
// rules already settled.

// supplementWorkers probes every recorded pid the runtime census
// cannot see. Live adds workers; malformed records and failed
// probes add unprovables — unknown dominates dead at this layer too.
func supplementWorkers(repoRoot string) (live, unprovable int) {
	for _, store := range recordStores {
		paths, err := filepath.Glob(filepath.Join(repoRoot, store, "*.json"))
		if err != nil {
			unprovable++
			continue
		}
		for _, p := range paths {
			data, err := os.ReadFile(p)
			if err != nil {
				unprovable++
				continue
			}
			var record struct {
				Pid          *int64 `json:"pid"`
				PidStartedAt *int64 `json:"pidStartedAt"`
			}
			if err := json.Unmarshal(data, &record); err != nil {
				unprovable++
				continue
			}
			if record.Pid == nil || *record.Pid <= 0 {
				continue // not a process record; nothing to prove
			}
			exact, state, err := identity.KernelProber{}.Probe(*record.Pid)
			if err != nil || state == identity.Unknown {
				unprovable++
				continue
			}
			if state != identity.Alive {
				continue // provably gone
			}
			if record.PidStartedAt != nil && exact.StartedAt.Unix() != *record.PidStartedAt {
				continue // the pid was reused; the recorded worker is gone
			}
			live++
		}
	}
	return live, unprovable
}
