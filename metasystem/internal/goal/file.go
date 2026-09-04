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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
)

// GoalFile is one parsed goal file.
type GoalFile struct {
	Id       string
	State    string // queued | approved | claimed | parked | done
	Tier     uint8  // 1 | 2 | 3; zero is tolerated during the classification migration
	Risk     *RiskRecord
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
	Pinned           string
	Budget           *Budget
	BudgetExceptions uint16
	// legacyFourBudget preserves the pre-tier approval digest until the
	// classification sweep rebinds it to a tier and five-member tuple.
	legacyFourBudget bool
	// NormApproval is the published proof for an admitted over-norm tuple.
	// It names the pre-touch goal revision the human approved.
	NormApproval *GoalNormApprovalClaim
	// legacyThreeNormApproval preserves that reviewRounds was absent on disk
	// until the next write renders the inferred fourth member explicitly.
	legacyThreeNormApproval bool
	// Approved binds the human's execution decision to one exact intent and
	// complete budget tuple. It remains as audit evidence after execution.
	Approved *ApprovalRecord
	// Sliced is the irreversible pre-reservation boundary. Once present, the
	// parent can only advance through Split.
	Sliced   *SlicedRecord
	Ratified *SplitRatification
	Claimed  *ClaimRecord
	// Obligation is the human-governed recurrence bound to this goal's
	// existing budget. Its revision changes only by replacing the whole record.
	Obligation        *GovernedObligation
	ReviewObligations []ReviewObligation
	AcceptedRisks     []AcceptedRiskRecord
	// StopCapability is the narrow authority minted with one claimed
	// revision. StopFence is present only after that authority closed launch
	// admission for the revision.
	StopCapability *StopCapability
	StopFence      *StopFence
	Parked         *ParkRecord
	Legacy         []string // LegacyNotes: verbatim non-field prose from migration
	History        []HistoryLine
}

type RiskRecord struct {
	Severity     uint8
	Novelty      uint8
	Exposure     uint8
	Accumulation uint8
	Basis        string
}

func (r RiskRecord) Validate() error {
	for _, score := range []struct {
		name  string
		value uint8
	}{{"severity", r.Severity}, {"novelty", r.Novelty}, {"exposure", r.Exposure}, {"accumulation", r.Accumulation}} {
		if score.value < 1 || score.value > 3 {
			return fmt.Errorf("%s must be 1, 2, or 3", score.name)
		}
	}
	if strings.TrimSpace(r.Basis) == "" || strings.ContainsAny(r.Basis, "\r\n") {
		return fmt.Errorf("basis must be one non-empty line")
	}
	return nil
}

func (r RiskRecord) DerivedTier() uint8 {
	tier := max(r.Severity, max(r.Novelty, r.Exposure))
	if tier == 1 && r.Accumulation >= 2 {
		return 2
	}
	return tier
}

func (r RiskRecord) GateWidth() string {
	if r.Accumulation >= 2 {
		return "full"
	}
	return "area"
}

func ParseRiskRecord(value, basis string) (RiskRecord, error) {
	parts := strings.Split(value, ",")
	want := []string{"severity", "novelty", "exposure", "accumulation"}
	if len(parts) != len(want) {
		return RiskRecord{}, fmt.Errorf("--risk must be severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n>")
	}
	values := make([]uint8, len(parts))
	for i, part := range parts {
		name, raw, ok := strings.Cut(part, "=")
		n, err := strconv.ParseUint(raw, 10, 8)
		if !ok || name != want[i] || err != nil || n < 1 || n > 3 {
			return RiskRecord{}, fmt.Errorf("--risk must be severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n>, with every answer 1, 2, or 3")
		}
		values[i] = uint8(n)
	}
	risk := RiskRecord{Severity: values[0], Novelty: values[1], Exposure: values[2], Accumulation: values[3], Basis: basis}
	if err := risk.Validate(); err != nil {
		return RiskRecord{}, err
	}
	return risk, nil
}

func (r RiskRecord) scoreArgs() string {
	return fmt.Sprintf("severity=%d,novelty=%d,exposure=%d,accumulation=%d", r.Severity, r.Novelty, r.Exposure, r.Accumulation)
}

type ReviewObligation struct{ Finding, Chain, Artifact, Test, State string }
type AcceptedRiskRecord struct{ Finding, Chain, By, Opid string }

// GoalNormApprovalClaim is the durable scope-norm exception beside a budget.
type GoalNormApprovalClaim struct {
	ApprovedRef  string
	Minutes      uint64
	ReviewRounds int64
	GoalRevision uint64
}

// ApprovalRecord is the durable human admission of one exact goal payload.
type ApprovalRecord struct {
	By        string
	At        string
	Revision  uint64
	Opid      string
	Authority string // proven | relayed | channel
	Digest    string
	ReviewBy  string // relayed only
}

const (
	ApprovalAuthorityProven  = "proven"
	ApprovalAuthorityRelayed = "relayed"
	ApprovalAuthorityChannel = "channel"
)

// ApprovalHorizon is the complete observation used to decide whether a
// relayed approval still admits a new execution revision. EnrolledAt is the
// fleet's first synced terminal enrollment, not a machine-local file.
type ApprovalHorizon struct {
	Now        time.Time
	EnrolledAt time.Time
}

func renderBudgetRecord(b Budget) string {
	return fmt.Sprintf("elapsedLimit=%s attemptLimit=%d reservedJobMinutesLimit=%d activeJobLimit=%d reviewRoundLimit=%d",
		b.ElapsedLimit, b.AttemptLimit, b.ReservedJobMinutesLimit, b.ActiveJobLimit, b.ReviewRoundLimit)
}

// ApprovalDigest binds exactly the intent and tuple that the human reviewed.
func ApprovalDigest(intent string, tier uint8, budget Budget, risks ...*RiskRecord) string {
	payload := "intent=" + intent + "\n" + fmt.Sprintf("tier=%d\n", tier)
	if len(risks) > 0 && risks[0] != nil {
		risk := risks[0]
		payload += fmt.Sprintf("risk=%d,%d,%d,%d\n", risk.Severity, risk.Novelty, risk.Exposure, risk.Accumulation)
	}
	payload += "budget=" + renderBudgetRecord(budget) + "\n"
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func legacyApprovalDigest(intent string, budget Budget) string {
	record := fmt.Sprintf("elapsedLimit=%s attemptLimit=%d reservedJobMinutesLimit=%d activeJobLimit=%d",
		budget.ElapsedLimit, budget.AttemptLimit, budget.ReservedJobMinutesLimit, budget.ActiveJobLimit)
	sum := sha256.Sum256([]byte("intent=" + intent + "\n" + "budget=" + record + "\n"))
	return hex.EncodeToString(sum[:])
}

// ApprovalExpired evaluates the relayed-word expiry at one caller-supplied
// instant. Proven terminal and verified channel approvals do not expire.
func (f *GoalFile) ApprovalExpired(h ApprovalHorizon) (bool, string) {
	if f == nil || f.Approved == nil || f.Approved.Authority != ApprovalAuthorityRelayed {
		return false, ""
	}
	if !h.EnrolledAt.IsZero() {
		return true, fmt.Sprintf("the fleet's first terminal was enrolled at %s", h.EnrolledAt.UTC().Format(time.RFC3339))
	}
	now := h.Now.UTC()
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if review, err := time.Parse("2006-01-02", f.Approved.ReviewBy); err == nil && nowDate.After(review) {
		return true, fmt.Sprintf("the review date %s has passed", f.Approved.ReviewBy)
	}
	if horizon, err := time.Parse("2006-01-02", governance.TemporaryGoalAuthorityHorizon); err == nil && nowDate.After(horizon) {
		return true, fmt.Sprintf("the temporary authority horizon %s has passed", governance.TemporaryGoalAuthorityHorizon)
	}
	return false, ""
}

// SlicedRecord proves that a claimed revision crossed the slicing boundary
// before dispatch published any reservation for that revision.
type SlicedRecord struct {
	Machine  string
	Lineage  string
	Revision uint64
	At       string
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
	// AccountingRevision is the first goal revision whose dispatch spend
	// belongs to the current human-approved budget. Legacy records default it
	// to Revision when parsed.
	AccountingRevision uint64
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
//     [authorityOutcome=<...> authorityReviewBy=<date>
//     authorityRuling=<id> temporaryHumanWord=<quoted words>]
//     [reason=<rest of line>]
type HistoryLine struct {
	At                 string
	Opid               string
	Verb               string
	Actor              string // machine+lineage or human:<name>
	Targets            []string
	Displaced          string
	Ack                bool
	Keep               int // -1 when absent; prune's root-record line only
	AuthorityOutcome   string
	AuthorityReviewBy  string
	AuthorityRuling    string
	TemporaryHumanWord string
	ChannelProvider    string
	ChannelUser        string
	ChannelRef         string
	ChannelContext     string
	ChannelStep        int64
	ApprovedRef        string
	Reason             string
}

func (h *HistoryLine) recordTemporaryRelay(reviewBy, ruling, word string) {
	h.AuthorityOutcome = AuthorityOutcomeTemporaryHumanWord
	h.AuthorityReviewBy = reviewBy
	h.AuthorityRuling = ruling
	h.TemporaryHumanWord = word
}

func firstRecordedRelayedActIn(historyLines []HistoryLine, goalID, verb, ruling string) (HistoryLine, bool) {
	for _, history := range historyLines {
		if history.Verb == verb && history.AuthorityOutcome == AuthorityOutcomeTemporaryHumanWord &&
			history.AuthorityRuling == ruling && (goalID == "" || contains(history.Targets, goalID)) {
			return history, true
		}
	}
	return HistoryLine{}, false
}

// firstRecordedRelayedAct finds the first landed use under one ruling. A new
// ruling starts a new bounded authority window; historical rulings remain
// readable facts but do not consume the renewed grant. Root history retains
// markers for pruned incarnations of the same goal identifier.
func firstRecordedRelayedAct(root *RootRecord, f *GoalFile, verb, ruling string) (HistoryLine, bool) {
	if root != nil {
		if history, ok := firstRecordedRelayedActIn(root.History, f.Id, verb, ruling); ok {
			return history, true
		}
	}
	return firstRecordedRelayedActIn(f.History, "", verb, ruling)
}

func repeatedRelayedActError(root *RootRecord, f *GoalFile, verb, ruling string) error {
	first, ok := firstRecordedRelayedAct(root, f, verb, ruling)
	if !ok {
		return nil
	}
	return fmt.Errorf("goal %s already used relayed %s authority on %s with recorded word %q; a further %s needs freshly observed enrolled-terminal authority",
		f.Id, verb, first.At, first.TemporaryHumanWord, verb)
}

func recordedRelayedAct(history HistoryLine) bool {
	return history.AuthorityOutcome == AuthorityOutcomeTemporaryHumanWord &&
		(history.Verb == "resume" || history.Verb == "set-obligation" || history.Verb == "approve" ||
			history.Verb == "unapprove" || history.Verb == "set-budget")
}

// States, closed.
const (
	StateQueued   = "queued"
	StateApproved = "approved"
	StateClaimed  = "claimed"
	StateParked   = "parked"
	StateDone     = "done"
)

func validState(s string) bool {
	switch s {
	case StateQueued, StateApproved, StateClaimed, StateParked, StateDone:
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
			parseFileField(f, strings.TrimPrefix(line, "- "), seen, func(format string, args ...any) { addProblem("line %d: "+format, append([]any{i + 1}, args...)...) })
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
		addProblem("state %q is not one of queued|approved|claimed|parked|done", f.State)
	}
	if f.Revision == 0 {
		addProblem("missing or zero Revision")
	}
	if f.Tier > 3 {
		addProblem("Tier %d is not 1, 2, or 3", f.Tier)
	}
	if f.Risk != nil {
		if err := f.Risk.Validate(); err != nil {
			addProblem("Risk: %v", err)
		}
	}
	inferLegacyReviewRounds(f)
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
	if f.NormApproval != nil {
		if f.NormApproval.ApprovedRef == "" || f.NormApproval.Minutes == 0 || f.NormApproval.ReviewRounds < 0 || f.NormApproval.GoalRevision == 0 {
			addProblem("NormApproval is incomplete")
		}
	}
	if f.Approved != nil {
		if err := f.ValidateApprovalRecord(); err != nil {
			addProblem("Approved: %v", err)
		}
	}
	if f.State == StateApproved && f.Approved == nil {
		addProblem("approved without an Approved record")
	}
	if f.State == StateQueued && f.Approved != nil {
		addProblem("Approved record on a queued goal")
	}
	if f.Sliced != nil {
		if f.Sliced.Machine == "" || f.Sliced.Lineage == "" || f.Sliced.Revision == 0 || !validStamp(f.Sliced.At) {
			addProblem("Sliced is incomplete")
		}
	}
	if f.Ratified != nil {
		if err := f.Ratified.Validate(); err != nil {
			addProblem("Ratified: %v", err)
		}
	}
	if f.Obligation != nil {
		riskRaise := f.Claimed != nil && f.Claimed.Revision > 0 && f.Claimed.Revision <= uint64(len(f.History)) && misclassificationRaises(f.History[f.Claimed.Revision-1].Reason)
		if err := validateGovernedObligation(f.Obligation, f.Revision, f.Claimed, f.Budget, riskRaise); err != nil {
			addProblem("%v", err)
		}
	}
	for _, obligation := range f.ReviewObligations {
		if !bareReviewID(obligation.Finding) || !bareReviewID(obligation.Chain) || obligation.Artifact == "" || obligation.Test == "" || (obligation.State != "open" && obligation.State != "discharged") {
			addProblem("ReviewObligation is incomplete or malformed")
		}
	}
	for _, risk := range f.AcceptedRisks {
		if !bareReviewID(risk.Finding) || !bareReviewID(risk.Chain) || !bareReviewID(risk.By) || !bareReviewID(risk.Opid) {
			addProblem("AcceptedRisk is incomplete or malformed")
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
		} else if (fence.Revision != f.StopCapability.Revision || fence.CapabilityGeneration != f.StopCapability.Generation) &&
			!f.riskRaisePreservesStopFence(fence) {
			addProblem("StopFence contradicts the stop capability")
		} else if fence.Epoch != f.StopCapability.FenceEpoch {
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

// ValidateApprovalRecord proves the record names one approval-bearing event
// and still describes the exact intent and budget in the file.
func (f *GoalFile) ValidateApprovalRecord() error {
	if f == nil || f.Approved == nil {
		return nil
	}
	a := f.Approved
	if !strings.HasPrefix(a.By, "human:") || strings.TrimSpace(strings.TrimPrefix(a.By, "human:")) == "" {
		return fmt.Errorf("by=%q does not name a human", a.By)
	}
	if !validStamp(a.At) || a.Revision == 0 || a.Revision > f.Revision || a.Revision > uint64(len(f.History)) ||
		a.Opid == "" || !hexDigest(a.Digest) {
		return fmt.Errorf("record has incomplete or out-of-range event coordinates")
	}
	event := f.History[a.Revision-1]
	raiseOpid, isRaise := strings.CutPrefix(a.Authority, "raise=")
	ordinary := event.Actor == a.By && (event.Verb == "approve" || event.Verb == "resume" || event.Verb == "set-budget")
	raise := isRaise && raiseOpid == event.Opid && event.Verb == "edit" && misclassificationRaises(event.Reason)
	if event.At != a.At || event.Opid != a.Opid || (!ordinary && !raise) {
		return fmt.Errorf("record does not bind its approval-bearing History event")
	}
	switch a.Authority {
	case ApprovalAuthorityProven:
		if a.ReviewBy != "" || event.AuthorityOutcome != "" {
			return fmt.Errorf("proven authority cannot carry relayed review facts")
		}
	case ApprovalAuthorityRelayed:
		if _, err := time.Parse("2006-01-02", a.ReviewBy); err != nil ||
			event.AuthorityOutcome != AuthorityOutcomeTemporaryHumanWord || event.AuthorityReviewBy != a.ReviewBy {
			return fmt.Errorf("relayed authority does not match its History review facts")
		}
	case ApprovalAuthorityChannel:
		if a.ReviewBy != "" || event.AuthorityOutcome != AuthorityOutcomeVerifiedChannelAnswer || event.ChannelContext == "" {
			return fmt.Errorf("channel authority does not match its verified answer facts")
		}
	default:
		if raise && a.ReviewBy == "" {
			break
		}
		return fmt.Errorf("authority %q is not proven|relayed|channel", a.Authority)
	}
	if f.Budget == nil {
		return fmt.Errorf("record requires a complete Budget")
	}
	want := ApprovalDigest(f.Intent, f.Tier, *f.Budget, f.Risk)
	// A tierless record is necessarily from before TierLaw. Its next ordinary
	// rewrite may expand the stored four-member tuple to five members before
	// classify-sweep can rebind the approval. Continue accepting the legacy
	// digest until the sweep supplies the tier and new digest together.
	if f.Tier == 0 {
		want = legacyApprovalDigest(f.Intent, *f.Budget)
	}
	if a.Digest != want {
		return fmt.Errorf("digest does not match the approved intent and budget")
	}
	return nil
}

func misclassificationRaises(reason string) bool {
	if first, _, combined := strings.Cut(reason, "; TierOverride: "); combined {
		reason = first
	}
	fields := strings.Fields(strings.TrimPrefix(reason, "Misclassified: "))
	if !strings.HasPrefix(reason, "Misclassified: ") || len(fields) != 3 || !strings.HasPrefix(fields[0], "from=") || !strings.HasPrefix(fields[1], "to=") || !strings.HasPrefix(fields[2], "evidence=") {
		return false
	}
	from, fromErr := strconv.ParseUint(strings.TrimPrefix(fields[0], "from="), 10, 8)
	to, toErr := strconv.ParseUint(strings.TrimPrefix(fields[1], "to="), 10, 8)
	return fromErr == nil && toErr == nil && to > from && strings.TrimPrefix(fields[2], "evidence=") != ""
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
		if !misclassificationRaises(event.Reason) {
			return fmt.Errorf("claimed at=%s contradicts History revision=%d at=%s", f.Claimed.At, revision, event.At)
		}
		foundOrigin := false
		for _, prior := range f.History[:revision-1] {
			if prior.At == f.Claimed.At {
				foundOrigin = true
				break
			}
		}
		if !foundOrigin {
			return fmt.Errorf("claimed at=%s has no pre-raise History event", f.Claimed.At)
		}
	}
	return nil
}

// riskRaisePreservesStopFence admits exactly the revision-only claim rebind:
// a rigor raise never clears or rewrites a fence that already stopped launch.
func (f *GoalFile) riskRaisePreservesStopFence(fence *StopFence) bool {
	if f.StopCapability == nil || fence.Epoch != f.StopCapability.FenceEpoch ||
		fence.Revision != fence.CapabilityGeneration || fence.Revision >= f.StopCapability.Revision ||
		f.StopCapability.Revision != f.StopCapability.Generation || f.StopCapability.Revision > uint64(len(f.History)) {
		return false
	}
	return misclassificationRaises(f.History[f.StopCapability.Revision-1].Reason)
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
	if seen[key] && key != "ReviewObligation" && key != "AcceptedRisk" {
		addProblem("duplicate field %q — the last write would silently win", key)
		return
	}
	seen[key] = true
	value = strings.TrimSpace(value)
	switch key {
	case "Risk":
		without, basis, present, err := cutQuotedRecordField(value, "basis")
		if err != nil || !present {
			addProblem("Risk: basis must be one quoted string")
			return
		}
		rec, err := parseKVRecord(without, []string{"severity", "novelty", "exposure", "accumulation"}, nil, "")
		if err != nil {
			addProblem("Risk: %v", err)
			return
		}
		risk, err := ParseRiskRecord(fmt.Sprintf("severity=%s,novelty=%s,exposure=%s,accumulation=%s", rec["severity"], rec["novelty"], rec["exposure"], rec["accumulation"]), basis)
		if err != nil {
			addProblem("Risk: %v", err)
			return
		}
		f.Risk = &risk
	case "BudgetExceptions":
		n, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			addProblem("BudgetExceptions %q is not an unsigned 16-bit integer", value)
			return
		}
		f.BudgetExceptions = uint16(n)
	case "ReviewObligation":
		without, artifact, present, err := cutQuotedRecordField(value, "artifact")
		if err != nil || !present {
			addProblem("ReviewObligation: artifact= %v", err)
			return
		}
		without, test, present, err := cutQuotedRecordField(without, "test")
		if err != nil || !present {
			addProblem("ReviewObligation: test= %v", err)
			return
		}
		rec, err := parseKVRecord(without, []string{"finding", "chain", "state"}, nil, "")
		if err != nil {
			addProblem("ReviewObligation: %v", err)
			return
		}
		f.ReviewObligations = append(f.ReviewObligations, ReviewObligation{Finding: rec["finding"], Chain: rec["chain"], Artifact: artifact, Test: test, State: rec["state"]})
	case "AcceptedRisk":
		rec, err := parseKVRecord(value, []string{"finding", "chain", "by", "opid"}, nil, "")
		if err != nil {
			addProblem("AcceptedRisk: %v", err)
			return
		}
		f.AcceptedRisks = append(f.AcceptedRisks, AcceptedRiskRecord{Finding: rec["finding"], Chain: rec["chain"], By: rec["by"], Opid: rec["opid"]})
	case "State":
		f.State = value
	case "Tier":
		n, err := strconv.ParseUint(value, 10, 8)
		if err != nil || n < 1 || n > 3 {
			addProblem("Tier %q is not 1, 2, or 3", value)
			return
		}
		f.Tier = uint8(n)
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
		budget, legacy, err := parseBudgetRecord(value)
		if err != nil {
			addProblem("Budget: %v", err)
			return
		}
		f.Budget = &budget
		f.legacyFourBudget = legacy
	case "NormApproval":
		rec, err := parseKVRecord(value, []string{"approvedRef", "minutes", "goalRevision"}, []string{"reviewRounds"}, "")
		if err != nil {
			addProblem("NormApproval: %v", err)
			return
		}
		minutes, minutesErr := strconv.ParseUint(rec["minutes"], 10, 64)
		revision, revisionErr := strconv.ParseUint(rec["goalRevision"], 10, 64)
		rounds := int64(0)
		roundsErr := error(nil)
		legacy := rec["reviewRounds"] == ""
		if !legacy {
			rounds, roundsErr = strconv.ParseInt(rec["reviewRounds"], 10, 64)
		}
		if minutesErr != nil || minutes == 0 || roundsErr != nil || rounds < 0 || revisionErr != nil || revision == 0 {
			addProblem("NormApproval has invalid numeric coordinates")
			return
		}
		f.NormApproval = &GoalNormApprovalClaim{ApprovedRef: rec["approvedRef"], Minutes: minutes, ReviewRounds: rounds, GoalRevision: revision}
		f.legacyThreeNormApproval = legacy
	case "Approved":
		rec, err := parseKVRecord(value, []string{"by", "at", "revision", "opid", "authority", "digest"}, []string{"reviewBy"}, "")
		if err != nil {
			addProblem("Approved: %v", err)
			return
		}
		revision, revisionErr := strconv.ParseUint(rec["revision"], 10, 64)
		if revisionErr != nil || revision == 0 {
			addProblem("Approved has invalid revision")
			return
		}
		f.Approved = &ApprovalRecord{By: rec["by"], At: rec["at"], Revision: revision, Opid: rec["opid"],
			Authority: rec["authority"], Digest: rec["digest"], ReviewBy: rec["reviewBy"]}
	case "Sliced":
		rec, err := parseKVRecord(value, []string{"machine", "lineage", "revision", "at"}, nil, "")
		if err != nil {
			addProblem("Sliced: %v", err)
			return
		}
		revision, revisionErr := strconv.ParseUint(rec["revision"], 10, 64)
		if revisionErr != nil || revision == 0 {
			addProblem("Sliced has invalid revision")
			return
		}
		f.Sliced = &SlicedRecord{Machine: rec["machine"], Lineage: rec["lineage"], Revision: revision, At: rec["at"]}
	case "Ratified":
		rec, err := parseKVRecord(value, []string{"tier", "draftSha256"}, []string{"by", "mainId", "claimEpoch"}, "")
		if err != nil {
			addProblem("Ratified: %v", err)
			return
		}
		epoch := int64(0)
		if rec["claimEpoch"] != "" {
			epoch, err = strconv.ParseInt(rec["claimEpoch"], 10, 64)
			if err != nil {
				addProblem("Ratified claimEpoch is not an integer")
				return
			}
		}
		f.Ratified = &SplitRatification{Tier: rec["tier"], By: rec["by"], MainID: rec["mainId"], ClaimEpoch: epoch, DraftSHA256: rec["draftSha256"]}
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
		rec, err := parseKVRecord(value, []string{"machine", "lineage", "at"}, []string{"revision", "accountingRevision", "appetite"}, "")
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
		accountingRevision := revision
		if rec["accountingRevision"] != "" {
			accountingRevision, err = strconv.ParseUint(rec["accountingRevision"], 10, 64)
			if err != nil || accountingRevision == 0 {
				addProblem("Claimed accountingRevision=%q is not a positive integer", rec["accountingRevision"])
				return
			}
		}
		f.Claimed = &ClaimRecord{Machine: rec["machine"], Lineage: rec["lineage"], At: rec["at"], Revision: revision, AccountingRevision: accountingRevision}
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

func tierBoxReviewRounds(tier uint8) int64 {
	switch tier {
	case 1:
		return 0
	case 2:
		return 2
	default:
		// Tierless records use the tier-three box during migration.
		return 3
	}
}

func inferLegacyReviewRounds(f *GoalFile) {
	boxRounds := tierBoxReviewRounds(f.Tier)
	switch {
	case f.Budget != nil && f.NormApproval != nil && f.legacyFourBudget && f.legacyThreeNormApproval:
		f.Budget.ReviewRoundLimit = boxRounds
		f.NormApproval.ReviewRounds = boxRounds
	case f.Budget != nil && f.legacyFourBudget:
		if f.NormApproval != nil {
			f.Budget.ReviewRoundLimit = f.NormApproval.ReviewRounds
		} else {
			f.Budget.ReviewRoundLimit = boxRounds
		}
	case f.NormApproval != nil && f.legacyThreeNormApproval:
		if f.Budget != nil {
			f.NormApproval.ReviewRounds = f.Budget.ReviewRoundLimit
		} else {
			f.NormApproval.ReviewRounds = boxRounds
		}
	}
}

func bareReviewID(value string) bool { return value != "" && !strings.ContainsAny(value, " \t\r\n") }

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

// cutQuotedRecordField removes one quoted key=value field without changing
// its decoded bytes or any surrounding free-text tail.
func cutQuotedRecordField(s, key string) (without, value string, present bool, err error) {
	marker := " " + key + "="
	tokenStart := strings.Index(s, marker)
	valueStart := 0
	if tokenStart >= 0 {
		valueStart = tokenStart + len(marker)
	} else if strings.HasPrefix(s, key+"=") {
		tokenStart = 0
		valueStart = len(key) + 1
	} else {
		return s, "", false, nil
	}
	quoted, quoteErr := strconv.QuotedPrefix(s[valueStart:])
	if quoteErr != nil {
		return "", "", true, fmt.Errorf("%s= must be one quoted string: %v", key, quoteErr)
	}
	decoded, quoteErr := strconv.Unquote(quoted)
	if quoteErr != nil {
		return "", "", true, fmt.Errorf("%s= must be one quoted string: %v", key, quoteErr)
	}
	tail := s[valueStart+len(quoted):]
	if tail != "" && tail[0] != ' ' && tail[0] != '\t' {
		return "", "", true, fmt.Errorf("%s= has an invalid trailing token", key)
	}
	without = strings.TrimSpace(s[:tokenStart] + tail)
	return without, decoded, true, nil
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
	if f.Risk != nil {
		fmt.Fprintf(&b, "- Risk: severity=%d novelty=%d exposure=%d accumulation=%d basis=%s\n", f.Risk.Severity, f.Risk.Novelty, f.Risk.Exposure, f.Risk.Accumulation, strconv.Quote(f.Risk.Basis))
	}
	if f.Tier != 0 {
		fmt.Fprintf(&b, "- Tier: %d\n", f.Tier)
	}
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
		fmt.Fprintf(&b, "- Budget: %s\n", renderBudgetRecord(*f.Budget))
	}
	fmt.Fprintf(&b, "- BudgetExceptions: %d\n", f.BudgetExceptions)
	if f.NormApproval != nil {
		fmt.Fprintf(&b, "- NormApproval: approvedRef=%s minutes=%d reviewRounds=%d goalRevision=%d\n",
			f.NormApproval.ApprovedRef, f.NormApproval.Minutes, f.NormApproval.ReviewRounds, f.NormApproval.GoalRevision)
	}
	if f.Approved != nil {
		a := f.Approved
		fmt.Fprintf(&b, "- Approved: by=%s at=%s revision=%d opid=%s authority=%s digest=%s",
			a.By, a.At, a.Revision, a.Opid, a.Authority, a.Digest)
		if a.ReviewBy != "" {
			fmt.Fprintf(&b, " reviewBy=%s", a.ReviewBy)
		}
		b.WriteByte('\n')
	}
	if f.Sliced != nil {
		fmt.Fprintf(&b, "- Sliced: machine=%s lineage=%s revision=%d at=%s\n",
			f.Sliced.Machine, f.Sliced.Lineage, f.Sliced.Revision, f.Sliced.At)
	}
	if f.Ratified != nil {
		fmt.Fprintf(&b, "- Ratified: tier=%s", f.Ratified.Tier)
		if f.Ratified.Tier == RatifierHuman {
			fmt.Fprintf(&b, " by=%s", f.Ratified.By)
		} else {
			fmt.Fprintf(&b, " mainId=%s claimEpoch=%d", f.Ratified.MainID, f.Ratified.ClaimEpoch)
		}
		fmt.Fprintf(&b, " draftSha256=%s\n", f.Ratified.DraftSHA256)
	}
	if f.Obligation != nil {
		o := f.Obligation
		empty := func(value string) string {
			if value == "" {
				return "-"
			}
			return value
		}
		fmt.Fprintf(&b, "- Obligation: revision=%d budgetRevision=%d state=%s owner=%s authorizedBy=%s authorizedAt=%s authorityOperation=%s reviewPolicy=%s reviewOutcome=%s effects=%s authorizedEffects=%s",
			o.Revision, o.BudgetRevision, o.State, o.Owner, empty(o.AuthorizedBy), empty(o.AuthorizedAt), empty(o.AuthorityOperation),
			empty(o.ReviewPolicy), empty(o.ReviewOutcome), renderEffects(o.Effects), renderEffects(o.AuthorizedEffects))
		if (o.AuthorityOutcome != "" || o.AuthorityReviewBy != "") && o.AuthorityRuling == "" && o.TemporaryHumanWord == "" {
			fmt.Fprintf(&b, " authorityOutcome=%s authorityReviewBy=%s", empty(o.AuthorityOutcome), empty(o.AuthorityReviewBy))
		} else if o.AuthorityOutcome != "" || o.AuthorityReviewBy != "" || o.AuthorityRuling != "" || o.TemporaryHumanWord != "" {
			fmt.Fprintf(&b, " authorityOutcome=%s authorityReviewBy=%s authorityRuling=%s temporaryHumanWord=%s",
				empty(o.AuthorityOutcome), empty(o.AuthorityReviewBy), empty(o.AuthorityRuling), strconv.Quote(o.TemporaryHumanWord))
		}
		b.WriteByte('\n')
		fmt.Fprintf(&b, "- ObligationAssumptions: recurrence=%s platform=%s toolchainIdentity=%s surfaceDigest=%s maxActiveJobs=%d timingEnvelopeSeconds=%d observationSource=%s\n",
			o.Assumptions.Recurrence, o.Assumptions.Platform, o.Assumptions.ToolchainIdentity, o.Assumptions.SurfaceDigest,
			o.Assumptions.MaxActiveJobs, o.Assumptions.TimingEnvelopeSeconds, o.Assumptions.ObservationSource)
		fmt.Fprintf(&b, "- ObligationTriggers: valueJudgment=%s reversibility=%s severeHarm=%s unfamiliarApproach=%s testDiscrimination=%s correlatedAssumptionRisk=%s authorityScopeChange=%s destructiveReach=%s\n",
			o.Triggers.ValueJudgment, o.Triggers.Reversibility, o.Triggers.SevereHarm, o.Triggers.UnfamiliarApproach,
			o.Triggers.TestDiscrimination, o.Triggers.CorrelatedAssumptionRisk, o.Triggers.AuthorityScopeChange, o.Triggers.DestructiveReach)
	}
	for _, obligation := range f.ReviewObligations {
		fmt.Fprintf(&b, "- ReviewObligation: finding=%s chain=%s artifact=%s test=%s state=%s\n", obligation.Finding, obligation.Chain, strconv.Quote(obligation.Artifact), strconv.Quote(obligation.Test), obligation.State)
	}
	for _, risk := range f.AcceptedRisks {
		fmt.Fprintf(&b, "- AcceptedRisk: finding=%s chain=%s by=%s opid=%s\n", risk.Finding, risk.Chain, risk.By, risk.Opid)
	}
	if f.Claimed != nil {
		fmt.Fprintf(&b, "- Claimed: machine=%s lineage=%s at=%s", f.Claimed.Machine, f.Claimed.Lineage, f.Claimed.At)
		if f.Claimed.Revision > 0 {
			accountingRevision := f.Claimed.AccountingRevision
			if accountingRevision == 0 {
				accountingRevision = f.Claimed.Revision
			}
			fmt.Fprintf(&b, " revision=%d accountingRevision=%d", f.Claimed.Revision, accountingRevision)
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

	reasonIndex := strings.Index(rest, " reason=")
	authorityIndex := strings.Index(rest, " authorityOutcome=")
	wordIndex := strings.Index(rest, " temporaryHumanWord=")
	if authorityIndex >= 0 && wordIndex >= 0 && (reasonIndex < 0 || authorityIndex < reasonIndex && wordIndex < reasonIndex) {
		withoutWord, humanWord, _, err := cutQuotedRecordField(rest, "temporaryHumanWord")
		if err != nil {
			return h, err
		}
		rest, h.TemporaryHumanWord = withoutWord, humanWord
	}

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
		case strings.HasPrefix(tok, "authorityOutcome="):
			if err := dup("authorityOutcome"); err != nil {
				return h, err
			}
			h.AuthorityOutcome = strings.TrimPrefix(tok, "authorityOutcome=")
		case strings.HasPrefix(tok, "authorityReviewBy="):
			if err := dup("authorityReviewBy"); err != nil {
				return h, err
			}
			h.AuthorityReviewBy = strings.TrimPrefix(tok, "authorityReviewBy=")
		case strings.HasPrefix(tok, "authorityRuling="):
			if err := dup("authorityRuling"); err != nil {
				return h, err
			}
			h.AuthorityRuling = strings.TrimPrefix(tok, "authorityRuling=")
		case strings.HasPrefix(tok, "channelProvider="):
			if err := dup("channelProvider"); err != nil {
				return h, err
			}
			h.ChannelProvider = strings.TrimPrefix(tok, "channelProvider=")
		case strings.HasPrefix(tok, "channelUser="):
			if err := dup("channelUser"); err != nil {
				return h, err
			}
			h.ChannelUser = strings.TrimPrefix(tok, "channelUser=")
		case strings.HasPrefix(tok, "channelRef="):
			if err := dup("channelRef"); err != nil {
				return h, err
			}
			h.ChannelRef = strings.TrimPrefix(tok, "channelRef=")
		case strings.HasPrefix(tok, "channelContext="):
			if err := dup("channelContext"); err != nil {
				return h, err
			}
			h.ChannelContext = strings.TrimPrefix(tok, "channelContext=")
		case strings.HasPrefix(tok, "channelStep="):
			if err := dup("channelStep"); err != nil {
				return h, err
			}
			step, err := strconv.ParseInt(strings.TrimPrefix(tok, "channelStep="), 10, 64)
			if err != nil || step < 1 {
				return h, fmt.Errorf("channelStep= wants a positive decimal")
			}
			h.ChannelStep = step
		case strings.HasPrefix(tok, "approvedRef="):
			if err := dup("approvedRef"); err != nil {
				return h, err
			}
			h.ApprovedRef = strings.TrimPrefix(tok, "approvedRef=")
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
	channelAuthority := h.AuthorityOutcome == AuthorityOutcomeAuthenticatedChannelWord || h.AuthorityOutcome == AuthorityOutcomeVerifiedChannelAnswer
	if !channelAuthority {
		if err := validateRecordedTemporaryAuthority(h.AuthorityOutcome, h.AuthorityReviewBy, h.AuthorityRuling, h.TemporaryHumanWord); err != nil {
			return h, fmt.Errorf("recorded temporary authority: %v", err)
		}
	}
	channelCount := 0
	for _, v := range []string{h.ChannelProvider, h.ChannelUser, h.ChannelRef} {
		if v != "" {
			channelCount++
		}
	}
	if h.ChannelStep > 0 {
		channelCount++
	}
	if h.ChannelContext != "" {
		channelCount++
	}
	if channelAuthority {
		wantCount := 4
		if h.AuthorityOutcome == AuthorityOutcomeVerifiedChannelAnswer {
			wantCount = 5
		}
		if channelCount != wantCount || h.AuthorityReviewBy != "" || h.AuthorityRuling != "" || h.TemporaryHumanWord != "" {
			return h, fmt.Errorf("channel authority requires provider, user, reference, step, and the context required by its proof class")
		}
	} else if channelCount != 0 {
		return h, fmt.Errorf("channel proof keys require a channel authority outcome")
	}
	if h.ApprovedRef != "" && h.Verb != "resume" && h.Verb != "set-obligation" {
		return h, fmt.Errorf("approvedRef= is only valid on resume and set-obligation history")
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
	if h.AuthorityOutcome == AuthorityOutcomeAuthenticatedChannelWord || h.AuthorityOutcome == AuthorityOutcomeVerifiedChannelAnswer {
		fmt.Fprintf(&b, " authorityOutcome=%s", h.AuthorityOutcome)
	} else if (h.AuthorityOutcome != "" || h.AuthorityReviewBy != "") && h.AuthorityRuling == "" && h.TemporaryHumanWord == "" {
		fmt.Fprintf(&b, " authorityOutcome=%s authorityReviewBy=%s", h.AuthorityOutcome, h.AuthorityReviewBy)
	} else if h.AuthorityOutcome != "" || h.AuthorityReviewBy != "" || h.AuthorityRuling != "" || h.TemporaryHumanWord != "" {
		fmt.Fprintf(&b, " authorityOutcome=%s authorityReviewBy=%s authorityRuling=%s temporaryHumanWord=%s",
			h.AuthorityOutcome, h.AuthorityReviewBy, h.AuthorityRuling, strconv.Quote(h.TemporaryHumanWord))
	}
	if h.AuthorityOutcome == AuthorityOutcomeAuthenticatedChannelWord || h.AuthorityOutcome == AuthorityOutcomeVerifiedChannelAnswer {
		fmt.Fprintf(&b, " channelProvider=%s channelUser=%s channelRef=%s channelStep=%d", h.ChannelProvider, h.ChannelUser, h.ChannelRef, h.ChannelStep)
		if h.ChannelContext != "" {
			b.WriteString(" channelContext=" + h.ChannelContext)
		}
	}
	if h.ApprovedRef != "" {
		b.WriteString(" approvedRef=" + h.ApprovedRef)
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
