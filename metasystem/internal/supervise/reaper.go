package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The reaper's per-interval job sweep. For each live job record it owns two
// terminal transitions the watcher is not allowed to make:
//
//   - process-lost: a job whose recorded custodian is PROVABLY gone. The job
//     will never finish on its own, so it is failed with error process-lost.
//   - budget-cap: a RUNNING job past its absolute capMin measured from
//     startedAt. The budget is a fact of the record alone, so it is judged
//     before liveness and the running job is transitioned to timeout.
//
// Only a definitive death reaps: an unreadable or still-live custodian is left
// alone, matching the census's three-way discipline (indeterminacy never acts).

// ReaperConfig drives one reap pass. Custody liveness is supplied as a function
// so the production path binds the kernel prober while tests bind a fake table.
type ReaperConfig struct {
	// JobsDir holds the job records (<repo>/artifacts/agents/jobs).
	JobsDir string
	// Now is the wall clock (injectable for tests).
	Now func() time.Time
	// Custodian proves a job's recorded custodian three-way: Alive when the pid
	// is live at its recorded start AND its command still carries the job's tag,
	// Dead when provably gone (or a stranger on a recycled pid), Unknown when
	// unreadable. Only Dead reaps.
	Custodian func(pid, start int64, tag string) identity.Liveness
	// Emit receives one line per reaped job; the reaper log in production.
	Emit func(string)
}

// ReaperPass sweeps every job record once. A single unreadable or unwritable
// record does not abort the sweep — the first such error is returned after all
// records are visited, so the caller can log it and continue.
func (cfg ReaperConfig) ReaperPass() error {
	paths, err := filepath.Glob(filepath.Join(cfg.JobsDir, "*.json"))
	if err != nil {
		return fmt.Errorf("scan job records: %w", err)
	}
	sort.Strings(paths)
	var firstErr error
	for _, path := range paths {
		if err := cfg.reapOne(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (cfg ReaperConfig) reapOne(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // vanished between glob and read: nothing to reap
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil {
		return nil // an unparseable record is not this reaper's to rewrite
	}
	status, _ := record["status"].(string)
	if status != "running" && status != "pending" {
		return nil // only a job still believed live can be reaped
	}

	now := cfg.Now()

	// The budget is judged first: an expired capMin is a fact of the record,
	// independent of whether the process happens to have exited already, so a
	// running job over budget is always a timeout rather than a race with loss.
	if status == "running" && CapExpired(record, now) {
		return cfg.transition(path, record, status, "timeout", "budget-cap", now)
	}

	// Otherwise a job whose custodian is provably gone is process-lost. A
	// missing or non-positive pid means no custodian to prove dead — those are
	// deferred (a job still inside its launch handshake), not reaped here.
	pid, hasPid := recordInt(record["pid"])
	if !hasPid || pid < 1 {
		return nil
	}
	start, _ := recordInt(record["pidStartedAt"])
	tag, _ := record["instanceTag"].(string)
	if cfg.Custodian(pid, start, tag) == identity.Dead {
		return cfg.transition(path, record, status, "failed", "process-lost", now)
	}
	return nil
}

// transition rewrites the record with its terminal verdict and republishes it
// atomically. The custody wind-down and lawful-transition CAS the full reaper
// performs are NOT done here (see the component's documented scope); this
// records the verdict so the rest of the system stops waiting on the job.
func (cfg ReaperConfig) transition(path string, record map[string]any, from, to, reason string, now time.Time) error {
	record["status"] = to
	record["error"] = reason
	record["phase"] = "supervision"
	record["groupDeathProvenAt"] = now.UTC().Format(isoSecond)
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode reaped record %s: %w", path, err)
	}
	if err := atomicWrite(path, append(encoded, '\n')); err != nil {
		return err
	}
	if cfg.Emit != nil {
		cfg.Emit(fmt.Sprintf("%s job=%s status=%s->%s", strings.ToUpper(reason), jobIDFor(record, path), from, to))
	}
	return nil
}

// CapExpired reports whether a job is past its absolute budget: the explicit
// capDeadline when the record carries one, else startedAt plus capMin minutes.
// Exported because it is THE budget verdict: the dispatch-side reap and the
// supervision reaper must reach the same conclusion from the same record.
func CapExpired(record map[string]any, now time.Time) bool {
	if deadline, ok := record["capDeadline"].(string); ok && deadline != "" {
		if t, err := parseISOSecond(deadline); err == nil {
			return !now.Before(t)
		}
	}
	capMin, ok := recordInt(record["capMin"])
	started, hasStarted := record["startedAt"].(string)
	if ok && capMin >= 1 && hasStarted {
		if t, err := parseISOSecond(started); err == nil {
			return now.Sub(t) >= time.Duration(capMin)*time.Minute
		}
	}
	return false
}

func jobIDFor(record map[string]any, path string) string {
	if id, ok := record["jobId"].(string); ok && id != "" {
		return id
	}
	return strings.TrimSuffix(filepath.Base(path), ".json")
}

// recordInt reads a JSON number as an integer, since encoding/json decodes
// every number into a float64.
func recordInt(v any) (int64, bool) {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return int64(f), true
	}
	return 0, false
}
