package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The reaper's per-interval job sweep. This reaper holds NO kill
// authority: it never signals a process and never
// records a death it has not proven. It owns terminal transitions only for
// jobs whose recorded custodian is PROVABLY dead:
//
//   - budget-cap: a dead custodian whose RUNNING record is past its budget
//     reads timeout — the budget verdict still outranks process-lost so this
//     reaper and the dispatch-side ladder agree on one expired record.
//   - process-lost: a dead custodian otherwise fails with process-lost.
//   - abandoned-setup: a pending-setup husk old enough that its creating
//     dispatcher is certainly gone; no process ever existed, so no death is
//     claimed.
//
// A LIVE over-budget job keeps its running status: winding it down belongs
// to the kill-capable dispatch path, and stamping timeout here would record
// a death nobody proved. An unreadable custodian defers, matching the
// census's three-way discipline (indeterminacy never acts).

// ReaperConfig drives one reap pass. Custody liveness is supplied as a function
// so the production path binds the kernel prober while tests bind a fake table.
type ReaperConfig struct {
	// Repo is the repository root used for mission fence recovery asks.
	Repo string
	// JobsDir holds the job records (<repo>/artifacts/agents/jobs).
	JobsDir string
	// Now is the wall clock (injectable for tests).
	Now func() time.Time
	// Custodian proves a job's recorded custodian three-way: Alive when the pid
	// is live at its recorded start AND its command still carries the job's tag,
	// Dead when provably gone (or a stranger on a recycled pid), Unknown when
	// unreadable. Only Dead reaps.
	Custodian func(pid, start int64, tag string) identity.Liveness
	// Survivors reports live tagged processes besides the custodian —
	// the group-death half of the proof (a dead custodian with tagged
	// survivors is a live group). nil binds identity.TaggedSurvivors;
	// tests bind a fake. certain=false defers like Unknown does.
	Survivors func(tag string, exclude, pgid int64) (alive bool, certain bool)
	// Apply lands one terminal verdict through the locked job-record
	// compare-and-swap owner: the transition happens only if the record still
	// carries the expected status, so a completion landing after this
	// reaper's read can never be clobbered by a stale verdict. A refusal is
	// not an error — it means the record moved on and the verdict is void.
	// The production binding wires the dispatch record owner in at the
	// command layer (the dispatch package imports this one, so the reverse
	// import would cycle).
	Apply func(job, expect, target string, patch map[string]any) (applied bool, err error)
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

	now := cfg.Now()

	// A pending-setup record is a reservation husk: its creating dispatcher
	// finishes setup in seconds, so one old enough has been abandoned by a
	// crashed dispatch. It carries no mission stamp yet, so no mission's own
	// drain can see it — while its fence reservation still consumes a
	// concurrency slot. This reaper is the layer that must clear it.
	if status == "pending-setup" {
		if created, ok := record["createdAt"].(string); ok {
			if at, err := time.Parse(time.RFC3339, created); err == nil && now.Sub(at) > dispatch.AbandonedSetupGrace {
				// No process ever existed for a reservation husk, so the
				// patch claims no death.
				_, err := cfg.transition(path, record, status, "failed", "abandoned-setup",
					map[string]any{"error": "abandoned-setup", "phase": "supervision"})
				return err
			}
		}
		return nil
	}
	if status != "running" && status != "pending" {
		return nil // only a job still believed live can be reaped
	}

	// No kill authority: this reaper acts only on a PROVABLY DEAD custodian.
	// A missing or non-positive pid means no custodian to prove dead — those
	// are deferred (a job still inside its launch handshake), and a live or
	// unreadable custodian is left to the kill-capable dispatch path even
	// when its budget has expired.
	pid, hasPid := recordInt(record["pid"])
	if !hasPid || pid < 1 {
		// A marked record with NO recorded process has no group to
		// prove dead and no launch that can still record one (the
		// ownership write voids on the marker and kills its child):
		// nothing but this pass will ever conclude it. No death is
		// claimed — cancelled-before-launch is its own honest shape.
		if phase, _ := record["phase"].(string); phase == "cancelling" {
			_, err := cfg.transition(path, record, status, "cancelled", "cancel-honored",
				map[string]any{"error": nil, "phase": "supervision"})
			return err
		}
		return nil
	}
	start, _ := recordInt(record["pidStartedAt"])
	tag, _ := record["instanceTag"].(string)
	if verdict := cfg.Custodian(pid, start, tag); verdict != identity.Dead {
		// The decline is a decision too: a running job past
		// its cap whose custodian this reaper may not kill is exactly the
		// state an operator needs to see. Said once per pass; the record
		// itself stays with the kill-capable dispatch path.
		if cfg.Emit != nil && status == "running" && dispatch.CapExpired(record, now) {
			cfg.Emit(fmt.Sprintf("REAP-DECLINED job=%s cap expired, custodian %s; kill authority stays with dispatch",
				jobIDFor(record, path), verdict))
		}
		return nil
	}
	// The custodian is dead, but groupDeathProvenAt claims the GROUP
	// died — and this reaper has no kill authority to make that true.
	// A tagged survivor (an orphaned child still carrying the
	// instance tag) means the group lives: the verdict belongs to the
	// kill-capable dispatch path, which winds the group down before
	// it stamps. Indeterminacy defers the same way.
	survivors := cfg.Survivors
	if survivors == nil {
		survivors = identity.TaggedSurvivors
	}
	recPgid, _ := recordInt(record["pgid"])
	if alive, certain := survivors(tag, pid, recPgid); !certain || alive {
		if cfg.Emit != nil {
			cfg.Emit(fmt.Sprintf("REAP-DEFERRED job=%s custodian dead but the tagged group is not proven dead; kill authority stays with dispatch",
				jobIDFor(record, path)))
		}
		return nil
	}
	patch := map[string]any{
		"phase":              "supervision",
		"groupDeathProvenAt": now.UTC().Format(isoSecond),
	}
	// A cancel marks the record BEFORE it kills (visible before
	// action); a dead group carrying the marker is that cancel's
	// outcome, not a process loss — however this pass and the
	// cancel's own concluding swap interleave, the record reads
	// cancelled. The marker outranks the budget: the operator's
	// explicit stop is the truer cause of this death.
	if phase, _ := record["phase"].(string); phase == "cancelling" {
		patch["error"] = nil
		_, err := cfg.transition(path, record, status, "cancelled", "cancel-honored", patch)
		return err
	}
	// Among dead-custodian records the budget still outranks loss, so this
	// reaper and the dispatch ladder read the same verdict from one expired
	// record.
	if status == "running" && dispatch.CapExpired(record, now) {
		patch["error"] = "budget-cap"
		applied, err := cfg.transition(path, record, status, "timeout", "budget-cap", patch)
		if err != nil || !applied {
			return err
		}
		missionID, _ := record["mission"].(string)
		if missionID == "" {
			return nil
		}
		resolution, _ := record["capResolution"].(map[string]any)
		return mission.RefuseBudgetCap(cfg.Repo, missionID, jobIDFor(record, path), resolution, cfg.Emit)
	}
	patch["error"] = "process-lost"
	_, err = cfg.transition(path, record, status, "failed", "process-lost", patch)
	return err
}

// transition lands the verdict through the injected compare-and-swap owner.
// A refused swap means the record moved on (a completion landed after this
// pass's read); the verdict is void and nothing is emitted.
func (cfg ReaperConfig) transition(path string, record map[string]any, from, to, reason string, patch map[string]any) (bool, error) {
	applied, err := cfg.Apply(jobIDFor(record, path), from, to, patch)
	if err != nil {
		return false, fmt.Errorf("apply reap verdict for %s: %w", path, err)
	}
	if applied && cfg.Emit != nil {
		cfg.Emit(fmt.Sprintf("%s job=%s status=%s->%s", strings.ToUpper(reason), jobIDFor(record, path), from, to))
	}
	return applied, nil
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
