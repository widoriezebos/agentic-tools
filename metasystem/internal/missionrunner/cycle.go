package missionrunner

import (
	"fmt"
	"os"
	"path/filepath"
)

// The continue/park/complete decision after a turn, and the state proposals
// it produces. Each function returns a proposed next state; the runner owns
// applying it through the hash-chained compare-and-write and anchoring it, so
// this package never writes the state itself.

// ProjectFences copies the fence counters into a state's fences block: the
// authoritative startedAt and cycle count from fences.json, the number of
// reserved jobs, how many of them are still active, and the aggregated usage
// units when present. A reservation whose job record is missing or unreadable
// counts as active — losing a record must never relax a fence.
func ProjectFences(root, mission string, state map[string]any) error {
	stateFences, ok := state["fences"].(map[string]any)
	if !ok {
		return fmt.Errorf("mission state fences are unreadable")
	}
	dir := missionDirPath(root, mission)
	fencesPath := filepath.Join(dir, "fences.json")
	if _, err := os.Stat(fencesPath); err == nil {
		fences, err := readJSONDoc(fencesPath)
		if err != nil {
			return fmt.Errorf("mission fence counters is unreadable: %s: %v", fencesPath, err)
		}
		reservations := map[string]any{}
		if raw, present := fences["reservations"]; present {
			reservations, ok = raw.(map[string]any)
			if !ok {
				return fmt.Errorf("mission fence reservations are unreadable")
			}
		}
		active := 0
		for job := range reservations {
			record, err := readJSONDoc(filepath.Join(jobsDirPath(root), job+".json"))
			status := ""
			if err == nil {
				status, _ = record["status"].(string)
			}
			if !terminalJobStatuses[status] {
				active++
			}
		}
		if value, present := fences["startedAt"]; present {
			stateFences["startedAt"] = value
		}
		if value, present := fences["cycles"]; present {
			stateFences["cycles"] = value
		}
		stateFences["jobs"] = len(reservations)
		stateFences["activeJobs"] = active
	}
	usagePath := filepath.Join(dir, "usage.json")
	if _, err := os.Stat(usagePath); err == nil {
		usage, err := readJSONDoc(usagePath)
		if err != nil {
			return fmt.Errorf("mission usage is unreadable: %s: %v", usagePath, err)
		}
		if units, ok := usage["units"].([]any); ok {
			stateFences["usage"] = units
		}
	}
	return nil
}

// TurnConclusion carries what the runner learned from a completed turn into
// its turn-log entry: the host session, the measurement (nil when the cycle
// was unmeasurable), the gate verdict, the adjudication's accepted and
// rejected claims, and the return's certification, facts, and gaps.
type TurnConclusion struct {
	SessionID      any
	Measurement    any
	GatePassed     bool
	Accepted       any
	Rejected       any
	Certified      any
	FactsForLedger any
	Gaps           any
}

// ConcludeTurn proposes the state after a turn whose return was accepted. The
// input state must already carry the adjudicated stream map. The proposal
// appends the turn-log entry, advances the ledger cycle count, refreshes the
// waiting list from the asks on disk, projects the fences, and decides the
// mission's next status: completed when the gate passed, parked when no
// stream is left active, otherwise still running.

// wallEntryPayload reads the turn's wall.json evidence back for the
// acceptance entry, FAIL CLOSED: a turn with no
// evidence, unreadable evidence, or a non-passed verdict never concludes
// into the log — deleting wall.json must never buy a wall-free acceptance.
func wallEntryPayload(root, mission, turnID string, state map[string]any) (map[string]any, []any, error) {
	doc, err := readJSONDoc(filepath.Join(missionDirPath(root, mission), "turns", turnID, "wall.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("turn %s has no readable wall evidence; the equation must run before any conclusion: %v", turnID, err)
	}
	if verdict, _ := doc["verdict"].(string); verdict != "passed" {
		return nil, nil, fmt.Errorf("turn %s wall evidence is not a pass; a violated turn never concludes into the log", turnID)
	}
	ordered, _ := doc["orderedDigests"].([]any)
	if ordered == nil {
		ordered = []any{}
	}
	// The payload's occurrence identity names the write that will land
	// it: this proposal builds on state at sequence S, so acceptance
	// lands at S+1 under the compare-and-write; the transition validator
	// refuses anything else.
	integrity, _ := state["integrity"].(map[string]any)
	sequence, ok := jsonInt(integrity["sequence"])
	if !ok {
		return nil, nil, fmt.Errorf("turn %s conclusion cannot read the state sequence", turnID)
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	segment, ok := jsonInt(taint["segment"])
	if !ok {
		return nil, nil, fmt.Errorf("turn %s conclusion cannot read the taint segment", turnID)
	}
	// The recorded posture rides the acceptance into the CHAIN (fail
	// closed): every between-turns comparison reads the chain, never
	// wall.json — evidence files are rewritable and prove nothing
	// forward.
	posture, _ := doc["posture"].(map[string]any)
	if posture == nil {
		return nil, nil, fmt.Errorf("turn %s wall evidence carries no recorded posture; the snapshot-scope inspection must run before any conclusion", turnID)
	}
	payload := map[string]any{
		"verdict": "passed", "preTree": doc["preTree"], "expectedTree": doc["expectedTree"],
		"postTree": doc["postTree"], "orderedDigests": ordered,
		"sequencePoint": map[string]any{"sequence": sequence + 1, "segment": segment},
	}
	for _, field := range []string{"headCommitPost", "refMapPost", "stagedTreePost", "topTreePost", "topStagedPost", "worktreeCensusPost", "capturedAt"} {
		payload[field] = posture[field]
	}
	consumed := make([]any, len(ordered))
	copy(consumed, ordered)
	return payload, consumed, nil
}

func ConcludeTurn(root, mission string, state map[string]any, turn Turn, conclusion TurnConclusion) (map[string]any, error) {
	proposed := deepCopyDoc(state)
	entry := map[string]any{
		"turnId":         turn.TurnID,
		"cycle":          turn.Cycle,
		"outcome":        "completed",
		"detail":         "host return accepted",
		"sessionId":      conclusion.SessionID,
		"measurement":    conclusion.Measurement,
		"accepted":       conclusion.Accepted,
		"rejected":       conclusion.Rejected,
		"certified":      conclusion.Certified,
		"factsForLedger": conclusion.FactsForLedger,
		"gaps":           conclusion.Gaps,
	}
	// The entry carries the wall verdict and its consumptions, and the
	// open-turn marker dies in the same write: acceptance is ONE write,
	// never a second one, and never without evidence.
	wall, consumed, err := wallEntryPayload(root, mission, turn.TurnID, proposed)
	if err != nil {
		return nil, err
	}
	entry["wall"] = wall
	entry["consumedAuthorizations"] = consumed
	entry["gatePassed"] = conclusion.GatePassed
	if err := appendTurnLog(proposed, entry); err != nil {
		return nil, err
	}
	// The acceptance append stays THE single commit point but
	// no longer concludes the turn: openTurn survives it, and the
	// post-verification entry concludes on a clean re-capture — so
	// COMPLETION (the one success outcome) defers to that write, while
	// non-success parks land here as before.
	if err := setLedgerCycles(proposed, turn.Cycle); err != nil {
		return nil, err
	}
	proposed["waitingList"] = openAskIDs(asksDirPath(root, mission))
	aggregateUsageForProjection(root, mission, "conclude")
	if err := ProjectFences(root, mission, proposed); err != nil {
		return nil, err
	}
	switch {
	case conclusion.GatePassed:
		proposed["status"] = "running"
		proposed["parkReason"] = nil
		proposed["gatePassed"] = false
	case !anyActiveStream(proposed):
		proposed["status"] = "parked"
		proposed["parkReason"] = "all-streams-parked"
		proposed["gatePassed"] = false
	default:
		proposed["status"] = "running"
		proposed["parkReason"] = nil
		proposed["gatePassed"] = false
	}
	return proposed, nil
}

// ConclusionSession is the session a concluded turn records in its turn-log
// entry, which the next turn's Host-Session announcement derives from: the
// harness-observed session when one exists (whatever the return's fate, so a
// trusted witness corrects a stale announcement instead of compounding it),
// else the envelope session a legacy unstamped turn carried, else the
// announced session the turn ran under.
func ConclusionSession(turn Turn, envelopeSession any) any {
	if turn.ObservedSession != nil {
		return turn.ObservedSession
	}
	if envelopeSession != nil {
		return envelopeSession
	}
	return turn.AnnouncedSession
}

// TurnFault names the fault a turn on the faulted-conclusion path carries
// beside its measurement: the recorded outcome ("failed" for a rejected
// return, "capped" for a fired cap), the one-line detail, whether the fault
// counts toward the consecutive-host-failure breaker (witnessed protocol
// violations and caps do; a no-witness session mismatch convicts nobody),
// and the ledger annotation lines recording the fault in the cycle block.
type TurnFault struct {
	Outcome      string
	Detail       string
	FeedsBreaker bool
	Annotations  []string
}

// ConcludeFaultedTurn is the one application rule, stated here once and
// called from both faulted paths (rejected return, fired cap): a return's
// state mutations — stream transitions, asks, waiting list — are applied
// ONLY when the return is accepted, so this proposal carries an empty
// verdict: no accepted entries, no asks, no stream transitions; streams keep
// the states they had at turn start and open asks stay open. Measurement
// effects always conclude from the measured tree, whatever the return's
// fate: the turn-log entry carries the measurement beside the fault, and
// completion on a measured gate pass is the runner's own transition, legal
// from any stream configuration. There is no third case. The breaker is
// decoupled from the classification: a breaker-fed fault parks host-failure
// on the second consecutive failure exactly as a plain failed turn does,
// unless the measured gate passed — runner-run measurement is truth, and a
// broken envelope does not un-build the product.
func ConcludeFaultedTurn(root, mission string, state map[string]any, turn Turn, fault TurnFault, measurement any, gatePassed bool, consecutiveFailures int) (map[string]any, error) {
	proposed := deepCopyDoc(state)
	entry := map[string]any{
		"turnId":       turn.TurnID,
		"cycle":        turn.Cycle,
		"outcome":      fault.Outcome,
		"detail":       fault.Detail,
		"sessionId":    ConclusionSession(turn, nil),
		"measurement":  measurement,
		"annotations":  fault.Annotations,
		"feedsBreaker": fault.FeedsBreaker,
	}
	// The entry carries the wall verdict and its consumptions, and the
	// open-turn marker dies in the same write: acceptance is ONE write,
	// never a second one, and never without evidence.
	wall, consumed, err := wallEntryPayload(root, mission, turn.TurnID, proposed)
	if err != nil {
		return nil, err
	}
	entry["wall"] = wall
	entry["consumedAuthorizations"] = consumed
	entry["gatePassed"] = gatePassed
	if err := appendTurnLog(proposed, entry); err != nil {
		return nil, err
	}
	// openTurn survives the append; the post-verification entry concludes
	// the turn, and completion — the one success outcome — defers there.
	if err := setLedgerCycles(proposed, turn.Cycle); err != nil {
		return nil, err
	}
	proposed["waitingList"] = openAskIDs(asksDirPath(root, mission))
	aggregateUsageForProjection(root, mission, "conclude-faulted")
	if err := ProjectFences(root, mission, proposed); err != nil {
		return nil, err
	}
	switch {
	case gatePassed:
		proposed["status"] = "running"
		proposed["parkReason"] = nil
		proposed["gatePassed"] = false
	case fault.FeedsBreaker && consecutiveFailures >= 2:
		proposed["status"] = "parked"
		proposed["parkReason"] = "host-failure"
		proposed["gatePassed"] = false
	default:
		proposed["status"] = "running"
		proposed["parkReason"] = nil
		proposed["gatePassed"] = false
	}
	return proposed, nil
}

// RecordFailureProposal proposes the state after a turn that produced no
// usable return: a turn-log entry with no session and no measurement, the
// ledger cycle advanced (a failed turn still spends its cycle), and — on the
// second consecutive failure — a host-failure park, because two dead turns in
// a row need a human, not a third attempt.
func RecordFailureProposal(root, mission string, state map[string]any, turn Turn, detail, outcome string, consecutiveFailures int) (map[string]any, error) {
	proposed := deepCopyDoc(state)
	entry := map[string]any{
		"turnId":      turn.TurnID,
		"cycle":       turn.Cycle,
		"outcome":     outcome,
		"detail":      detail,
		"sessionId":   nil,
		"measurement": nil,
	}
	// The entry carries the wall verdict and its consumptions, and the
	// open-turn marker dies in the same write: acceptance is ONE write,
	// never a second one, and never without evidence.
	wall, consumed, err := wallEntryPayload(root, mission, turn.TurnID, proposed)
	if err != nil {
		return nil, err
	}
	entry["wall"] = wall
	entry["consumedAuthorizations"] = consumed
	if err := appendTurnLog(proposed, entry); err != nil {
		return nil, err
	}
	// openTurn survives the append; the post-verification entry concludes.
	if err := setLedgerCycles(proposed, turn.Cycle); err != nil {
		return nil, err
	}
	aggregateUsageForProjection(root, mission, "record-failure")
	if err := ProjectFences(root, mission, proposed); err != nil {
		return nil, err
	}
	if consecutiveFailures >= 2 {
		proposed["status"] = "parked"
		proposed["parkReason"] = "host-failure"
		proposed["gatePassed"] = false
	}
	return proposed, nil
}

// ParkOutcome is a park proposal: the parked state and the asks the runner
// must write before applying it.
type ParkOutcome struct {
	State map[string]any   `json:"state"`
	Asks  []map[string]any `json:"asks"`
}

// ParkProposal proposes parking the mission for a reason. Parking for
// host-failure or stop-loss guarantees an open ask with that reason class
// exists — proposing one when none is open — because a park a human cannot
// answer is a mission that can never resume. The waiting list reflects the
// asks once the caller has written them.
func ParkProposal(root, mission string, state map[string]any, reason, nowISO string) (*ParkOutcome, error) {
	question := ""
	switch reason {
	case "host-failure":
		question = "Acknowledge the host failure before resuming the mission."
	case "stop-loss":
		question = "Amend, price, reseal, and sign the mission budget before requesting stop-loss unpark."
	case "wall-violation":
		question = "The workspace differs from the pre-tree plus authorized patches. Resolve the taint by name (restore the recorded safe tree, or adopt the disputed tree) before the mission can continue; a free-text answer never clears taint."
	}
	return parkOutcome(root, mission, state, reason, question, "", nowISO)
}

// StopLossParkProposal proposes the stop-loss park for a derived verdict:
// the ask's wording matches the trip kind, and a replay-semantics ask records
// that kind so the answer path and the reconciliation tolerance can tell a
// resettable stagnation park from an amendment-only cycle-budget park.
func StopLossParkProposal(root, mission string, state map[string]any, kind, question, nowISO string) (*ParkOutcome, error) {
	askKind := ""
	if kind == StopLossStagnation || kind == StopLossCycleBudget {
		askKind = kind
	}
	return parkOutcome(root, mission, state, "stop-loss", question, askKind, nowISO)
}

// parkOutcome builds a park proposal: the parked state, and — when the
// reason carries a question — the open ask that makes the park answerable.
func parkOutcome(root, mission string, state map[string]any, reason, question, stopLossKind, nowISO string) (*ParkOutcome, error) {
	proposed := deepCopyDoc(state)
	asksDir := asksDirPath(root, mission)
	outcome := &ParkOutcome{Asks: []map[string]any{}}
	newAskIDs := []string{}
	// A wall-violation ask suppresses a new one only while it is STILL
	// RELEVANT — bound to a taint the state records as unresolved:
	// a stale tail (resolved taint, crash
	// before its answer landed) or an unbound ask must never strand the
	// NEXT taint askless.
	suppressed := hasOpenAskWithReason(asksDir, reason)
	if reason == "wall-violation" {
		suppressed = hasOpenWallAskForUnresolvedTaint(asksDir, proposed)
	}
	if question != "" && !suppressed {
		streams, ok := proposed["streams"].(map[string]any)
		if !ok || len(streams) == 0 {
			return nil, fmt.Errorf("mission state has no streams")
		}
		askID := nextAskID(asksDir, reason, map[string]bool{})
		ask := askRecord(askID, fallbackStream(streams), reason, question, nowISO)
		if stopLossKind != "" {
			ask["stopLossKind"] = stopLossKind
		}
		outcome.Asks = append(outcome.Asks, ask)
		newAskIDs = append(newAskIDs, askID)
	}
	proposed["status"] = "parked"
	proposed["parkReason"] = reason
	proposed["gatePassed"] = false
	proposed["waitingList"] = mergedOpenAskIDs(asksDir, newAskIDs)
	aggregateUsageForProjection(root, mission, "park")
	if err := ProjectFences(root, mission, proposed); err != nil {
		return nil, err
	}
	outcome.State = proposed
	return outcome, nil
}

// hasOpenAskWithReason reports whether any unanswered ask on disk already
// carries the reason class, so a re-park does not stack duplicate asks.
// A SUPERSEDED ask does not count: prompts and the
// waiting list hide it, so letting it satisfy the park guarantee would
// leave a parked mission with no visible answer path.
func hasOpenAskWithReason(asksDir, reason string) bool {
	superseded := supersededAskIDs(asksDir)
	paths, _ := filepath.Glob(filepath.Join(asksDir, "*.json"))
	for _, path := range paths {
		doc, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		askID, _ := doc["askId"].(string)
		if _, closed := superseded[askID]; closed {
			continue
		}
		if doc["answeredAt"] == nil && doc["supersededBy"] == nil && doc["reasonClass"] == reason {
			return true
		}
	}
	return false
}

func appendTurnLog(state map[string]any, entry map[string]any) error {
	log, ok := state["turnLog"].([]any)
	if !ok {
		return fmt.Errorf("mission state turn log is unreadable")
	}
	state["turnLog"] = append(log, entry)
	return nil
}

func setLedgerCycles(state map[string]any, cycle any) error {
	ledger, ok := state["ledger"].(map[string]any)
	if !ok {
		return fmt.Errorf("mission state ledger block is unreadable")
	}
	ledger["cycles"] = cycle
	return nil
}

func anyActiveStream(state map[string]any) bool {
	streams, _ := state["streams"].(map[string]any)
	for _, raw := range streams {
		if stream, ok := raw.(map[string]any); ok {
			if streamState, _ := stream["state"].(string); streamState == "active" {
				return true
			}
		}
	}
	return false
}
