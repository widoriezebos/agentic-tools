package goal

// The per-goal file format of the multi-machine ledger
// (plans/goals/<id>.md).
// One goal per file; every verb write appends a History line carrying
// its operation id and bumps Revision by one; a trailing Integrity
// line pins the bytes above it. Parsing is strict: the validator and
// recovery parse exactly this grammar and nothing looser.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GoalFile is one parsed goal file.
type GoalFile struct {
	Id       string
	State    string // queued | claimed | parked | done
	Intent   string
	Origin   string
	NextStep string
	Conclude string // done only
	OpenedAt string // ISO 8601, written once at open
	Revision uint64 // 1 at creation, +1 per verb write
	Blocked  []string
	Labels   []string
	Arc      string
	// Pinned names the ONE machine that may claim this goal — set when
	// the work needs a setup, network, or resource only that machine
	// has. Empty means any machine may claim.
	Pinned  string
	Budget  *Budget
	Claimed *ClaimRecord
	// Obligation is the human-governed recurrence bound to this goal's
	// existing budget. Its revision changes only by replacing the whole record.
	Obligation *GovernedObligation
	// StopCapability is the narrow authority minted with one claimed
	// revision. StopFence is present only after that authority closed launch
	// admission for the revision.
	StopCapability *StopCapability
	StopFence      *StopFence
	Parked         *ParkRecord
	Legacy         []string // LegacyNotes: verbatim non-field prose from migration
	History        []HistoryLine
}

// ClaimRecord is the ownership record of a claimed goal.
type ClaimRecord struct {
	Machine string
	Lineage string
	At      string
	// Revision is the exact goal revision whose work this claim began.
	// Zero is tolerated only for legacy records written before revision
	// binding existed.
	Revision uint64
}

// StopCapability binds breach-stop authority to one exact local claim. The
// goal id is the containing record's id; every other coordinate is explicit.
type StopCapability struct {
	Generation uint64
	Revision   uint64
	Machine    string
	ClaimEpoch int64
	FenceEpoch uint64
}

// StopFence is the absorbing launch refusal for one stopped revision. A human
// resume removes it only after the named batch is complete.
type StopFence struct {
	StopID               string
	Revision             uint64
	Epoch                uint64
	CapabilityGeneration uint64
	ClosedAt             string
	Reason               string
}

const (
	StopReasonElapsedLimit     = "ELAPSED_LIMIT"
	StopReasonCorruptOverLimit = "CORRUPT_OVER_LIMIT"
)

// ParkRecord is the pause record of a parked goal.
type ParkRecord struct {
	By        string
	At        string
	Because   string
	Displaced string // machine+lineage@claimedAt of a displaced claimant, or empty
}

// HistoryLine is one entry of the append-only History block, exactly
// the design's grammar:
//
//   - <iso8601> <opid> <verb> actor=<...> [targets=<ids>]
//     [displaced=<machine>+<lineage>@<at>] [ack] [keep=<n>]
//     [reason=<rest of line>]
type HistoryLine struct {
	At        string
	Opid      string
	Verb      string
	Actor     string // machine+lineage or human:<name>
	Targets   []string
	Displaced string
	Ack       bool
	Keep      int // -1 when absent; prune's root-record line only
	Reason    string
}

// States, closed.
const (
	StateQueued  = "queued"
	StateClaimed = "claimed"
	StateParked  = "parked"
	StateDone    = "done"
)

func validState(s string) bool {
	switch s {
	case StateQueued, StateClaimed, StateParked, StateDone:
		return true
	}
	return false
}

// ParseFile parses one goal file. Every problem is returned; a file
// with problems must be refused by name, never partially trusted.
func ParseFile(data []byte) (*GoalFile, []Problem) {
	var problems []Problem
	addProblem := func(format string, args ...any) {
		problems = append(problems, Problem(fmt.Sprintf(format, args...)))
	}

	body, integrity, ok := splitIntegrity(data)
	if !ok {
		addProblem("missing Integrity line")
	} else if got := IntegrityDigest(body); got != integrity {
		addProblem("Integrity mismatch: recorded %s, computed %s", integrity, got)
	}

	f := &GoalFile{}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	section := ""
	seen := map[string]bool{}
	for i, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		switch {
		case strings.HasPrefix(line, "# "):
			if f.Id != "" {
				addProblem("second heading %q — one goal per file", line)
				continue
			}
			f.Id = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if !validId(f.Id) {
				addProblem("invalid goal id %q", f.Id)
			}
		case line == "History:":
			section = "history"
		case line == "LegacyNotes:":
			section = "legacy"
		case section == "history" && strings.HasPrefix(line, "- "):
			h, err := ParseHistoryLine(line)
			if err != nil {
				addProblem("History line %d: %v", i+1, err)
				continue
			}
			f.History = append(f.History, h)
		case section == "legacy" && line != "":
			f.Legacy = append(f.Legacy, strings.TrimPrefix(line, "  "))
		case strings.HasPrefix(line, "- "):
			parseFileField(f, strings.TrimPrefix(line, "- "), seen, addProblem)
		case strings.TrimSpace(line) == "":
			// blank lines end no section; History and LegacyNotes run to
			// the next section heading or end of body
		default:
			if section == "" {
				addProblem("unparseable line %d: %q", i+1, line)
			}
		}
	}

	if f.Id == "" {
		addProblem("missing goal heading")
	}
	if !validState(f.State) {
		addProblem("state %q is not one of queued|claimed|parked|done", f.State)
	}
	if f.Revision == 0 {
		addProblem("missing or zero Revision")
	}
	if len(f.History) == 0 {
		addProblem("empty History — every goal file carries its opid lines")
	}
	if f.Intent == "" {
		addProblem("missing Intent — a goal without its why is unreadable")
	}
	if f.Origin != OriginHuman && f.Origin != OriginMain {
		addProblem("Origin %q is not human|main — provenance is immutable and closed", f.Origin)
	}
	if f.OpenedAt == "" {
		addProblem("missing OpenedAt")
	} else if !validStamp(f.OpenedAt) {
		addProblem("OpenedAt %q is not an RFC3339 timestamp", f.OpenedAt)
	}
	if f.State == StateClaimed && f.Claimed == nil {
		addProblem("claimed without a Claimed record")
	}
	if f.State != StateClaimed && f.Claimed != nil {
		addProblem("Claimed record on a %s goal", f.State)
	}
	if f.State != StateClaimed && (f.StopCapability != nil || f.StopFence != nil) {
		addProblem("stop authority on a %s goal", f.State)
	}
	if f.Pinned != "" && !validPinnedNickname(f.Pinned) {
		addProblem("Pinned %q is not a machine nickname (one word, no whitespace of any kind)", f.Pinned)
	}
	for _, label := range f.Labels {
		if !labelRe.MatchString(label) {
			addProblem("label %q must match %s", label, labelRe.String())
		}
	}
	if f.Claimed != nil && !validStamp(f.Claimed.At) {
		addProblem("Claimed at=%q is not an RFC3339 timestamp", f.Claimed.At)
	}
	if err := f.ValidateClaimRevision(); err != nil {
		addProblem("BUDGET_UNKNOWN %v", err)
	}
	if f.Budget != nil {
		if err := f.Budget.Validate(); err != nil {
			addProblem("Budget: %v", err)
		}
	}
	if f.Obligation != nil {
		if err := validateGovernedObligation(f.Obligation, f.Revision, f.Claimed, f.Budget); err != nil {
			addProblem("%v", err)
		}
	}
	if f.State == StateClaimed && f.Claimed != nil && f.Claimed.Revision == 0 && f.Budget != nil {
		addProblem("Budget: a structured tuple requires a revision-bound claim record")
	}
	if f.StopCapability != nil {
		capability := f.StopCapability
		if capability.Generation == 0 || capability.Revision == 0 || capability.Machine == "" || capability.ClaimEpoch < 1 {
			addProblem("StopCapability is incomplete")
		} else if f.Claimed == nil || capability.Revision != f.Claimed.Revision || capability.Machine != f.Claimed.Machine {
			addProblem("StopCapability contradicts the claim binding")
		}
	}
	if f.StopFence != nil {
		fence := f.StopFence
		if f.StopCapability == nil {
			addProblem("StopFence has no stop capability")
		} else if !safeStopID(fence.StopID) || fence.Revision == 0 || fence.Epoch == 0 || fence.CapabilityGeneration == 0 || !validStamp(fence.ClosedAt) {
			addProblem("StopFence is incomplete")
		} else if fence.Revision != f.StopCapability.Revision || fence.Epoch != f.StopCapability.FenceEpoch ||
			fence.CapabilityGeneration != f.StopCapability.Generation {
			addProblem("StopFence contradicts the stop capability")
		}
		if fence.Reason != StopReasonElapsedLimit && fence.Reason != StopReasonCorruptOverLimit {
			addProblem("StopFence reason %q is not a live-stop reason", fence.Reason)
		}
	}
	if f.State == StateParked && f.Parked == nil {
		addProblem("parked without a Parked record")
	}
	if f.State != StateParked && f.Parked != nil {
		addProblem("Parked record on a %s goal", f.State)
	}
	if f.Parked != nil && !validStamp(f.Parked.At) {
		addProblem("Parked at=%q is not an RFC3339 timestamp", f.Parked.At)
	}
	if f.Parked != nil && strings.TrimSpace(f.Parked.Because) == "" {
		addProblem("Parked without its because — a pause without a why is a stall in disguise")
	}
	if f.State == StateDone && f.Conclude == "" {
		addProblem("done without Concluded")
	}
	return f, problems
}

// ValidateClaimRevision proves that a revision-bound claim names one event in
// this goal's history and uses that event's timestamp as its elapsed origin.
// A revisionless claim has no binding to validate; structured admission
// refuses it rather than guessing which history event began the work.
func (f *GoalFile) ValidateClaimRevision() error {
	if f == nil || f.Claimed == nil || f.Claimed.Revision == 0 {
		return nil
	}
	revision := f.Claimed.Revision
	if revision > f.Revision {
		return fmt.Errorf("claimed revision=%d does not exist in goal Revision=%d", revision, f.Revision)
	}
	if revision > uint64(len(f.History)) {
		return fmt.Errorf("claimed revision=%d has no History event; the goal records %d event(s)", revision, len(f.History))
	}
	event := f.History[revision-1]
	if event.At != f.Claimed.At {
		return fmt.Errorf("claimed at=%s contradicts History revision=%d at=%s", f.Claimed.At, revision, event.At)
	}
	return nil
}

// validStamp is the one timestamp form the grammar admits.
func validStamp(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func parseFileField(f *GoalFile, field string, seen map[string]bool, addProblem func(string, ...any)) {
	key, value, found := strings.Cut(field, ":")
	if !found {
		addProblem("field without colon: %q", field)
		return
	}
	if seen[key] {
		addProblem("duplicate field %q — the last write would silently win", key)
		return
	}
	seen[key] = true
	value = strings.TrimSpace(value)
	switch key {
	case "State":
		f.State = value
	case "Intent":
		f.Intent = value
	case "Origin":
		f.Origin = value
	case "Next step":
		f.NextStep = value
	case "Concluded":
		f.Conclude = value
	case "OpenedAt":
		f.OpenedAt = value
	case "Revision":
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			addProblem("Revision %q is not an unsigned integer", value)
			return
		}
		f.Revision = n
	case "BlockedBy":
		if value != "" && value != "-" {
			for _, id := range strings.Split(value, ",") {
				f.Blocked = append(f.Blocked, strings.TrimSpace(id))
			}
		}
	case "Labels":
		if value != "" && value != "-" {
			for _, label := range strings.Split(value, ",") {
				f.Labels = append(f.Labels, strings.TrimSpace(label))
			}
		}
	case "Arc":
		f.Arc = value
	case "Pinned":
		f.Pinned = value
	case "Budget":
		budget, err := parseBudgetRecord(value)
		if err != nil {
			addProblem("Budget: %v", err)
			return
		}
		f.Budget = &budget
	case "Obligation":
		obligation, err := parseObligationRecord(value)
		if err != nil {
			addProblem("Obligation: %v", err)
			return
		}
		f.Obligation = obligation
	case "ObligationAssumptions":
		if f.Obligation == nil {
			addProblem("ObligationAssumptions appears before Obligation")
			return
		}
		assumptions, err := parseObligationAssumptions(value)
		if err != nil {
			addProblem("ObligationAssumptions: %v", err)
			return
		}
		f.Obligation.Assumptions = assumptions
	case "ObligationTriggers":
		if f.Obligation == nil {
			addProblem("ObligationTriggers appears before Obligation")
			return
		}
		triggers, err := parseObligationTriggers(value)
		if err != nil {
			addProblem("ObligationTriggers: %v", err)
			return
		}
		f.Obligation.Triggers = triggers
	case "Claimed":
		// appetite= has no budget authority. Discarding it keeps the claim
		// readable so admission can name the record whose structured tuple is
		// missing; the value never enters GoalFile.
		rec, err := parseKVRecord(value, []string{"machine", "lineage", "at"}, []string{"revision", "appetite"}, "")
		if err != nil {
			addProblem("Claimed: %v", err)
			return
		}
		var revision uint64
		if rec["revision"] != "" {
			revision, err = strconv.ParseUint(rec["revision"], 10, 64)
			if err != nil || revision == 0 {
				addProblem("Claimed revision=%q is not a positive integer", rec["revision"])
				return
			}
		}
		f.Claimed = &ClaimRecord{Machine: rec["machine"], Lineage: rec["lineage"], At: rec["at"], Revision: revision}
	case "StopCapability":
		rec, err := parseKVRecord(value, []string{"generation", "revision", "machine", "claimEpoch", "fenceEpoch"}, nil, "")
		if err != nil {
			addProblem("StopCapability: %v", err)
			return
		}
		generation, generationErr := strconv.ParseUint(rec["generation"], 10, 64)
		revision, revisionErr := strconv.ParseUint(rec["revision"], 10, 64)
		claimEpoch, claimEpochErr := strconv.ParseInt(rec["claimEpoch"], 10, 64)
		fenceEpoch, fenceEpochErr := strconv.ParseUint(rec["fenceEpoch"], 10, 64)
		if generationErr != nil || generation == 0 || revisionErr != nil || revision == 0 ||
			claimEpochErr != nil || claimEpoch < 1 || fenceEpochErr != nil {
			addProblem("StopCapability has invalid numeric coordinates")
			return
		}
		f.StopCapability = &StopCapability{
			Generation: generation, Revision: revision, Machine: rec["machine"],
			ClaimEpoch: claimEpoch, FenceEpoch: fenceEpoch,
		}
	case "StopFence":
		rec, err := parseKVRecord(value,
			[]string{"stopId", "revision", "epoch", "capabilityGeneration", "closedAt", "reason"}, nil, "")
		if err != nil {
			addProblem("StopFence: %v", err)
			return
		}
		revision, revisionErr := strconv.ParseUint(rec["revision"], 10, 64)
		epoch, epochErr := strconv.ParseUint(rec["epoch"], 10, 64)
		generation, generationErr := strconv.ParseUint(rec["capabilityGeneration"], 10, 64)
		if revisionErr != nil || revision == 0 || epochErr != nil || epoch == 0 || generationErr != nil || generation == 0 {
			addProblem("StopFence has invalid numeric coordinates")
			return
		}
		f.StopFence = &StopFence{
			StopID: rec["stopId"], Revision: revision, Epoch: epoch,
			CapabilityGeneration: generation, ClosedAt: rec["closedAt"], Reason: rec["reason"],
		}
	case "Parked":
		rec, err := parseKVRecord(value, []string{"by", "at"}, []string{"displaced"}, "because")
		if err != nil {
			addProblem("Parked: %v", err)
			return
		}
		because, displaced := splitParkTail(value)
		f.Parked = &ParkRecord{By: rec["by"], At: rec["at"], Because: because, Displaced: displaced}
	default:
		addProblem("unknown field %q", key)
	}
}

// parseKVRecord reads space-separated key=value pairs against a
// CLOSED key set: required keys must appear, optional keys may, and
// anything else refuses — a key the grammar does not know is a tree
// nobody reviewed. Free text (because=) is handled by the caller.
func parseKVRecord(s string, required, optional []string, freeTail string) (map[string]string, error) {
	allowed := map[string]bool{}
	for _, k := range required {
		allowed[k] = true
	}
	for _, k := range optional {
		allowed[k] = true
	}
	out := map[string]string{}
	for _, tok := range strings.Fields(s) {
		k, v, found := strings.Cut(tok, "=")
		if !found {
			return nil, fmt.Errorf("stray token %q — the record grammar is key=value", tok)
		}
		if k == freeTail && freeTail != "" {
			break // the lawful free-text tail consumes the rest; the caller extracts it
		}
		if !allowed[k] {
			return nil, fmt.Errorf("unknown key %q — the record grammar is closed", k)
		}
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("duplicate key %q", k)
		}
		out[k] = v
	}
	for _, k := range required {
		if out[k] == "" {
			return nil, fmt.Errorf("missing %s=", k)
		}
	}
	return out, nil
}

// splitParkTail extracts because= (rest of value, after any displaced=).
func splitParkTail(s string) (because, displaced string) {
	// TOKEN boundaries, never substrings: a because= or displaced=
	// buried inside another value must not fabricate a reason or
	// displacement state. because= takes the raw remainder
	// BYTE-EXACT — the reason is free text, and a render/parse
	// round trip must never rewrite its interior whitespace.
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		token := s[start:i]
		if strings.HasPrefix(token, "because=") {
			return s[start+len("because="):], displaced
		}
		if strings.HasPrefix(token, "displaced=") {
			displaced = strings.TrimPrefix(token, "displaced=")
		}
	}
	return "", displaced
}

// RenderFile writes the canonical bytes of a goal file, Integrity
// line included. Rendering then parsing yields the same file.
func RenderFile(f *GoalFile) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", f.Id)
	fmt.Fprintf(&b, "- State: %s\n", f.State)
	fmt.Fprintf(&b, "- Intent: %s\n", f.Intent)
	if f.Origin != "" {
		fmt.Fprintf(&b, "- Origin: %s\n", f.Origin)
	}
	if f.NextStep != "" {
		fmt.Fprintf(&b, "- Next step: %s\n", f.NextStep)
	}
	if f.Conclude != "" {
		fmt.Fprintf(&b, "- Concluded: %s\n", f.Conclude)
	}
	fmt.Fprintf(&b, "- OpenedAt: %s\n", f.OpenedAt)
	fmt.Fprintf(&b, "- Revision: %d\n", f.Revision)
	if len(f.Blocked) > 0 {
		fmt.Fprintf(&b, "- BlockedBy: %s\n", strings.Join(f.Blocked, ", "))
	}
	if len(f.Labels) > 0 {
		fmt.Fprintf(&b, "- Labels: %s\n", strings.Join(f.Labels, ", "))
	}
	if f.Arc != "" {
		fmt.Fprintf(&b, "- Arc: %s\n", f.Arc)
	}
	if f.Pinned != "" {
		fmt.Fprintf(&b, "- Pinned: %s\n", f.Pinned)
	}
	if f.Budget != nil {
		fmt.Fprintf(&b, "- Budget: elapsedLimit=%s attemptLimit=%d reservedJobMinutesLimit=%d activeJobLimit=%d\n",
			f.Budget.ElapsedLimit, f.Budget.AttemptLimit, f.Budget.ReservedJobMinutesLimit, f.Budget.ActiveJobLimit)
	}
	if f.Obligation != nil {
		o := f.Obligation
		empty := func(value string) string {
			if value == "" {
				return "-"
			}
			return value
		}
		fmt.Fprintf(&b, "- Obligation: revision=%d budgetRevision=%d state=%s owner=%s authorizedBy=%s authorizedAt=%s authorityOperation=%s reviewPolicy=%s reviewOutcome=%s effects=%s authorizedEffects=%s\n",
			o.Revision, o.BudgetRevision, o.State, o.Owner, empty(o.AuthorizedBy), empty(o.AuthorizedAt), empty(o.AuthorityOperation),
			empty(o.ReviewPolicy), empty(o.ReviewOutcome), renderEffects(o.Effects), renderEffects(o.AuthorizedEffects))
		fmt.Fprintf(&b, "- ObligationAssumptions: recurrence=%s platform=%s toolchainIdentity=%s surfaceDigest=%s maxActiveJobs=%d timingEnvelopeSeconds=%d observationSource=%s\n",
			o.Assumptions.Recurrence, o.Assumptions.Platform, o.Assumptions.ToolchainIdentity, o.Assumptions.SurfaceDigest,
			o.Assumptions.MaxActiveJobs, o.Assumptions.TimingEnvelopeSeconds, o.Assumptions.ObservationSource)
		fmt.Fprintf(&b, "- ObligationTriggers: valueJudgment=%s reversibility=%s severeHarm=%s unfamiliarApproach=%s testDiscrimination=%s correlatedAssumptionRisk=%s authorityScopeChange=%s destructiveReach=%s\n",
			o.Triggers.ValueJudgment, o.Triggers.Reversibility, o.Triggers.SevereHarm, o.Triggers.UnfamiliarApproach,
			o.Triggers.TestDiscrimination, o.Triggers.CorrelatedAssumptionRisk, o.Triggers.AuthorityScopeChange, o.Triggers.DestructiveReach)
	}
	if f.Claimed != nil {
		fmt.Fprintf(&b, "- Claimed: machine=%s lineage=%s at=%s", f.Claimed.Machine, f.Claimed.Lineage, f.Claimed.At)
		if f.Claimed.Revision > 0 {
			fmt.Fprintf(&b, " revision=%d", f.Claimed.Revision)
		}
		b.WriteByte('\n')
	}
	if f.StopCapability != nil {
		fmt.Fprintf(&b, "- StopCapability: generation=%d revision=%d machine=%s claimEpoch=%d fenceEpoch=%d\n",
			f.StopCapability.Generation, f.StopCapability.Revision, f.StopCapability.Machine,
			f.StopCapability.ClaimEpoch, f.StopCapability.FenceEpoch)
	}
	if f.StopFence != nil {
		fmt.Fprintf(&b, "- StopFence: stopId=%s revision=%d epoch=%d capabilityGeneration=%d closedAt=%s reason=%s\n",
			f.StopFence.StopID, f.StopFence.Revision, f.StopFence.Epoch,
			f.StopFence.CapabilityGeneration, f.StopFence.ClosedAt, f.StopFence.Reason)
	}
	if f.Parked != nil {
		line := fmt.Sprintf("- Parked: by=%s at=%s", f.Parked.By, f.Parked.At)
		if f.Parked.Displaced != "" {
			line += " displaced=" + f.Parked.Displaced
		}
		line += " because=" + f.Parked.Because
		b.WriteString(line + "\n")
	}
	if len(f.Legacy) > 0 {
		b.WriteString("\nLegacyNotes:\n")
		for _, l := range f.Legacy {
			// The indent makes carried prose OPAQUE to the parser: a
			// legacy line shaped like a heading or a section marker
			// must never be re-read as structure.
			b.WriteString("  " + l + "\n")
		}
	}
	b.WriteString("\nHistory:\n")
	for _, h := range f.History {
		b.WriteString(RenderHistoryLine(h) + "\n")
	}
	body := []byte(b.String())
	return append(body, []byte(fmt.Sprintf("Integrity: sha256=%s\n", IntegrityDigest(body)))...)
}

// IntegrityDigest is sha256 over the file's bytes above the Integrity
// line, LF-normalized — the entire canonical byte domain.
func IntegrityDigest(body []byte) string {
	normalized := strings.ReplaceAll(string(body), "\r\n", "\n")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// splitIntegrity separates the body from the trailing Integrity line.
func splitIntegrity(data []byte) (body []byte, digest string, ok bool) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	const prefix = "Integrity: sha256="
	i := strings.LastIndex(s, prefix)
	if i < 0 || (i > 0 && s[i-1] != '\n') {
		return data, "", false
	}
	digest = strings.TrimSpace(s[i+len(prefix):])
	return []byte(s[:i]), digest, true
}

// ParseHistoryLine parses one History entry against the exact grammar.
func ParseHistoryLine(line string) (HistoryLine, error) {
	h := HistoryLine{Keep: -1}
	rest := strings.TrimPrefix(line, "- ")

	// reason= is always last and consumes the remainder.
	if i := strings.Index(rest, " reason="); i >= 0 {
		h.Reason = rest[i+len(" reason="):]
		rest = rest[:i]
	}

	fields := strings.Fields(rest)
	if len(fields) < 4 {
		return h, fmt.Errorf("wants <at> <opid> <verb> actor=..., got %q", line)
	}
	h.At, h.Opid, h.Verb = fields[0], fields[1], fields[2]
	if !validOpidShape(h.Opid) {
		return h, fmt.Errorf("opid %q is not <ulid>-<machine>-<hash8>", h.Opid)
	}
	seenKeys := map[string]bool{}
	dup := func(key string) error {
		if seenKeys[key] {
			return fmt.Errorf("duplicate %s= — the last write would silently win", key)
		}
		seenKeys[key] = true
		return nil
	}
	for _, tok := range fields[3:] {
		switch {
		case strings.HasPrefix(tok, "actor="):
			if err := dup("actor"); err != nil {
				return h, err
			}
			h.Actor = strings.TrimPrefix(tok, "actor=")
		case strings.HasPrefix(tok, "targets="):
			if err := dup("targets"); err != nil {
				return h, err
			}
			h.Targets = strings.Split(strings.TrimPrefix(tok, "targets="), ",")
		case strings.HasPrefix(tok, "displaced="):
			if err := dup("displaced"); err != nil {
				return h, err
			}
			h.Displaced = strings.TrimPrefix(tok, "displaced=")
		case tok == "ack":
			if err := dup("ack"); err != nil {
				return h, err
			}
			h.Ack = true
		case strings.HasPrefix(tok, "keep="):
			if err := dup("keep"); err != nil {
				return h, err
			}
			n, err := strconv.Atoi(strings.TrimPrefix(tok, "keep="))
			if err != nil {
				return h, fmt.Errorf("keep= wants an integer: %q", tok)
			}
			h.Keep = n
		default:
			return h, fmt.Errorf("unknown History key %q", tok)
		}
	}
	if h.Actor == "" {
		return h, fmt.Errorf("missing actor= in %q", line)
	}
	if !validStamp(h.At) {
		return h, fmt.Errorf("timestamp %q is not RFC3339", h.At)
	}
	return h, nil
}

// RenderHistoryLine writes one History entry in the exact grammar.
func RenderHistoryLine(h HistoryLine) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- %s %s %s actor=%s", h.At, h.Opid, h.Verb, h.Actor)
	if len(h.Targets) > 0 {
		b.WriteString(" targets=" + strings.Join(h.Targets, ","))
	}
	if h.Displaced != "" {
		b.WriteString(" displaced=" + h.Displaced)
	}
	if h.Ack {
		b.WriteString(" ack")
	}
	if h.Keep >= 0 {
		fmt.Fprintf(&b, " keep=%d", h.Keep)
	}
	if h.Reason != "" {
		b.WriteString(" reason=" + h.Reason)
	}
	return b.String()
}

// validOpidShape binds the opid grammar from the right (machine
// names may carry dashes): a trailing 8-hex lineage hash, a
// non-empty machine, and a leading 26-character Crockford ulid.
func validOpidShape(opid string) bool {
	lastDash := strings.LastIndex(opid, "-")
	if lastDash < 0 || len(opid)-lastDash-1 != 8 {
		return false
	}
	for _, r := range opid[lastDash+1:] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	if len(opid) < 28 || opid[26] != '-' {
		return false
	}
	for _, r := range opid[:26] {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			return false
		}
	}
	return lastDash > 27 // a non-empty machine between the ulid and the hash
}

// Opid builds an operation id: <ulid>-<machine>-<lineage-hash8>.
// The opid attributes execution; the History actor attributes
// authority.
func Opid(ulid, machine, lineage string) string {
	sum := sha256.Sum256([]byte(lineage))
	return fmt.Sprintf("%s-%s-%s", ulid, machine, hex.EncodeToString(sum[:4]))
}

// HistoryIsPrefix reports whether a's History is a strict prefix of
// b's — the apparent-rewind DIAGNOSTIC (never an acceptance gate).
func HistoryIsPrefix(a, b []HistoryLine) bool {
	if len(a) >= len(b) {
		return false
	}
	for i := range a {
		if RenderHistoryLine(a[i]) != RenderHistoryLine(b[i]) {
			return false
		}
	}
	return true
}

// SortIds sorts goal ids in the canonical target order.
func SortIds(ids []string) {
	sort.Strings(ids)
}
