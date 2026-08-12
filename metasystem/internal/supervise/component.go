package supervise

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Shared plumbing for the standing components (watcher and reaper): the
// supervision directory layout they publish into, and the atomic file write
// both use so a reader never sees a half-written verdict or record.

// isoSecond is the whole-second UTC timestamp shape the supervision surface
// uses everywhere (state.json, verdicts, records).
const isoSecond = "2006-01-02T15:04:05Z"

// SupervisionDir is <repo>/artifacts/agents/supervision for a checkout root —
// the census-writer lock, the component heartbeats, and the census verdict all
// live here.
func SupervisionDir(repo string) string {
	return filepath.Join(repo, "artifacts", "agents", "supervision")
}

// JobsDir is <repo>/artifacts/agents/jobs — the job records the reaper sweeps.
func JobsDir(repo string) string {
	return filepath.Join(repo, "artifacts", "agents", "jobs")
}

// WriteHeartbeat atomically rewrites a component's heartbeat with a fresh
// observedAtEpoch. The owner reads pid and observedAtEpoch to judge liveness;
// the component calls this every interval so a stale heartbeat means the
// component is stuck or gone. capMin is the watcher's loaded cap ceiling —
// the attestation dispatch reads (internal/dispatch/attest.go) and arming
// verifies against state.derivedWatcherCapMin; it must be the derived cap
// the armer loaded, never a placeholder.
func WriteHeartbeat(path, component string, self identity.Ref, tag string, intervalSec, capMin int) error {
	record := map[string]any{
		"function":        component,
		"pid":             self.Pid,
		"pidStartedAt":    self.StartedAtSec,
		"instanceTag":     tag,
		"observedAtEpoch": time.Now().Unix(),
		"loadedCapMin":    capMin,
		"engine":          "go",
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return atomicWrite(path, append(line, '\n'))
}

// atomicWrite publishes content to path via a temp file in the same directory
// plus a rename, so a concurrent reader sees either the old file or the new one
// and never a partial write. The parent directory is created if missing.
func atomicWrite(path string, content []byte) error {
	// Through the durable-write owner (go-production-grade B5); the
	// empty anchor preserves this writer's previous behavior exactly
	// until its caller is converted to the two-outcome contract.
	_, err := atomicfile.WriteText(path, string(content), "")
	return err
}

// parseISOSecond parses a whole-second UTC timestamp, accepting the trailing-Z
// form the supervision surface writes as well as any offset RFC 3339 form.
func parseISOSecond(value string) (time.Time, error) {
	if t, err := time.Parse(isoSecond, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}
