package missionrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The detached run loop: the process that holds the mission lease, records
// itself, and turns cycles until the mission leaves the running state. State
// writes go through the hash-chained compare-and-write and every applied
// proposal is anchored, so the state's single-writer discipline survives the
// runner's crash at any point.

// RunLoop is the detached runner process's whole life. It establishes the
// runner's recorded identity, then hands off to the loop; the exit code is
// the process exit code.
func (e *Engine) RunLoop(mode, tag, startSignal string) int {
	if started, err := processStartedAt(os.Getpid()); err == nil {
		e.emitter.PidStartedAt = started
	}
	e.emit("runner-started", "mode="+mode, map[string]string{"missionId": e.Mission})
	return e.internalRun(mode, tag, startSignal)
}

func (e *Engine) internalRun(mode, tag, startSignal string) int {
	recordPath, _, _ := e.runnerPaths()
	pid := os.Getpid()
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	started, err := processStartedAt(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}
	if err := atomicWriteJSON(recordPath, e.runnerRecord(pid, pgid, started, tag)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	if err := e.heartbeat(nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}

	notified := false
	leaseHeld := false
	// fail is the one exit ramp for a runner that dies mid-mission: tell the
	// launcher (when it is still waiting), settle the usage books, free the
	// lease, finalize the record, and surface the refusal's own exit code.
	// The ramp writes NO ledger annotation — it fires for lease-acquisition
	// failures, pre-cycle init errors, and mid-run errors alike, so no
	// ledger position is safe here; usage aggregation is idempotent and
	// needs no ledger block, and an interrupted mission's landed returns are
	// re-listed by its next assembled prompt on resume.
	fail := func(err error) int {
		if !notified {
			_ = writeStartSignal(startSignal, false, nil, err.Error())
		}
		if leaseHeld {
			aggregateUsageForProjection(e.Root, e.Mission, "failure-ramp")
			e.releaseLease()
			leaseHeld = false
		}
		e.finishRunner("failed", err.Error())
		return exitFor(err)
	}
	defer func() {
		if leaseHeld {
			e.releaseLease()
		}
	}()

	leasePath, err := e.acquireLease(tag)
	if err != nil {
		return fail(err)
	}
	leaseHeld = true

	var statePath, ledger string
	var state map[string]any
	if mode == "start" {
		statePath, ledger, state, err = e.initializeState(leasePath)
	} else {
		statePath, ledger, state, err = e.resumeState()
	}
	if err != nil {
		return fail(err)
	}
	for state["status"] == "running" {
		if err := e.heartbeat(nil); err != nil {
			return fail(err)
		}
		if state, err = e.oneCycle(statePath, ledger, state, leasePath, startSignal, &notified); err != nil {
			return fail(err)
		}
	}
	if err := e.closeTerminalChains(); err != nil {
		return fail(err)
	}
	if !notified {
		message := fmt.Sprintf("mission parked before a host turn started: %s", valueString(state["parkReason"]))
		if err := writeStartSignal(startSignal, false, nil, message); err != nil {
			return fail(err)
		}
	}
	e.releaseLease()
	leaseHeld = false
	e.finishRunner("completed", nil)
	return 0
}

// runnerRecord is the runner's process record, the artifact status reads to
// decide whether anyone is actually driving a running mission.
func (e *Engine) runnerRecord(pid, pgid int, started int64, tag string) map[string]any {
	return map[string]any{
		"missionId":     e.Mission,
		"status":        "running",
		"error":         nil,
		"workspaceRoot": e.Root,
		"pid":           pid,
		"pidStartedAt":  started,
		"pgid":          pgid,
		"instanceTag":   tag,
		"startedAt":     nowISO(),
		"endedAt":       nil,
	}
}

// finishRunner finalizes the runner record. Best-effort by design: the
// record is a witness for status, and failing to finalize it must not change
// how the mission itself ended.
func (e *Engine) finishRunner(status string, errMsg any) {
	if status == "failed" {
		message := valueString(errMsg)
		if message == "" {
			message = "unknown"
		}
		e.emit("runner-failed", clipSummary(message), map[string]string{
			"missionId": e.Mission, "error": message,
		})
	}
	recordPath, _, _ := e.runnerPaths()
	record, err := readDocLabeled(recordPath, "mission runner record", 3)
	if err != nil {
		return
	}
	record["status"] = status
	record["error"] = errMsg
	record["endedAt"] = nowISO()
	_ = atomicWriteJSON(recordPath, record)
}

// heartbeat republishes the runner's liveness, carrying the turn it is
// driving when one is in flight.
func (e *Engine) heartbeat(turnID any) error {
	recordPath, heartbeatPath, _ := e.runnerPaths()
	record, err := readDocLabeled(recordPath, "mission runner record", 3)
	if err != nil {
		return err
	}
	return atomicWriteJSON(heartbeatPath, map[string]any{
		"function":        "mission-runner",
		"missionId":       e.Mission,
		"turnId":          turnID,
		"pid":             record["pid"],
		"pidStartedAt":    record["pidStartedAt"],
		"instanceTag":     record["instanceTag"],
		"observedAtEpoch": time.Now().Unix(),
	})
}

// acquireLease takes the mission's single-runner lease: the marker directory
// is the atomic claim, the owner record and lease file describe the claimant.
func (e *Engine) acquireLease(tag string) (string, error) {
	dir := e.missionDir()
	marker := filepath.Join(dir, "lease.d")
	leasePath := filepath.Join(dir, "lease.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.Mkdir(marker, 0o755); err != nil {
		if os.IsExist(err) {
			return "", failf(3, "mission lease is busy")
		}
		return "", err
	}
	pid := os.Getpid()
	// The probe is a guard: a runner whose own identity cannot be read must
	// not hold a lease other processes will judge by that identity.
	if _, err := processStartedAt(pid); err != nil {
		_ = os.Remove(marker)
		return "", err
	}
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		_ = os.Remove(marker)
		return "", err
	}
	now := nowISO()
	value := map[string]any{
		"missionId":   e.Mission,
		"pid":         pid,
		"pgid":        pgid,
		"instanceTag": tag,
		"startedAt":   now,
		"renewedAt":   now,
	}
	if err := atomicWriteJSON(filepath.Join(marker, "owner.json"), value); err != nil {
		return "", err
	}
	if err := atomicWriteJSON(leasePath, value); err != nil {
		return "", err
	}
	return leasePath, nil
}

// releaseLease frees the mission lease; missing pieces are already released.
func (e *Engine) releaseLease() {
	dir := e.missionDir()
	marker := filepath.Join(dir, "lease.d")
	_ = os.Remove(filepath.Join(marker, "owner.json"))
	_ = os.Remove(marker)
	_ = os.Remove(filepath.Join(dir, "lease.json"))
}

// verifyState validates the mission state — shape only, or shape plus ledger
// anchor — and returns the verified document.
func (e *Engine) verifyState(statePath string, anchor bool) (map[string]any, error) {
	var err error
	if anchor {
		_, _, err = mission.VerifyStateWithAnchor(statePath, e.Root, filepath.Join(filepath.Dir(statePath), "ledger.md"))
	} else {
		_, _, err = mission.VerifyStateShape(statePath)
	}
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if message == "" {
			message = "mission state is unreadable"
		}
		return nil, failf(7, "%s", message)
	}
	return readDocLabeled(statePath, "mission state", 7)
}

// writeState advances the state through its compare-and-write: the proposal
// lands only against the hash the runner last read.
func (e *Engine) writeState(statePath string, proposed map[string]any) (map[string]any, error) {
	current, err := readDocLabeled(statePath, "mission state", 7)
	if err != nil {
		return nil, err
	}
	integrity, _ := current["integrity"].(map[string]any)
	expected, ok := integrity["hash"].(string)
	if !ok {
		return nil, failf(7, "mission state integrity hash is unreadable")
	}
	source, err := os.CreateTemp("", "mission-state-proposed.*.json")
	if err != nil {
		return nil, err
	}
	sourcePath := source.Name()
	defer os.Remove(sourcePath)
	data, err := json.MarshalIndent(proposed, "", "  ")
	if err != nil {
		source.Close()
		return nil, err
	}
	if _, err := source.Write(append(data, '\n')); err != nil {
		source.Close()
		return nil, err
	}
	if err := source.Close(); err != nil {
		return nil, err
	}
	if err := mission.WriteState(statePath, sourcePath, expected); err != nil {
		message := strings.TrimSpace(err.Error())
		if message == "" {
			message = "mission state update refused"
		}
		return nil, failf(3, "%s", message)
	}
	return readDocLabeled(statePath, "mission state", 7)
}

// anchorState writes the local anchor commit binding the state hash and
// ledger. It runs through the binary rather than in-process so the anchor's
// git author can be pinned to the acting identity per invocation and its
// printed commit sha stays out of the runner's own output.
func (e *Engine) anchorState(statePath, ledgerPath, identityName string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	stdout, stderr, code := runCaptured(e.Root, gitAuthorEnvironment(identityName), self,
		"mission-state", "anchor", "--state", statePath, "--repo", e.Root, "--ledger", ledgerPath)
	if code != 0 {
		return failf(3, "mission anchor refused: %s", firstDetail(stderr, stdout))
	}
	return nil
}

// initializeState creates the mission's ledger and state from the pinned
// contract and anchors the opening position.
func (e *Engine) initializeState(leasePath string) (statePath, ledger string, state map[string]any, err error) {
	dir := e.missionDir()
	statePath = filepath.Join(dir, "state.json")
	ledger = filepath.Join(dir, "ledger.md")
	if pathExists(statePath) || pathExists(ledger) {
		return "", "", nil, failf(3, "mission state already exists; use resume")
	}
	_, values, _, err := e.parseContract(true)
	if err != nil {
		return "", "", nil, err
	}
	cycleBudget, err := intFromString(values["ledger.cycle-budget"])
	if err != nil {
		return "", "", nil, failf(3, "mission ledger initialization refused: %v", err)
	}
	noGainBudget, err := intFromString(values["ledger.no-gain-budget"])
	if err != nil {
		return "", "", nil, failf(3, "mission ledger initialization refused: %v", err)
	}
	if err := mission.InitLedger(ledger, cycleBudget, noGainBudget); err != nil {
		return "", "", nil, failf(3, "mission ledger initialization refused: %v", err)
	}
	if err := mission.InitState(statePath, e.approvedContractPath(), ledger, leasePath, ""); err != nil {
		return "", "", nil, failf(3, "mission state initialization refused: %v", err)
	}
	if err := e.anchor(statePath, ledger, e.Mission); err != nil {
		return "", "", nil, err
	}
	state, err = e.verifyState(statePath, true)
	if err != nil {
		return "", "", nil, err
	}
	return statePath, ledger, state, nil
}

// resumeState reconciles an existing mission's state against its ledger and
// anchor and refuses to drive anything but a running mission. A stop-loss
// park whose reset ask was answered but whose unpark never landed (a crash
// after the ask was marked answered) is applied here: the ledger's reset
// line is the authoritative fact, and resume applies its state effect. A
// cycle that was reserved in the fence counters but never reached its ledger
// append — a runner death anywhere inside the turn — is healed here too,
// recorded honestly as a lost turn (healReservedCycle).
func (e *Engine) resumeState() (statePath, ledger string, state map[string]any, err error) {
	dir := e.missionDir()
	statePath = filepath.Join(dir, "state.json")
	ledger = filepath.Join(dir, "ledger.md")
	if !pathExists(statePath) {
		return "", "", nil, failf(7, "mission state does not exist")
	}
	code, reconcileErr := mission.Reconcile(statePath, e.Root, ledger)
	if reconcileErr != nil || code != 0 {
		detail := ""
		if reconcileErr != nil {
			detail = reconcileErr.Error()
		}
		return "", "", nil, failf(3, "mission state reconciliation parked the mission: %s", detail)
	}
	state, err = e.verifyState(statePath, false)
	if err != nil {
		return "", "", nil, err
	}
	if state["status"] == "parked" && state["parkReason"] == "stop-loss" {
		applied, err := e.applyPendingReset(statePath, ledger, state)
		if err != nil {
			return "", "", nil, err
		}
		if applied {
			state, err = e.verifyState(statePath, false)
			if err != nil {
				return "", "", nil, err
			}
		}
	}
	if state["status"] == "parked" && state["parkReason"] == drainStalledReason {
		// The drain-stalled park writes state then ask; a crash between the
		// two leaves a park nobody can answer. Re-raise the missing ask
		// idempotently before anything else — resume is the command the
		// human was already going to run.
		if err := e.ensureDrainStallAsk(state); err != nil {
			return "", "", nil, err
		}
	}
	if state["status"] != "running" {
		return "", "", nil, failf(3, "mission is %s; answer or amend its park reason before resume", valueString(state["status"]))
	}
	if _, err := e.healReservedCycle(statePath, ledger, state); err != nil {
		return "", "", nil, err
	}
	state, err = e.verifyState(statePath, true)
	if err != nil {
		return "", "", nil, err
	}
	return statePath, ledger, state, nil
}

// healReservedCycle repairs the reserve/append crash window on resume: a
// cycle spends its number in the fence counters before anything else exists
// and lands in the ledger only at conclusion, so a runner death in between
// leaves the fences exactly one cycle ahead of the ledger. Every later
// append would pass the fence-derived number, which the ledger's contiguity
// check rightly refuses — without this heal the mission wedges at its first
// append instead of parking. The heal supplies the missing truth rather than
// weakening that check: it records the reserved cycle as a lost turn —
// classification no-progress, observed unmeasurable:turn-lost, candidate sha
// the repository HEAD at heal time — then advances the state and anchors,
// the same binding order the failed-turn path uses. One gap kind is
// distinguishable: a drain-stalled park whose resume: answer left the
// lastDrainStall label naming this exact cycle heals as
// unmeasurable:drain-stalled with the survivor-count annotation, the label
// consumed in the same conclude write. The append runs under
// the ledger lock, and its contiguity check makes healing idempotent: once
// the line exists the fences are no longer ahead, and a stale second heal is
// refused rather than double-appended. It reports whether it healed.
func (e *Engine) healReservedCycle(statePath, ledger string, state map[string]any) (bool, error) {
	if !pathExists(e.fencesPath()) {
		return false, nil
	}
	fences, err := readDocLabeled(e.fencesPath(), "mission fence counters", 3)
	if err != nil {
		return false, err
	}
	spent, ok := jsonInt(fences["cycles"])
	if !ok {
		return false, failf(3, "mission fence counters carry an invalid cycle count")
	}
	_, _, cycles, err := mission.ParseLedger(ledger)
	if err != nil {
		return false, failf(3, "mission resume cannot read the ledger: %v", err)
	}
	if spent != int64(len(cycles))+1 {
		return false, nil
	}
	candidateSHA, err := e.gitRevParse("HEAD")
	if err != nil {
		return false, err
	}
	// When the unpark's durable label names exactly this reserved cycle, the
	// gap is a drain-stalled park, not a plain lost turn: book it
	// distinguishably (observed unmeasurable:drain-stalled, plus the
	// survivor-count annotation) and consume the label in the same conclude
	// write. Any other gap heals as turn-lost, exactly as before.
	observed := "unmeasurable:turn-lost"
	var annotations []string
	drainStalled := false
	if stall, ok := state["lastDrainStall"].(map[string]any); ok {
		if stallCycle, ok := jsonInt(stall["cycle"]); ok && stallCycle == spent {
			survivors, _ := stall["survivors"].([]any)
			observed = mission.DrainStalledObserved
			annotations = []string{mission.DrainStalledAnnotation(len(survivors))}
			drainStalled = true
		}
	}
	if err := e.appendLedger(state, ledger, spent, "no-progress", candidateSHA, observed, nil, annotations...); err != nil {
		return false, err
	}
	proposed := deepCopyDoc(state)
	if drainStalled {
		delete(proposed, "lastDrainStall")
	}
	ledgerRef, ok := proposed["ledger"].(map[string]any)
	if !ok {
		return false, failf(3, "mission state ledger reference is unreadable")
	}
	ledgerRef["cycles"] = spent
	if fencesRef, ok := proposed["fences"].(map[string]any); ok {
		if recorded, ok := jsonInt(fencesRef["cycles"]); !ok || recorded < spent {
			fencesRef["cycles"] = spent
		}
	}
	if _, err := e.writeState(statePath, proposed); err != nil {
		return false, err
	}
	if err := e.anchor(statePath, ledger, e.Mission); err != nil {
		return false, err
	}
	summary := fmt.Sprintf("resume recorded reserved cycle %d as a lost turn", spent)
	if drainStalled {
		summary = fmt.Sprintf("resume recorded reserved cycle %d as drain-stalled", spent)
	}
	e.emit("reserved-cycle-healed", summary, map[string]string{
		"missionId": e.Mission, "cycle": fmt.Sprintf("%d", spent),
	})
	return true, nil
}

// applyPendingReset applies the unpark a recorded stop-loss reset still owes:
// the ledger's last event is a reset line whose named ask was answered with
// reset: on a stagnation park, yet the mission is still parked. It reports
// whether it unparked; a reset line whose ask is still open is left for the
// human to answer.
func (e *Engine) applyPendingReset(statePath, ledger string, state map[string]any) (bool, error) {
	_, _, events, err := mission.ParseLedgerEvents(ledger)
	if err != nil {
		return false, failf(3, "mission stop-loss replay refused: %v", err)
	}
	if len(events) == 0 {
		return false, nil
	}
	last := events[len(events)-1]
	if !last.Reset {
		return false, nil
	}
	ask, err := readJSONDoc(filepath.Join(asksDirPath(e.Root, e.Mission), last.AskID+".json"))
	if err != nil {
		return false, nil
	}
	answer, _ := ask["answer"].(string)
	if ask["askId"] != last.AskID || ask["answeredAt"] == nil ||
		!strings.HasPrefix(answer, "reset:") || ask["stopLossKind"] != StopLossStagnation {
		return false, nil
	}
	proposed := deepCopyDoc(state)
	proposed["status"] = "running"
	proposed["parkReason"] = nil
	proposed["gatePassed"] = false
	proposed["waitingList"] = openAskIDs(asksDirPath(e.Root, e.Mission))
	if _, err := e.writeState(statePath, proposed); err != nil {
		return false, err
	}
	if err := e.anchor(statePath, ledger, e.Mission); err != nil {
		return false, err
	}
	e.emit("stop-loss-reset-applied", clipSummary("resume applied the recorded reset"), map[string]string{
		"missionId": e.Mission, "askId": last.AskID,
	})
	return true, nil
}

// allocateTurn mints this cycle's turn id and its directory. The directory
// creation is the collision check.
func (e *Engine) allocateTurn(cycle int64) (turnID, turnDir string, err error) {
	turnID = fmt.Sprintf("%s-t%d-%s", e.Mission, cycle, randomHex(2))
	turnDir = filepath.Join(e.missionDir(), "turns", turnID)
	if err := os.MkdirAll(filepath.Dir(turnDir), 0o755); err != nil {
		return "", "", err
	}
	if err := os.Mkdir(turnDir, 0o755); err != nil {
		if os.IsExist(err) {
			return "", "", failf(3, "turn id collision refused: %s", turnID)
		}
		return "", "", err
	}
	return turnID, turnDir, nil
}

// writeProposedAsks writes the ask records a proposal produced, exactly as
// proposed. The proposal's waiting list assumes these asks land; writing
// anything else would make the state lie about what can be answered.
func (e *Engine) writeProposedAsks(asks []map[string]any) error {
	asksDir := asksDirPath(e.Root, e.Mission)
	for _, ask := range asks {
		askID, _ := ask["askId"].(string)
		if err := atomicWriteJSON(filepath.Join(asksDir, askID+".json"), ask); err != nil {
			return err
		}
	}
	return nil
}

// parkState parks the mission for a reason, writing the asks the park
// proposal raised so a human can always answer the park.
func (e *Engine) parkState(statePath, ledger, reason, identityName string) (map[string]any, error) {
	e.emit("mission-parked", clipSummary(reason), map[string]string{
		"missionId": e.Mission, "parkReason": reason,
	})
	state, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	outcome, err := ParkProposal(e.Root, e.Mission, state, reason, nowISO())
	if err != nil {
		return nil, err
	}
	return e.applyPark(statePath, ledger, identityName, outcome)
}

// parkStopLoss parks the mission on a tripped stop-loss verdict, with the
// ask worded for the trip kind: a stagnation park names the vocal reset:
// answer, a cycle-budget park is amendment-only, and a legacy-semantics park
// keeps the wording those missions started with.
func (e *Engine) parkStopLoss(statePath, ledger, identityName string, verdict *StopLossVerdict) (map[string]any, error) {
	e.emit("mission-parked", clipSummary("stop-loss: "+verdict.Detail), map[string]string{
		"missionId": e.Mission, "parkReason": "stop-loss", "stopLossKind": verdict.Kind,
	})
	state, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	outcome, err := StopLossParkProposal(e.Root, e.Mission, state, verdict.Kind, verdict.askQuestion(), nowISO())
	if err != nil {
		return nil, err
	}
	return e.applyPark(statePath, ledger, identityName, outcome)
}

// applyPark applies a park proposal: asks first so the state never names an
// unanswerable ask, then the state write, then its anchor.
func (e *Engine) applyPark(statePath, ledger, identityName string, outcome *ParkOutcome) (map[string]any, error) {
	if err := e.writeProposedAsks(outcome.Asks); err != nil {
		return nil, err
	}
	updated, err := e.writeState(statePath, outcome.State)
	if err != nil {
		return nil, err
	}
	if err := e.anchor(statePath, ledger, identityName); err != nil {
		return nil, err
	}
	return updated, nil
}

// gitRevParse resolves a ref in the mission's repository.
func (e *Engine) gitRevParse(ref string) (string, error) {
	stdout, stderr, code := runCaptured(e.Root, nil, "git", "-C", e.Root, "rev-parse", ref)
	if code != 0 {
		return "", failf(3, "cannot resolve candidate sha: %s", firstDetail(stderr, stdout))
	}
	return strings.TrimSpace(stdout), nil
}

// appendLedger records one cycle's verdict in the stop-loss ledger, stamping
// the new-best marker on missions pinned to the replay semantics; a
// legacy-semantics mission keeps marker-less lines so it finishes under the
// rules it started with. Annotations land in the same atomic append, as
// separate lines beside the classification line.
func (e *Engine) appendLedger(state map[string]any, ledger string, cycle int64, classification, candidateSHA, observed string, inflightCertified any, annotations ...string) error {
	best, err := e.bestMarker(state, ledger, observed)
	if err != nil {
		return err
	}
	// Patience rides the SAME atomic append as the cycle line, on every
	// booking path (plans/patience-satellite-4.md): the shared function is
	// what makes ordinary, faulted, failed, and heal bookings all evaluate.
	annotations = append(append([]string(nil), annotations...),
		e.patienceBookingAnnotations(state, inflightCertified)...)
	if err := mission.AppendCycle(ledger, int(cycle), classification, candidateSHA, observed, best, annotations...); err != nil {
		return failf(3, "mission ledger append refused: %v", err)
	}
	return nil
}

// patienceBookingAnnotations derives this booking's Patience lines from the
// sealed floors, the mission's job records, the durable turn log, and the
// in-flight conclusion's certified entries (accepted returns only — a
// rejected return's certifications witness nothing). Unconfigured missions
// return nothing and stay byte-identical.
func (e *Engine) patienceBookingAnnotations(state map[string]any, inflightCertified any) []string {
	_, authored, _, err := e.parseContract(true)
	if err != nil {
		return nil
	}
	floors := parsePatienceFloors(authored)
	if len(floors) == 0 {
		return nil
	}
	turnLog, _ := state["turnLog"].([]any)
	return patienceEvaluate(floors, missionJobs(e.Root, e.Mission), turnLog, inflightCertified)
}

// continueOrParkStopLoss derives the stop-loss verdict for a still-running
// mission and parks it when the fuse fired; the verdict is a pure replay of
// (sealed contract, ledger), never a cached counter.
func (e *Engine) continueOrParkStopLoss(statePath, ledger, identityName string, updated map[string]any) (map[string]any, error) {
	if updated["status"] != "running" {
		return updated, nil
	}
	verdict, err := e.stopLossVerdict(updated, ledger)
	if err != nil {
		return nil, err
	}
	if !verdict.Tripped {
		return updated, nil
	}
	return e.parkStopLoss(statePath, ledger, identityName, verdict)
}

// recordFailedTurn spends the failed turn's cycle in the ledger, applies the
// failure proposal, and follows the proposal's park decision (including the
// stop-loss check when the mission keeps running).
func (e *Engine) recordFailedTurn(statePath, ledger string, state map[string]any, turnPath, detail, outcome string, consecutiveFailures int) (map[string]any, error) {
	branch, _ := state["branch"].(string)
	candidateSHA, err := e.gitRevParse(branch)
	if err != nil {
		return nil, err
	}
	turnDoc, err := readDocLabeled(turnPath, "turn record", 3)
	if err != nil {
		return nil, err
	}
	turn, err := TurnFromDoc(turnDoc)
	if err != nil {
		return nil, err
	}
	cycle, ok := jsonInt(turn.Cycle)
	if !ok {
		return nil, failf(3, "turn record cycle is invalid")
	}
	observed := "unmeasurable:" + strings.ReplaceAll(detail, "\n", " ")
	if err := e.appendLedger(state, ledger, cycle, "no-progress", candidateSHA, observed, nil); err != nil {
		return nil, err
	}
	diskState, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	proposed, err := RecordFailureProposal(e.Root, e.Mission, diskState, turn, detail, outcome, consecutiveFailures)
	if err != nil {
		return nil, err
	}
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		return nil, err
	}
	if err := e.anchor(statePath, ledger, turn.TurnID); err != nil {
		return nil, err
	}
	if updated["status"] == "parked" && updated["parkReason"] == "host-failure" {
		return e.parkState(statePath, ledger, "host-failure", turn.TurnID)
	}
	return e.continueOrParkStopLoss(statePath, ledger, turn.TurnID, updated)
}

// deliverLandedUnconsumed is terminal delivery (plans/patience-orphan-usage.md
// O1): a completed mission produces no next prompt, so its Landed Returns
// list is appended to the final cycle's ledger block as Landed unconsumed
// annotations — the place a terminal mission is read. It runs ONLY at the
// completion conclude, where a ledger block for the final cycle exists and
// is safely writable, and it runs before the state write so the closing
// anchor binds the annotated ledger bytes. Best-effort: a mission that
// passed its gate is never failed over its reminder list, so a refused
// append is reported on stderr and the returns stay recoverable in the tree.
func (e *Engine) deliverLandedUnconsumed(ledger string, cycle int64, state map[string]any) {
	turnLog, _ := state["turnLog"].([]any)
	annotations := []string{}
	for _, row := range mission.LandedReturns(e.Root, e.Mission, turnLog) {
		if len(row) != 3 {
			continue
		}
		annotations = append(annotations, mission.LandedUnconsumedAnnotation(row[0], row[1], row[2]))
	}
	if len(annotations) == 0 {
		return
	}
	if err := mission.AppendAnnotations(ledger, int(cycle), annotations...); err != nil {
		fmt.Fprintf(os.Stderr, "landed-return terminal delivery refused: %v\n", err)
	}
}

// closeTerminalChains reaps and closes each fully-terminal delegation chain
// at mission end, so no chain outlives the mission unclosed.
func (e *Engine) closeTerminalChains() error {
	dispatch := filepath.Join(e.Root, "scripts", "agents", "dispatch.sh")
	for _, rootJob := range CloseableChains(e.Root, e.Mission) {
		runCaptured(e.Root, nil, dispatch, "reap", "--job", rootJob)
		stdout, stderr, code := runCaptured(e.Root, nil, dispatch, "close", "--job", rootJob, "--runner-closed")
		if code != 0 {
			return failf(3, "runner could not close terminal job chain %s: %s", rootJob, firstDetail(stderr, stdout))
		}
	}
	return nil
}

// measure runs the contract's gate against the candidate. Any failure to
// measure is itself a measurement: an unmeasurable no-progress cycle, never
// a dead runner.
func (e *Engine) measure(state map[string]any) (classification, observed string, measurement map[string]any, gatePassed bool) {
	unmeasurable := func(err error) (string, string, map[string]any, bool) {
		return "no-progress", "unmeasurable:" + strings.ReplaceAll(err.Error(), "\n", " "), nil, false
	}
	_, values, _, err := e.parseContract(false)
	if err != nil {
		return unmeasurable(err)
	}
	turnLog, _ := state["turnLog"].([]any)
	result, err := mission.ContractMeasure(e.contractPath(), PreviousMetrics(turnLog, gateMetricNames(values)))
	if err != nil {
		return unmeasurable(err)
	}
	measurement = map[string]any{
		"metrics":      result.Metrics,
		"guards":       result.Guards,
		"candidateSha": result.CandidateSHA,
	}
	return result.Classification, result.Observed, measurement, result.GatePassed
}

// stampObservedSession records the session identity the harness itself
// observed for a turn, into the turn record, from the earliest source the
// runtime produces: a session-established signal where the runtime's
// capability snapshot declares one — no host adapter emits a launch signal
// today (host session discovery is post-hoc) — else the adapter's terminal
// result envelope, the universal source for host turns. The return's own
// claim is never a source. It reports the stamped value; nil when no
// harness artifact named a session.
func (e *Engine) stampObservedSession(turnDir string, result map[string]any) (any, error) {
	envelope := result
	if envelope == nil {
		// A capped or failed adapter may still have written its terminal
		// envelope before the wind-down; what exists on disk is still the
		// harness's own artifact.
		if doc, err := readJSONDoc(filepath.Join(turnDir, "result.json")); err == nil {
			envelope = doc
		}
	}
	session, _ := envelope["sessionId"].(string)
	if session == "" {
		return nil, nil
	}
	if _, err := patchTurn(filepath.Join(turnDir, "turn.json"), map[string]any{"observedSession": session}); err != nil {
		return nil, err
	}
	return session, nil
}

// concludeFaultedTurn drives the cycle's remaining duties for a turn whose
// return was not accepted (rejected or capped), in the binding order: drain
// jobs FIRST so measurement never races live delegates, measure the
// committed tree, append the ledger line with the fault annotations in the
// same cycle block, then conclude with the empty verdict plus the
// measurement (ConcludeFaultedTurn). A host-failure park and the stop-loss
// check follow exactly as on the plain failed-turn path.
func (e *Engine) concludeFaultedTurn(statePath, ledger string, state map[string]any, turnPath, turnDir string, fault TurnFault, consecutiveFailures int) (map[string]any, error) {
	turnDoc, err := readDocLabeled(turnPath, "turn record", 3)
	if err != nil {
		return nil, err
	}
	turn, err := TurnFromDoc(turnDoc)
	if err != nil {
		return nil, err
	}
	cycle, ok := jsonInt(turn.Cycle)
	if !ok {
		return nil, failf(3, "turn record cycle is invalid")
	}
	parked, err := e.drainJobs(statePath, ledger, turn.TurnID, cycle)
	if err != nil {
		return nil, err
	}
	if parked != nil {
		// The drain stalled: the reserved cycle never concludes here — the
		// resume heal books it once the human answers the park.
		return parked, nil
	}
	classification, observed, measurement, gatePassed := e.measure(state)
	var candidateSHA string
	if measurement != nil {
		candidateSHA, _ = measurement["candidateSha"].(string)
	} else {
		branch, _ := state["branch"].(string)
		var err error
		if candidateSHA, err = e.gitRevParse(branch); err != nil {
			return nil, err
		}
	}
	if err := e.appendLedger(state, ledger, cycle, classification, candidateSHA, observed, nil, fault.Annotations...); err != nil {
		return nil, err
	}
	var measurementValue any
	if measurement != nil {
		measurementValue = measurement
	}
	if err := atomicWriteJSON(filepath.Join(turnDir, "measurement.json"), map[string]any{
		"measurement": measurementValue, "gatePassed": gatePassed,
	}); err != nil {
		return nil, err
	}
	diskState, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	proposed, err := ConcludeFaultedTurn(e.Root, e.Mission, diskState, turn, fault, measurementValue, gatePassed, consecutiveFailures)
	if err != nil {
		return nil, err
	}
	if proposed["status"] == "completed" {
		e.deliverLandedUnconsumed(ledger, cycle, proposed)
	}
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		return nil, err
	}
	if err := e.anchor(statePath, ledger, turn.TurnID); err != nil {
		return nil, err
	}
	if updated["status"] == "parked" && updated["parkReason"] == "host-failure" {
		return e.parkState(statePath, ledger, "host-failure", turn.TurnID)
	}
	return e.continueOrParkStopLoss(statePath, ledger, turn.TurnID, updated)
}

// oneCycle drives one full mission cycle: reserve it, build and check the
// turn, run the host, adjudicate and apply the return, measure, account, and
// decide whether the mission continues. It returns the state the mission is
// left in; an error is a runner defect that fails the mission.
func (e *Engine) oneCycle(statePath, ledger string, state map[string]any, leasePath, startSignal string, notified *bool) (map[string]any, error) {
	if err := mission.ReserveCycle(e.Root, e.Mission); err != nil {
		return e.parkState(statePath, ledger, "fence", e.Mission)
	}
	fences, err := readDocLabeled(e.fencesPath(), "mission fence counters", 3)
	if err != nil {
		return nil, err
	}
	cycle, ok := jsonInt(fences["cycles"])
	if !ok || cycle < 1 {
		return nil, failf(3, "reserved mission cycle number is invalid")
	}
	turnLog, ok := state["turnLog"].([]any)
	if !ok {
		return nil, failf(3, "mission state turn log is unreadable")
	}
	hostSession, reconciliation, priorFailures := PriorContext(turnLog)
	_, values, _, err := e.parseContract(true)
	if err != nil {
		return nil, err
	}
	turnCapMin, err := intFromString(values["host.turn-cap-min"])
	if err != nil {
		return nil, failf(3, "mission contract host.turn-cap-min is invalid: %v", err)
	}
	turnID, turnDir, err := e.allocateTurn(cycle)
	if err != nil {
		return nil, err
	}
	turnPath := filepath.Join(turnDir, "turn.json")
	turn := map[string]any{
		"missionId": e.Mission,
		"turnId":    turnID,
		"cycle":     cycle,
		"runtime":   values["host.runtime"],
		"model":     values["host.model"],
		// The announced session is what the prompt's Host-Session header will
		// say: a hint from the previous concluded turn, never an authority.
		// hostSession keeps the same value under its legacy name until the
		// fixtures migrate. The observed session is stamped after the host
		// runs, from the harness's own artifacts.
		"hostSession":      hostSession,
		"announcedSession": hostSession,
		"observedSession":  nil,
		"reconciliation":   reconciliation,
		"startedAt":        nowISO(),
		"turnCapMin":       turnCapMin,
		"pid":              nil,
		"pidStartedAt":     nil,
		"pgid":             nil,
		"instanceTag":      nil,
		"status":           "pending",
		"outcome":          nil,
		"error":            nil,
		"detail":           nil,
		"resultPath":       filepath.Join(turnDir, "result.json"),
		"returnPath":       filepath.Join(turnDir, "return.json"),
		"rawPath":          filepath.Join(turnDir, "raw.out"),
		"endedAt":          nil,
	}
	if err := atomicWriteJSON(turnPath, turn); err != nil {
		return nil, err
	}
	if err := mission.AssemblePrompt(e.Root, e.Mission, turnID, filepath.Join(turnDir, "prompt.md")); err != nil {
		detail := strings.TrimSpace(err.Error())
		if detail == "" {
			detail = "prompt assembly refused"
		}
		return e.failTurnBeforeLaunch(statePath, ledger, state, turnPath, detail)
	}
	stdout, stderr, code := runCaptured(e.Root, nil,
		filepath.Join(e.Root, "scripts", "assert-turn-prompt.sh"),
		"--file", filepath.Join(turnDir, "prompt.md"), "--turn", turnDir)
	if code != 0 {
		detail := firstDetail(stderr, stdout)
		if detail == "" {
			detail = "turn prompt checker refused launch"
		}
		return e.failTurnBeforeLaunch(statePath, ledger, state, turnPath, detail)
	}

	e.emit("turn-launched", fmt.Sprintf("cycle %d", cycle), map[string]string{
		"missionId": e.Mission, "turnId": turnID,
	})
	exitCode, result, launchDetail, err := e.launchHost(turnID, turnDir, turn, leasePath, startSignal, notified)
	if err != nil {
		return nil, err
	}
	summary := fmt.Sprintf("exit %d", exitCode)
	outcomeField := launchDetail
	if launchDetail != "" {
		summary += fmt.Sprintf(" (%s)", launchDetail)
	} else if exitCode == 0 {
		outcomeField = "ok"
	} else {
		outcomeField = fmt.Sprintf("exit-%d", exitCode)
	}
	e.emit("turn-result", summary, map[string]string{
		"missionId": e.Mission, "turnId": turnID, "outcome": outcomeField,
	})
	if _, err := e.stampObservedSession(turnDir, result); err != nil {
		return nil, err
	}
	if launchDetail == "start-unverified" {
		return e.recordFailedTurn(statePath, ledger, state, turnPath, launchDetail, "failed", 2)
	}
	if exitCode == 6 {
		// The adapter's genuine fault signal: the envelope carries no session
		// at all. Rotation no longer lands here — a rotated session is
		// reported in the envelope and judged at adjudication.
		if _, err := patchTurn(turnPath, map[string]any{
			"status": "failed", "outcome": "unresumable", "error": "unresumable",
			"detail": "host session is not resumable", "endedAt": nowISO(),
		}); err != nil {
			return nil, err
		}
		return e.recordFailedTurn(statePath, ledger, state, turnPath, "host session is not resumable", "unresumable", priorFailures)
	}
	if launchDetail == "capped" {
		// The cap fired: the turn keeps outcome=capped, and the cycle still
		// drains, measures, and concludes, so a cap that landed real work
		// registers as the progress it made.
		return e.concludeFaultedTurn(statePath, ledger, state, turnPath, turnDir, TurnFault{
			Outcome:      "capped",
			Detail:       "host turn reached host.turn-cap-min",
			FeedsBreaker: true,
			Annotations:  []string{mission.CappedAnnotation},
		}, priorFailures+1)
	}
	if exitCode != 0 || result == nil {
		detail := launchDetail
		if result != nil {
			detail = fmt.Sprintf("host exited non-zero (%d)", exitCode)
		}
		if _, err := patchTurn(turnPath, map[string]any{
			"status": "failed", "outcome": "failed", "error": "host-failure",
			"detail": detail, "endedAt": nowISO(),
		}); err != nil {
			return nil, err
		}
		return e.recordFailedTurn(statePath, ledger, state, turnPath, detail, "failed", priorFailures+1)
	}

	verdict, err := AdjudicateFiles(e.Root, e.Mission, statePath, turnPath, filepath.Join(turnDir, "result.json"), turnDir, nowISO())
	if err != nil {
		detail := err.Error()
		if _, patchErr := patchTurn(turnPath, map[string]any{
			"status": "failed", "outcome": "failed", "error": "protocol-error",
			"detail": detail, "endedAt": nowISO(), "result": result,
		}); patchErr != nil {
			return nil, patchErr
		}
		// A rejected return is never applied, but the cycle keeps its duties:
		// drain, measure, conclude with both facts. Only a mismatch nobody
		// witnessed is kept off the breaker.
		var sessionFault *SessionFault
		feedsBreaker := true
		if errors.As(err, &sessionFault) && !sessionFault.Witnessed {
			feedsBreaker = false
		}
		return e.concludeFaultedTurn(statePath, ledger, state, turnPath, turnDir, TurnFault{
			Outcome:      "failed",
			Detail:       detail,
			FeedsBreaker: feedsBreaker,
			Annotations:  []string{mission.ReturnRejectedAnnotation(detail)},
		}, priorFailures+1)
	}

	// The verdict is the audit record of what this turn's return claimed and
	// what the runner made of it; the conclusion reads it back below.
	verdictDoc, err := docFromValue(verdict)
	if err != nil {
		return nil, err
	}
	verdictPath := filepath.Join(turnDir, "adjudication.json")
	if err := atomicWriteJSON(verdictPath, verdictDoc); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(asksDirPath(e.Root, e.Mission), 0o755); err != nil {
		return nil, err
	}
	if err := e.writeProposedAsks(verdict.Asks); err != nil {
		return nil, err
	}
	parked, err := e.drainJobs(statePath, ledger, turnID, cycle)
	if err != nil {
		return nil, err
	}
	if parked != nil {
		// The drain stalled: the reserved cycle never concludes here — the
		// resume heal books it once the human answers the park.
		return parked, nil
	}
	classification, observed, measurement, gatePassed := e.measure(state)
	var candidateSHA string
	if measurement != nil {
		candidateSHA, _ = measurement["candidateSha"].(string)
	} else {
		branch, _ := state["branch"].(string)
		if candidateSHA, err = e.gitRevParse(branch); err != nil {
			return nil, err
		}
	}
	// The accepted return's certified entries participate in this booking's
	// patience evaluation before anything is written (r2/P4-016): a job
	// certified by THIS turn is not booked barren in the same breath.
	var inflightCertified any
	if returnDoc, err := readJSONDoc(filepath.Join(turnDir, "return.json")); err == nil {
		inflightCertified = returnDoc["certified"]
	}
	if err := e.appendLedger(state, ledger, cycle, classification, candidateSHA, observed, inflightCertified); err != nil {
		return nil, err
	}
	measurementPath := filepath.Join(turnDir, "measurement.json")
	var measurementValue any
	if measurement != nil {
		measurementValue = measurement
	}
	if err := atomicWriteJSON(measurementPath, map[string]any{
		"measurement": measurementValue, "gatePassed": gatePassed,
	}); err != nil {
		return nil, err
	}
	proposed, err := ConcludeFiles(e.Root, e.Mission, statePath, turnPath,
		verdictPath, verdict.ReturnPath, filepath.Join(turnDir, "result.json"), measurementPath)
	if err != nil {
		return nil, err
	}
	if proposed["status"] == "completed" {
		e.deliverLandedUnconsumed(ledger, cycle, proposed)
	}
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		return nil, err
	}
	if _, err := patchTurn(turnPath, map[string]any{
		"status": "completed", "outcome": "completed", "error": nil,
		"detail": "host return accepted", "result": result, "endedAt": nowISO(),
		"rawPath": verdict.RawPath, "returnPath": verdict.ReturnPath,
	}); err != nil {
		return nil, err
	}
	if err := e.anchor(statePath, ledger, turnID); err != nil {
		return nil, err
	}
	return e.continueOrParkStopLoss(statePath, ledger, turnID, updated)
}

// failTurnBeforeLaunch records a turn that never reached its host: the
// prompt could not be assembled or was refused. That is a runner-side
// defect, so it parks the mission for a human immediately rather than
// burning a second cycle on the same refusal.
func (e *Engine) failTurnBeforeLaunch(statePath, ledger string, state map[string]any, turnPath, detail string) (map[string]any, error) {
	if _, err := patchTurn(turnPath, map[string]any{
		"status": "failed", "outcome": "failed", "error": "prompt-refused",
		"detail": detail, "endedAt": nowISO(),
	}); err != nil {
		return nil, err
	}
	return e.recordFailedTurn(statePath, ledger, state, turnPath, detail, "failed", 2)
}

// docFromValue round-trips a typed value into the JSON document shape the
// runner's artifacts use.
func docFromValue(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return decodeJSONDoc(data)
}
