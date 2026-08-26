package missionrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The finite drain and the runner's mission-scoped reap.
// Holding the mission lease
// authorizes the runner to act on exactly the records its mission's fence
// reservations name; authority selects the candidates, never the verdict.
// The verdict needs the standing reaper's proof bar — record facts from
// reap-facts plus the kernel custodian discipline, or a never-launched
// husk's abandoned-setup fact — and lands through the existing record CAS
// under the record lock. Unknown never reaps. Every drain terminates:
// drained empty, or a drain-stalled park naming every survivor.

// drainStalledReason is the park reason, ask reason class, and ask id prefix
// of a drain that reached its deadline with unprovable survivors.
const drainStalledReason = "drain-stalled"

// handshakeGrace is the dispatch backstop's handshake slack as a duration —
// the one grace the drain deadline extends each per-record clock by. This
// design introduces no new grace.
var handshakeGrace = time.Duration(dispatch.HandshakeBackstopGraceSec) * time.Second

// drainSurvivor is one non-terminal record in a drain snapshot: what the
// park ask names so a human can verify or clear it.
type drainSurvivor struct {
	ID           string
	Status       string
	Age          string
	MissingProof string
}

// drainJobs reaps the mission's jobs until none is active, within a finite
// deadline recomputed each pass over the current active set. Each pass beats
// the runner heartbeat (a lawful drain that waits until the last job cap
// must read as a live runner to supervision), applies the runner's own
// mission-scoped reaps and the dispatch-owned reap, and only then waits.
// When the deadline passes with unprovable records remaining, the mission
// parks drain-stalled with the survivors ask; the returned state is non-nil
// exactly when it parked, and the reserved cycle then never concludes here —
// the resume heal books it.
func (e *Engine) drainJobs(statePath, ledger, turnID string, cycle int64) (map[string]any, error) {
	poll, err := Interval("METASYSTEM_HEARTBEAT_INTERVAL_MS", 100)
	if err != nil {
		return nil, err
	}
	// The reap cadence is DECOUPLED from the heartbeat (missionrunner-5):
	// the heartbeat must beat in milliseconds so a lawful cap-length drain
	// reads as a live runner, but a job cannot become provably dead more
	// often than the reap facts change — and each reap pass re-scans the
	// jobs directory and spawns dispatch.sh per active job. At the old
	// 100ms coupling, one 30-minute drain was ~18,000 subprocess spawns
	// manufacturing exactly the machine load the timing fixtures flake
	// under. Fixtures override the seconds-scale default through the env.
	reapEvery, err := Interval("METASYSTEM_DRAIN_REAP_INTERVAL_MS", 5000)
	if err != nil {
		return nil, err
	}
	dispatchScript := filepath.Join(e.Root, "scripts", "agents", "dispatch.sh")
	var lastReap time.Time
	for {
		if err := e.heartbeat(turnID); err != nil {
			return nil, err
		}
		active := activeJobRecords(e.Root, e.Mission)
		if len(active) == 0 {
			return nil, nil
		}
		if time.Since(lastReap) >= reapEvery {
			lastReap = time.Now()
			// Reaps come before waits: the runner's R1/R2 reap clears what
			// is provably dead, then the dispatch-owned reap runs per job
			// exactly as the drain always ran it (budget wind-downs stay
			// with the code that owns process lifecycles — the runner has
			// no kill authority).
			if err := e.reapReservedRecords(time.Now()); err != nil {
				return nil, err
			}
			for _, record := range active {
				e.dispatchReap(dispatchScript, jobRecordID(record))
			}
		}
		live := activeJobRecords(e.Root, e.Mission)
		if len(live) == 0 {
			return nil, nil
		}
		// The deadline recomputes each pass over the CURRENT active set: a
		// follow-up reserved mid-drain lawfully extends it, a record gaining
		// a capDeadline moves to the real clock, and the park condition is
		// judged against this same pass's deadline.
		now := time.Now()
		if deadline := drainDeadline(live, now); !deadline.After(now) {
			// A kill-capable reap is still OWED before any park: the reap
			// cadence is coarser than the slack the deadline leaves, so a
			// record can pass its capDeadline inside a window the periodic
			// reap never serves. Run the dispatch reap for every live record NOW,
			// re-read, and only park what a fresh reap could not resolve.
			for _, record := range live {
				e.dispatchReap(dispatchScript, jobRecordID(record))
			}
			// The marker-aware Go reap gets the same fresh pass: a
			// shell verdict voided by an in-progress cancellation
			// must not park the drain while this branch — the one
			// that concludes a marked dead group cancelled — never
			// saw the record's current phase.
			if err := e.reapReservedRecords(time.Now()); err != nil {
				return nil, err
			}
			lastReap = time.Now()
			live = activeJobRecords(e.Root, e.Mission)
			if len(live) == 0 {
				return nil, nil
			}
			now = time.Now()
			if deadline := drainDeadline(live, now); deadline.After(now) {
				continue // the owed reap extended or resolved the deadline
			}
			return e.parkDrainStalled(statePath, ledger, turnID, cycle, e.drainSurvivors(live, now))
		}
		time.Sleep(poll)
	}
}

// dispatchReap runs the kill-capable dispatch reap for one job and
// WITNESSES its answer: dropping runCaptured's stdout, stderr, and exit
// code would leave a failed reap undiagnosable by artifact, with the
// runner log empty exactly when a stalled drain needs it.
func (e *Engine) dispatchReap(dispatchScript, jobID string) {
	stdout, stderr, code := runCaptured(e.Root, nil, dispatchScript, "reap", "--job", jobID)
	if code != 0 {
		fmt.Fprintf(os.Stderr, "drain reap %s exited %d: %s\n", jobID, code, firstDetail(stderr, stdout))
		e.emit("job-refused", "drain reap failed for "+jobID, map[string]string{
			"jobId": jobID, "missionId": e.Mission, "reasonClass": "drain-reap",
		})
	}
}

// drainDeadline is the finite bound of one drain pass: the latest per-record
// due time over the live set. Each record contributes its own clock —
// capDeadline plus the handshake grace; else startedAt plus its immutable
// capMin plus the grace for a launched record; else createdAt plus the setup
// grace for a pending-setup husk; a record with nothing parseable is already
// due — so the deadline is always finite.
func drainDeadline(records []jobRecord, now time.Time) time.Time {
	var deadline time.Time
	for _, record := range records {
		due := recordDrainDue(record.doc, now)
		if due.After(deadline) {
			deadline = due
		}
	}
	return deadline
}

// recordDrainDue is one record's drain clock.
func recordDrainDue(doc map[string]any, now time.Time) time.Time {
	if status, _ := doc["status"].(string); status == "pending-setup" {
		if created, ok := doc["createdAt"].(string); ok {
			if at, err := time.Parse(time.RFC3339, created); err == nil {
				return at.Add(dispatch.AbandonedSetupGrace)
			}
		}
		return now
	}
	if deadline, ok := doc["capDeadline"].(string); ok && deadline != "" {
		if at, err := time.Parse(time.RFC3339, deadline); err == nil {
			return at.Add(handshakeGrace)
		}
	}
	capMin, hasCap := jsonInt(doc["capMin"])
	if started, ok := doc["startedAt"].(string); ok && hasCap && capMin >= 1 {
		if at, err := time.Parse(time.RFC3339, started); err == nil {
			return at.Add(time.Duration(capMin)*time.Minute + handshakeGrace)
		}
	}
	return now
}

// reapReservedRecords applies the runner's mission-scoped reap to every live
// record the mission's fence reservations name. Facts that cannot be read
// prove nothing, and a record the CAS finds advanced is left alone this
// pass — the deadline, not the loop, decides the outcome.
func (e *Engine) reapReservedRecords(now time.Time) error {
	reserved := reservedJobIDs(e.Root, e.Mission)
	var firstErr error
	for _, record := range activeJobRecords(e.Root, e.Mission) {
		job := jobRecordID(record)
		if !reserved[job] {
			// The runner's authority ends at its reservation set: a record
			// it merely sees is never a record it may judge.
			continue
		}
		facts, err := dispatch.ComputeReapFacts(record.path, dispatch.HandshakeBackstopGraceSec, now)
		if err != nil {
			continue
		}
		if err := e.applyReapVerdict(job, record.doc, facts); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// applyReapVerdict maps one candidate's proven facts to its terminal
// verdict, the standing reaper's fixed mapping: a pending-setup husk past
// the setup grace provably never launched (no process was ever recorded, so
// its abandoned-setup fact suffices); a record with no recorded process is
// never reapable (the process may exist unrecorded — no proof, no verdict);
// a custodian that is not provably dead reaps nothing (Unknown never acts);
// a proven-dead custodian books budget expiry first (timeout/budget-cap on a
// running record) and process loss otherwise.
func (e *Engine) applyReapVerdict(job string, doc map[string]any, facts dispatch.ReapFacts) error {
	switch facts.Status {
	case "pending-setup":
		if facts.SetupAbandoned {
			e.reapCAS(job, "pending-setup", "failed", map[string]any{
				"error": "abandoned-setup", "phase": "claim-sweep",
			})
		}
	case "pending", "running":
		pid, ok := jsonInt(doc["pid"])
		if !ok || pid < 1 {
			// A marked record with no recorded process mirrors the
			// supervision reaper's exception: nothing else will ever
			// conclude it, and no death is claimed.
			if phase, _ := doc["phase"].(string); phase == "cancelling" {
				e.reapCAS(job, facts.Status, "cancelled", map[string]any{
					"error": nil, "phase": "supervision",
				})
			}
			return nil
		}
		start, _ := jsonInt(doc["pidStartedAt"])
		tag, _ := doc["instanceTag"].(string)
		if e.custodian(pid, start, tag) != identity.Dead {
			return nil
		}
		// A dead custodian is only HALF the proof groupDeathProvenAt
		// claims: a tagged survivor means the group lives, and this
		// kill-less reap must defer to the kill-capable dispatch path
		// that can wind it down. Indeterminacy defers the same way.
		recPgid, _ := jsonInt(doc["pgid"])
		if alive, certain := e.taggedSurvivors(tag, pid, recPgid); !certain || alive {
			return nil
		}
		// A cancel marks the record before it kills: a dead marked
		// group is that cancel's outcome here exactly as in the
		// supervision reaper — the marker outranks the budget.
		if phase, _ := doc["phase"].(string); phase == "cancelling" {
			e.reapCAS(job, facts.Status, "cancelled", map[string]any{
				"error": nil, "phase": "supervision",
				"groupDeathProvenAt": nowISO(),
			})
			return nil
		}
		if facts.Status == "running" && facts.BudgetExpired {
			applied, _ := e.reapCAS(job, "running", "timeout", map[string]any{
				"error": "budget-cap", "phase": "supervision",
				"groupDeathProvenAt": nowISO(),
			})
			if !applied {
				return nil
			}
			missionID, _ := doc["mission"].(string)
			if missionID == "" {
				return nil
			}
			resolution, _ := doc["capResolution"].(map[string]any)
			return mission.RefuseBudgetCap(e.Root, missionID, job, resolution, func(line string) {
				e.emit("job-refused", line, map[string]string{
					"missionId": missionID, "jobId": job, "reasonClass": "fence-ask",
				})
			})
		}
		e.reapCAS(job, facts.Status, "failed", map[string]any{
			"error": "process-lost", "phase": "supervision",
			"groupDeathProvenAt": nowISO(),
		})
	}
	return nil
}

// reapCAS lands one reap verdict through the existing record CAS under the
// record lock — lawful transitions only. A lost compare means the record
// advanced under someone else's authority and is left exactly as it is.
func (e *Engine) reapCAS(job, expect, target string, patch map[string]any) (bool, error) {
	source, err := os.CreateTemp("", "mission-reap-patch.*.json")
	if err != nil {
		return false, err
	}
	sourcePath := source.Name()
	source.Close()
	defer os.Remove(sourcePath)
	if err := atomicWriteJSON(sourcePath, patch); err != nil {
		return false, err
	}
	observed, err := dispatch.RecordCAS(e.Root, job, expect, target, sourcePath)
	if observed != "" {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	verdict, _ := patch["error"].(string)
	e.emit("mission-reap", fmt.Sprintf("job %s reaped %s -> %s (%s)", job, expect, target, verdict), map[string]string{
		"missionId": e.Mission, "jobId": job, "verdict": verdict,
	})
	return true, nil
}

// taggedSurvivors runs the group-death half of the reap proof through
// the engine's bound scanner: identity.TaggedSurvivors in production,
// a fake in tests.
func (e *Engine) taggedSurvivors(tag string, exclude, pgid int64) (bool, bool) {
	if e.survivorsFn != nil {
		return e.survivorsFn(tag, exclude, pgid)
	}
	return identity.TaggedSurvivors(tag, exclude, pgid)
}

// custodian proves a recorded custodian through the engine's bound prober:
// the shared kernel custodian discipline in production, a fake table in
// tests.
func (e *Engine) custodian(pid, start int64, tag string) identity.Liveness {
	if e.custodianFn != nil {
		return e.custodianFn(pid, start, tag)
	}
	authorization, err := e.fixtures()
	if err != nil {
		return identity.Unknown // leaked fixture authorizes nothing
	}
	return identity.Custodian(pid, start, tag, authorization.Identity())
}

// drainSurvivors snapshots the live set for the park ask: id, status, age,
// and the proof each record is missing. Best-known-at-park — the ask advises
// the human; the resume re-proves against the live set.
func (e *Engine) drainSurvivors(records []jobRecord, now time.Time) []drainSurvivor {
	survivors := make([]drainSurvivor, 0, len(records))
	for _, record := range records {
		status, _ := record.doc["status"].(string)
		survivors = append(survivors, drainSurvivor{
			ID:           jobRecordID(record),
			Status:       status,
			Age:          survivorAge(record.doc, now),
			MissingProof: e.survivorMissingProof(record, now),
		})
	}
	return survivors
}

// survivorAge renders how long a record has existed, from its start when it
// launched, else its creation.
func survivorAge(doc map[string]any, now time.Time) string {
	for _, key := range []string{"startedAt", "createdAt"} {
		if raw, ok := doc[key].(string); ok {
			if at, err := time.Parse(time.RFC3339, raw); err == nil {
				age := now.Sub(at).Round(time.Second)
				if age < 0 {
					age = 0
				}
				return age.String()
			}
		}
	}
	return "unknown"
}

// survivorMissingProof words why the runner could not fail this record: the
// human's side of invariant 5 — no proof means no verdict.
func (e *Engine) survivorMissingProof(record jobRecord, now time.Time) string {
	facts, err := dispatch.ComputeReapFacts(record.path, dispatch.HandshakeBackstopGraceSec, now)
	if err != nil {
		return "record facts are unreadable"
	}
	if facts.Status == "pending-setup" {
		if facts.SetupAbandoned {
			return "abandoned setup, record update refused"
		}
		return "inside the setup grace, no process was ever recorded"
	}
	pid, ok := jsonInt(record.doc["pid"])
	if !ok || pid < 1 {
		if facts.HandshakeWaiting {
			return "no recorded process yet, handshake window still open"
		}
		return "handshake expired, no recorded process to prove"
	}
	start, _ := jsonInt(record.doc["pidStartedAt"])
	tag, _ := record.doc["instanceTag"].(string)
	switch e.custodian(pid, start, tag) {
	case identity.Alive:
		return "process is still alive, no death to prove"
	case identity.Dead:
		return "custodian proven dead, record update refused"
	default:
		return "process identity unreadable, and unknown never reaps"
	}
}

// drainStalledQuestion words the survivors ask in one line: every survivor
// as snapshotted (id, status, age, missing proof) and the resume: answer
// that unparks.
func drainStalledQuestion(cycle int64, survivors []drainSurvivor) string {
	if len(survivors) == 0 {
		return fmt.Sprintf("The job drain stalled in cycle %d and no unprovable job remains on re-check. Answer resume:<note> to resume the mission.", cycle)
	}
	parts := make([]string, 0, len(survivors))
	for _, survivor := range survivors {
		parts = append(parts, fmt.Sprintf("%s (status=%s, age=%s, missing proof: %s)",
			survivor.ID, survivor.Status, survivor.Age, survivor.MissingProof))
	}
	noun := "job the runner cannot prove dead"
	if len(survivors) > 1 {
		noun = "jobs the runner cannot prove dead"
	}
	return fmt.Sprintf("The job drain stalled in cycle %d: the deadline passed with %d %s: %s. Verify or clear these jobs, then answer resume:<note>; the resume re-proves against the live set.",
		cycle, len(survivors), noun, strings.Join(parts, "; "))
}

// drainStalledAsk shapes the survivors ask: the human-readable question plus
// the snapshot the resume: answer copies into lastDrainStall.
func drainStalledAsk(askID, streamID string, cycle int64, survivors []drainSurvivor) map[string]any {
	ids := make([]any, 0, len(survivors))
	for _, survivor := range survivors {
		ids = append(ids, survivor.ID)
	}
	ask := askRecord(askID, streamID, drainStalledReason, drainStalledQuestion(cycle, survivors), nowISO())
	ask["drainStall"] = map[string]any{"cycle": cycle, "survivors": ids}
	return ask
}

// parkDrainStalled parks the mission at the drain deadline: reason
// drain-stalled, the survivors ask, NO ledger line and no claim — the
// reserved cycle simply never concluded, which is exactly the state the
// reserve/append heal recovers after the human's resume: answer. The write
// order is state, anchor, then ask: a crash before the ask leaves a parked
// state whose missing ask `mission-runner resume` re-raises idempotently.
func (e *Engine) parkDrainStalled(statePath, ledger, identityName string, cycle int64, survivors []drainSurvivor) (map[string]any, error) {
	e.emit("mission-parked", clipSummary(fmt.Sprintf("drain-stalled: %d unprovable survivors", len(survivors))), map[string]string{
		"missionId": e.Mission, "parkReason": drainStalledReason,
	})
	state, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	streams, ok := state["streams"].(map[string]any)
	if !ok || len(streams) == 0 {
		return nil, failf(3, "mission state has no streams")
	}
	asksDir := asksDirPath(e.Root, e.Mission)
	if err := os.MkdirAll(asksDir, 0o755); err != nil {
		return nil, err
	}
	askID := nextAskID(asksDir, drainStalledReason, map[string]bool{})
	proposed := deepCopyDoc(state)
	proposed["status"] = "parked"
	proposed["parkReason"] = drainStalledReason
	proposed["gatePassed"] = false
	proposed["waitingList"] = mergedOpenAskIDs(asksDir, []string{askID})
	aggregateUsageForProjection(e.Root, e.Mission, "park-drain-stalled")
	if err := ProjectFences(e.Root, e.Mission, proposed); err != nil {
		return nil, err
	}
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		return nil, err
	}
	if err := e.anchor(statePath, ledger, identityName); err != nil {
		return nil, err
	}
	if err := e.writeProposedAsks([]map[string]any{drainStalledAsk(askID, fallbackStream(streams), cycle, survivors)}); err != nil {
		return nil, err
	}
	return updated, nil
}

// ensureDrainStallAsk re-raises a drain-stalled park's missing ask — the
// recovery for a crash between the park's state write and its ask write, run
// by resume before anything else. Idempotent: an open drain-stalled ask on
// disk means nothing to do. The snapshot is re-derived from the LIVE set,
// the same set the resume re-proves against.
func (e *Engine) ensureDrainStallAsk(state map[string]any) error {
	asksDir := asksDirPath(e.Root, e.Mission)
	if hasOpenAskWithReason(asksDir, drainStalledReason) {
		return nil
	}
	streams, ok := state["streams"].(map[string]any)
	if !ok || len(streams) == 0 {
		return failf(3, "mission state has no streams")
	}
	fences, err := readDocLabeled(e.fencesPath(), "mission fence counters", 3)
	if err != nil {
		return err
	}
	cycle, ok := jsonInt(fences["cycles"])
	if !ok || cycle < 1 {
		return failf(3, "mission fence counters carry an invalid cycle count")
	}
	if err := os.MkdirAll(asksDir, 0o755); err != nil {
		return err
	}
	now := time.Now()
	survivors := e.drainSurvivors(activeJobRecords(e.Root, e.Mission), now)
	askID := nextAskID(asksDir, drainStalledReason, map[string]bool{})
	e.emit("drain-stall-ask-reraised", fmt.Sprintf("resume re-raised the missing %s ask", drainStalledReason), map[string]string{
		"missionId": e.Mission, "askId": askID,
	})
	return e.writeProposedAsks([]map[string]any{drainStalledAsk(askID, fallbackStream(streams), cycle, survivors)})
}
