package missionrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
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
	// The FIRST act of the runner's life: refuse ledger semantics this
	// binary does not implement — before the runner-started event, the
	// lease, the runner record, the heartbeat, and every resume healer
	// (issue-4 round 3: even the failure ramp aggregates usage and
	// emits, so the guard cannot live behind it). Only the start signal
	// is written, so a waiting launcher learns the refusal.
	if err := stateShapeRefusal(filepath.Join(e.missionDir(), "state.json")); err != nil {
		_ = writeStartSignal(startSignal, false, nil, err.Error())
		fmt.Fprintln(os.Stderr, err.Error())
		return exitFor(err)
	}
	if err := refuseUnsupportedSemantics(filepath.Join(e.missionDir(), "state.json")); err != nil {
		_ = writeStartSignal(startSignal, false, nil, err.Error())
		fmt.Fprintln(os.Stderr, err.Error())
		return exitFor(err)
	}
	if started, err := processStartedAt(os.Getpid()); err == nil {
		e.emitter.PidStartedAt = started
	}
	e.emit("runner-started", "mode="+mode, map[string]string{"missionId": e.Mission})
	return e.internalRun(mode, tag, startSignal)
}

func (e *Engine) internalRun(mode, tag, startSignal string) int {
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
			// Finalize BEFORE releasing: the shared record belongs to the
			// lease winner, and after release another runner may own it
			// (goal-system GOAL-22). A loser that never held the lease
			// never touches the record at all.
			e.finishRunner("failed", err.Error())
			e.releaseLease()
			leaseHeld = false
		}
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
	e.unattendedCheckout = !lease.CheckoutForeignToLineage(e.Root, MissionLineage(e.Mission))

	// Publication is lease-serialized (goal-system GOAL-22): only the
	// winner writes the shared runner record, so a losing contender can
	// never replace or finalize the record of a live runner.
	pid := os.Getpid()
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		return fail(err)
	}
	started, err := processStartedAt(pid)
	if err != nil {
		return fail(err)
	}
	recordPath, _, _ := e.runnerPaths()
	// The taint STOP (HIW-R2-04): an unresolved workspace taint refuses
	// every run mode before any turn machinery — only a human's typed
	// RESTORE or ADOPT_DISPUTED_TREE resolution reopens the mission.
	if doc, derr := readJSONDoc(filepath.Join(e.missionDir(), "state.json")); derr == nil {
		if reason := unresolvedTaint(doc); reason != "" {
			return fail(failf(3, "mission workspace is tainted (%s); resolve the taint before any further turn", clipSummary(reason)))
		}
	}
	if err := atomicWriteJSON(recordPath, e.runnerRecord(pid, pgid, started, tag)); err != nil {
		return fail(err)
	}
	if err := e.heartbeat(nil); err != nil {
		return fail(err)
	}

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
	// Finalize before releasing (goal-system GOAL-22): after release the
	// record may belong to the next winner.
	e.finishRunner("completed", nil)
	e.releaseLease()
	leaseHeld = false
	return 0
}

// runnerRecord is the runner's process record, the artifact status reads to
// decide whether anyone is actually driving a running mission.
func (e *Engine) runnerRecord(pid, pgid int, started int64, tag string) map[string]any {
	// The clock-step-immune pair rides beside the legacy second (issue
	// #1): readers prefer it, and a time-synced guest stepping the clock
	// can no longer make this record's live runner read as Dead.
	var startTicks int64
	var bootID string
	if exact, state, err := (identity.KernelProber{}).Probe(int64(pid)); err == nil && state == identity.Alive {
		startTicks, bootID = exact.StartTicks, exact.BootID
	}
	return map[string]any{
		"missionId":     e.Mission,
		"status":        "running",
		"error":         nil,
		"workspaceRoot": e.Root,
		"pid":           pid,
		"pidStartedAt":  started,
		"pidStartTicks": startTicks,
		"bootId":        bootID,
		"pgid":          pgid,
		"instanceTag":   tag,
		"startedAt":     nowISO(),
		"endedAt":       nil,
	}
}

// finishRunner finalizes the runner record. Best-effort by design: the
// record is a witness for status, and failing to finalize it must not change
// how the mission itself ended. Terminal writes are OWNER-ONLY (goal-system
// GOAL-22): the record must be the one THIS process published — on a resume
// that failed before publication, the previous runner's concluded record
// survives untouched, and a contender that never won the lease never gets
// here at all.
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
	if pid, ok := jsonInt(record["pid"]); !ok || int(pid) != os.Getpid() {
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

// writeStateResolution is resolve-taint's writer: the only caller allowed
// to land a typed resolution or a segment move (mission.WriteStateResolution;
// slice-6 round-2 finding 1).
func (e *Engine) writeStateResolution(statePath string, proposed map[string]any, expect string) (map[string]any, error) {
	return e.writeStateWith(statePath, proposed, expect, mission.WriteStateResolution)
}

// writeState advances the state through its compare-and-write: the proposal
// lands only against the hash the runner last read.
func (e *Engine) writeState(statePath string, proposed map[string]any) (map[string]any, error) {
	return e.writeStateWith(statePath, proposed, "", mission.WriteState)
}

// writeStateWith lands one proposal. A non-empty expect PINS the
// compare-and-write to a hash the caller VERIFIED (slice-6 round-3
// finding 2: the resolution's base must be the anchor-verified read, not
// whatever a reread finds); empty expect derives it from the current
// file, the ordinary runner behavior.
func (e *Engine) writeStateWith(statePath string, proposed map[string]any, expected string, write func(string, string, string) error) (map[string]any, error) {
	if expected == "" {
		current, err := readDocLabeled(statePath, "mission state", 7)
		if err != nil {
			return nil, err
		}
		integrity, _ := current["integrity"].(map[string]any)
		hash, ok := integrity["hash"].(string)
		if !ok {
			return nil, failf(7, "mission state integrity hash is unreadable")
		}
		expected = hash
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
	if err := write(statePath, sourcePath, expected); err != nil {
		// A proposal-validation refusal keeps its type: the conclude path
		// parks adjudicated host content instead of dying on it (issue #3).
		var proposal *mission.ProposalError
		if errors.As(err, &proposal) {
			return nil, err
		}
		message := strings.TrimSpace(err.Error())
		if message == "" {
			message = "mission state update refused"
		}
		return nil, failf(3, "%s", message)
	}
	return readDocLabeled(statePath, "mission state", 7)
}

// anchorState writes the local anchor commit binding the state hash and
// ledger — IN-PROCESS (slice-6 successor finding 4: the state-anchor CLI
// verb is human-reserved now, and the runner is not a human); the acting
// identity pins the git author through AnchorNamed.
func (e *Engine) anchorState(statePath, ledgerPath, identityName string) error {
	if err := mission.AnchorNamed(statePath, e.Root, ledgerPath, identityName, "", ""); err != nil {
		return failf(3, "mission anchor refused: %s", strings.TrimSpace(err.Error()))
	}
	return nil
}

// anchorStatePinned is the resolution's anchor: it binds EXACTLY the
// verified position — the state hash the resolution wrote and the ledger
// bytes its recheck examined (successor finding 2).
func (e *Engine) anchorStatePinned(statePath, ledgerPath, identityName, stateHash, ledgerSHA string) error {
	if err := e.reclaimCheckout(); err != nil {
		return err
	}
	if err := mission.AnchorNamed(statePath, e.Root, ledgerPath, identityName, stateHash, ledgerSHA); err != nil {
		return failf(3, "mission anchor refused: %s", strings.TrimSpace(err.Error()))
	}
	return nil
}

// anchorPinnedTo routes a CONCLUDING write's anchor through the verified
// position (WSS I11-2): the stubbed-anchor beds keep their stub, and the
// real path refuses if state or ledger moved past the caller's proof.
func (e *Engine) anchorPinnedTo(statePath, ledgerPath, identityName, stateHash, ledgerSHA string) error {
	if e.anchorFn != nil {
		if err := e.reclaimCheckout(); err != nil {
			return err
		}
		return e.anchorFn(statePath, ledgerPath, identityName)
	}
	return e.anchorStatePinned(statePath, ledgerPath, identityName, stateHash, ledgerSHA)
}

// stateIntegrityHash reads the written state's own integrity hash — the
// pin origin for the anchors of writes this process just performed.
func stateIntegrityHash(state map[string]any) string {
	integrity, _ := state["integrity"].(map[string]any)
	hash, _ := integrity["hash"].(string)
	return hash
}

// initializeState creates the mission's ledger and state from the pinned
// contract and anchors the opening position.
func (e *Engine) initializeState(leasePath string) (statePath, ledger string, state map[string]any, err error) {
	dir := e.missionDir()
	statePath = filepath.Join(dir, "state.json")
	ledger = filepath.Join(dir, "ledger.md")
	// The child's whole birth holds the launch lock: its evidence
	// checks, the birth stamp, and state publication are one exclusive
	// section against any concurrent launcher's check-then-write.
	launchHold, err := e.acquireLaunchLock()
	if err != nil {
		return "", "", nil, err
	}
	defer launchHold.release()
	if err := stateShapeRefusal(statePath); err != nil {
		return "", "", nil, err
	}
	if stateBorn(statePath) {
		return "", "", nil, failf(3, "mission state already exists; use resume")
	}
	if err := e.startAmbiguityRefusal(ledger); err != nil {
		return "", "", nil, err
	}
	if pathExists(ledger) {
		// A ledger without a state is a stillborn remnant: the
		// mission was never born, so the remnant is retryable, never
		// a wedge.
		if err := os.Remove(ledger); err != nil {
			return "", "", nil, failf(3, "mission initialization cannot clear a stillborn ledger: %v", err)
		}
	}
	approvedText, values, _, err := e.parseContract(true)
	if err != nil {
		return "", "", nil, err
	}
	if e.afterApprovedParse != nil {
		e.afterApprovedParse()
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
	// The CHILD re-admits the baseline at the moment of state birth —
	// the parent's preflight ran in another process, and the gap
	// between the two processes is not trusted. What is admitted here
	// is RECORDED as E0, so continuity holds even if this process dies
	// before the first turn opens.
	// ONE authenticated snapshot binds the whole birth: the parse above
	// verified these bytes against the fence digest, and every later
	// consumer takes them as given — a pin file replaced after that read
	// can influence nothing.
	// E0 and the accounting origins derive from ONE STABLE observation
	// (WSS-1): admission and origin capture re-run until two consecutive
	// passes agree, so a repository moving during birth can never leave
	// origins that disagree with the admitted baseline.
	var admitted string
	var origins map[string]any
	birthStable := false
	for attempt := 0; attempt < 3 && !birthStable; attempt++ {
		admitted, err = e.admittedBaseline(values, []byte(approvedText))
		if err != nil {
			e.cleanupStillborn(ledger)
			return "", "", nil, err
		}
		origins, err = mission.CaptureAdmissionOrigins(e.Root, e.Mission)
		if err != nil {
			e.cleanupStillborn(ledger)
			return "", "", nil, failf(3, "mission initialization cannot capture the admission origins: %v", err)
		}
		admittedAgain, aerr := e.admittedBaseline(values, []byte(approvedText))
		if aerr != nil {
			e.cleanupStillborn(ledger)
			return "", "", nil, aerr
		}
		originsAgain, oerr := mission.CaptureAdmissionOrigins(e.Root, e.Mission)
		if oerr != nil {
			e.cleanupStillborn(ledger)
			return "", "", nil, failf(3, "mission initialization cannot capture the admission origins: %v", oerr)
		}
		birthStable = admitted == admittedAgain && admissionOriginsStable(origins, originsAgain)
	}
	if !birthStable {
		e.cleanupStillborn(ledger)
		return "", "", nil, failf(3, "mission initialization refused: the repository would not hold still during admission")
	}
	workspace := gittree.Workspace{Dir: e.Root}
	if err := workspace.Anchor(e.Mission, admitted); err != nil {
		e.cleanupStillborn(ledger)
		return "", "", nil, failf(3, "mission initialization cannot anchor the admitted baseline: %v", err)
	}
	// The birth record lands BEFORE state publication: a crash between
	// the two writes then leaves evidence that freezes the mission id
	// for a human, never an unmarked birth a later state-file loss
	// could mistake for a stillborn remnant. Only this pass's own
	// PROVEN publication failure may unstamp it — in-process certainty,
	// not the ambiguity a crash leaves.
	if err := atomicWriteJSON(e.birthRecordPath(), map[string]any{
		"missionId": e.Mission, "bornAt": nowISO(),
	}); err != nil {
		e.cleanupStillborn(ledger)
		return "", "", nil, failf(3, "mission birth record cannot be written: %v", err)
	}
	if err := mission.InitStateFromSource(statePath, e.approvedContractPath(), ledger, leasePath, "", admitted, []byte(approvedText), origins); err != nil {
		// The state write is atomic, so a refusal here means the
		// mission is provably UNBORN: unstamp the birth record, then
		// the stillborn sweep applies — without it the ledger, pin,
		// fences, stamp, and E0 anchor all outlive a failed birth.
		if rerr := os.Remove(e.birthRecordPath()); rerr != nil && !os.IsNotExist(rerr) {
			e.emit("mission-stillborn-cleanup", "leftover born.json", map[string]string{
				"missionId": e.Mission, "error": rerr.Error(),
			})
		}
		e.cleanupStillborn(ledger)
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

// admissionOriginsStable compares two origin captures whole, the capture
// instants excluded.
func admissionOriginsStable(a, b map[string]any) bool {
	for key, value := range a {
		if key == "capturedAt" {
			continue
		}
		if !jsonDocEqual(value, b[key]) {
			return false
		}
	}
	return len(a) == len(b)
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
	// The shape check runs FIRST: a dereferencing existence check reads
	// a dangling symlink as honest absence and would misreport the one
	// object that says someone touched the path.
	if err := stateShapeRefusal(statePath); err != nil {
		return "", "", nil, err
	}
	if !pathExists(statePath) {
		return "", "", nil, failf(7, "mission state does not exist")
	}
	// The RESUME child re-checks the fileMode pin too: the parent's
	// preflight ran in another process, and reconciliation below trusts
	// the tree equation — a gap-time flip to false would hide mode
	// drift from everything that follows.
	if err := e.checkFileModePinned(); err != nil {
		return "", "", nil, err
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
	// A verified living state IS birth evidence: stamp the durable
	// record for any mission born before the record existed, so a later
	// state-file loss cannot be mistaken for a stillborn remnant.
	if !pathExists(e.birthRecordPath()) {
		if err := atomicWriteJSON(e.birthRecordPath(), map[string]any{
			"missionId": e.Mission, "bornAt": nowISO(),
		}); err != nil {
			return "", "", nil, failf(3, "mission birth record cannot be written: %v", err)
		}
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
	// A consumed-but-unconcluded acceptance completes BEFORE the
	// terminal-status refusal: a park written at the acceptance (breaker,
	// all-streams) must not strand its turn unconcluded — but a
	// wall-violation park with a pending acceptance stays exactly where
	// the human's resolution left to rule on it.
	if openTurn, ok := state["openTurn"].(map[string]any); ok && state["parkReason"] != "wall-violation" {
		if turnID, _ := openTurn["turnId"].(string); turnID != "" && mission.UnverifiedAcceptance(state) == turnID {
			final, parked, verr := e.completePendingVerification(statePath, ledger, state, turnID)
			if verr != nil {
				return "", "", nil, verr
			}
			if parked {
				_ = final
				return "", "", nil, failf(3, "mission workspace failed the wall at resume; resolve the taint before any further turn")
			}
			if state, err = e.verifyState(statePath, false); err != nil {
				return "", "", nil, err
			}
		}
	}
	if state["status"] != "running" {
		return "", "", nil, failf(3, "mission is %s; answer or amend its park reason before resume", valueString(state["status"]))
	}
	// An unfinished open turn is inspected BEFORE healing or any new
	// baseline (the design's binding order, critique F-1): the crashed
	// turn's workspace proves the equation before the mission moves.
	if openTurn, ok := state["openTurn"].(map[string]any); ok {
		turnID, _ := openTurn["turnId"].(string)
		cycle, _ := jsonInt(openTurn["cycle"])
		turnDir := filepath.Join(e.missionDir(), "turns", turnID)
		if mission.UnverifiedAcceptance(state) == turnID {
			// Completed above before the terminal-status check; a state
			// still carrying the pending acceptance here is a
			// wall-violation park a human must rule on first.
			return "", "", nil, failf(3, "mission workspace failed the wall at resume; resolve the taint before any further turn")
		} else {
			if _, _, violated, werr := e.wallGate(statePath, ledger, turnID, turnDir, cycle, nil, true); werr != nil {
				return "", "", nil, werr
			} else if violated {
				return "", "", nil, failf(3, "mission workspace failed the wall at resume; resolve the taint before any further turn")
			}
			// The equation held, so the crashed turn is UNACCEPTED (a
			// ledger-ahead state and a reserve gap alike: no acceptance
			// entry, nothing consumed). Close its marker so a fresh turn can
			// open; the reserved-cycle heal below books any fence gap.
			closed := deepCopyDoc(state)
			closed["openTurn"] = nil
			closedFinal, cerr := e.writeState(statePath, closed)
			if cerr != nil {
				return "", "", nil, cerr
			}
			// The open-commit anchor retires only after the close is durable.
			e.dropTurnOpenHead()
			closedIntegrity, _ := closedFinal["integrity"].(map[string]any)
			closedHash, _ := closedIntegrity["hash"].(string)
			if e.preAnchorHook != nil {
				// Test seam (round-14 finding 2): the movement must land
				// BEFORE the pin is acquired, so a reverted post-write
				// reread would select the moved bytes and anchor them.
				e.preAnchorHook()
			}
			// The pin ORIGINATES at reconciliation's verified position
			// (round-11 finding 3): the close writes no ledger bytes, so the
			// lawful ledger sha is the one the anchor tip already certifies —
			// a fresh reread would self-select any bytes moved since.
			anchoredSHA, lerr := e.verifiedLedgerPin()
			if lerr != nil {
				return "", "", nil, lerr
			}
			if err := e.anchorStatePinned(statePath, ledger, turnID, closedHash, anchoredSHA); err != nil {
				return "", "", nil, err
			}
			if state, err = e.verifyState(statePath, false); err != nil {
				return "", "", nil, err
			}
		}
	}
	if _, err := e.healReservedCycle(statePath, ledger, state); err != nil {
		return "", "", nil, err
	}
	state, err = e.verifyState(statePath, true)
	if err != nil {
		return "", "", nil, err
	}
	// A resolution's crash tail — RESOLVED taint, ask still open —
	// repairs here, AFTER reconciliation verified the anchor (slice-6
	// successor finding 6: ask answers must never derive from unverified
	// state), so a stale ask can never re-enter the waiting list or
	// suppress the next violation's ask.
	if err := e.repairResolvedTaintAsks(state); err != nil {
		return "", "", nil, err
	}
	// Sticky evidence outranks the CYCLE FENCE (round-10 finding 2): the
	// orphan sweep runs here too, before any new reservation can burn the
	// last allowed cycle into a fence park that buries the violation.
	if orphanTurn, orphanViolation := e.orphanedViolationEvidence(state); orphanViolation != "" {
		orphanDir := filepath.Join(e.missionDir(), "turns", orphanTurn)
		fences, _ := readJSONDoc(e.fencesPath())
		fenceCycle, _ := jsonInt(fences["cycles"])
		final, perr := e.parkWallViolation(statePath, ledger, orphanTurn, orphanDir, fenceCycle, orphanViolation, state, false)
		if perr != nil {
			return "", "", nil, perr
		}
		return statePath, ledger, final, nil
	}
	return statePath, ledger, state, nil
}

// cleanupStillborn tidies a mission that was never born (state.json
// absent): the ledger, approved copy, stamp, and fence pin go. BEST
// EFFORT ONLY, and safely so: birth is keyed on state.json everywhere —
// the pin check and the init exists-check both tolerate remnants — so a
// crash or failure mid-cleanup can never wedge the mission id; failures
// are surfaced as events.
func (e *Engine) cleanupStillborn(ledger string) {
	// The birth rule is the belt here too: once state.json exists — or
	// the durable birth evidence says the mission lived — NOTHING here
	// may run; these artifacts are the living mission's. A non-regular
	// object at the path does NOT block the sweep: entry refusals keep
	// pre-existing objects untouched, so anything swept here is this
	// attempt's own staging.
	evidence, evidenceErr := e.bornEvidence(ledger)
	if stateBorn(filepath.Join(e.missionDir(), "state.json")) || evidence != "" || evidenceErr != nil {
		// An unreadable probe blocks the sweep the same way evidence
		// does: destruction needs PROVEN emptiness.
		return
	}
	for _, path := range []string{ledger, e.approvedContractPath(),
		filepath.Join(e.missionDir(), "pending-block.json"), e.fencesPath()} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			e.emit("mission-stillborn-cleanup", "leftover "+filepath.Base(path), map[string]string{
				"missionId": e.Mission, "error": err.Error(),
			})
		}
	}
	// Anchor refs are stillborn artifacts too: a failed initialization
	// may already have anchored its admitted E0. Before birth there is
	// no living anchor to protect, so the mission's whole anchor
	// namespace drops with the rest.
	workspace := gittree.Workspace{Dir: e.Root}
	if err := workspace.DropAnchors(e.Mission); err != nil {
		e.emit("mission-stillborn-cleanup", "leftover anchors", map[string]string{
			"missionId": e.Mission, "error": err.Error(),
		})
	}
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
	ledgerSHA, err := e.appendLedger(state, ledger, spent, "no-progress", candidateSHA, observed, nil, annotations...)
	if err != nil {
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
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		return false, err
	}
	// The heal's anchor pins the written hash and the healed booking's
	// own bytes (WSS I12-3) — the same discipline as every conclusion.
	if err := e.anchorPinnedTo(statePath, ledger, e.Mission, stateIntegrityHash(updated), ledgerSHA); err != nil {
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
		// The SUCCESSOR lands first, THEN its predecessor closes (issue
		// #11 critique F4): a crash between the two leaves a visible
		// duplicate for one turn — never a hidden ask refusing answers
		// toward a successor that does not exist.
		if err := atomicWriteJSON(filepath.Join(asksDir, askID+".json"), ask); err != nil {
			return err
		}
		if named, _ := ask["supersedes"].(string); named != "" {
			priorPath := filepath.Join(asksDir, named+".json")
			prior, err := readJSONDoc(priorPath)
			if err != nil {
				return failf(3, "superseded ask %s is unreadable: %v", named, err)
			}
			prior["supersededBy"] = askID
			prior["supersededAt"] = nowISO()
			if err := atomicWriteJSON(priorPath, prior); err != nil {
				return err
			}
			e.emit("ask-superseded", named+" -> "+askID, map[string]string{
				"missionId": e.Mission, "askId": named, "supersededBy": askID,
			})
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
		// A park must SURVIVE an unanchorable ledger (round-11 finding
		// 1): the tainted state is durable and the STOP works from it;
		// the anchor lag heals once the disputed bytes are restored.
		// Every other park cause still surfaces the anchor failure.
		if reason, _ := outcome.State["parkReason"].(string); reason == "wall-violation" {
			e.emit("wall-violation", "park anchor deferred: "+clipSummary(err.Error()), map[string]string{
				"missionId": e.Mission, "error": err.Error(),
			})
		} else {
			return nil, err
		}
	}
	return updated, nil
}

// gitRevParse resolves a ref in the mission's repository.
func (e *Engine) gitRevParse(ref string) (string, error) {
	stdout, stderr, code := gitCaptured(e.Root, "-C", e.Root, "rev-parse", ref)
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
// It returns the sha256 of the complete post-append ledger bytes — the
// verified position the concluding anchor pins to (WSS I11-2).
func (e *Engine) appendLedger(state map[string]any, ledger string, cycle int64, classification, candidateSHA, observed string, inflightCertified any, annotations ...string) (string, error) {
	best, err := e.bestMarker(state, ledger, observed)
	if err != nil {
		return "", err
	}
	// Patience rides the SAME atomic append as the cycle line, on every
	// booking path (plans/patience-satellite-4.md): the shared function is
	// what makes ordinary, faulted, failed, and heal bookings all evaluate.
	annotations = append(append([]string(nil), annotations...),
		e.patienceBookingAnnotations(state, inflightCertified)...)
	sha, err := mission.AppendCycle(ledger, int(cycle), classification, candidateSHA, observed, best, annotations...)
	if err != nil {
		return "", failf(3, "mission ledger append refused: %v", err)
	}
	return sha, nil
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
	// Even a turn that failed before or during its host runs the wall
	// (HIW-O3 "after EVERY host exit"): a failed turn consumed nothing,
	// so any product-byte drift from the pre-tree is a violation. The
	// wall runs BEFORE any ledger-narration identity is resolved (WSS
	// I14-1): a host that deleted the candidate branch must meet the
	// inspection — which names that deletion — not a runner error that
	// skips it.
	ctx, final, violated, werr := e.wallGate(statePath, ledger, turn.TurnID, filepath.Dir(turnPath), cycle, nil, false)
	if werr != nil || violated {
		return final, werr
	}
	branch, _ := state["branch"].(string)
	candidateSHA, err := e.gitRevParse(branch)
	if err != nil {
		return nil, err
	}
	observed := "unmeasurable:" + strings.ReplaceAll(detail, "\n", " ")
	ledgerSHA, err := e.appendLedger(state, ledger, cycle, "no-progress", candidateSHA, observed, nil)
	if err != nil {
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
	// The VERIFIED CAPTURE is the failure acceptance's authority exactly
	// as at the ordinary conclude (WSS I13-2): the proposal transports
	// wall.json's rewritable evidence, and a payload differing from what
	// this process judged was built over tampered evidence.
	if violation := acceptancePayloadMismatch(proposed, turn.TurnID, ctx, false, nil); violation != "" {
		parkState, derr := readDocLabeled(statePath, "mission state", 3)
		if derr != nil {
			return nil, derr
		}
		final, ferr := e.parkWallViolation(statePath, ledger, turn.TurnID, filepath.Dir(turnPath), cycle, violation, parkState, true)
		if ferr != nil {
			return nil, ferr
		}
		return final, nil
	}
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		return nil, err
	}
	// The failure acceptance pins its anchor exactly as the ordinary
	// conclude does (WSS I12-3): the written state hash and the runner's
	// own appended bytes, so a peer rewrite before publication refuses
	// instead of authenticating.
	if err := e.anchorPinnedTo(statePath, ledger, turn.TurnID, stateIntegrityHash(updated), ledgerSHA); err != nil {
		return nil, err
	}
	// The failure record concludes through the same post-verification
	// entry as every other acceptance append.
	verified, wasParked, err := e.verifyAcceptance(statePath, ledger, turn.TurnID, filepath.Dir(turnPath), cycle, ctx.Declared)
	if err != nil {
		return nil, err
	}
	if wasParked {
		return verified, nil
	}
	if verified["status"] == "parked" && verified["parkReason"] == "host-failure" {
		return e.parkState(statePath, ledger, "host-failure", turn.TurnID)
	}
	return e.continueOrParkStopLoss(statePath, ledger, turn.TurnID, verified)
}

// deliverLandedUnconsumed is terminal delivery (plans/patience-orphan-usage.md
// O1): a completed mission produces no next prompt, so its Landed Returns
// list is appended to the final cycle's ledger block as Landed unconsumed
// annotations — the place a terminal mission is read. It runs ONLY at the
// completion conclude, where a ledger block for the final cycle exists and
// is safely writable, and it runs AFTER the state write that owns the
// transition and BEFORE the closing anchor, which therefore still binds
// the annotated bytes (WSS I11-5). Best-effort: a mission that
// passed its gate is never failed over its reminder list, so a refused
// append is reported on stderr and the returns stay recoverable in the tree.
// It returns the sha256 of the post-delivery ledger bytes (expectSHA on
// a delivery that appended nothing), so the concluding anchor can pin
// the exact position (WSS I11-2); a non-empty expectSHA makes the append
// refuse if the ledger moved past the caller's proof, and re-delivered
// annotations are skipped line-idempotently (WSS I11-5).
func (e *Engine) deliverLandedUnconsumed(ledger string, cycle int64, state map[string]any, expectSHA string) string {
	turnLog, _ := state["turnLog"].([]any)
	annotations := []string{}
	for _, row := range mission.LandedReturns(e.Root, e.Mission, turnLog) {
		if len(row) != 3 {
			continue
		}
		annotations = append(annotations, mission.LandedUnconsumedAnnotation(row[0], row[1], row[2]))
	}
	if len(annotations) == 0 {
		return expectSHA
	}
	sha, err := mission.AppendAnnotations(ledger, int(cycle), expectSHA, annotations...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "landed-return terminal delivery refused: %v\n", err)
		return expectSHA
	}
	return sha
}

// closeTerminalChains reaps and closes each fully-terminal delegation chain
// at mission end, so no chain outlives the mission unclosed.
func (e *Engine) closeTerminalChains() error {
	dispatch := filepath.Join(e.Root, "scripts", "agents", "dispatch.sh")
	var failures []string
	for _, rootJob := range CloseableChains(e.Root, e.Mission) {
		runCaptured(e.Root, nil, dispatch, "reap", "--job", rootJob)
		stdout, stderr, code := runCaptured(e.Root, nil, dispatch, "close", "--job", rootJob, "--runner-closed")
		if code != 0 {
			// One refusing chain must not strand the chains behind it:
			// finish the sweep, then report every failure by name.
			failures = append(failures, rootJob+": "+firstDetail(stderr, stdout))
		}
	}
	if len(failures) > 0 {
		return failf(3, "runner could not close %d terminal job chain(s): %s",
			len(failures), strings.Join(failures, "; "))
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
	result, err := contract.Measure(e.contractPath(), PreviousMetrics(turnLog, gateMetricNames(values)))
	if err != nil {
		return unmeasurable(err)
	}
	measurement = map[string]any{
		"metrics":      result.Metrics,
		"guards":       result.Guards,
		"candidateSha": result.CandidateSHA,
	}
	observedLine := result.Observed
	// Semantics 3 (issue #4): the same gate runs a SECOND time on the
	// mission's active candidate branch, in a scratch worktree the runner
	// owns. A binary gate of record cannot register the serial pipeline's
	// progress, so a perfect tree awaiting the wall parked as stagnant;
	// the candidate tokens let the stop-loss see it without ever letting
	// it outrank a real merge. A failed candidate measurement is an
	// absent token — the directed worst — never a dead runner.
	if semantics, err := stopLossSemantics(state); err == nil && semantics >= 3 {
		if candidate := e.measureCandidate(state); candidate != "" {
			observedLine = observedLine + "," + candidate
			measurement["candidate"] = candidate
		}
	}
	return result.Classification, observedLine, measurement, result.GatePassed
}

// measureCandidate runs the declared gate on the newest open implementer
// chain's branch in a disposable worktree, returning candidate-prefixed
// observed tokens, or "" when there is no candidate or it cannot be
// measured. Best-effort by design: the gate of record is untouched.
func (e *Engine) measureCandidate(state map[string]any) string {
	branch := e.activeCandidateBranch(state)
	if branch == "" {
		return ""
	}
	sha, err := e.gitRevParse(branch)
	if err != nil {
		e.emit("candidate-measure-skipped", clipSummary("candidate branch unreadable: "+branch), map[string]string{
			"missionId": e.Mission, "error": "candidate branch unreadable: " + branch,
		})
		return ""
	}
	metrics, err := contract.MeasureCandidate(e.contractPath(), sha)
	if err != nil {
		e.emit("candidate-measure-skipped", clipSummary(err.Error()), map[string]string{
			"missionId": e.Mission, "error": err.Error(),
		})
		return ""
	}
	tokens := make([]string, 0, len(metrics)+1)
	for metric, value := range metrics {
		tokens = append(tokens, "candidate-"+metric+"="+value)
	}
	sort.Strings(tokens)
	tokens = append(tokens, "candidate-ref="+branch+"@"+sha)
	return strings.Join(tokens, ",")
}

// activeCandidateBranch resolves the mission's active integration branch:
// the branch of the newest non-terminal implementer job of this mission,
// else the newest terminal one whose return has not yet been consumed —
// read from job records the runner already trusts, never from a claim.
func (e *Engine) activeCandidateBranch(state map[string]any) string {
	jobsDir := jobsDirPath(e.Root)
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		return ""
	}
	newest := ""
	newestStarted := ""
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		record, err := readJSONDoc(filepath.Join(jobsDir, name))
		if err != nil {
			continue
		}
		if record["role"] != "implementer" || !numericEqual(record["mission"], e.Mission) {
			continue
		}
		branch, _ := record["branch"].(string)
		jobID, _ := record["jobId"].(string)
		// EXACTLY the dispatch-produced branch of THIS job, and free of
		// the observed line's grammar characters (issue-4 F-3: a crafted
		// branch like agent/x,candidate-score=999 is a valid git ref and
		// its candidate-ref token would inject a forged metric).
		if jobID == "" || branch != "agent/"+jobID || strings.ContainsAny(branch, ",=@ \t") {
			continue
		}
		started, _ := record["startedAt"].(string)
		if started > newestStarted {
			newestStarted = started
			newest = branch
		}
	}
	return newest
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

// concludeSpec parameterizes the ONE cycle-conclusion sequence with the
// only lines on which the accepted and faulted paths genuinely differ.
type concludeSpec struct {
	turnID            string
	cycle             int64
	turnDir           string
	annotations       []string
	inflightCertified any
	// certified is the turn's ADJUDICATED certification list — the wall
	// inspection derives the consumed patch chain from it (nil on the
	// faulted paths: a rejected return consumes nothing).
	certified []map[string]any
	// propose builds the state proposal from the measurement.
	propose func(measurementValue any, gatePassed bool) (map[string]any, error)
	// afterWrite runs between the state write and the anchor (the
	// accepted path patches its turn terminal here). Optional.
	afterWrite func(updated map[string]any) error
	// parkHostFailure: the faulted path parks when the proposal came
	// back parked host-failure. Optional.
	parkHostFailure bool
}

// concludeCycle is the single home of the cycle-conclusion binding order —
// the package's central correctness argument: drain jobs FIRST so
// measurement never races live delegates, measure the committed tree,
// append the ledger line in the same cycle block, write the measurement,
// build and write the state proposal, then anchor AFTER the state so a
// crash between the two is heal-forward, and finally judge the stop-loss.
func (e *Engine) concludeCycle(statePath, ledger string, state map[string]any, spec concludeSpec) (map[string]any, error) {
	// The wall inspects the tree FIRST — before the drain can park the
	// mission for a lesser reason and before any measurement (HIW-O3,
	// critique F-5): delegates write their own worktrees, never the
	// checkout, so the inspection races nothing, and a host that altered
	// the workspace hides behind neither a stalled drain nor a gate pass.
	ctx, final, violated, err := e.wallGate(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle, spec.certified, false)
	if err != nil || violated {
		return final, err
	}
	parked, err := e.drainJobs(statePath, ledger, spec.turnID, spec.cycle)
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
	ledgerSHA, err := e.appendLedger(state, ledger, spec.cycle, classification, candidateSHA, observed, spec.inflightCertified, spec.annotations...)
	if err != nil {
		return nil, err
	}
	var measurementValue any
	if measurement != nil {
		measurementValue = measurement
	}
	if err := atomicWriteJSON(filepath.Join(spec.turnDir, "measurement.json"), map[string]any{
		"measurement": measurementValue, "gatePassed": gatePassed,
	}); err != nil {
		return nil, err
	}
	// The judged posture is re-taken and compared whole around the
	// ACCEPTANCE WRITE, which happens after the drain and the
	// arbitrary-bash measurement: a changed capture re-runs the
	// inspection; a repository that will not hold still is a violation.
	for attempt := 0; ; attempt++ {
		recheck, cerr := e.captureWallPostureStable(ctx.Expected, ctx.Declared)
		if cerr != nil {
			if answer := stateAnswerOf(cerr); answer != "" {
				diskState, derr := readDocLabeled(statePath, "mission state", 3)
				if derr != nil {
					return nil, derr
				}
				final, ferr := e.parkWallViolation(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle, answer, diskState, true)
				if ferr != nil {
					return nil, ferr
				}
				return final, nil
			}
			return nil, cerr
		}
		if violation, jerr := e.judgeCaptureIntegrity(recheck, ctx.OpenAnchor, state); jerr != nil && stateAnswerOf(jerr) == "" {
			return nil, jerr
		} else if violation != "" || jerr != nil {
			if violation == "" {
				violation = stateAnswerOf(jerr)
			}
			diskState, derr := readDocLabeled(statePath, "mission state", 3)
			if derr != nil {
				return nil, derr
			}
			final, ferr := e.parkWallViolation(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle, violation, diskState, true)
			if ferr != nil {
				return nil, ferr
			}
			return final, nil
		}
		if recheck.equalTo(ctx.Capture) {
			break
		}
		if attempt >= 2 {
			diskState, derr := readDocLabeled(statePath, "mission state", 3)
			if derr != nil {
				return nil, derr
			}
			final, ferr := e.parkWallViolation(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle,
				"repository would not hold still during inspection", diskState, true)
			if ferr != nil {
				return nil, ferr
			}
			return final, nil
		}
		// The ledger was lawfully appended by this conclusion, so the
		// re-run skips the in-turn ledger guard exactly as resume does.
		rerun, final, violated, rerr := e.wallGate(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle, spec.certified, true)
		if rerr != nil || violated {
			return final, rerr
		}
		ctx = rerun
	}
	// The MEASURED CANDIDATE is the concluded tip (WSS I13-1): the
	// stability loop lawfully re-admits accounted HEAD motion, but the
	// measurement's success evidence was produced for candidateSHA — a
	// gate command that moves the branch to any other accounted commit
	// must not complete the mission on evidence measured elsewhere.
	if candidateSHA != "" && !ctx.Capture.Unborn && ctx.Capture.Head != candidateSHA {
		diskState, derr := readDocLabeled(statePath, "mission state", 3)
		if derr != nil {
			return nil, derr
		}
		final, ferr := e.parkWallViolation(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle,
			fmt.Sprintf("the concluded tip %s is not the measured candidate %s", ctx.Capture.Head, candidateSHA), diskState, true)
		if ferr != nil {
			return nil, ferr
		}
		return final, nil
	}
	proposed, err := spec.propose(measurementValue, gatePassed)
	if err != nil {
		return nil, err
	}
	// The VERIFIED CAPTURE is the acceptance's authority (never
	// wall.json, which is rewritable evidence): a proposal whose payload
	// posture differs from what this process judged was built over
	// tampered evidence and parks before it can commit.
	if violation := acceptancePayloadMismatch(proposed, spec.turnID, ctx, gatePassed, measurementValue); violation != "" {
		diskState, derr := readDocLabeled(statePath, "mission state", 3)
		if derr != nil {
			return nil, derr
		}
		final, ferr := e.parkWallViolation(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle, violation, diskState, true)
		if ferr != nil {
			return nil, ferr
		}
		return final, nil
	}
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		// A PROPOSAL-validation refusal is a fault in ADJUDICATED HOST
		// CONTENT (or, rarely, a runner proposal bug — an acceptable
		// containment trade: a parked mission surfaces an ask, a dead
		// runner surfaces nothing). The invariant that refused the write
		// is the mission's protection; the outcome is a parked mission
		// with the refusal surfaced — never the fail ramp. I/O,
		// corruption, and compare-and-write misses keep the ramp: those
		// are the runner's own to fail loudly on (issue #3, critique
		// round 1: the guard must be reachable, correctly scoped, and
		// the park must stay ledger-consistent).
		var proposalRefusal *mission.ProposalError
		if errors.As(err, &proposalRefusal) {
			e.emit("turn-proposal-refused", clipSummary(err.Error()), map[string]string{
				"missionId": e.Mission, "turnId": spec.turnID, "error": err.Error(),
			})
			// The cycle's ledger block is already appended, so the park
			// proposal must carry this cycle's ledger count or the
			// anchor refuses and the fail ramp fires anyway.
			outcome, parkErr := ParkProposal(e.Root, e.Mission, state, "host-failure", nowISO())
			if parkErr != nil {
				return nil, parkErr
			}
			if err := setLedgerCycles(outcome.State, spec.cycle); err != nil {
				return nil, err
			}
			e.emit("mission-parked", clipSummary("host-failure"), map[string]string{
				"missionId": e.Mission, "parkReason": "host-failure",
			})
			return e.applyPark(statePath, ledger, spec.turnID, outcome)
		}
		return nil, err
	}
	if spec.afterWrite != nil {
		if err := spec.afterWrite(updated); err != nil {
			return nil, err
		}
	}
	// The acceptance anchor binds EXACTLY the position this conclusion
	// proved — the state hash the write produced and the ledger bytes
	// the runner's own append wrote (WSS I11-2): a peer that rewrites
	// the appended cycle into different parseable bytes before
	// publication makes the anchor REFUSE, never re-authenticate the
	// reread.
	if err := e.anchorPinnedTo(statePath, ledger, spec.turnID, stateIntegrityHash(updated), ledgerSHA); err != nil {
		return nil, err
	}
	// The acceptance append is the commit point but not the conclusion:
	// the post-verification entry re-captures the posture after
	// publication and concludes the turn only on a clean match.
	verified, wasParked, err := e.verifyAcceptance(statePath, ledger, spec.turnID, spec.turnDir, spec.cycle, ctx.Declared)
	if err != nil {
		return nil, err
	}
	if wasParked {
		return verified, nil
	}
	if spec.parkHostFailure && verified["status"] == "parked" && verified["parkReason"] == "host-failure" {
		return e.parkState(statePath, ledger, "host-failure", spec.turnID)
	}
	return e.continueOrParkStopLoss(statePath, ledger, spec.turnID, verified)
}

// concludeFaultedTurn drives the cycle's remaining duties for a turn whose
// return was not accepted (rejected or capped): the shared conclusion
// sequence with the fault annotations, the ConcludeFaultedTurn proposal,
// and the host-failure park.
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
	return e.concludeCycle(statePath, ledger, state, concludeSpec{
		turnID:      turn.TurnID,
		cycle:       cycle,
		turnDir:     turnDir,
		annotations: fault.Annotations,
		propose: func(measurementValue any, gatePassed bool) (map[string]any, error) {
			diskState, err := readDocLabeled(statePath, "mission state", 3)
			if err != nil {
				return nil, err
			}
			return ConcludeFaultedTurn(e.Root, e.Mission, diskState, turn, fault, measurementValue, gatePassed, consecutiveFailures)
		},
		parkHostFailure: true,
	})
}

// A mission cycle in five named steps (Phase 3b): reserve and build the
// turn, gate its prompt, run the host and classify the outcome, adjudicate
// the return, then drain, measure, and conclude. Each step either advances
// the shared cycle context or ends the cycle with the mission's resulting
// state; oneCycle is only the sequence.
type cycleContext struct {
	statePath, ledger string
	state             map[string]any
	leasePath         string
	startSignal       string
	notified          *bool

	cycle         int64
	turnID        string
	turnDir       string
	turnPath      string
	priorFailures int
	turn          map[string]any

	exitCode     int
	result       map[string]any
	launchDetail string
	verdict      *Verdict
	verdictPath  string
}

// oneCycle drives one full mission cycle: reserve it, build and check the
// turn, run the host, adjudicate and apply the return, measure, account, and
// decide whether the mission continues. It returns the state the mission is
// left in; an error is a runner defect that fails the mission.
func (e *Engine) oneCycle(statePath, ledger string, state map[string]any, leasePath, startSignal string, notified *bool) (map[string]any, error) {
	c := &cycleContext{statePath: statePath, ledger: ledger, state: state,
		leasePath: leasePath, startSignal: startSignal, notified: notified}
	for _, step := range []func(*cycleContext) (map[string]any, bool, error){
		e.cycleReserveAndBuildTurn,
		e.cycleGatePrompt,
		e.cycleRunHost,
		e.cycleAdjudicate,
		e.cycleConclude,
	} {
		final, done, err := step(c)
		if err != nil || done {
			return final, err
		}
	}
	return nil, failf(3, "cycle steps exhausted without a conclusion")
}

// cycleReserveAndBuildTurn reserves the cycle against the fences, reads the
// contract, allocates the turn, and publishes the pending turn record.
func (e *Engine) cycleReserveAndBuildTurn(c *cycleContext) (map[string]any, bool, error) {
	if err := mission.ReserveCycle(e.Root, e.Mission); err != nil {
		final, ferr := e.parkState(c.statePath, c.ledger, "fence", e.Mission)
		return final, true, ferr
	}
	fences, err := readDocLabeled(e.fencesPath(), "mission fence counters", 3)
	if err != nil {
		return nil, true, err
	}
	cycle, ok := jsonInt(fences["cycles"])
	if !ok || cycle < 1 {
		return nil, true, failf(3, "reserved mission cycle number is invalid")
	}
	c.cycle = cycle
	turnLog, ok := c.state["turnLog"].([]any)
	if !ok {
		return nil, true, failf(3, "mission state turn log is unreadable")
	}
	hostSession, reconciliation, priorFailures := PriorContext(turnLog)
	c.priorFailures = priorFailures
	_, values, _, err := e.parseContract(true)
	if err != nil {
		return nil, true, err
	}
	turnCapMin, err := intFromString(values["host.turn-cap-min"])
	if err != nil {
		return nil, true, failf(3, "mission contract host.turn-cap-min is invalid: %v", err)
	}
	turnID, turnDir, err := e.allocateTurn(cycle)
	if err != nil {
		return nil, true, err
	}
	// The wall's pre-tree: the shippable projection at turn open, anchored
	// against garbage collection for the mission's life (HIW-O1; the state
	// openTurn write joins in the acceptance-write step of the build map).
	workspace := gittree.Workspace{Dir: e.Root}
	preTree, err := wallSnapshot(workspace, e.Mission)
	if err != nil {
		return nil, true, failf(3, "turn open cannot snapshot the workspace: %v", err)
	}
	if err := workspace.Anchor(e.Mission, preTree); err != nil {
		return nil, true, failf(3, "turn open cannot anchor the pre-tree: %v", err)
	}
	c.turnID, c.turnDir = turnID, turnDir
	c.turnPath = filepath.Join(turnDir, "turn.json")
	c.turn = map[string]any{
		"missionId": e.Mission,
		"turnId":    turnID,
		"cycle":     cycle,
		"preTree":   preTree,
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
	if err := atomicWriteJSON(c.turnPath, c.turn); err != nil {
		return nil, true, err
	}
	// The open-turn marker is the wall's sequence point (HIW-O1): the state
	// records, before the host launches, WHICH turn is in flight and the
	// exact chain position and taint segment it opened under — the identity
	// every authorization issued this turn binds. The pending turn record
	// is published first: a crash between the two leaves an unopened
	// pending turn for resume healing, never a marker without its record.
	diskState, err := readDocLabeled(c.statePath, "mission state", 3)
	if err != nil {
		return nil, true, err
	}
	// STICKY EVIDENCE without a marker (successor round-2 finding 2): a
	// violation whose evidence landed but whose park crashed must
	// re-execute before anything opens — even if the workspace was
	// cleaned up in between, the detected violation is a fact.
	if orphanTurn, orphanViolation := e.orphanedViolationEvidence(diskState); orphanViolation != "" {
		orphanDir := filepath.Join(e.missionDir(), "turns", orphanTurn)
		final, err := e.parkWallViolation(c.statePath, c.ledger, orphanTurn, orphanDir, cycle, orphanViolation, diskState, false)
		return final, true, err
	}
	// E-CONTINUITY at reservation: between turns the filtered
	// projection must still equal the current expected tree — the
	// admitted baseline before any turn, the last accepted post-tree
	// after one, the resolution tree after a ruling. Anything else is
	// drift the tree equation would silently grandfather as the next
	// baseline, so it parks as a wall violation BEFORE a marker opens
	// on it.
	expectedNow := mission.CurrentExpectedTree(diskState)
	if expectedNow == "" {
		// Every state that passes shape validation carries its
		// baseline, so this branch is a fail-closed belt for shapes
		// that never should reach reservation: adopting the current
		// workspace as the expected tree here would silently redefine
		// the baseline the wall exists to pin.
		return nil, true, failf(3, "mission state records no initial baseline and no expected-tree event; re-provision the mission under the wall's preflight")
	}
	if expectedNow != preTree {
		violation := fmt.Sprintf("workspace drifted between turns: the filtered projection %s does not equal the expected tree %s", preTree, expectedNow)
		// The violation writes its wall.json evidence like every other
		// (round-3 finding 7): the observed projection IS both the pre-
		// and post-tree of a turn that never launched.
		evidence := &wallInspection{PreTree: preTree, ExpectedTree: expectedNow, PostTree: preTree, Violation: violation}
		if changed, derr := workspace.ChangedPaths(expectedNow, preTree); derr == nil {
			evidence.Unaccounted = changed
		}
		if err := atomicWriteJSON(filepath.Join(turnDir, "wall.json"), evidence.document()); err != nil {
			return nil, true, err
		}
		final, err := e.parkWallViolation(c.statePath, c.ledger, turnID, turnDir, cycle, violation, diskState, false)
		return final, true, err
	}
	// Between-turns continuity beyond the worktree: HEAD, the ref map,
	// both staged scopes, the toplevel, and the worktree census are
	// judged from the PREVIOUS acceptance's recorded posture (turn one:
	// the birth record's admission origins) — no host or peer motion
	// between turns escapes; an illicit commit made after one acceptance
	// refuses the next open exactly like mid-turn motion.
	origin := lastAcceptancePosture(diskState)
	if origin == nil {
		return nil, true, failf(3, "mission state records no accounting origin; re-provision the mission under the wall's preflight")
	}
	// The open capture skips the seeded worktree projection: the E-continuity
	// equality above already judged the worktree, and the continuity rules
	// judge only the other carriers.
	capture, err := e.captureWallPostureStable("", nil)
	if err != nil {
		if answer := stateAnswerOf(err); answer != "" {
			evidence := &wallInspection{PreTree: preTree, ExpectedTree: expectedNow, PostTree: preTree, Violation: answer}
			if werr := atomicWriteJSON(filepath.Join(turnDir, "wall.json"), evidence.document()); werr != nil {
				return nil, true, werr
			}
			final, ferr := e.parkWallViolation(c.statePath, c.ledger, turnID, turnDir, cycle, answer, diskState, false)
			return final, true, ferr
		}
		return nil, true, err
	}
	acct, err := e.newWallAccountant(expectedNow, diskState, nil, nil)
	if err != nil {
		return nil, true, err
	}
	continuityViolation, err := e.judgeScope(origin, capture, acct, diskState)
	if err != nil {
		// A ran-and-answered probe defeat inside the judgment is a wall
		// answer (WSS I13-6), the same as one raised during capture.
		if answer := stateAnswerOf(err); answer != "" {
			continuityViolation = answer
		} else {
			return nil, true, err
		}
	}
	if continuityViolation != "" {
		evidence := &wallInspection{PreTree: preTree, ExpectedTree: expectedNow, PostTree: preTree, Violation: continuityViolation}
		evidence.Scope = scopeEvidence(origin, capture)
		if err := atomicWriteJSON(filepath.Join(turnDir, "wall.json"), evidence.document()); err != nil {
			return nil, true, err
		}
		final, err := e.parkWallViolation(c.statePath, c.ledger, turnID, turnDir, cycle, continuityViolation, diskState, false)
		return final, true, err
	}
	// The open origins anchor BEFORE the host launches: the open commit
	// stays reachable through a runner-owned CAS ref so a mid-turn reset
	// cannot orphan the accounting origin, and the recorded toplevel
	// trees anchor against garbage collection.
	if err := workspace.AnchorCommit(e.Mission, "turn-open-head", capture.Head); err != nil {
		return nil, true, failf(3, "turn open cannot anchor the open commit: %v", err)
	}
	headTreeRaw, err := workspace.TreeOf(capture.Head)
	if err != nil {
		return nil, true, failf(3, "turn open cannot read the open commit's tree: %v", err)
	}
	headTree, err := workspace.FilterTree(headTreeRaw, []string{missionLedgerRel(e.Mission)})
	if err != nil {
		return nil, true, failf(3, "turn open cannot project the open commit's tree: %v", err)
	}
	var openTopTree any
	var openTopStaged any
	if capture.Nested {
		for _, tree := range []string{capture.TopTree, capture.TopStaged.Tree} {
			if err := workspace.Anchor(e.Mission, tree); err != nil {
				return nil, true, failf(3, "turn open cannot anchor %s: %v", tree, err)
			}
		}
		openTopTree = capture.TopTree
		openTopStaged = mission.StagedPostureDoc(capture.TopStaged)
	}
	openRefMap := map[string]any{}
	for name, oid := range capture.RefMap {
		openRefMap[name] = oid
	}
	// The marker's occurrence identity is the CURRENT SEQUENCE POINT —
	// the acceptance that produced this expected tree — never the raw
	// chain position (critique F-2: parks and heals advance the chain
	// without changing the expected tree, and tree ids repeat).
	sequence, segment := mission.CurrentSequencePoint(diskState)
	opened := deepCopyDoc(diskState)
	opened["openTurn"] = map[string]any{
		"turnId":     turnID,
		"cycle":      cycle,
		"preTree":    preTree,
		"sequence":   sequence,
		"segment":    segment,
		"openedAt":   nowISO(),
		"headCommit": capture.Head,
		"headTree":   headTree,
		"topTree":    openTopTree,
		"refMap":     openRefMap,
		"topStaged":  openTopStaged,
	}
	updated, err := e.writeState(c.statePath, opened)
	if err != nil {
		return nil, true, err
	}
	// The open write anchors like every other state write (critique F-1):
	// a crash mid-turn must reconcile against THIS state, not park on an
	// anchor mismatch.
	if err := e.anchor(c.statePath, c.ledger, turnID); err != nil {
		return nil, true, err
	}
	c.state = updated
	return nil, false, nil
}

// cycleGatePrompt assembles the turn prompt and holds it to the prompt
// checker; a refusal parks the mission rather than burning a second cycle.
func (e *Engine) cycleGatePrompt(c *cycleContext) (map[string]any, bool, error) {
	if err := mission.AssemblePrompt(e.Root, e.Mission, c.turnID, filepath.Join(c.turnDir, "prompt.md")); err != nil {
		detail := strings.TrimSpace(err.Error())
		if detail == "" {
			detail = "prompt assembly refused"
		}
		final, ferr := e.failTurnBeforeLaunch(c.statePath, c.ledger, c.state, c.turnPath, detail)
		return final, true, ferr
	}
	stdout, stderr, code := runCaptured(e.Root, nil,
		filepath.Join(e.Root, "scripts", "assert-turn-prompt.sh"),
		"--file", filepath.Join(c.turnDir, "prompt.md"), "--turn", c.turnDir)
	if code != 0 {
		detail := firstDetail(stderr, stdout)
		if detail == "" {
			detail = "turn prompt checker refused launch"
		}
		final, ferr := e.failTurnBeforeLaunch(c.statePath, c.ledger, c.state, c.turnPath, detail)
		return final, true, ferr
	}
	return nil, false, nil
}

// cycleRunHost launches the host, stamps the observed session, and settles
// the launch-level outcomes: unverified start, unresumable session, the
// cap, and a plain non-zero exit.
func (e *Engine) cycleRunHost(c *cycleContext) (map[string]any, bool, error) {
	e.emit("turn-launched", fmt.Sprintf("cycle %d", c.cycle), map[string]string{
		"missionId": e.Mission, "turnId": c.turnID,
	})
	exitCode, result, launchDetail, err := e.launchHost(c.turnID, c.turnDir, c.turn, c.leasePath, c.startSignal, c.notified)
	if err != nil {
		return nil, true, err
	}
	c.exitCode, c.result, c.launchDetail = exitCode, result, launchDetail
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
		"missionId": e.Mission, "turnId": c.turnID, "outcome": outcomeField,
	})
	if _, err := e.stampObservedSession(c.turnDir, result); err != nil {
		return nil, true, err
	}
	if launchDetail == "start-unverified" {
		final, ferr := e.recordFailedTurn(c.statePath, c.ledger, c.state, c.turnPath, launchDetail, "failed", 2)
		return final, true, ferr
	}
	if exitCode == 6 {
		// The adapter's genuine fault signal: the envelope carries no session
		// at all. Rotation no longer lands here — a rotated session is
		// reported in the envelope and judged at adjudication.
		if _, err := patchTurn(c.turnPath, map[string]any{
			"status": "failed", "outcome": "unresumable", "error": "unresumable",
			"detail": "host session is not resumable", "endedAt": nowISO(),
		}); err != nil {
			return nil, true, err
		}
		final, ferr := e.recordFailedTurn(c.statePath, c.ledger, c.state, c.turnPath, "host session is not resumable", "unresumable", c.priorFailures)
		return final, true, ferr
	}
	if launchDetail == "capped" {
		// The cap fired: the turn keeps outcome=capped, and the cycle still
		// drains, measures, and concludes, so a cap that landed real work
		// registers as the progress it made.
		final, ferr := e.concludeFaultedTurn(c.statePath, c.ledger, c.state, c.turnPath, c.turnDir, TurnFault{
			Outcome:      "capped",
			Detail:       "host turn reached host.turn-cap-min",
			FeedsBreaker: true,
			Annotations:  []string{mission.CappedAnnotation},
		}, c.priorFailures+1)
		return final, true, ferr
	}
	if exitCode != 0 || result == nil {
		detail := launchDetail
		if result != nil {
			detail = fmt.Sprintf("host exited non-zero (%d)", exitCode)
		}
		// A host CUT OFF by its native turn cap is a different fact
		// than a host that crashed (issue #6): the ledger and the
		// operator must be able to tell them apart, and the next turn
		// can be told to continue rather than restart.
		if subtype := providerErrorSubtype(filepath.Join(c.turnDir, "claude-result.json")); subtype == "error_max_turns" {
			detail = "host-cap-exhausted: the adapter's native turn cap ended the turn (error_max_turns)"
		}
		if _, err := patchTurn(c.turnPath, map[string]any{
			"status": "failed", "outcome": "failed", "error": "host-failure",
			"detail": detail, "endedAt": nowISO(),
		}); err != nil {
			return nil, true, err
		}
		final, ferr := e.recordFailedTurn(c.statePath, c.ledger, c.state, c.turnPath, detail, "failed", c.priorFailures+1)
		return final, true, ferr
	}
	return nil, false, nil
}

// cycleAdjudicate judges the host's return, records the verdict, and
// publishes the proposed asks; a rejected return concludes as a faulted
// turn that still drains and measures.
func (e *Engine) cycleAdjudicate(c *cycleContext) (map[string]any, bool, error) {
	verdict, err := AdjudicateFiles(e.Root, e.Mission, c.statePath, c.turnPath, filepath.Join(c.turnDir, "result.json"), c.turnDir, nowISO())
	if err != nil {
		detail := err.Error()
		if _, patchErr := patchTurn(c.turnPath, map[string]any{
			"status": "failed", "outcome": "failed", "error": "protocol-error",
			"detail": detail, "endedAt": nowISO(), "result": c.result,
		}); patchErr != nil {
			return nil, true, patchErr
		}
		// A rejected return is never applied, but the cycle keeps its duties:
		// drain, measure, conclude with both facts. Only a mismatch nobody
		// witnessed is kept off the breaker.
		var sessionFault *SessionFault
		feedsBreaker := true
		if errors.As(err, &sessionFault) && !sessionFault.Witnessed {
			feedsBreaker = false
		}
		final, ferr := e.concludeFaultedTurn(c.statePath, c.ledger, c.state, c.turnPath, c.turnDir, TurnFault{
			Outcome:      "failed",
			Detail:       detail,
			FeedsBreaker: feedsBreaker,
			Annotations:  []string{mission.ReturnRejectedAnnotation(detail)},
		}, c.priorFailures+1)
		return final, true, ferr
	}
	c.verdict = verdict

	// The verdict is the audit record of what this turn's return claimed and
	// what the runner made of it; the conclusion reads it back below.
	verdictDoc, err := docFromValue(verdict)
	if err != nil {
		return nil, true, err
	}
	c.verdictPath = filepath.Join(c.turnDir, "adjudication.json")
	if err := atomicWriteJSON(c.verdictPath, verdictDoc); err != nil {
		return nil, true, err
	}
	if err := os.MkdirAll(asksDirPath(e.Root, e.Mission), 0o755); err != nil {
		return nil, true, err
	}
	if err := e.writeProposedAsks(verdict.Asks); err != nil {
		return nil, true, err
	}
	return nil, false, nil
}

// cycleConclude drains the jobs, measures, books the ledger, concludes the
// state, patches the turn terminal, anchors, and applies the stop-loss.
func (e *Engine) cycleConclude(c *cycleContext) (map[string]any, bool, error) {
	// The turn's ADJUDICATED certifications participate in this booking's
	// patience evaluation before anything is written (r2/P4-016): a job
	// certified by THIS turn is not booked barren in the same breath —
	// but only a claim that survived verification counts (HIW-O5).
	var inflightCertified any
	if c.verdict != nil {
		entries := make([]any, 0, len(c.verdict.Certified))
		for _, entry := range c.verdict.Certified {
			entries = append(entries, entry)
		}
		inflightCertified = entries
	}
	final, err := e.concludeCycle(c.statePath, c.ledger, c.state, concludeSpec{
		turnID:            c.turnID,
		cycle:             c.cycle,
		turnDir:           c.turnDir,
		inflightCertified: inflightCertified,
		certified:         c.verdict.Certified,
		propose: func(measurementValue any, gatePassed bool) (map[string]any, error) {
			return ConcludeFiles(e.Root, e.Mission, c.statePath, c.turnPath,
				c.verdictPath, c.verdict.ReturnPath, filepath.Join(c.turnDir, "result.json"),
				filepath.Join(c.turnDir, "measurement.json"))
		},
		afterWrite: func(map[string]any) error {
			// The record carries the artifacts, never the SUCCESS: the
			// terminal completed outcome lands only after the
			// post-verification (a crash in between leaves a non-terminal
			// record that verification completes at resume).
			_, err := patchTurn(c.turnPath, map[string]any{
				"result":  c.result,
				"rawPath": c.verdict.RawPath, "returnPath": c.verdict.ReturnPath,
			})
			return err
		},
	})
	return final, true, err
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

// reclaimCheckout re-announces the RUN LOOP before an anchor (issue #2,
// D102, round-3): the loop — not the launcher whose announcement armed
// the checkout — is the process that anchors, so it announces ITSELF
// under the mission lineage and the claim ladder does the rest: a dead
// prior holder (a finished host turn, the exited detached launcher)
// yields by ordinary same-lineage succession, and a LIVE runner-side
// holder (the foreground launcher) yields through the ancestry edge,
// because the loop is its real kernel child. STRICTLY a no-op in the
// ATTENDED branch: the unattended marker is the checkout lease carrying
// THIS mission's owner lineage — a human-held or foreign lease means
// nothing here is ours to reclaim (round-2 F3's shadow stays impossible).
// The announce passes the FRESHLY probed start second — self-consistent
// at call time by construction — and reannouncement matching recognizes
// the loop's earlier announcement by the clock-step-immune pair, so
// drift cannot desynchronize it (round-3 F2). A FAILED reclaim is
// returned, not swallowed: anchoring without holdership is exactly the
// defect the reclaim exists to prevent.
func (e *Engine) reclaimCheckout() error {
	if !e.unattendedCheckout {
		return nil
	}
	self := int64(os.Getpid())
	// Class HOLDER exactly (round-6): RequireHolder answers Holder:true
	// for HUMAN before reading the lease, and a detached runner whose
	// launcher ancestor died classifies HUMAN — that must reclaim, not
	// silently skip and anchor ungated.
	if view, err := lease.RequireHolder(e.Root, self, nil); err == nil && view.Class == "HOLDER" {
		return nil
	}
	started, ok := lease.StartedAt(self, nil)
	if !ok {
		return failf(3, "checkout reclaim cannot read its own identity")
	}
	session := fmt.Sprintf("mission-runner-%s-%d", e.Mission, self)
	_, announceErr := lease.Announce(e.Root, session, self, started,
		"mission-runner.sh", "metasystem", MissionLineage(e.Mission))
	// Holdership is PROVEN through the PRODUCTION gate (round-5): the
	// same RequireHolder every gated verb answers to — holder identity
	// AND a complete claim stamp; a saved-but-unstamped succession, a
	// silent live-holder refusal, and the check-to-claim race all fail
	// this gate and surface here.
	view, err := lease.RequireHolder(e.Root, self, nil)
	if err == nil && view.Class == "HOLDER" {
		return nil
	}
	detail := "checkout is held elsewhere"
	if announceErr != nil {
		detail = announceErr.Error()
	} else if err != nil {
		detail = err.Error()
	}
	e.emit("lease-reclaim-skipped", clipSummary(detail), map[string]string{
		"missionId": e.Mission, "error": detail,
	})
	return failf(3, "checkout reclaim refused: %s", detail)
}

// providerErrorSubtype reads the error subtype from a provider result
// file, empty when absent or not an error.
func providerErrorSubtype(path string) string {
	doc, err := readJSONDoc(path)
	if err != nil {
		return ""
	}
	if isError, _ := doc["is_error"].(bool); !isError {
		return ""
	}
	subtype, _ := doc["subtype"].(string)
	return subtype
}

// refuseUnsupportedSemantics raw-reads a mission state's ledgerSemantics
// and refuses values this runner does not implement, before ANY side
// effect. An absent state (fresh start) or unreadable file passes — init
// writes a supported value, and unreadable states fail loudly in the
// validated read paths that follow.
func refuseUnsupportedSemantics(statePath string) error {
	doc, err := readJSONDoc(statePath)
	if err != nil {
		return nil
	}
	semantics, ok := jsonInt(doc["ledgerSemantics"])
	if !ok {
		return nil // legacy semantics-1 states carry no field
	}
	if semantics < 1 || semantics > 3 {
		return failf(3, "mission ledgerSemantics %d is newer than this runner", semantics)
	}
	return nil
}
