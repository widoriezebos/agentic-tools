package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const ledgerAttentionStateSchema = 2

const (
	ledgerAttentionStateWriteFailed = "STATE_WRITE_FAILED"
	ledgerAttentionFetchFailed      = "FETCH_FAILED"
)

// LedgerAttentionEvent is one accepted ledger change with facts for this
// machine. SourceID is stable across retry and unique across sanctioned
// accepted-ref rewinds.
type LedgerAttentionEvent struct {
	SourceID  string   `json:"sourceId"`
	Tip       string   `json:"tip"`
	At        string   `json:"at"`
	Claimable []string `json:"claimable,omitempty"`
	Pins      []string `json:"pins,omitempty"`
	QueueNow  []string `json:"queueNow,omitempty"`
	QueueWas  []string `json:"queueWas,omitempty"`
}

type ledgerAttentionStage struct {
	From                string                 `json:"from"`
	Tip                 string                 `json:"tip"`
	Events              []LedgerAttentionEvent `json:"events,omitempty"`
	Ready               []string               `json:"ready,omitempty"`
	Pinned              []string               `json:"pinned,omitempty"`
	Queue               []string               `json:"queue,omitempty"`
	TopologyEpoch       uint64                 `json:"topologyEpoch,omitempty"`
	RepairBaseline      []string               `json:"repairBaseline,omitempty"`
	RepairBaselineReady bool                   `json:"repairBaselineReady,omitempty"`
}

type ledgerAttentionState struct {
	Schema          int                    `json:"schema"`
	LastAttemptAt   string                 `json:"lastAttemptAt,omitempty"`
	LastOutcome     string                 `json:"lastOutcome,omitempty"`
	LastFailure     string                 `json:"lastFailure,omitempty"`
	FailingSince    string                 `json:"failingSince,omitempty"`
	RemoteTip       string                 `json:"remoteTip,omitempty"`
	RemoteTipAt     string                 `json:"remoteTipAt,omitempty"`
	ExaminedTip     string                 `json:"examinedTip,omitempty"`
	MovedAt         string                 `json:"movedAt,omitempty"`
	DiffedTip       string                 `json:"diffedTip,omitempty"`
	Ready           []string               `json:"ready,omitempty"`
	Pinned          []string               `json:"pinned,omitempty"`
	Queue           []string               `json:"queue,omitempty"`
	Pending         []LedgerAttentionEvent `json:"pending,omitempty"`
	Staged          *ledgerAttentionStage  `json:"staged,omitempty"`
	TopologyEpoch   uint64                 `json:"topologyEpoch,omitempty"`
	JournalBaseline []string               `json:"journalBaseline,omitempty"`
	JournalReady    bool                   `json:"journalBaselineReady,omitempty"`
}

// LedgerAttentionReport is the tick-facing result. FailureKind prevents a
// state publication refusal from being mislabeled as a fetch failure.
type LedgerAttentionReport struct {
	Outcome     string
	Tip         string
	Failure     string
	FailureKind string
	Pending     []LedgerAttentionEvent
	MovedAt     time.Time
}

var ledgerAttentionFetchBudget = 60 * time.Second
var ledgerAttentionWriter = atomicfile.WriteText

func ledgerAttentionStatePath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "ledger-attention.json")
}

func loadLedgerAttentionState(repoRoot string) (ledgerAttentionState, bool, error) {
	data, err := os.ReadFile(ledgerAttentionStatePath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return ledgerAttentionState{Schema: ledgerAttentionStateSchema}, false, nil
		}
		return ledgerAttentionState{}, false, err
	}
	var state ledgerAttentionState
	if err := json.Unmarshal(data, &state); err != nil {
		return ledgerAttentionState{}, true, fmt.Errorf("ledger-attention state is malformed: %w", err)
	}
	if state.Schema != ledgerAttentionStateSchema {
		return ledgerAttentionState{}, true, fmt.Errorf("ledger-attention state has schema %d, want %d", state.Schema, ledgerAttentionStateSchema)
	}
	seen := map[string]bool{}
	for _, event := range state.Pending {
		if event.SourceID == "" || event.Tip == "" || seen[event.SourceID] {
			return ledgerAttentionState{}, true, fmt.Errorf("ledger-attention state has an invalid pending event")
		}
		seen[event.SourceID] = true
	}
	return state, true, nil
}

func saveLedgerAttentionState(repoRoot string, state ledgerAttentionState) error {
	state.Schema = ledgerAttentionStateSchema
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := ledgerAttentionWriter(ledgerAttentionStatePath(repoRoot), string(append(data, '\n')), repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("ledger-attention state was published with durability unknown")
	}
	return nil
}

func ledgerAttentionStaleMinutes(repoRoot string) (int, error) {
	minutes, err := config.LedgerAttentionStaleMinutes(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return 0, err
	}
	maxInt := uint64(^uint(0) >> 1)
	if minutes > maxInt {
		return 0, fmt.Errorf("%s exceeds this platform's integer range", config.LedgerAttentionStaleMinutesKey)
	}
	return int(minutes), nil
}

type ledgerSnapshot struct {
	Ready  []string
	Pinned []string
	Queue  []string
}

func snapshotLedger(projection goal.Projection, machine string) ledgerSnapshot {
	verdict := goal.Next(projection, machine)
	snapshot := ledgerSnapshot{Ready: append([]string(nil), verdict.Ready...)}
	type queueRow struct {
		id, opened string
	}
	var rows []queueRow
	for id, file := range projection.Tree.Live {
		if file.State != goal.StateQueued {
			continue
		}
		rows = append(rows, queueRow{id: id, opened: file.OpenedAt})
		if file.Pinned == machine {
			snapshot.Pinned = append(snapshot.Pinned, id)
		}
	}
	sort.Strings(snapshot.Pinned)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].opened != rows[j].opened {
			return rows[i].opened < rows[j].opened
		}
		return rows[i].id < rows[j].id
	})
	for _, row := range rows {
		snapshot.Queue = append(snapshot.Queue, row.id)
	}
	return snapshot
}

func addedStrings(before, after []string) []string {
	present := make(map[string]bool, len(before))
	for _, item := range before {
		present[item] = true
	}
	var added []string
	for _, item := range after {
		if !present[item] {
			added = append(added, item)
		}
	}
	return added
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func eventSourceID(tip string, epoch uint64) string {
	if epoch == 0 {
		return tip
	}
	return tip + "-epoch-" + strconv.FormatUint(epoch, 10)
}

func confirmedRepairOperationIDs(entries []goal.Entry) []string {
	var ids []string
	for _, entry := range entries {
		if entry.Intent.Verb == "repair-accept-remote" && entry.Phase == goal.PhaseTerminal && entry.Outcome == goal.OutcomeConfirmed {
			ids = append(ids, entry.Opid)
		}
	}
	sort.Strings(ids)
	return ids
}

func buildLedgerAttentionStage(repoRoot, machine, before, after string, epoch uint64, at time.Time) (ledgerAttentionStage, error) {
	entries, err := goal.Entries(repoRoot)
	if err != nil {
		return ledgerAttentionStage{}, err
	}
	previousProjection, err := goal.ProjectAt(repoRoot, before)
	if err != nil {
		return ledgerAttentionStage{}, err
	}
	previous := snapshotLedger(previousProjection, machine)
	changes, err := goal.LedgerChanges(repoRoot, before, after)
	if err != nil {
		return ledgerAttentionStage{}, err
	}
	stage := ledgerAttentionStage{
		From: before, Tip: after, TopologyEpoch: epoch,
		RepairBaseline: confirmedRepairOperationIDs(entries), RepairBaselineReady: true,
	}
	for _, change := range changes {
		if !change.Consecutive {
			stage.TopologyEpoch++
		}
		projection, err := goal.ProjectAt(repoRoot, change.Tip)
		if err != nil {
			return ledgerAttentionStage{}, err
		}
		current := snapshotLedger(projection, machine)
		event := LedgerAttentionEvent{
			SourceID:  eventSourceID(change.Tip, stage.TopologyEpoch),
			Tip:       change.Tip,
			At:        at.UTC().Format(time.RFC3339Nano),
			Claimable: addedStrings(previous.Ready, current.Ready),
			Pins:      addedStrings(previous.Pinned, current.Pinned),
		}
		if !sameStrings(previous.Queue, current.Queue) {
			event.QueueWas = append([]string(nil), previous.Queue...)
			event.QueueNow = append([]string(nil), current.Queue...)
		}
		if len(event.Claimable) > 0 || len(event.Pins) > 0 || event.QueueWas != nil || event.QueueNow != nil {
			stage.Events = append(stage.Events, event)
		}
		previous = current
	}
	stage.Ready = previous.Ready
	stage.Pinned = previous.Pinned
	stage.Queue = previous.Queue
	return stage, nil
}

func promoteLedgerAttentionStage(state *ledgerAttentionState) {
	stage := state.Staged
	state.Pending = append(state.Pending, stage.Events...)
	state.DiffedTip = stage.Tip
	state.Ready = append([]string(nil), stage.Ready...)
	state.Pinned = append([]string(nil), stage.Pinned...)
	state.Queue = append([]string(nil), stage.Queue...)
	state.TopologyEpoch = stage.TopologyEpoch
	state.Staged = nil
}

func reportLedgerAttention(state ledgerAttentionState) LedgerAttentionReport {
	report := LedgerAttentionReport{
		Outcome: state.LastOutcome, Tip: state.RemoteTip,
		Pending: append([]LedgerAttentionEvent(nil), state.Pending...),
	}
	report.MovedAt, _ = parseLedgerAttentionTime(state.MovedAt)
	return report
}

func stateWriteFailureReport(repoRoot string, err error) LedgerAttentionReport {
	report := LedgerAttentionReport{Outcome: "failed", Failure: err.Error(), FailureKind: ledgerAttentionStateWriteFailed}
	if durable, _, loadErr := loadLedgerAttentionState(repoRoot); loadErr == nil {
		report.Pending = append([]LedgerAttentionEvent(nil), durable.Pending...)
		report.Tip = durable.RemoteTip
		report.MovedAt, _ = parseLedgerAttentionTime(durable.MovedAt)
	}
	return report
}

func failedLedgerAttention(repoRoot string, state ledgerAttentionState, now time.Time, cause error) LedgerAttentionReport {
	state.LastAttemptAt = now.UTC().Format(time.RFC3339Nano)
	state.LastOutcome = "failed"
	state.LastFailure = cause.Error()
	if state.FailingSince == "" {
		state.FailingSince = state.LastAttemptAt
	}
	if err := saveLedgerAttentionState(repoRoot, state); err != nil {
		return stateWriteFailureReport(repoRoot, err)
	}
	report := reportLedgerAttention(state)
	report.Failure = cause.Error()
	report.FailureKind = ledgerAttentionFetchFailed
	return report
}

func journalOperationIDs(entries []goal.Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Opid)
	}
	sort.Strings(ids)
	return ids
}

func clearLedgerAttentionFromJournal(repoRoot string, state *ledgerAttentionState) (bool, error) {
	if state.RemoteTip == "" || state.RemoteTip == state.ExaminedTip || !state.JournalReady {
		return false, nil
	}
	entries, err := goal.Entries(repoRoot)
	if err != nil {
		return false, err
	}
	baseline := make(map[string]bool, len(state.JournalBaseline))
	for _, id := range state.JournalBaseline {
		baseline[id] = true
	}
	for _, entry := range entries {
		if baseline[entry.Opid] || entry.FetchedOid == "" {
			continue
		}
		examined, err := goal.IsAncestor(repoRoot, state.RemoteTip, entry.FetchedOid)
		if err != nil {
			return false, err
		}
		if examined {
			state.ExaminedTip = state.RemoteTip
			state.MovedAt = ""
			return true, nil
		}
	}
	return false, nil
}

func stagedTipRetiredByRepair(repoRoot string, stage *ledgerAttentionStage) (bool, error) {
	if !stage.RepairBaselineReady {
		// A stage written before repair baselines existed cannot prove that
		// its classification still postdates the human's accepted world.
		return true, nil
	}
	entries, err := goal.Entries(repoRoot)
	if err != nil {
		return false, err
	}
	baseline := make(map[string]bool, len(stage.RepairBaseline))
	for _, id := range stage.RepairBaseline {
		baseline[id] = true
	}
	for _, entry := range entries {
		if baseline[entry.Opid] || entry.Intent.Verb != "repair-accept-remote" || entry.Phase != goal.PhaseTerminal || entry.Outcome != goal.OutcomeConfirmed {
			continue
		}
		if entry.Intent.Args["newTip"] == stage.From {
			return true, nil
		}
	}
	return false, nil
}

func recoverLedgerAttentionStage(repoRoot string, state *ledgerAttentionState, accepted string) (bool, error) {
	if state.Staged == nil {
		return false, nil
	}
	stage := state.Staged
	if accepted == stage.From {
		retired, err := stagedTipRetiredByRepair(repoRoot, stage)
		if err != nil {
			return false, err
		}
		if retired {
			state.Staged = nil
			return true, nil
		}
		if err := goal.AdvanceAccepted(repoRoot, stage.Tip); err != nil {
			return false, err
		}
		accepted = stage.Tip
	}
	held, err := goal.IsAncestor(repoRoot, stage.Tip, accepted)
	if err != nil {
		return false, err
	}
	if held {
		promoteLedgerAttentionStage(state)
		return true, nil
	}
	// A sanctioned repair chose another accepted world before this staged
	// capture landed. It was never canonical here and must surface nothing.
	state.Staged = nil
	return true, nil
}

func resetRetiredLedgerAttentionState(state *ledgerAttentionState) {
	state.RemoteTip = ""
	state.RemoteTipAt = ""
	state.ExaminedTip = ""
	state.MovedAt = ""
	state.DiffedTip = ""
	state.Ready = nil
	state.Pinned = nil
	state.Queue = nil
	state.Pending = nil
	state.Staged = nil
	state.JournalBaseline = nil
	state.JournalReady = false
}

func recordAcceptedLedgerTransition(repoRoot, machine string, state *ledgerAttentionState, accepted string, now time.Time) error {
	if state.DiffedTip == accepted {
		return nil
	}
	stage, err := buildLedgerAttentionStage(repoRoot, machine, state.DiffedTip, accepted, state.TopologyEpoch, now)
	if err != nil {
		return err
	}
	state.Staged = &stage
	promoteLedgerAttentionStage(state)
	return saveLedgerAttentionState(repoRoot, *state)
}

// RunLedgerAttention fetches and validates the shared ledger, records every
// surfaced change before the accepted world can outrun it, and never mutates
// a goal or grants a claim.
func RunLedgerAttention(repoRoot string, now time.Time) LedgerAttentionReport {
	state, _, err := loadLedgerAttentionState(repoRoot)
	if err != nil {
		return LedgerAttentionReport{Outcome: "failed", Failure: err.Error(), FailureKind: ledgerAttentionFetchFailed}
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	state.LastAttemptAt = now.UTC().Format(time.RFC3339Nano)
	if endpoint.LocalMode() {
		state.LastOutcome, state.LastFailure, state.FailingSince = "local", "", ""
		if err := saveLedgerAttentionState(repoRoot, state); err != nil {
			return stateWriteFailureReport(repoRoot, err)
		}
		return reportLedgerAttention(state)
	}

	accepted, migrated, err := goal.AcceptedLedgerTip(repoRoot)
	if err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	if !migrated {
		resetRetiredLedgerAttentionState(&state)
		state.LastOutcome, state.LastFailure, state.FailingSince = "pre-bootstrap", "", ""
		if err := saveLedgerAttentionState(repoRoot, state); err != nil {
			return stateWriteFailureReport(repoRoot, err)
		}
		return reportLedgerAttention(state)
	}
	machine, err := goal.ResolveMachine(repoRoot)
	if err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}

	if state.DiffedTip == "" {
		projection, err := goal.ProjectAt(repoRoot, accepted)
		if err != nil {
			return failedLedgerAttention(repoRoot, state, now, err)
		}
		baseline := snapshotLedger(projection, machine)
		state.DiffedTip, state.ExaminedTip = accepted, accepted
		state.Ready, state.Pinned, state.Queue = baseline.Ready, baseline.Pinned, baseline.Queue
		if err := saveLedgerAttentionState(repoRoot, state); err != nil {
			return stateWriteFailureReport(repoRoot, err)
		}
	}

	if changed, err := recoverLedgerAttentionStage(repoRoot, &state, accepted); err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	} else if changed {
		if err := saveLedgerAttentionState(repoRoot, state); err != nil {
			return stateWriteFailureReport(repoRoot, err)
		}
		accepted, _, err = goal.AcceptedLedgerTip(repoRoot)
		if err != nil {
			return failedLedgerAttention(repoRoot, state, now, err)
		}
	}
	if err := recordAcceptedLedgerTransition(repoRoot, machine, &state, accepted, now); err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	if cleared, err := clearLedgerAttentionFromJournal(repoRoot, &state); err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	} else if cleared {
		if err := saveLedgerAttentionState(repoRoot, state); err != nil {
			return stateWriteFailureReport(repoRoot, err)
		}
	}

	capture, err := goal.CaptureTipBounded(endpoint, ledgerAttentionFetchBudget)
	if err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	defer goal.CleanupRefs(endpoint, capture.OperationID)
	if err := goal.SyncModeGate(endpoint, capture.Tip); err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	accepted, acceptedExists, err := goal.AcceptedLedgerTip(repoRoot)
	if err != nil || !acceptedExists {
		if err == nil {
			err = fmt.Errorf("the accepted goal ledger disappeared during its attention pass")
		}
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	if err := recordAcceptedLedgerTransition(repoRoot, machine, &state, accepted, now); err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	if accepted != capture.Tip {
		if err := goal.AcceptanceGates(repoRoot, accepted, capture.Tip); err != nil {
			return failedLedgerAttention(repoRoot, state, now, err)
		}
	}
	if err := goal.ValidateCommit(repoRoot, capture.Tip); err != nil {
		return failedLedgerAttention(repoRoot, state, now, err)
	}
	advanced := accepted != capture.Tip
	if advanced {
		stage, err := buildLedgerAttentionStage(repoRoot, machine, state.DiffedTip, capture.Tip, state.TopologyEpoch, now)
		if err != nil {
			return failedLedgerAttention(repoRoot, state, now, err)
		}
		state.Staged = &stage
		if err := saveLedgerAttentionState(repoRoot, state); err != nil {
			return stateWriteFailureReport(repoRoot, err)
		}
		if err := goal.AdvanceAccepted(repoRoot, capture.Tip); err != nil {
			return failedLedgerAttention(repoRoot, state, now, err)
		}
		promoteLedgerAttentionStage(&state)
	}

	remoteChanged := state.RemoteTip != capture.Tip
	state.RemoteTip = capture.Tip
	if remoteChanged {
		state.RemoteTipAt = time.Now().UTC().Format(time.RFC3339Nano)
		entries, entriesErr := goal.Entries(repoRoot)
		if entriesErr == nil {
			state.JournalBaseline = journalOperationIDs(entries)
			state.JournalReady = true
		} else {
			state.JournalBaseline = nil
			state.JournalReady = false
		}
	}
	if !state.JournalReady {
		if entries, entriesErr := goal.Entries(repoRoot); entriesErr == nil {
			state.JournalBaseline = journalOperationIDs(entries)
			state.JournalReady = true
		}
	}
	if state.RemoteTip == state.ExaminedTip {
		state.MovedAt = ""
	} else if state.MovedAt == "" {
		state.MovedAt = state.RemoteTipAt
	}
	state.LastOutcome = "current"
	if advanced {
		state.LastOutcome = "advanced"
	}
	state.LastFailure, state.FailingSince = "", ""
	if err := saveLedgerAttentionState(repoRoot, state); err != nil {
		return stateWriteFailureReport(repoRoot, err)
	}
	return reportLedgerAttention(state)
}

// PersistLedgerAttentionMark removes only events whose two durable surfaces
// landed during this tick.
func PersistLedgerAttentionMark(repoRoot string, surfacedIDs []string) error {
	if len(surfacedIDs) == 0 {
		return nil
	}
	state, _, err := loadLedgerAttentionState(repoRoot)
	if err != nil {
		return err
	}
	marked := make(map[string]bool, len(surfacedIDs))
	for _, id := range surfacedIDs {
		marked[id] = true
	}
	pending := state.Pending[:0]
	for _, event := range state.Pending {
		if !marked[event.SourceID] {
			pending = append(pending, event)
		}
	}
	state.Pending = pending
	return saveLedgerAttentionState(repoRoot, state)
}

func parseLedgerAttentionTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, raw)
}

func shortLedgerTip(tip string) string {
	if len(tip) > 12 {
		return tip[:12]
	}
	return tip
}

func ledgerAge(now, since time.Time) time.Duration {
	if since.IsZero() || now.Before(since) {
		return 0
	}
	return now.Sub(since)
}

func roundedLedgerAge(age time.Duration) string {
	minutes := int(age / time.Minute)
	return fmt.Sprintf("%dm", minutes)
}

func checkLedgerAttention(repoRoot string, now time.Time) RoleVerdict {
	state, exists, err := loadLedgerAttentionState(repoRoot)
	// A coordinator-turn timestamp is intentionally not a remedy: the
	// human-reserved accepted-ref repair can rewind after remoteTip was held,
	// so hook timing cannot prove that the turn examined this stored tip.
	remedy := "run a journaling goal verb that examines the canonical tip; 'metasystem goal fetch' does not examine"
	if err != nil {
		return roleUnknown(RoleLedgerAttention, "the ledger-attention state is unreadable: "+err.Error(), remedy)
	}
	if !exists {
		return roleUnknown(RoleLedgerAttention, "no ledger-attention pass is recorded", remedy)
	}
	minutes, err := ledgerAttentionStaleMinutes(repoRoot)
	if err != nil {
		return roleUnknown(RoleLedgerAttention, "the ledger-attention threshold is invalid: "+err.Error(), remedy)
	}
	threshold := time.Duration(minutes) * time.Minute
	if state.LastOutcome == "local" {
		return roleAlive(RoleLedgerAttention, "single-machine mode has no shared ledger to examine")
	}
	if state.LastOutcome == "pre-bootstrap" {
		return roleAlive(RoleLedgerAttention, "the shared goal ledger is not bootstrapped")
	}
	movedAt, movedErr := parseLedgerAttentionTime(state.MovedAt)
	failingSince, failingErr := parseLedgerAttentionTime(state.FailingSince)
	if movedErr != nil || failingErr != nil {
		return roleUnknown(RoleLedgerAttention, "the ledger-attention clock is unreadable", remedy)
	}
	moveAge := ledgerAge(now, movedAt)
	if state.RemoteTip != "" && state.RemoteTip != state.ExaminedTip && !movedAt.IsZero() && moveAge >= threshold {
		role := roleDead(RoleLedgerAttention,
			fmt.Sprintf("the shared ledger moved to %s %s ago and is unexamined past %dm", shortLedgerTip(state.RemoteTip), roundedLedgerAge(moveAge), minutes), remedy)
		role.NoAutomaticRemedy = true
		return role
	}
	failureAge := ledgerAge(now, failingSince)
	if !failingSince.IsZero() && failureAge >= threshold {
		return roleUnknown(RoleLedgerAttention,
			fmt.Sprintf("the shared ledger has been unreachable for %s: %s", roundedLedgerAge(failureAge), state.LastFailure), remedy)
	}
	if state.RemoteTip != "" && state.RemoteTip == state.ExaminedTip {
		return roleAlive(RoleLedgerAttention, "examined at the canonical tip "+shortLedgerTip(state.RemoteTip))
	}
	if !movedAt.IsZero() {
		return roleAlive(RoleLedgerAttention,
			fmt.Sprintf("the shared ledger moved %s ago; quiet until %dm", roundedLedgerAge(moveAge), minutes))
	}
	if !failingSince.IsZero() {
		return roleAlive(RoleLedgerAttention,
			fmt.Sprintf("the last fetch failed %s ago; quiet until %dm", roundedLedgerAge(failureAge), minutes))
	}
	return roleUnknown(RoleLedgerAttention, "the ledger-attention state has no canonical tip", remedy)
}
