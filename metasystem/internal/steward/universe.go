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
	"artifacts/agents/missions/runners",
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
		dir := filepath.Join(repoRoot, store)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				// An unreadable store must not look empty: it blocks
				// the proof it was hiding from.
				unprovable++
			}
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				unprovable++
				continue
			}
			var record struct {
				Pid           *int64  `json:"pid"`
				PidStartedAt  *int64  `json:"pidStartedAt"`
				PidStartTicks *int64  `json:"pidStartTicks"`
				BootID        string  `json:"bootId"`
				EndedAt       *string `json:"endedAt"`
				ExitCode      *int64  `json:"exitCode"`
			}
			if err := json.Unmarshal(data, &record); err != nil {
				unprovable++
				continue
			}
			terminal := record.EndedAt != nil || record.ExitCode != nil
			if record.Pid == nil || *record.Pid <= 0 {
				// Only a RUN can be busy without a pid (launching):
				// a non-terminal run record blocks the proof. Other
				// stores keep pid-less bookkeeping — cursors, lease
				// markers — which proves nothing either way.
				if store == "artifacts/agents/runs" && !terminal {
					unprovable++
				}
				continue
			}
			exact, state, err := identity.KernelProber{}.Probe(*record.Pid)
			if err != nil || state == identity.Unknown {
				unprovable++
				continue
			}
			gone := state != identity.Alive
			if !gone {
				// The strongest identity the record carries decides
				// whether this live pid IS the recorded worker.
				if record.PidStartTicks != nil && record.BootID != "" &&
					exact.StartTicks > 0 && exact.BootID != "" {
					gone = exact.StartTicks != *record.PidStartTicks || exact.BootID != record.BootID
				} else if record.PidStartedAt != nil {
					gone = exact.StartedAt.Unix() != *record.PidStartedAt
				}
			}
			if gone {
				// A run whose leader died can still be DRAINING with
				// live descendants: a non-terminal run record is
				// doubt, never proof.
				if store == "artifacts/agents/runs" && !terminal {
					unprovable++
				}
				continue
			}
			live++
		}
	}
	return live, unprovable
}
