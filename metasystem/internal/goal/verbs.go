package goal

// The verb surface: open, claim, release,
// done — the single-file rows of the transition table. Every verb
// is one transaction: its mutation callback re-reads the fetched
// tip and re-decides on the current world (a rebuilt tip classifies
// idempotent success, loss, or refusal by name), and its write set
// touches NO path outside the table's row. Common effects, applied
// once here: every touched file's Revision increments by exactly
// one and its History gains exactly the opid's line with the verb
// and actor. Arc cascades land with the arcs layer.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type AnswerProof struct {
	Provider, User, Ref string
	Step                int64
}

func ResumeApprovalToken(goalID string, b Budget) string {
	args := budgetIntentArgs(b)
	return fmt.Sprintf("goal=%s resume elapsed=%s attempts=%s minutes=%s active=%s", goalID, args["elapsedLimit"], args["attemptLimit"], args["reservedJobMinutesLimit"], args["activeJobLimit"])
}

func SetObligationApprovalToken(goalID string, state ObligationState, owner string) string {
	return fmt.Sprintf("goal=%s set-obligation state=%s owner=%s", goalID, state, owner)
}

func Asked(r VerbRequest, id, qid, kind, firstFact string) (PublishResult, error) {
	marker := "ASKED " + qid + " (" + kind + "): " + firstFact
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Intent: Intent{Verb: "ask", Targets: []string{id}}, Message: "goal ask " + id, Mutate: func(tip string) ([]Change, error) {
		t, err := loadTree(r.Endpoint.Root, tip)
		if err != nil {
			return nil, err
		}
		f := t.Live[id]
		if f == nil {
			return nil, fmt.Errorf("goal %s is not live", id)
		}
		if opidLanded(f, r) {
			return nil, AlreadyApplied{}
		}
		if strings.TrimSpace(f.NextStep) == "" {
			f.NextStep = marker
		} else if !strings.Contains(f.NextStep, marker) {
			f.NextStep += "; " + marker
		}
		touch(f, r, "ask", []string{id})
		return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
	}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }})
}

func AuthenticatedChannelApproval(repoRoot, goalID, opid, strictToken string, now time.Time) (governance.RecordedChannelAuthority, error) {
	ep, err := ResolveEndpoint(repoRoot)
	if err != nil {
		return governance.RecordedChannelAuthority{}, err
	}
	p, err := Project(ep, false, now)
	if err != nil {
		return governance.RecordedChannelAuthority{}, err
	}
	f := p.Tree.Live[goalID]
	if f == nil {
		return governance.RecordedChannelAuthority{}, fmt.Errorf("goal %s is not live", goalID)
	}
	for _, h := range f.History {
		if h.ApprovedRef == opid && (h.Verb == "resume" || h.Verb == "set-obligation" || h.Verb == "carrying" || h.Verb == "carried") {
			return governance.RecordedChannelAuthority{}, fmt.Errorf("operation %s was already consumed by %s on goal %s", opid, h.Verb, goalID)
		}
	}
	for _, h := range f.History {
		if h.Opid == opid && h.Verb == "answer" && h.AuthorityOutcome == AuthorityOutcomeAuthenticatedChannelWord {
			if !containsContiguousFields(h.Reason, strictToken) {
				return governance.RecordedChannelAuthority{}, fmt.Errorf("operation %s does not carry the required token %q", opid, strictToken)
			}
			return governance.RecordedChannelAuthority{Outcome: h.AuthorityOutcome, Provider: h.ChannelProvider, UserID: h.ChannelUser, MessageRef: h.ChannelRef, Step: h.ChannelStep}, nil
		}
	}
	return governance.RecordedChannelAuthority{}, fmt.Errorf("operation %s is not an authenticated channel word on goal %s", opid, goalID)
}

func containsContiguousFields(text, token string) bool {
	fields, wanted := strings.Fields(text), strings.Fields(token)
	if len(wanted) == 0 {
		return false
	}
	matches := 0
	for i := 0; i+len(wanted) <= len(fields); i++ {
		if strings.Join(fields[i:i+len(wanted)], " ") == strings.Join(wanted, " ") {
			matches++
		}
	}
	return matches == 1
}

// Answer records an authenticated channel reply and its next-step marker in
// the same goal transaction. Replaying its operation identifier is a no-op.
func Answer(r VerbRequest, id, qid, text, wants string, proof AnswerProof) (PublishResult, error) {
	if proof.Provider == "" || proof.User == "" || proof.Ref == "" || proof.Step < 1 || strings.TrimSpace(text) == "" {
		return PublishResult{}, fmt.Errorf("answer requires complete authenticated channel proof and text")
	}
	return Publish(r.Endpoint, answerRequest(r, id, qid, text, wants, proof))
}

func answerRequest(r VerbRequest, id, qid, text, wants string, proof AnswerProof) PublishRequest {
	reason := text
	if wants != "" && !strings.Contains(reason, wants) {
		reason += " " + wants
	}
	args := map[string]string{"question": qid, "text": text, "wants": wants, "provider": proof.Provider, "user": proof.User, "ref": proof.Ref, "step": strconv.FormatInt(proof.Step, 10)}
	return PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Intent: Intent{Verb: "answer", Targets: []string{id}, Args: args}, Message: "goal answer " + id, Mutate: func(tip string) ([]Change, error) {
		t, err := loadTree(r.Endpoint.Root, tip)
		if err != nil {
			return nil, err
		}
		f := t.Live[id]
		if f == nil {
			return nil, fmt.Errorf("goal %s is not live", id)
		}
		if opidLanded(f, r) {
			return nil, AlreadyApplied{}
		}
		change, err := answerGoalChange(t, id, qid, text, reason, r.opid(), r.stamp(), proof)
		if err != nil {
			return nil, err
		}
		return []Change{change}, nil
	}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }}
}

func answerGoalChange(t *TreeGoals, id, qid, text, reason, opid, at string, proof AnswerProof) (Change, error) {
	f := t.Live[id]
	if f == nil {
		return Change{}, fmt.Errorf("goal %s is not live", id)
	}
	marker := "ANSWERED " + qid + ": " + text
	if !strings.Contains(f.NextStep, marker) {
		if strings.TrimSpace(f.NextStep) == "" {
			f.NextStep = marker
		} else {
			f.NextStep += "; " + marker
		}
	}
	f.Revision++
	f.History = append(f.History, HistoryLine{
		At: at, Opid: opid, Verb: "answer", Actor: "human:wido", Targets: []string{id}, Keep: -1,
		AuthorityOutcome: AuthorityOutcomeAuthenticatedChannelWord,
		ChannelProvider:  proof.Provider,
		ChannelUser:      proof.User,
		ChannelRef:       proof.Ref,
		ChannelStep:      proof.Step,
		Reason:           reason,
	})
	return Change{Path: livePath(id), Content: RenderFile(f)}, nil
}

// Actor is the executing identity: the machine+lineage pair always;
// Human names the directing human when one did (authority vs
// execution — the opid attributes execution, the History actor
// attributes authority).
type Actor struct {
	Machine string
	Lineage string
	Human   string // empty for agent-directed verbs
}

func (a Actor) historyActor() string {
	if a.Human != "" {
		return "human:" + a.Human
	}
	return a.Machine + "+" + a.Lineage
}

// VerbRequest carries what every verb needs.
type VerbRequest struct {
	Endpoint Endpoint
	Actor    Actor
	Ulid     string // caller-minted; the opid derives from it
	Now      time.Time
	// ApprovedRef names a recorded human exception for verbs that admit an
	// over-norm goal budget. Other verbs reject the flag at the command edge.
	ApprovedRef string
	// ClaimEpoch is the authenticated checkout lease generation. Only
	// transitions that create a claimed revision consume it.
	ClaimEpoch int64
	// CallerClass is the one command-edge classification used by every
	// human-word mutation. It prevents later verbs from reclassifying a
	// different process view after the actor was assembled.
	CallerClass string
}

func (r VerbRequest) opid() string {
	return Opid(r.Ulid, r.Actor.Machine, r.Actor.Lineage)
}

func (r VerbRequest) stamp() string {
	return r.Now.UTC().Format(time.RFC3339)
}

// loadTree parses one tip's whole ledger subtree; a tree that does
// not parse refuses the verb by name (the verb never writes onto a
// world it cannot read).
// TreeReadError preserves the named parse problems for read-side consumers
// that must distinguish a malformed budget from a mechanically unreadable
// ledger.
type TreeReadError struct {
	Tip      string
	Problems []Problem
	Files    map[string][]byte
}

func (e *TreeReadError) Error() string {
	lines := make([]string, len(e.Problems))
	for i, problem := range e.Problems {
		lines[i] = string(problem)
	}
	return fmt.Sprintf("the ledger tree at %s does not parse:\n%s", short(e.Tip), strings.Join(lines, "\n"))
}

func loadTree(root, tip string) (*TreeGoals, error) {
	files, err := ReadCommitGoals(root, tip)
	if err != nil {
		return nil, err
	}
	t, problems := ParseTreeFiles(files)
	if len(problems) > 0 {
		return nil, &TreeReadError{Tip: tip, Problems: problems, Files: files}
	}
	return t, nil
}

// touch applies the common effects to one goal file: Revision +1,
// History + the opid's line.
func touch(f *GoalFile, r VerbRequest, verb string, targets []string) {
	f.Revision++
	f.History = append(f.History, HistoryLine{
		At: r.stamp(), Opid: r.opid(), Verb: verb,
		Actor: r.Actor.historyActor(), Targets: targets, Keep: -1,
	})
}

func newClaimRecord(machine, lineage, at string, revision uint64) *ClaimRecord {
	return &ClaimRecord{Machine: machine, Lineage: lineage, At: at, Revision: revision, AccountingRevision: revision}
}

func bindClaim(f *GoalFile, machine, lineage, at string, revision uint64, claimEpoch int64) error {
	if claimEpoch < 1 {
		return fmt.Errorf("claim requires the authenticated lease holder's positive claim epoch")
	}
	f.Claimed = newClaimRecord(machine, lineage, at, revision)
	f.Claimed.EpisodeAt = at
	f.Claimed.EpisodeRevision = revision
	f.StopCapability = &StopCapability{
		Generation: revision, Revision: revision, Machine: machine, ClaimEpoch: claimEpoch,
	}
	f.StopFence = nil
	// A fresh claim or budget revision cannot inherit authority from an older
	// obligation. The human creates a new immutable binding explicitly.
	f.Obligation = nil
	return nil
}

// rebindClaimKeepEpisode advances the budget and accounting revision without
// changing when the current ownership episode began or which discharge moved
// its clock.
func rebindClaimKeepEpisode(f *GoalFile, at string, revision uint64, claimEpoch int64) error {
	if f.Claimed == nil {
		return fmt.Errorf("goal %s has no claim to rebind", f.Id)
	}
	if f.Claimed.Revision == 0 {
		if f.Claimed.EpisodeAt != "" || f.Claimed.EpisodeRevision != 0 || f.Claimed.EpisodeObligationRevision != 0 {
			return f.ValidateClaimRevision()
		}
		return bindClaim(f, f.Claimed.Machine, f.Claimed.Lineage, at, revision, claimEpoch)
	}
	machine, lineage := f.Claimed.Machine, f.Claimed.Lineage
	episodeAt, episodeRevision := f.Claimed.EpisodeAt, f.Claimed.EpisodeRevision
	if episodeRevision == 0 {
		episodeAt = f.Claimed.At
		episodeRevision = f.Claimed.AccountingRevision
		if episodeRevision == 0 {
			episodeRevision = f.Claimed.Revision
		}
		candidate := *f.Claimed
		candidate.EpisodeAt = episodeAt
		candidate.EpisodeRevision = episodeRevision
		checked := *f
		checked.Claimed = &candidate
		if err := checked.ValidateClaimRevision(); err != nil {
			return err
		}
	} else if err := f.ValidateClaimRevision(); err != nil {
		return err
	}
	episodeObligationRevision := f.Claimed.EpisodeObligationRevision
	if f.Obligation != nil {
		episodeObligationRevision = f.Obligation.Revision
	}
	if err := bindClaim(f, machine, lineage, at, revision, claimEpoch); err != nil {
		return err
	}
	f.Claimed.EpisodeAt = episodeAt
	f.Claimed.EpisodeRevision = episodeRevision
	f.Claimed.EpisodeObligationRevision = episodeObligationRevision
	return nil
}

func clearClaimBinding(f *GoalFile) error {
	if f.StopFence != nil {
		return fmt.Errorf("goal %s is breach-stopped by %s; only goal resume may clear its launch fence", f.Id, f.StopFence.StopID)
	}
	f.Claimed = nil
	f.StopCapability = nil
	f.StopFence = nil
	f.Obligation = nil
	return nil
}

func claimIntentArgs(r VerbRequest, args map[string]string) map[string]string {
	if args == nil {
		args = map[string]string{}
	}
	if r.ClaimEpoch > 0 {
		args["claimEpoch"] = strconv.FormatInt(r.ClaimEpoch, 10)
	}
	return intentArgs(r, args)
}

// ownPair reports whether a claim names the actor's machine AND
// lineage — the pair is the ownership key, never the
// machine alone: a second lineage on the machine is a stranger.
func ownPair(c *ClaimRecord, a Actor) bool {
	return c != nil && c.Machine == a.Machine && c.Lineage == a.Lineage
}

// pairMarker renders a claim as the displaced= marker the design
// pins: <machine>+<lineage>@<claimedAt>.
func pairMarker(c *ClaimRecord) string {
	return c.Machine + "+" + c.Lineage + "@" + c.At
}

// touchDisplaced is touch with the displacement marker:
// every foreign-human mutation of a claimed goal records displaced=
// uniformly, so no lawful override changes a claim's scope without
// leaving the signal.
func touchDisplaced(f *GoalFile, r VerbRequest, verb string, targets []string, displaced string) {
	f.Revision++
	f.History = append(f.History, HistoryLine{
		At: r.stamp(), Opid: r.opid(), Verb: verb,
		Actor: r.Actor.historyActor(), Targets: targets,
		Displaced: displaced, Keep: -1,
	})
}

// ackDisplacements answers displacement addressed to this actor's
// pair: the displaced pair's next History-appending
// publication piggybacks one automatic root-record line per
// unanswered displacement — the published verb with ack,
// targets=<the displaced goal>, and the displaced=<pair>@<at> it
// answers — in the same commit. The root record needs no goal
// authority, which is exactly why the ack lives there and not on
// the (now foreign or parked) goal file.
func ackDisplacements(t *TreeGoals, r VerbRequest, changes []Change) []Change {
	if t.Root == nil {
		return changes
	}
	me := r.Actor.Machine + "+" + r.Actor.Lineage + "@"
	answered := map[string]bool{}
	for _, h := range t.Root.History {
		if h.Ack && h.Displaced != "" {
			for _, target := range h.Targets {
				answered[target+" "+h.Displaced] = true
			}
		}
	}
	// One acknowledgment per displaced pair: goals group
	// under their marker, one line per marker.
	pending := map[string][]string{}
	var markers []string
	collect := func(f *GoalFile) {
		note := func(marker string) {
			if marker == "" || !strings.HasPrefix(marker, me) {
				return
			}
			key := f.Id + " " + marker
			if answered[key] {
				return
			}
			answered[key] = true
			if len(pending[marker]) == 0 {
				markers = append(markers, marker)
			}
			pending[marker] = append(pending[marker], f.Id)
		}
		if f.Parked != nil {
			note(f.Parked.Displaced)
		}
		for _, h := range f.History {
			note(h.Displaced)
		}
	}
	for _, id := range sortedGoalIds(t.Live) {
		collect(t.Live[id])
	}
	for _, id := range sortedGoalIds(t.Done) {
		collect(t.Done[id])
	}
	if len(markers) == 0 {
		return changes
	}
	rootIncluded := false
	for _, c := range changes {
		if c.Path == goalsPrefix+"backlog.md" {
			rootIncluded = true
		}
	}
	// One commit is one write of the root file: bump only when the
	// verb has not already written it this transaction.
	if !rootIncluded {
		t.Root.Revision++
	}
	for _, marker := range markers {
		t.Root.History = append(t.Root.History, HistoryLine{
			At: r.stamp(), Opid: r.opid(), Verb: "ack",
			Actor: r.Actor.historyActor(), Targets: pending[marker],
			Displaced: marker, Ack: true, Keep: -1,
		})
	}
	rendered := Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}
	if rootIncluded {
		for i := range changes {
			if changes[i].Path == goalsPrefix+"backlog.md" {
				changes[i] = rendered
			}
		}
		return changes
	}
	return append(changes, rendered)
}

// intentArgs stamps the directing human into every journaled
// intent: recovery reconstructs authority from the
// stored args, and a live human-directed verb that omitted by= was
// replayed as an AGENT — refused by its own gates or written with
// machine attribution.
func intentArgs(r VerbRequest, args map[string]string) map[string]string {
	if r.Actor.Human == "" && r.ApprovedRef == "" {
		return args
	}
	if args == nil {
		args = map[string]string{}
	}
	if r.Actor.Human != "" {
		args["by"] = r.Actor.Human
	}
	if r.ApprovedRef != "" {
		args["approvedRef"] = r.ApprovedRef
	}
	return args
}

func mergeIntentArgs(left, right map[string]string) map[string]string {
	merged := make(map[string]string, len(left)+len(right))
	for key, value := range left {
		merged[key] = value
	}
	for key, value := range right {
		merged[key] = value
	}
	return merged
}

// opidLanded reports whether this operation's opid is already in
// the file's History — the idempotent-success half of the
// postcondition, per touched file.
func opidLanded(f *GoalFile, r VerbRequest) bool {
	for _, h := range f.History {
		if h.Opid == r.opid() {
			return true
		}
	}
	return false
}

// livePath and donePath name the engine's two write locations. ArchivedPath
// preserves the source location for moves and deletions during the dual-read
// soak.
func livePath(id string) string { return goalsPrefix + id + ".md" }
func donePath(id string) string { return recordsGoalsPrefix + id + ".md" }
func archivedPath(t *TreeGoals, id string) string {
	return doneLocation(t, id)
}

// Open adds a tier-3 queued goal for in-package callers that predate the
// tiered command surface. The command surface never uses this compatibility
// entry point: goal open requires an explicit tier and calls OpenTiered.
// Open and OpenTiered are fixture conveniences: they take no risk record
// and no blocker, so they never model a seat's open. The command surface
// admits only OpenRisked, which carries the seat rule (R-93-m1e).
func Open(r VerbRequest, id, intent, origin, nextStep string, labels ...string) (PublishResult, error) {
	return OpenTiered(r, id, intent, origin, nextStep, 3, nil, labels...)
}

// OpenTiered adds a queued goal at the caller-selected tier. Goal-free clears
// in the same commit when it was declared.
func OpenTiered(r VerbRequest, id, intent, origin, nextStep string, tier uint8, supplied *Budget, labels ...string) (PublishResult, error) {
	req, err := openRequest(r, id, intent, origin, nextStep, "", tier, supplied, nil, "", labels)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, req)
}

// OpenRisked is the open the command surface runs. A seat's open (origin
// main) names the live goal it unblocks in blocks: that goal parks in the
// same publish with the blocker recorded and returns when the blocker is
// done (R-93-m1e, Wido 2026-09-11: seats open only blockers). A person's
// open (origin human) may name a blocker or not.
func OpenRisked(r VerbRequest, id, intent, origin, nextStep, blocks string, risk RiskRecord, requestedTier uint8, why string, supplied *Budget, proof *humanauthority.Proof, labels ...string) (PublishResult, error) {
	if origin == OriginMain && strings.TrimSpace(blocks) == "" {
		return PublishResult{}, fmt.Errorf("%s", SeatOpenNeedsBlocker)
	}
	if err := risk.Validate(); err != nil {
		return PublishResult{}, fmt.Errorf("invalid risk: %v", err)
	}
	derived := risk.DerivedTier()
	tier := requestedTier
	if tier == 0 {
		tier = derived
	}
	if tier < 1 || tier > 3 {
		return PublishResult{}, fmt.Errorf("tier must be 1, 2, or 3")
	}
	if tier != derived && strings.TrimSpace(why) == "" {
		return PublishResult{}, fmt.Errorf("a tier override requires --why")
	}
	if tier < derived {
		if r.Actor.Human == "" {
			return PublishResult{}, fmt.Errorf("a tier below the risk derivation is a human act and requires --by")
		}
		if _, _, _, err := approvalProofClass(r.Endpoint.Root, proof); err != nil {
			return PublishResult{}, err
		}
	}
	req, err := openRequest(r, id, intent, origin, nextStep, blocks, tier, supplied, &risk, why, labels)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, req)
}

// SeatOpenNeedsBlocker is the refusal a seat's open without --blocks gets:
// the ruling, the lawful forms, and where a non-blocking discovery goes.
const SeatOpenNeedsBlocker = "a seat opens only the defect that blocks its claimed goal: name that goal with --blocks <goal-id>, and it parks with the blocker recorded until the blocker is done (R-93-m1e, Wido 2026-09-11). An improvement that blocks nothing is a proposal in memory/backlog-notes.md, never a goal; every other goal is opened by a person (--origin human)"

// openRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func openRequest(r VerbRequest, id, intent, origin, nextStep, blocks string, tier uint8, supplied *Budget, risk *RiskRecord, why string, labels []string) (PublishRequest, error) {
	if tier < 1 || tier > 3 {
		return PublishRequest{}, fmt.Errorf("goal open requires --tier 1, 2, or 3")
	}
	blocks = strings.TrimSpace(blocks)
	if blocks == id {
		return PublishRequest{}, fmt.Errorf("goal %s cannot block itself", id)
	}
	budget := supplied
	if budget == nil {
		box, err := config.TierBox(filepath.Join(r.Endpoint.Root, "metasystem.conf"), tier)
		if err != nil {
			return PublishRequest{}, err
		}
		budget = &box
	} else {
		maximum, err := config.ReviewRoundMax(filepath.Join(r.Endpoint.Root, "metasystem.conf"))
		if err != nil {
			return PublishRequest{}, err
		}
		if err := budget.Validate(maximum); err != nil {
			return PublishRequest{}, fmt.Errorf("invalid budget: %v", err)
		}
	}
	canonical, err := canonicalLabels(labels)
	if err != nil {
		return PublishRequest{}, err
	}
	args := map[string]string{
		"intent": intent, "origin": origin, "next": nextStep, "labels": strings.Join(canonical, ","), "tier": strconv.Itoa(int(tier)),
	}
	targets := []string{id}
	message := "goal open " + id
	if blocks != "" {
		args["blocks"] = blocks
		targets = append(targets, blocks)
		message += " (blocks " + blocks + ")"
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "open", Targets: targets, Args: intentArgs(r, mergeIntentArgs(args,
			mergeIntentArgs(budgetIntentArgs(*budget), riskIntentArgs(risk, why))))},
		Message: message,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if retired, ok := rootDecomposed(t.Root, id); ok {
				return nil, fmt.Errorf("goal id %s is retired: it names a decomposed parent (split opid %s); pick a different id", id, retired.Opid)
			}
			if f, exists := t.Live[id]; exists {
				if opidLanded(f, r) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			if _, archived := t.Done[id]; archived {
				return nil, fmt.Errorf("goal %s is in the archive; reopen is the explicit exception", id)
			}
			f := &GoalFile{
				Id: id, State: StateQueued, Tier: tier, Intent: intent, Origin: origin,
				NextStep: nextStep, OpenedAt: r.stamp(), Revision: 0, Labels: canonical, Budget: budget, Risk: risk,
			}
			touch(f, r, "open", targets)
			if risk != nil && tier != risk.DerivedTier() {
				f.History[len(f.History)-1].Reason = fmt.Sprintf("TierOverride: derived=%d set=%d why=%s", risk.DerivedTier(), tier, why)
			}
			changes := []Change{{Path: livePath(id), Content: RenderFile(f)}}
			if blocks != "" {
				blocked, err := parkBehindBlocker(t, r, blocks, id)
				if err != nil {
					return nil, err
				}
				changes = append(changes, Change{Path: livePath(blocks), Content: RenderFile(blocked)})
			}
			// Opening clears a declared Goal-free in the same commit.
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "open",
					Actor: r.Actor.historyActor(), Targets: targets, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}, nil
}

// parkBehindBlocker records that the goal being opened (blocker) blocks
// an existing live goal, in the open's own publish: the edge lands on the
// blocked goal and, unless it is parked already, the goal parks with the
// blocker named. A seat may name only the goal it holds: the park clears
// that claim, which frees the seat's one claim for the blocker. The goal's
// origin does not matter here: a person's standing reservation parks too,
// because the ruling asks for exactly this park, it is recorded with its
// blocker, and the goal returns by itself when the blocker is done. A
// person may name any live goal, and only a person reaches the branch
// that adds a second blocker to a goal that is parked already.
func parkBehindBlocker(t *TreeGoals, r VerbRequest, blocked, blocker string) (*GoalFile, error) {
	f, live := t.Live[blocked]
	if !live {
		if _, done := t.Done[blocked]; done {
			return nil, fmt.Errorf("--blocks %s names a done goal; a blocker opens only for live work", blocked)
		}
		return nil, fmt.Errorf("--blocks %s names a goal that is not live", blocked)
	}
	if r.Actor.Human == "" && (f.State != StateClaimed || f.Claimed == nil || !ownPair(f.Claimed, r.Actor)) {
		holder := "unclaimed"
		if f.Claimed != nil {
			holder = "claimed by " + f.Claimed.Machine + "+" + f.Claimed.Lineage
		}
		return nil, fmt.Errorf("a seat opens only the defect that blocks the goal it holds (R-93-m1e): goal %s is %s, %s, not this seat's claim", blocked, f.State, holder)
	}
	if !contains(f.Blocked, blocker) {
		f.Blocked = sortedUnique(append(append([]string(nil), f.Blocked...), blocker))
	}
	because := "blocked by " + blocker + "; returns when it is done"
	if f.State == StateParked {
		// The park stands; only the edge is new.
		f.Revision++
		f.History = append(f.History, HistoryLine{
			At: r.stamp(), Opid: r.opid(), Verb: "edit",
			Actor: r.Actor.historyActor(), Targets: []string{blocked}, Keep: -1,
			Reason: "blockedBy adds " + blocker + " (its open)",
		})
		return f, nil
	}
	if f.State != StateQueued && f.State != StateApproved && f.State != StateClaimed {
		return nil, fmt.Errorf("goal %s is %s; only queued, approved, claimed or parked goals take a blocker", blocked, f.State)
	}
	displaced := ""
	if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
		displaced = pairMarker(f.Claimed)
	}
	f.State = StateParked
	f.Parked = &ParkRecord{
		By: r.Actor.historyActor(), At: r.stamp(),
		Because: because, Displaced: displaced, Blocker: blocker,
	}
	if err := clearClaimBinding(f); err != nil {
		return nil, err
	}
	f.Revision++
	f.History = append(f.History, HistoryLine{
		At: r.stamp(), Opid: r.opid(), Verb: "park",
		Actor: r.Actor.historyActor(), Targets: []string{blocked},
		Displaced: displaced, Keep: -1, Reason: because,
	})
	return f, nil
}

// returnBlockerParks lifts every park a blocker's open recorded whose
// blockers are now all done — done, the verb that finishes the last
// blocker, calls it after moving that goal to the archive. The goal
// returns to its resting state (approved when its approval stands,
// queued otherwise) and is claimable again at once.
func returnBlockerParks(t *TreeGoals, r VerbRequest, finished string) []*GoalFile {
	var returned []*GoalFile
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		if f.State != StateParked || f.Parked == nil || f.Parked.Blocker == "" {
			continue
		}
		clear := true
		for _, dep := range f.Blocked {
			if depState(t, dep) != StateDone {
				clear = false
				break
			}
		}
		if !clear {
			continue
		}
		f.State = restingState(f)
		f.Parked = nil
		touch(f, r, "unpark", []string{id})
		f.History[len(f.History)-1].Reason = "blocker " + finished + " is done; the park lifts"
		returned = append(returned, f)
	}
	return returned
}

// Claim takes ownership of a human-approved goal for the actor's pair.
// Claim is AGENT-ONLY: humans direct agents; no human
// lineage exists, so no human claim row.
func Claim(r VerbRequest, id string, budgets ...Budget) (PublishResult, error) {
	if detail := brain.Fence(r.Endpoint.Root, "claim", ExistingLedgerIdentity(r.Endpoint.Root)); detail != "" {
		return PublishResult{}, fmt.Errorf("%s", detail)
	}
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("claim is agent-only: humans direct agents; steal reassigns a standing claim under --by")
	}
	if len(budgets) != 0 || r.ApprovedRef != "" {
		return PublishResult{}, fmt.Errorf("the budget and any norm approval were bound by the human's approval; goal claim carries no tuple or --approved-ref")
	}
	return Publish(r.Endpoint, claimRequest(r, id, nil))
}

// claimRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).

func claimRequest(r VerbRequest, id string, supplied *Budget) PublishRequest {
	args := map[string]string(nil)
	if supplied != nil {
		args = budgetIntentArgs(*supplied)
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "claim", Targets: []string{id}, Args: claimIntentArgs(r, args)},
		Message: "goal claim " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to claim", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State == StateClaimed {
				if ownPair(f.Claimed, r.Actor) {
					return nil, NothingToDo{Reason: "already claimed by this pair (not by this operation)"}
				}
				if f.Claimed != nil && f.Claimed.Machine == r.Actor.Machine {
					return nil, fmt.Errorf("goal %s is claimed by this machine's lineage %s; the pair is the ownership key and a second lineage is refused by name", id, f.Claimed.Lineage)
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			if f.State != StateApproved {
				return nil, approvalRequired(f, "claim")
			}
			if f.Pinned != "" && f.Pinned != r.Actor.Machine {
				return nil, fmt.Errorf("goal %s is pinned to machine %s and this machine is %s; only the pinned machine may claim it (a human re-pins with set-pin)", id, f.Pinned, r.Actor.Machine)
			}
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					return nil, fmt.Errorf("goal %s is blocked by %s, which is not done", id, dep)
				}
			}
			if supplied != nil || r.ApprovedRef != "" {
				return nil, fmt.Errorf("the budget was bound by the human's approval; claim carries no tuple")
			}
			budget, err := requireApprovedForClaim(r.Endpoint.Root, t, f, r.Now, "claim")
			if err != nil {
				return nil, err
			}
			f.State = StateClaimed
			f.Budget = &budget
			touch(f, r, "claim", []string{id})
			if err := bindClaim(f, r.Actor.Machine, r.Actor.Lineage, r.stamp(), f.Revision, r.ClaimEpoch); err != nil {
				return nil, err
			}
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// SetBudget without human proof is retired. SetBudgetApproved replaces the
// whole tuple under the same authority boundary as approval. On claimed work
// it advances the claim and spending boundary while preserving the ownership
// episode used by the elapsed clock.
func SetBudget(r VerbRequest, id string, budget Budget) (PublishResult, error) {
	return PublishResult{}, fmt.Errorf("the budget was bound by the human's approval; goal set-budget requires the human authority proof")
}

func SetBudgetApproved(r VerbRequest, id string, budget Budget, proof *humanauthority.Proof) (PublishResult, error) {
	maximum, maxErr := config.ReviewRoundMax(filepath.Join(r.Endpoint.Root, "metasystem.conf"))
	if maxErr != nil {
		return PublishResult{}, maxErr
	}
	if err := budget.Validate(maximum); err != nil {
		return PublishResult{}, fmt.Errorf("invalid budget: %v", err)
	}
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("goal set-budget is the human's approval act and requires --by")
	}
	authority, reviewBy, temporary, err := approvalProofClass(r.Endpoint.Root, proof)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, setBudgetRequest(r, id, budget, proof, authority, reviewBy, temporary))
}

func setBudgetRequest(r VerbRequest, id string, budget Budget, proof *humanauthority.Proof, authority, reviewBy string, temporary bool) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "set-budget", Targets: []string{id}, Args: claimIntentArgs(r, budgetIntentArgs(budget))},
		Message: "goal set-budget " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; the archive changes through reopen", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if proof == nil || r.Actor.Human == "" || authority == "" {
				return nil, approvalRequired(f, "set-budget recovery")
			}
			if err := refuseRelayedAfterFleetEnrollment(t, temporary); err != nil {
				return nil, err
			}
			if f.State != StateClaimed || f.Claimed == nil {
				return nil, fmt.Errorf("budgets on unclaimed work are the human's approval act, set through goal approve --id %s with --budget", id)
			}
			boxTier := f.Tier
			if boxTier == 0 {
				// Before TierLaw, every tierless migration record is served under
				// tier-three rigor; set-budget must use that same effective box so
				// classify-sweep can backfill its Risk record afterward.
				boxTier = 3
			}
			box, boxErr := config.TierBox(filepath.Join(r.Endpoint.Root, "metasystem.conf"), boxTier)
			if boxErr != nil {
				return nil, boxErr
			}
			overBox := budget.ElapsedDuration() > box.ElapsedDuration() || budget.AttemptLimit > box.AttemptLimit ||
				budget.ReservedJobMinutesLimit > box.ReservedJobMinutesLimit || budget.ActiveJobLimit > box.ActiveJobLimit ||
				budget.ReviewRoundLimit > box.ReviewRoundLimit
			if f.StopFence != nil {
				return nil, fmt.Errorf("goal %s revision %d is breach-stopped by %s; only goal resume with its standing approved budget may reopen admission", id, f.StopFence.Revision, f.StopFence.StopID)
			}
			if temporary {
				if err := repeatedRelayedActError(t.Root, f, "set-budget", proof.Departure); err != nil {
					return nil, err
				}
			}
			if f.Approved != nil && f.Budget != nil && *f.Budget == budget && r.ApprovedRef == "" && f.Claimed.Revision > 0 {
				return nil, NothingToDo{Reason: "the complete budget tuple already reads exactly that"}
			}
			approval, err := goalNormApproval(r.Endpoint.Root, t, f, budget, r.ApprovedRef, proof)
			if err != nil {
				return nil, err
			}
			bound := f.Claimed.Revision > 0
			if f.Approved != nil && f.Budget != nil && *f.Budget == budget && sameGoalNormApproval(f.NormApproval, approval) && bound {
				return nil, NothingToDo{Reason: "the complete budget tuple already reads exactly that"}
			}
			displaced := ""
			if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
				displaced = pairMarker(f.Claimed)
			}
			f.Budget = &budget
			if overBox && f.BudgetExceptions < ^uint16(0) {
				f.BudgetExceptions++
			}
			f.NormApproval = approval
			touchDisplaced(f, r, "set-budget", []string{id}, displaced)
			recordApprovalRelay(f, proof, temporary)
			bindApproval(f, r, authority, reviewBy)
			if f.State == StateClaimed && f.Claimed != nil {
				claimEpoch := r.ClaimEpoch
				if claimEpoch < 1 && r.Actor.Human != "" && f.StopCapability != nil {
					claimEpoch = f.StopCapability.ClaimEpoch
				}
				if err := rebindClaimKeepEpisode(f, r.stamp(), f.Revision, claimEpoch); err != nil {
					return nil, err
				}
			}
			changes := armApprovalGate(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}})
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// SetObligation records the human decision that turns an already claimed,
// budgeted goal into a governed recurrence. It replaces the complete record;
// no field-level mutation can rewrite an earlier obligation revision.
func SetObligation(r VerbRequest, id string, proposed GovernedObligation, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" || proof == nil || !proof.AuthorizesSetObligation(r.Endpoint.Root) {
		return PublishResult{}, fmt.Errorf("set-obligation requires freshly observed enrolled-human authority or a recorded temporary relay whose human provenance is not verified")
	}
	temporaryAuthority := proof.TemporarySetObligationFor(r.Endpoint.Root)
	if !validObligationState(proposed.State) {
		return PublishResult{}, fmt.Errorf("unknown obligation state %q", proposed.State)
	}
	policy, err := config.CorrelationPolicy(r.Endpoint.Root)
	if err != nil {
		return PublishResult{}, err
	}
	if (proposed.State == ObligationLimited || proposed.State == ObligationEnforced) && policy == "" {
		return PublishResult{}, fmt.Errorf("LIMITED and ENFORCED remain unavailable while Wido's correlation-policy slot is empty")
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "set-obligation", Targets: []string{id}, Args: intentArgs(r, map[string]string{
			"state": string(proposed.State), "owner": proposed.Owner,
		})},
		Message: "goal set-obligation " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if err := refuseRelayedAfterFleetEnrollment(t, temporaryAuthority); err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil || f.State != StateClaimed || f.Claimed == nil || f.Budget == nil {
				return nil, fmt.Errorf("goal %s must be claimed with a complete budget before it can own an obligation", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if temporaryAuthority {
				if err := repeatedRelayedActError(t.Root, f, "set-obligation", proof.Departure); err != nil {
					return nil, err
				}
			}
			if f.StopFence != nil {
				return nil, fmt.Errorf("goal %s is breach-stopped; resume under its standing approved tuple before creating another obligation", id)
			}
			o := proposed
			o.Revision = f.Revision + 1
			o.BudgetRevision = f.Claimed.Revision
			if o.State == ObligationDraft || o.State == ObligationObserve {
				o.AuthorizedBy, o.AuthorizedAt, o.AuthorityOperation, o.ReviewPolicy, o.ReviewOutcome, o.AuthorizedEffects = "", "", "", "", "", nil
			} else {
				o.AuthorizedBy, o.AuthorizedAt, o.AuthorityOperation = r.Actor.Human, r.stamp(), r.opid()
				o.ReviewPolicy, o.ReviewOutcome = policy, ReviewOutcomeHumanApproved
				o.AuthorizedEffects = append([]GoverningEffect(nil), o.Effects...)
			}
			o.AuthorityOutcome, o.AuthorityReviewBy, o.AuthorityRuling, o.TemporaryHumanWord = "", "", "", ""
			if temporaryAuthority {
				o.AuthorityOutcome, o.AuthorityReviewBy = AuthorityOutcomeTemporaryHumanWord, proof.ReviewBy
				o.AuthorityRuling, o.TemporaryHumanWord = proof.Departure, proof.TemporaryHumanWord
				if o.State == ObligationLimited || o.State == ObligationEnforced {
					// A relay authorizes this temporary act but cannot authenticate
					// the person named by --by.
					o.AuthorizedBy, o.ReviewOutcome = AuthorizedByRecordedRelay, ReviewOutcomeRecordedRelay
				}
			}
			if err := validateGovernedObligation(&o, o.Revision, f.Claimed, f.Budget, false); err != nil {
				return nil, err
			}
			touch(f, r, "set-obligation", []string{id})
			f.History[len(f.History)-1].ApprovedRef = r.ApprovedRef
			if temporaryAuthority {
				f.History[len(f.History)-1].recordTemporaryRelay(proof.ReviewBy, proof.Departure, proof.TemporaryHumanWord)
			}
			f.Obligation = &o
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Release returns the actor's claimed goal to its approved or queued resting state.
func Release(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, releaseRequest(r, id))
}

// releaseRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func releaseRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "release", Targets: []string{id}, Args: intentArgs(r, nil)},
		Message: "goal release " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to release", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil {
				return nil, fmt.Errorf("goal %s is %s, not claimed", id, f.State)
			}
			if !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is claimed by %s+%s; a foreign release is a human act (steal has its own verb)", id, f.Claimed.Machine, f.Claimed.Lineage)
			}
			displaced := ""
			if !ownPair(f.Claimed, r.Actor) {
				displaced = pairMarker(f.Claimed)
			}
			f.State = restingState(f)
			if err := clearClaimBinding(f); err != nil {
				return nil, err
			}
			touchDisplaced(f, r, "release", []string{id}, displaced)
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// Done concludes one goal and moves it to the archive — the one
// member only; sibling arc members stay untouched.
// residueVocabRe recognizes the conclusion vocabulary that declares
// residue; residueLinkRe is the token that schedules it. R-4: a recorded
// residue is a scheduled debt — prose alone refuses.
var (
	residueVocabRe = regexp.MustCompile(`(?i)\bresidu`)
	residueLinkRe  = regexp.MustCompile(`goal:([a-z0-9][a-z0-9-]*)`)
)

func Done(r VerbRequest, id, conclusion string) (PublishResult, error) {
	if strings.TrimSpace(conclusion) == "" {
		return PublishResult{}, fmt.Errorf("done needs its conclusion — the archive is the record")
	}
	if residueVocabRe.MatchString(conclusion) {
		links := residueLinkRe.FindAllStringSubmatch(conclusion, -1)
		if len(links) == 0 {
			return PublishResult{}, fmt.Errorf("the conclusion names residue without scheduling it: link each residue's open backlog item as goal:<id>, or open one first (R-4: residue is a scheduled debt, not a prose note)")
		}
		for _, link := range links {
			if _, err := os.Stat(filepath.Join(r.Endpoint.Root, "plans", "goals", link[1]+".md")); err != nil {
				return PublishResult{}, fmt.Errorf("the conclusion's residue link goal:%s does not resolve to an open backlog item", link[1])
			}
		}
	}
	result, err := Publish(r.Endpoint, doneRequest(r, id, conclusion))
	if err != nil || (result.Outcome != OutcomeConfirmed && result.Outcome != OutcomeConfirmedLate) {
		return result, err
	}
	tree, treeErr := loadTree(r.Endpoint.Root, result.Tip)
	if treeErr != nil {
		return result, fmt.Errorf("goal done confirmed but arc retro debt could not be classified: %w", treeErr)
	}
	archived := tree.Done[id]
	if archived == nil || archived.Arc == "" {
		return result, nil
	}
	for _, live := range tree.Live {
		if live.Arc == archived.Arc {
			return result, nil
		}
	}
	if _, debtErr := retrodebt.Raise(r.Endpoint.Root, retrodebt.KindArc, archived.Arc+":"+r.opid(), r.Now); debtErr != nil {
		return result, fmt.Errorf("goal done confirmed but its arc retro debt did not land: %w", debtErr)
	}
	return result, nil
}

func DeferFindings(r VerbRequest, id string, obligations []ReviewObligation) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "defer-findings", Targets: []string{id}}, Message: "goal defer-findings " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || !ownPair(f.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s defer-findings requires its owning pair", id)
			}
			for _, incoming := range obligations {
				found := false
				for _, existing := range f.ReviewObligations {
					if existing.Finding == incoming.Finding && existing.Chain == incoming.Chain {
						found = true
						break
					}
				}
				if !found {
					incoming.State = "open"
					f.ReviewObligations = append(f.ReviewObligations, incoming)
				}
			}
			touch(f, r, "defer-findings", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }})
}

func DischargeReviewObligation(r VerbRequest, id, finding, chain, by, citation string) (PublishResult, error) {
	if finding == "" || chain == "" || by == "" || strings.TrimSpace(citation) == "" {
		return PublishResult{}, fmt.Errorf("discharge-review-obligation requires --finding, --chain, --by, and --test")
	}
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "discharge-review-obligation", Targets: []string{id}}, Message: "goal discharge-review-obligation " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s discharge requires the human or owning pair", id)
			}
			match, matchErr := reviewObligationMatch(f.ReviewObligations, finding, chain)
			if matchErr != nil {
				return nil, matchErr
			}
			f.ReviewObligations[match].State = "discharged"
			f.ReviewObligations[match].Test = citation
			touch(f, r, "discharge-review-obligation", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }})
}

func reviewObligationMatch(obligations []ReviewObligation, finding, chain string) (int, error) {
	var matches []int
	for i, obligation := range obligations {
		if obligation.Finding == finding && obligation.Chain == chain {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return 0, fmt.Errorf("no such obligation finding=%s chain=%s", finding, chain)
	}
	if len(matches) != 1 {
		return 0, fmt.Errorf("ambiguous obligation finding=%s chain=%s matches=%v", finding, chain, matches)
	}
	return matches[0], nil
}

func AcceptedRiskDecision(r VerbRequest, id, finding, chain, by, why string, proof *humanauthority.Proof) (PublishResult, error) {
	why = strings.TrimSpace(why)
	if r.Actor.Human == "" || by == "" || finding == "" || chain == "" || why == "" {
		return PublishResult{}, fmt.Errorf("goal accept-risk is a human act and requires --id, --finding, --chain, --by, and --why")
	}
	if _, _, _, err := approvalProofClass(r.Endpoint.Root, proof); err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "accept-risk", Targets: []string{id}}, Message: "goal accept-risk " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed {
				return nil, fmt.Errorf("goal %s is not claimed", id)
			}
			for _, existing := range f.AcceptedRisks {
				if existing.Finding == finding && existing.Chain == chain {
					if existing.By != by {
						return nil, fmt.Errorf("finding %s on chain %s was already accepted by %s", finding, chain, existing.By)
					}
					if chain == HumanCarriedChain {
						var prior *HistoryLine
						for index := range f.History {
							if f.History[index].Opid == existing.Opid {
								prior = &f.History[index]
								break
							}
						}
						if prior == nil {
							return nil, fmt.Errorf("accepted-risk replay is unrepeatable: history row %s is missing", existing.Opid)
						}
						if prior.Reason != why {
							return nil, fmt.Errorf("accepted-risk replay refused: why differs: recorded=%s given=%s", prior.Reason, why)
						}
					}
					return nil, AlreadyApplied{}
				}
			}
			f.AcceptedRisks = append(f.AcceptedRisks, AcceptedRiskRecord{Finding: finding, Chain: chain, By: by, Opid: r.opid()})
			if chain == HumanCarriedChain {
				match, matchErr := reviewObligationMatch(f.ReviewObligations, finding, chain)
				if matchErr != nil {
					return nil, matchErr
				}
				if f.ReviewObligations[match].State != "open" {
					return nil, fmt.Errorf("review obligation finding=%s chain=%s is not open", finding, chain)
				}
				f.ReviewObligations[match].State = "discharged"
				f.ReviewObligations[match].Test = "accepted-risk:" + r.opid()
			}
			touch(f, r, "accept-risk", []string{id})
			f.History[len(f.History)-1].Reason = why
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }})
}

func AcceptedRiskDecisionOpID(repoRoot, id, finding, chain string, now time.Time) (string, error) {
	endpoint, err := ResolveEndpoint(repoRoot)
	if err != nil {
		return "", err
	}
	projection, err := Project(endpoint, false, now)
	if err != nil {
		return "", err
	}
	var file *GoalFile
	if projection.Tree != nil {
		file = projection.Tree.Live[id]
		if file == nil {
			file = projection.Tree.Done[id]
		}
	}
	if file == nil {
		return "", fmt.Errorf("goal %s is absent", id)
	}
	for _, risk := range file.AcceptedRisks {
		if risk.Finding == finding && risk.Chain == chain {
			return risk.Opid, nil
		}
	}
	return "", fmt.Errorf("goal %s has no accepted-risk decision for finding %s on chain %s", id, finding, chain)
}

// doneRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func doneRequest(r VerbRequest, id, conclusion string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "done", Targets: []string{id}, Args: intentArgs(r, map[string]string{
			"conclusion": conclusion,
		})},
		Message: "goal done " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if f, archived := t.Done[id]; archived {
				if opidLanded(f, r) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to conclude", id)
			}
			for _, obligation := range f.ReviewObligations {
				if obligation.State == "open" {
					return nil, fmt.Errorf("goal %s has open review obligation finding=%s chain=%s test=%s", id, obligation.Finding, obligation.Chain, obligation.Test)
				}
			}
			if err := doneCarryRefusal(r.Endpoint.Root, t, carryCodeTip(r.Endpoint, tip), id, f, r.Now); err != nil {
				return nil, err
			}
			// Queued concludes directly; a foreign
			// claim concludes only under a human, and the override
			// leaves the displacement signal.
			if f.State == StateClaimed && !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is claimed by %s+%s; concluding another's work is a human act", id, f.Claimed.Machine, f.Claimed.Lineage)
			}
			if f.State == StateParked && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is parked; concluding it is a human act", id)
			}
			if f.Origin == OriginHuman && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s was opened by the human; concluding it is a human act", id)
			}
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					return nil, fmt.Errorf("goal %s is blocked by %s, which is not done", id, dep)
				}
			}
			displaced := ""
			if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
				displaced = pairMarker(f.Claimed)
			}
			f.State = StateDone
			f.Conclude = conclusion
			if err := clearClaimBinding(f); err != nil {
				return nil, err
			}
			f.Parked = nil
			t.Done[id] = f
			delete(t.Live, id)
			returned := returnBlockerParks(t, r, id)
			compactions := compactDepartedPriorities(t.Live, []*GoalFile{f})
			targets := []string{id}
			if len(compactions) > 0 {
				targets = compactions[0].Targets
			}
			touchDisplaced(f, r, "done", targets, displaced)
			changes := []Change{
				{Path: livePath(id), Delete: true},
				{Path: donePath(id), Content: RenderFile(f)},
			}
			written := map[string]bool{}
			for _, compaction := range compactions {
				for _, change := range compaction.Changed {
					mergePriorityEvent(change.File, r, "done", compaction.Targets, change.Before, change.After)
					changes = append(changes, Change{Path: livePath(change.File.Id), Content: RenderFile(change.File)})
					written[change.File.Id] = true
				}
			}
			// A returned goal that the compaction already rendered
			// carries both effects in that one render; one write per file.
			for _, g := range returned {
				if !written[g.Id] {
					changes = append(changes, Change{Path: livePath(g.Id), Content: RenderFile(g)})
				}
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

func depState(t *TreeGoals, id string) string {
	if f, ok := t.Live[id]; ok {
		return f.State
	}
	if _, ok := t.Done[id]; ok {
		return StateDone
	}
	return ""
}

// lastOpid names the most recent operation on a file — the winner a
// losing competitor reports.
func lastOpid(f *GoalFile) string {
	if len(f.History) == 0 {
		return "unknown"
	}
	return f.History[len(f.History)-1].Opid
}

// Park pauses a goal with its reason. Parking another machine's
// claim is a human act, and the displaced claimant is recorded —
// displacement is a stop signal the serving machine hears (the
// notification legs land with the projection). Arc cascades land
// with the arcs layer.
func Park(r VerbRequest, id, because string) (PublishResult, error) {
	if strings.TrimSpace(because) == "" {
		return PublishResult{}, fmt.Errorf("park needs its reason — a pause without a why is a stall in disguise")
	}
	return Publish(r.Endpoint, parkRequest(r, id, because))
}

// parkRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func parkRequest(r VerbRequest, id, because string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "park", Targets: []string{id}, Args: intentArgs(r, map[string]string{
			"because": because,
		})},
		Message: "goal park " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to park", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State == StateParked {
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			if f.State != StateQueued && f.State != StateApproved && f.State != StateClaimed {
				return nil, fmt.Errorf("goal %s is %s; only queued, approved, or claimed goals park", id, f.State)
			}
			if f.Origin == OriginHuman && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s was opened by the human; an agent cannot silently remove a standing human reservation (park is a human act here)", id)
			}
			displaced := ""
			if f.State == StateClaimed && f.Claimed != nil {
				if !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
					return nil, fmt.Errorf("goal %s is claimed by %s+%s; parking another's claim is a human act", id, f.Claimed.Machine, f.Claimed.Lineage)
				}
				if !ownPair(f.Claimed, r.Actor) {
					displaced = pairMarker(f.Claimed)
				}
			}
			f.State = StateParked
			f.Parked = &ParkRecord{
				By: r.Actor.historyActor(), At: r.stamp(),
				Because: because, Displaced: displaced,
			}
			if err := clearClaimBinding(f); err != nil {
				return nil, err
			}
			f.Revision++
			f.History = append(f.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "park",
				Actor: r.Actor.historyActor(), Targets: []string{id},
				Displaced: displaced, Keep: -1, Reason: because,
			})
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// Unpark returns a parked goal to its approved or queued resting state. The park's records
// stay in the history; Goal-free clears when it was declared.
func Unpark(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, unparkRequest(r, id))
}

// unparkRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func unparkRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "unpark", Targets: []string{id}, Args: intentArgs(r, nil)},
		Message: "goal unpark " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to unpark", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateParked {
				return nil, fmt.Errorf("goal %s is %s, not parked", id, f.State)
			}
			// A human's park is a standing reservation an agent cannot
			// silently lift (the table's human-origin-park row).
			if f.Parked != nil && strings.HasPrefix(f.Parked.By, "human:") && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s was parked by %s; lifting a human's pause is a human act", id, f.Parked.By)
			}
			// A blocker's park lifts by itself when every blocker is done
			// (R-93-m1e); an agent cannot lift it earlier, a human can.
			if f.Parked != nil && f.Parked.Blocker != "" && r.Actor.Human == "" {
				for _, dep := range f.Blocked {
					if depState(t, dep) != StateDone {
						return nil, fmt.Errorf("goal %s is parked behind %s, which is not done; it returns by itself when every blocker is done (R-93-m1e), and lifting it earlier is a human act", id, dep)
					}
				}
			}
			f.State = restingState(f)
			f.Parked = nil
			touch(f, r, "unpark", []string{id})
			changes := []Change{{Path: livePath(id), Content: RenderFile(f)}}
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "unpark",
					Actor: r.Actor.historyActor(), Targets: []string{id}, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// Reopen is done's explicit exception: the archived file moves back
// to the live set as queued. Goal-free clears when it was declared.
func Reopen(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, reopenRequest(r, id))
}

// reopenRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func reopenRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "reopen", Targets: []string{id}, Args: intentArgs(r, nil)},
		Message: "goal reopen " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if f, live := t.Live[id]; live {
				if opidLanded(f, r) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			f, archived := t.Done[id]
			if !archived {
				return nil, fmt.Errorf("goal %s is not in the archive; reopen moves archived goals back", id)
			}
			if _, decomposed := rootDecomposed(t.Root, id); decomposed {
				return nil, fmt.Errorf("goal %s was decomposed into arc %s; a decomposed parent never returns — reopen or claim its member goals, or open a new goal under a new id", id, id)
			}
			// Reopening under claimed dependents is the transition
			// closure's refusal: a claimed goal's blockers must stay
			// done.
			for _, liveId := range sortedGoalIds(t.Live) {
				dependent := t.Live[liveId]
				if dependent.State != StateClaimed {
					continue
				}
				for _, dep := range dependent.Blocked {
					if dep == id {
						return nil, fmt.Errorf("goal %s cannot reopen: %s is claimed and depends on it staying done", id, liveId)
					}
				}
			}
			beforeRank := rankOf(f)
			archivePath := archivedPath(t, id)
			// A mixed arc has no single standing state. Reopen defaults
			// queued, except that an all-parked destination copies its
			// newest park record under human authority, while the caller's
			// own claimed member may adopt this member after the ordinary
			// blocker, pin, budget, and norm guards.
			f.State = StateQueued
			f.Conclude = ""
			f.Approved = nil
			f.Budget = nil
			f.NormApproval = nil
			if f.Arc != "" {
				standing := classifyArcJoin(t, f.Arc, id, r.Actor)
				switch {
				case standing.count == 0:
					// The arc has no live members; the reopen re-founds it queued.
				case standing.allParked:
					if r.Actor.Human == "" {
						return nil, fmt.Errorf("goal %s rejoins arc %s, whose every live member is parked; reopening into an all-parked arc is a human act", id, f.Arc)
					}
					parked := standing.newestParked.Parked
					f.State = StateParked
					f.Parked = &ParkRecord{By: parked.By, At: parked.At, Because: parked.Because}
				case standing.ownClaimed != nil:
					return nil, fmt.Errorf("goal %s reopens queued; joining a claimed arc no longer manufactures approval or a claim", id)
				}
			}
			if f.Priority != 0 {
				f.Sequence = uint64(len(priorityIDs(t.Live, f.Priority, "")) + 1)
			}
			touch(f, r, "reopen", []string{id})
			if afterRank := rankOf(f); afterRank != beforeRank {
				mergePriorityEvent(f, r, "reopen", []string{id}, beforeRank, afterRank)
			}
			delete(t.Done, id)
			t.Live[id] = f
			changes := []Change{
				{Path: archivePath, Delete: true},
				{Path: livePath(id), Content: RenderFile(f)},
			}
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "reopen",
					Actor: r.Actor.historyActor(), Targets: []string{id}, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// EditFields is the edit verb's delta set: nil-able fields change
// only when set. Prose caps are REMOVED by design — a
// multi-kilobyte intent is lawful. Origin is NOT here:
// provenance is immutable authority-bearing fact, refused on every
// surface — hand edit, verb, and recovery alike.
type EditFields struct {
	Intent   *string
	Tier     *uint8
	Risk     *RiskRecord
	NextStep *string
	Blocked  *[]string
	Labels   *[]string
	Why      string
	Evidence string
	Proof    *humanauthority.Proof
}

// Edit applies field deltas to one live goal.
func Edit(r VerbRequest, id string, fields EditFields) (PublishResult, error) {
	var riskRaised bool
	req, err := editRequestReportingRiskRaise(r, id, fields, &riskRaised)
	if err != nil {
		return PublishResult{}, err
	}
	result, err := Publish(r.Endpoint, req)
	result.RiskRaised = riskRaised
	return result, err
}

// editRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func editRequest(r VerbRequest, id string, fields EditFields) (PublishRequest, error) {
	return editRequestReportingRiskRaise(r, id, fields, nil)
}

func editRequestReportingRiskRaise(r VerbRequest, id string, fields EditFields, riskRaised *bool) (PublishRequest, error) {
	if fields.Tier != nil && (*fields.Tier < 1 || *fields.Tier > 3) {
		return PublishRequest{}, fmt.Errorf("tier must be 1, 2, or 3")
	}
	if fields.Labels != nil {
		canonical, err := canonicalLabels(*fields.Labels)
		if err != nil {
			return PublishRequest{}, err
		}
		fields.Labels = &canonical
	}
	if fields.Risk != nil {
		if err := fields.Risk.Validate(); err != nil {
			return PublishRequest{}, fmt.Errorf("invalid risk: %v", err)
		}
		derived := fields.Risk.DerivedTier()
		if fields.Tier != nil && *fields.Tier != derived && strings.TrimSpace(fields.Why) == "" {
			return PublishRequest{}, fmt.Errorf("a tier override requires --why")
		}
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "edit", Targets: []string{id}, Deltas: editDeltas(id, fields), Args: intentArgs(r, map[string]string{"why": fields.Why, "evidence": fields.Evidence})},
		Message: "goal edit " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; the archive edits through reopen", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.Approved != nil && fields.Intent != nil {
				return nil, fmt.Errorf("the human approved this intent; unapprove the goal, edit it, then approve the new intent")
			}
			oldDerived, oldWidth := f.Tier, "area"
			if f.Risk != nil {
				oldDerived, oldWidth = f.Risk.DerivedTier(), f.Risk.GateWidth()
			}
			newDerived, newWidth := oldDerived, oldWidth
			if fields.Risk != nil {
				newDerived, newWidth = fields.Risk.DerivedTier(), fields.Risk.GateWidth()
			}
			raise := fields.Risk != nil && newDerived > oldDerived && f.Approved != nil
			targetTier := f.Tier
			if fields.Tier != nil {
				targetTier = *fields.Tier
			} else if fields.Risk != nil {
				targetTier = newDerived
				if raise && f.Tier > oldDerived && f.Tier >= newDerived {
					targetTier = f.Tier
				}
			}
			if f.Tier != targetTier && (f.Approved != nil || f.State == StateClaimed || f.State == StateParked) && !raise {
				return nil, fmt.Errorf("goal %s is approved, claimed, or parked; unapprove it, edit --tier, then approve it", id)
			}
			if fields.Risk != nil && f.Approved != nil && !raise {
				return nil, fmt.Errorf("goal %s is approved; unapprove it, edit --risk, then approve it", id)
			}
			lowering := riskAnswersLower(f.Risk, fields.Risk) || newDerived < oldDerived || targetTier < f.Tier || (oldWidth == "full" && newWidth == "area")
			if lowering {
				if r.Actor.Human == "" {
					return nil, fmt.Errorf("lowering a risk score, derived tier, set tier, or gate width is a human act")
				}
				if _, _, _, proofErr := approvalProofClass(r.Endpoint.Root, fields.Proof); proofErr != nil {
					return nil, proofErr
				}
			}
			if raise && strings.TrimSpace(fields.Evidence) == "" {
				return nil, fmt.Errorf("raising the derived tier after approval requires --evidence")
			}
			// The table's edit rows: queued is open to all, claimed is
			// the claimant's or a human's (the foreign-human override
			// leaves the displacement signal), parked has no
			// agent row — the pause stands until a human moves it.
			if f.State == StateClaimed && !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is claimed by %s+%s; editing another's claimed goal is a human act", id, f.Claimed.Machine, f.Claimed.Lineage)
			}
			if f.State == StateParked && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is parked; editing a parked goal is a human act", id)
			}
			// The standing invariant: a claimed goal is never blocked — a new
			// blocker must be DONE for EVERY actor (a human who wants
			// the edge parks or releases first).
			if f.State == StateClaimed && fields.Blocked != nil {
				for _, dep := range *fields.Blocked {
					if depState(t, dep) != StateDone {
						return nil, fmt.Errorf("goal %s is claimed and never blocked: %s is not done — park or release first", id, dep)
					}
				}
			}
			displaced := ""
			if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
				displaced = pairMarker(f.Claimed)
			}
			if fields.Intent != nil {
				f.Intent = *fields.Intent
			}
			if fields.Tier != nil || fields.Risk != nil {
				f.Tier = targetTier
			}
			if fields.Risk != nil {
				copyRisk := *fields.Risk
				f.Risk = &copyRisk
			}
			if fields.NextStep != nil {
				f.NextStep = *fields.NextStep
			}
			if fields.Blocked != nil {
				f.Blocked = append([]string(nil), (*fields.Blocked)...)
			}
			if fields.Labels != nil {
				f.Labels = append([]string(nil), (*fields.Labels)...)
			}
			touchDisplaced(f, r, "edit", []string{id}, displaced)
			if raise {
				if riskRaised != nil {
					*riskRaised = true
				}
				f.History[len(f.History)-1].Reason = fmt.Sprintf("Misclassified: from=%d to=%d evidence=%s", oldDerived, newDerived, fields.Evidence)
				if f.Tier != newDerived {
					f.History[len(f.History)-1].Reason += fmt.Sprintf("; TierOverride: derived=%d set=%d why=%s", newDerived, f.Tier, fields.Why)
				}
				box, boxErr := config.TierBox(filepath.Join(r.Endpoint.Root, "metasystem.conf"), f.Tier)
				if boxErr != nil {
					return nil, boxErr
				}
				if f.Budget.ReviewRoundLimit < box.ReviewRoundLimit {
					f.Budget.ReviewRoundLimit = box.ReviewRoundLimit
				}
				prior := f.Approved
				f.Approved = &ApprovalRecord{By: prior.By, At: r.stamp(), Revision: f.Revision, Opid: r.opid(), Authority: "raise=" + r.opid(), Digest: ApprovalDigest(f.Intent, f.Tier, *f.Budget, f.Risk)}
				if f.Claimed != nil {
					if err := rebindClaimRevisionForRiskRaise(f, f.Revision); err != nil {
						return nil, err
					}
				}
			} else if fields.Risk != nil && f.Tier != fields.Risk.DerivedTier() {
				f.History[len(f.History)-1].Reason = fmt.Sprintf("TierOverride: derived=%d set=%d why=%s", fields.Risk.DerivedTier(), f.Tier, fields.Why)
			}
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}, nil
}

func riskAnswersLower(old, next *RiskRecord) bool {
	return old != nil && next != nil && (next.Severity < old.Severity || next.Novelty < old.Novelty || next.Exposure < old.Exposure || next.Accumulation < old.Accumulation)
}

func riskIntentArgs(risk *RiskRecord, why string) map[string]string {
	if risk == nil {
		return nil
	}
	return map[string]string{"risk": risk.scoreArgs(), "basis": risk.Basis, "why": why}
}

// rebindClaimRevisionForRiskRaise strengthens the claimed revision without
// resetting its elapsed origin, lease epoch, launch fence, or obligation.
func rebindClaimRevisionForRiskRaise(f *GoalFile, revision uint64) error {
	if f.Claimed == nil || f.StopCapability == nil {
		return fmt.Errorf("goal %s has no complete claim authority to rebind", f.Id)
	}
	f.Claimed.Revision = revision
	f.StopCapability.Generation = revision
	f.StopCapability.Revision = revision
	return nil
}

// DeclareFree declares the absence of intent: no queued or claimed
// goals may exist, parked coexistence is lawful, renewal is
// idempotent.
func DeclareFree(r VerbRequest, origin, digest string) (PublishResult, error) {
	return Publish(r.Endpoint, declareFreeRequest(r, origin, digest))
}

// declareFreeRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func declareFreeRequest(r VerbRequest, origin, digest string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "declare-free", Args: intentArgs(r, map[string]string{
			"origin": origin, "digest": digest,
		})},
		Message: "goal declare-free",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if t.Root == nil {
				return nil, fmt.Errorf("no root record; the ledger is not adopted")
			}
			for _, id := range sortedGoalIds(t.Live) {
				switch t.Live[id].State {
				case StateQueued, StateApproved, StateClaimed:
					return nil, fmt.Errorf("goal %s is %s; Goal-free declares over parked and done only", id, t.Live[id].State)
				}
			}
			if t.Root.Free != nil && t.Root.Free.Digest == digest {
				if rootOpidLanded(t.Root, r) {
					return nil, AlreadyApplied{}
				}
				return nil, NothingToDo{Reason: "the declaration already stands at this digest"}
			}
			t.Root.Free = &FreeRecord{Declared: r.stamp(), Origin: origin, Digest: digest}
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "declare-free",
				Actor: r.Actor.historyActor(), Keep: -1,
			})
			return ackDisplacements(t, r, []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// Steal reassigns another machine's claim under a human's name —
// steal without its human refuses up front, and the
// history line carries the human authority.
func Steal(r VerbRequest, id string) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("steal is a human act and names its human (--by)")
	}
	return Publish(r.Endpoint, stealRequest(r, id))
}

// stealRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func stealRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "steal", Targets: []string{id}, Args: intentArgs(r, map[string]string{
			"by": r.Actor.Human,
		})},
		Message: "goal steal " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to steal", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil {
				return nil, fmt.Errorf("goal %s is %s; steal reassigns a standing claim (claim takes a queued goal)", id, f.State)
			}
			if ownPair(f.Claimed, r.Actor) {
				return nil, NothingToDo{Reason: "already claimed by this pair (not by this operation)"}
			}
			// Steal follows the selected old pair across the arc. Other
			// independently claimed, parked, or queued members neither move
			// nor lend their fence, pin, or budget to this preflight.
			oldPair := f.Claimed
			members := arcMembers(t, id)
			for _, member := range members {
				if member.State != StateClaimed || !ownPair(member.Claimed, Actor{Machine: oldPair.Machine, Lineage: oldPair.Lineage}) {
					continue
				}
				if member.StopFence != nil {
					return nil, fmt.Errorf("goal %s is breach-stopped by %s; only goal resume may replace its claim authority", member.Id, member.StopFence.StopID)
				}
				if member.Pinned != "" && member.Pinned != r.Actor.Machine {
					return nil, fmt.Errorf("goal %s is pinned to machine %s and this machine is %s; even a steal honors the pin — clear the pin, steal, then re-pin", member.Id, member.Pinned, r.Actor.Machine)
				}
				if r.ApprovedRef != "" {
					return nil, fmt.Errorf("steal uses the standing approval and does not take --approved-ref")
				}
				if _, err := requireApprovedForClaim(r.Endpoint.Root, t, member, r.Now, "steal"); err != nil {
					return nil, err
				}
			}
			targets := make([]string, 0, len(members))
			for _, m := range members {
				targets = append(targets, m.Id)
			}
			var changes []Change
			for _, m := range members {
				if m.State != StateClaimed || !ownPair(m.Claimed, Actor{Machine: oldPair.Machine, Lineage: oldPair.Lineage}) {
					continue // independently owned or idle members stay untouched
				}
				displaced := pairMarker(m.Claimed)
				touchDisplaced(m, r, "steal", targets, displaced)
				if err := bindClaim(m, r.Actor.Machine, r.Actor.Lineage, r.stamp(), m.Revision, r.ClaimEpoch); err != nil {
					return nil, err
				}
				changes = append(changes, Change{Path: livePath(m.Id), Content: RenderFile(m)})
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// OpenClaim is the retired open-and-claim surface. Approval must be a distinct
// human act before an execution claim, so this helper always refuses.
func OpenClaim(r VerbRequest, id, intent, origin, nextStep string, budget Budget, labels ...string) (PublishResult, error) {
	return PublishResult{}, fmt.Errorf("APPROVAL_REQUIRED: open --claim is retired; open the goal queued, have the human approve its exact intent and budget, then claim it")
}

// openClaimRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func openClaimRequest(r VerbRequest, id, intent, origin, nextStep string, budget Budget, labels []string) (PublishRequest, error) {
	canonical, err := canonicalLabels(labels)
	if err != nil {
		return PublishRequest{}, err
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "open-claim", Targets: []string{id}, Args: claimIntentArgs(r, mergeIntentArgs(
			map[string]string{"intent": intent, "origin": origin, "next": nextStep, "labels": strings.Join(canonical, ",")},
			budgetIntentArgs(budget)))},
		Message: "goal open --claim " + id,
		Mutate: func(tip string) ([]Change, error) {
			return nil, fmt.Errorf("APPROVAL_REQUIRED: recovery cannot replay retired open --claim for goal %s; close this entry by hand", id)
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}, nil
}

// Prune deletes archived goals outside the retention closure: every
// done goal reachable from a LIVE goal's blocker edges (following
// done-to-done edges — no dangling reference by construction) stays,
// the newest keep-count done goals by OpenedAt stay, the rest die.
// The opid and the literal keep=<n> land in the root record's
// History, since the target files themselves are gone.
func Prune(r VerbRequest, keep int) (PublishResult, error) {
	if keep < 0 {
		return PublishResult{}, fmt.Errorf("prune keeps a non-negative count")
	}
	return Publish(r.Endpoint, pruneRequest(r, keep))
}

// pruneRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func pruneRequest(r VerbRequest, keep int) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "prune", Args: intentArgs(r, map[string]string{
			"keep": fmt.Sprintf("%d", keep),
		})},
		Message: "goal prune",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if t.Root == nil {
				return nil, fmt.Errorf("no root record; the ledger is not adopted")
			}
			if rootOpidLanded(t.Root, r) {
				return nil, AlreadyApplied{}
			}
			// Selection first, closure second: retained =
			// closure(live goals ∪ the keep-count newest done goals),
			// walked through done-to-done edges — a keep-count
			// survivor's own older done blocker is retained WITH it,
			// so no edge can dangle by construction.
			keepSet := map[string]bool{}
			var walk func(id string)
			walk = func(id string) {
				f, done := t.Done[id]
				if !done || keepSet[id] {
					return
				}
				keepSet[id] = true
				for _, dep := range f.Blocked {
					walk(dep)
				}
			}
			for _, id := range sortedGoalIds(t.Live) {
				for _, dep := range t.Live[id].Blocked {
					walk(dep)
				}
			}
			// The keep-count selects the NEWEST by (OpenedAt, id) and
			// each survivor seeds the same closure walk; deletion
			// prefers the oldest.
			ids := sortedGoalIds(t.Done)
			byAge := append([]string(nil), ids...)
			sortDoneNewestFirst(t, byAge)
			for i := 0; i < keep && i < len(byAge); i++ {
				walk(byAge[i])
			}
			var changes []Change
			var dropped []string
			for _, id := range ids {
				if !keepSet[id] {
					for _, history := range t.Done[id].History {
						if recordedRelayedAct(history) {
							retained := history
							retained.Targets = []string{id}
							t.Root.History = append(t.Root.History, retained)
						}
					}
					changes = append(changes, Change{Path: archivedPath(t, id), Delete: true})
					dropped = append(dropped, id)
				}
			}
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "prune",
				Actor: r.Actor.historyActor(), Targets: dropped, Keep: keep,
			})
			changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

func rootOpidLanded(root *RootRecord, r VerbRequest) bool {
	for _, h := range root.History {
		if h.Opid == r.opid() {
			return true
		}
	}
	return false
}

// sortDoneNewestFirst orders done goal ids newest-first by
// (OpenedAt, id) — the retention order the design pins.
func sortDoneNewestFirst(t *TreeGoals, ids []string) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0; j-- {
			a, b := t.Done[ids[j-1]], t.Done[ids[j]]
			older := a.OpenedAt < b.OpenedAt || (a.OpenedAt == b.OpenedAt && a.Id < b.Id)
			if older {
				ids[j-1], ids[j] = ids[j], ids[j-1]
			} else {
				break
			}
		}
	}
}

// arcMembers collects the LIVE members of a goal's arc, the asked
// goal included — the cascade set claim/release/park/unpark move as
// one atomic unit. A goal with no arc is its own cascade.
func arcMembers(t *TreeGoals, id string) []*GoalFile {
	f := t.Live[id]
	if f == nil {
		return nil
	}
	if f.Arc == "" {
		return []*GoalFile{f}
	}
	var members []*GoalFile
	for _, liveId := range sortedGoalIds(t.Live) {
		if t.Live[liveId].Arc == f.Arc {
			members = append(members, t.Live[liveId])
		}
	}
	return members
}

// arcJoinState classifies a mixed destination without inventing a single
// "standing member". An own claimed member is selected deterministically;
// all-parked destinations carry the newest park record, with lexical id as
// the total tie-break.
type arcJoinState struct {
	count        int
	allParked    bool
	ownClaimed   *GoalFile
	newestParked *GoalFile
}

func classifyArcJoin(t *TreeGoals, arc, excludeID string, actor Actor) arcJoinState {
	state := arcJoinState{allParked: true}
	for _, liveID := range sortedGoalIds(t.Live) {
		member := t.Live[liveID]
		if member.Id == excludeID || member.Arc != arc {
			continue
		}
		state.count++
		if member.State != StateParked || member.Parked == nil {
			state.allParked = false
		}
		if state.ownClaimed == nil && member.State == StateClaimed && member.Claimed != nil && ownPair(member.Claimed, actor) {
			state.ownClaimed = member
		}
		if member.State == StateParked && member.Parked != nil &&
			(state.newestParked == nil || member.Parked.At > state.newestParked.Parked.At ||
				(member.Parked.At == state.newestParked.Parked.At && member.Id < state.newestParked.Id)) {
			state.newestParked = member
		}
	}
	if state.count == 0 {
		state.allParked = false
	}
	return state
}

// ClaimArc is an opt-in cascade over one planning arc. It claims approved
// members, skips already-owned and parked members, and loses atomically to
// any foreign claim it encounters.
func ClaimArc(r VerbRequest, id string, budgets ...Budget) (PublishResult, error) {
	if detail := brain.Fence(r.Endpoint.Root, "claim", ExistingLedgerIdentity(r.Endpoint.Root)); detail != "" {
		return PublishResult{}, fmt.Errorf("%s", detail)
	}
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("claim is agent-only: humans direct agents; steal reassigns a standing claim under --by")
	}
	if len(budgets) != 0 || r.ApprovedRef != "" {
		return PublishResult{}, fmt.Errorf("the budget and any norm approval were bound by the human's approval; goal claim carries no tuple or --approved-ref")
	}
	return Publish(r.Endpoint, claimArcRequest(r, id, nil))
}

// claimArcRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func claimArcRequest(r VerbRequest, id string, supplied *Budget) PublishRequest {
	args := map[string]string{"cascade": "arc"}
	if supplied != nil {
		args = mergeIntentArgs(args, budgetIntentArgs(*supplied))
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "claim", Targets: []string{id}, Args: claimIntentArgs(r, args)},
		Message: "goal claim " + id + " (arc cascade)",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			members := arcMembers(t, id)
			if members == nil {
				return nil, fmt.Errorf("goal %s is not live; nothing to claim", id)
			}
			targets := make([]string, 0, len(members))
			for _, m := range members {
				targets = append(targets, m.Id)
			}
			var changes []Change
			for _, m := range members {
				if opidLanded(m, r) {
					return nil, AlreadyApplied{}
				}
				if m.State == StateClaimed {
					if ownPair(m.Claimed, r.Actor) {
						continue // already ours; the cascade completes the set
					}
					if m.Claimed != nil && m.Claimed.Machine == r.Actor.Machine {
						return nil, fmt.Errorf("arc member %s is claimed by this machine's lineage %s; the pair is the ownership key and a second lineage is refused by name", m.Id, m.Claimed.Lineage)
					}
					return nil, LostToCompetitor{Winner: lastOpid(m)}
				}
				if m.State == StateParked {
					continue // parked members are not movable; claim the queued remainder
				}
				if m.State == StateQueued {
					return nil, approvalRequired(m, "arc claim")
				}
				if m.State != StateApproved {
					return nil, fmt.Errorf("arc member %s is %s; the cascade claims approved members only", m.Id, m.State)
				}
				for _, dep := range m.Blocked {
					if depState(t, dep) != StateDone {
						return nil, fmt.Errorf("arc member %s is blocked by %s, which is not done", m.Id, dep)
					}
				}
				if err := pinRefusal(m, r.Actor.Machine, "the arc claim"); err != nil {
					return nil, err
				}
				if supplied != nil || r.ApprovedRef != "" {
					return nil, fmt.Errorf("the budget was bound by the human's approval; claim carries no tuple")
				}
				budget, err := requireApprovedForClaim(r.Endpoint.Root, t, m, r.Now, "arc claim")
				if err != nil {
					return nil, err
				}
				m.State = StateClaimed
				m.Budget = &budget
				touch(m, r, "claim", targets)
				if err := bindClaim(m, r.Actor.Machine, r.Actor.Lineage, r.stamp(), m.Revision, r.ClaimEpoch); err != nil {
					return nil, err
				}
				changes = append(changes, Change{Path: livePath(m.Id), Content: RenderFile(m)})
			}
			if len(changes) == 0 {
				return nil, NothingToDo{Reason: "the cascade found nothing to move"}
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// ReleaseArc releases the members movable by this actor and skips independent
// claims belonging to other pairs (a human may still release them explicitly).
func ReleaseArc(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, releaseArcRequest(r, id))
}

// releaseArcRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func releaseArcRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "release", Targets: []string{id}, Args: intentArgs(r, map[string]string{"cascade": "arc"})},
		Message: "goal release " + id + " (arc cascade)",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			members := arcMembers(t, id)
			if members == nil {
				return nil, fmt.Errorf("goal %s is not live; nothing to release", id)
			}
			targets := make([]string, 0, len(members))
			for _, m := range members {
				targets = append(targets, m.Id)
			}
			var changes []Change
			for _, m := range members {
				if opidLanded(m, r) {
					return nil, AlreadyApplied{}
				}
				if m.State != StateClaimed || m.Claimed == nil {
					continue // queued or parked members ride along untouched
				}
				if !ownPair(m.Claimed, r.Actor) && r.Actor.Human == "" {
					continue // another pair's independently claimed member is not movable
				}
				displaced := ""
				if !ownPair(m.Claimed, r.Actor) {
					displaced = pairMarker(m.Claimed)
				}
				m.State = restingState(m)
				if err := clearClaimBinding(m); err != nil {
					return nil, err
				}
				touchDisplaced(m, r, "release", targets, displaced)
				changes = append(changes, Change{Path: livePath(m.Id), Content: RenderFile(m)})
			}
			if len(changes) == 0 {
				return nil, NothingToDo{Reason: "the cascade found nothing to move"}
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// ParkArc pauses every member the caller may move. Independently held or
// human-reserved members are skipped for an agent. A human may displace
// several pairs; each touched member carries its own pair marker so the
// acknowledgment fold emits one line per distinct pair.
func ParkArc(r VerbRequest, id, because string) (PublishResult, error) {
	if strings.TrimSpace(because) == "" {
		return PublishResult{}, fmt.Errorf("park needs its reason — a pause without a why is a stall in disguise")
	}
	return Publish(r.Endpoint, parkArcRequest(r, id, because))
}

// parkArcRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func parkArcRequest(r VerbRequest, id, because string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "park", Targets: []string{id}, Args: intentArgs(r, map[string]string{
			"because": because, "cascade": "arc",
		})},
		Message: "goal park " + id + " (arc cascade)",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			members := arcMembers(t, id)
			if members == nil {
				return nil, fmt.Errorf("goal %s is not live; nothing to park", id)
			}
			targets := make([]string, 0, len(members))
			for _, m := range members {
				targets = append(targets, m.Id)
			}
			var changes []Change
			for _, m := range members {
				if opidLanded(m, r) {
					return nil, AlreadyApplied{}
				}
				if m.State == StateParked {
					continue // already parked members ride along
				}
				if r.Actor.Human == "" && m.Origin == OriginHuman {
					continue // an agent cannot move the human's standing reservation
				}
				if r.Actor.Human == "" && m.State == StateClaimed && m.Claimed != nil && !ownPair(m.Claimed, r.Actor) {
					continue // another pair's claim is not movable by this agent
				}
				if m.State != StateQueued && m.State != StateApproved && m.State != StateClaimed {
					return nil, fmt.Errorf("arc member %s is %s; only queued, approved, or claimed goals park", m.Id, m.State)
				}
				memberDisplaced := ""
				if m.State == StateClaimed && m.Claimed != nil && !ownPair(m.Claimed, r.Actor) {
					memberDisplaced = pairMarker(m.Claimed)
				}
				m.State = StateParked
				m.Parked = &ParkRecord{
					By: r.Actor.historyActor(), At: r.stamp(),
					Because: because, Displaced: memberDisplaced,
				}
				if err := clearClaimBinding(m); err != nil {
					return nil, err
				}
				m.Revision++
				m.History = append(m.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "park",
					Actor: r.Actor.historyActor(), Targets: targets,
					Displaced: memberDisplaced, Keep: -1, Reason: because,
				})
				changes = append(changes, Change{Path: livePath(m.Id), Content: RenderFile(m)})
			}
			if len(changes) == 0 {
				return nil, NothingToDo{Reason: "the cascade found nothing to move"}
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// UnparkArc restores every parked member the caller may move and skips all
// other states; an agent also skips parks carrying human authority.
func UnparkArc(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, unparkArcRequest(r, id))
}

// unparkArcRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func unparkArcRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "unpark", Targets: []string{id}, Args: intentArgs(r, map[string]string{"cascade": "arc"})},
		Message: "goal unpark " + id + " (arc cascade)",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			members := arcMembers(t, id)
			if members == nil {
				return nil, fmt.Errorf("goal %s is not live; nothing to unpark", id)
			}
			targets := make([]string, 0, len(members))
			for _, m := range members {
				targets = append(targets, m.Id)
			}
			var changes []Change
			for _, m := range members {
				if opidLanded(m, r) {
					return nil, AlreadyApplied{}
				}
				if m.State != StateParked {
					continue
				}
				if m.Parked != nil && strings.HasPrefix(m.Parked.By, "human:") && r.Actor.Human == "" {
					continue // a human pause is not movable by this agent
				}
				m.State = restingState(m)
				m.Parked = nil
				touch(m, r, "unpark", targets)
				changes = append(changes, Change{Path: livePath(m.Id), Content: RenderFile(m)})
			}
			if len(changes) == 0 {
				return nil, NothingToDo{Reason: "the cascade found nothing to move"}
			}
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "unpark",
					Actor: r.Actor.historyActor(), Targets: targets, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// Detach removes one member from its arc. A member claimed under
// the arc's claimant RELEASES on the way out — the quota never
// splits one claim into two (the no-quota-split rule).
func Detach(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, detachRequest(r, id))
}

// detachRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func detachRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "detach", Targets: []string{id}, Args: intentArgs(r, nil)},
		Message: "goal detach " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to detach", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.Arc == "" {
				return nil, NothingToDo{Reason: "the goal is not in an arc"}
			}
			if f.State == StateParked && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is parked; a parked arc's membership edits are human acts", id)
			}
			displaced := ""
			if f.State == StateClaimed && f.Claimed != nil {
				if !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
					return nil, fmt.Errorf("goal %s is claimed by %s+%s; detaching another's claimed member is a human act", id, f.Claimed.Machine, f.Claimed.Lineage)
				}
				if !ownPair(f.Claimed, r.Actor) {
					displaced = pairMarker(f.Claimed)
				}
				// The departing member releases: one claim, one arc.
				f.State = restingState(f)
				if err := clearClaimBinding(f); err != nil {
					return nil, err
				}
			}
			f.Arc = ""
			touchDisplaced(f, r, "detach", []string{id}, displaced)
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// SetArc moves a goal into an arc under the membership matrix
// : a move between arcs composes detach-then-join in
// ONE transaction under the stricter of the two rules. Leaving a
// claimed arc releases the member on the way out (never a quota
// split); a parked source or a parked destination is human-only; a
// caller-owned claimed destination may auto-claim a member with done
// blockers; otherwise a mixed destination leaves it queued. An all-parked
// destination copies the newest park record under human authority.
// SetPin pins a goal to one machine (or clears the pin with "-"): only
// that machine may claim it afterwards, because it alone has the
// setup, network, or resources the work needs. Pinning is directive,
// so it is a human act.
// pinRefusal is the ONE pin check every claim-assigning path runs: a
// pinned goal is claimed only by its pinned machine, whatever verb
// carries the assignment.
func pinRefusal(f *GoalFile, machine, how string) error {
	if f.Pinned != "" && f.Pinned != machine {
		return fmt.Errorf("goal %s is pinned to machine %s and %s would claim it for %s; only the pinned machine may hold it (a human re-pins with set-pin)", f.Id, f.Pinned, how, machine)
	}
	return nil
}

func SetPin(r VerbRequest, id, pin string) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("set-pin is a human act and names its human (--by): pinning directs machines")
	}
	if pin == "" {
		return PublishResult{}, fmt.Errorf("set-pin names its machine; \"-\" clears the pin")
	}
	if pin != "-" && !validPinnedNickname(pin) {
		return PublishResult{}, fmt.Errorf("set-pin machine %q is not a machine nickname (one word, no whitespace of any kind — exactly the vocabulary claims carry)", pin)
	}
	return Publish(r.Endpoint, setPinRequest(r, id, pin))
}

// validPinnedNickname admits what a claim's machine field can carry —
// any whitespace-free word, Unicode whitespace refused too (the file
// grammar trims on reparse, so admitting it would let a confirmed pin
// dissolve into no pin), no length cap — with ONE reserved word: "-"
// is set-pin's clear form, so a machine enrolled under that name can
// claim but can never be a pin target; enroll a real name.
func validPinnedNickname(pin string) bool {
	return pin != "-" && ValidateMachineNickname(pin) == nil
}

// setPinRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func setPinRequest(r VerbRequest, id, pin string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "set-pin", Targets: []string{id}, Args: intentArgs(r, map[string]string{"pin": pin})},
		Message: "goal set-pin " + id + " -> " + pin,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			next := pin
			if pin == "-" {
				next = ""
			}
			if f.Pinned == next {
				return nil, NothingToDo{Reason: "the pin already reads exactly that"}
			}
			// A standing claim on another machine outlives a new pin
			// only by explicit direction: refuse so the human decides
			// between waiting, releasing, and stealing.
			if next != "" && f.State == StateClaimed && f.Claimed != nil && f.Claimed.Machine != next {
				return nil, fmt.Errorf("goal %s is claimed by machine %s; release it first (or clear the pin, steal, and re-pin) before pinning it to %s", id, f.Claimed.Machine, next)
			}
			f.Pinned = next
			touch(f, r, "set-pin", []string{id})
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

func SetArc(r VerbRequest, id, arc string) (PublishResult, error) {
	if arc == "" {
		return PublishResult{}, fmt.Errorf("set-arc names its arc; detach removes membership")
	}
	return Publish(r.Endpoint, setArcRequest(r, id, arc))
}

// setArcRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func setArcRequest(r VerbRequest, id, arc string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "set-arc", Targets: []string{id}, Args: intentArgs(r, map[string]string{"arc": arc})},
		Message: "goal set-arc " + id + " -> " + arc,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.Arc == arc {
				return nil, NothingToDo{Reason: "already a member of that arc"}
			}
			displaced := ""
			sourceWasClaimed := false
			// The source side: leaving an arc under its row's rule;
			// the detach result feeds the join.
			switch f.State {
			case StateQueued:
			case StateApproved:
			case StateClaimed:
				sourceWasClaimed = true
				if f.Arc == "" {
					return nil, fmt.Errorf("goal %s is claimed and in no arc; a claimed member cannot join an arc — release first", id)
				}
				if !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
					return nil, fmt.Errorf("goal %s is claimed by %s+%s; moving another's claimed member is a human act", id, f.Claimed.Machine, f.Claimed.Lineage)
				}
				if f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
					displaced = pairMarker(f.Claimed)
				}
				// Released as it detaches — the claim never splits.
				f.State = restingState(f)
				if err := clearClaimBinding(f); err != nil {
					return nil, err
				}
			case StateParked:
				if r.Actor.Human == "" {
					return nil, fmt.Errorf("goal %s is parked; a parked member moves under a human", id)
				}
			default:
				return nil, fmt.Errorf("goal %s is %s; arc membership moves live goals", id, f.State)
			}
			// The destination is classified across every member. Mixed
			// states default queued; only all-parked and caller-owned claim
			// cases inherit authority-bearing state.
			standing := classifyArcJoin(t, arc, id, r.Actor)
			switch {
			case standing.count == 0:
				if f.State == StateParked {
					f.State = restingState(f)
					f.Parked = nil
				}
			case standing.allParked:
				if r.Actor.Human == "" {
					return nil, fmt.Errorf("arc %s has every live member parked; changing its membership is a human act", arc)
				}
				parked := standing.newestParked.Parked
				f.State = StateParked
				f.Parked = &ParkRecord{By: parked.By, At: parked.At, Because: parked.Because}
			case standing.ownClaimed != nil:
				if sourceWasClaimed {
					// TWO displaced pairs in one move — the source
					// claimant losing a member and the destination
					// claimant gaining one — is the composed-move row
					// the design explicitly refuses:
					// release first, then join.
					return nil, fmt.Errorf("goal %s moves from one claimed arc into another; two claimants cannot trade a member in one move — release it first", id)
				}
				if f.State == StateQueued {
					// Joining a claimed arc does not manufacture approval.
					break
				}
				if f.State == StateParked {
					f.State = restingState(f)
					f.Parked = nil
					break
				}
				if f.State != StateApproved {
					return nil, approvalRequired(f, "set-arc claim")
				}
				for _, dep := range f.Blocked {
					if depState(t, dep) != StateDone {
						return nil, fmt.Errorf("goal %s is blocked by %s, which is not done; it cannot join the claimed arc unclaimed-late", id, dep)
					}
				}
				if err := pinRefusal(f, r.Actor.Machine, "joining the claimed arc"); err != nil {
					return nil, err
				}
				if _, err := requireApprovedForClaim(r.Endpoint.Root, t, f, r.Now, "set-arc claim"); err != nil {
					return nil, err
				}
				f.State = StateClaimed
				f.Parked = nil
				claimEpoch := r.ClaimEpoch
				if claimEpoch < 1 && standing.ownClaimed.StopCapability != nil {
					claimEpoch = standing.ownClaimed.StopCapability.ClaimEpoch
				}
				if err := bindClaim(f, r.Actor.Machine, r.Actor.Lineage, r.stamp(), f.Revision+1, claimEpoch); err != nil {
					return nil, err
				}
			default:
				if f.State == StateParked {
					f.State = restingState(f)
				}
				f.Parked = nil
			}
			f.Arc = arc
			touchDisplaced(f, r, "set-arc", []string{id}, displaced)
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// editDeltas serializes an edit's field changes into the journal's
// durable intent — enough to rebuild the edit without the original
// process (recovery completes from the stored intent).
func editDeltas(id string, fields EditFields) []FieldDelta {
	var deltas []FieldDelta
	if fields.Intent != nil {
		deltas = append(deltas, FieldDelta{Target: id, Field: "intent", New: *fields.Intent})
	}
	if fields.Tier != nil {
		deltas = append(deltas, FieldDelta{Target: id, Field: "tier", New: strconv.Itoa(int(*fields.Tier))})
	}
	if fields.Risk != nil {
		deltas = append(deltas, FieldDelta{Target: id, Field: "risk", New: fields.Risk.scoreArgs()})
		deltas = append(deltas, FieldDelta{Target: id, Field: "basis", New: fields.Risk.Basis})
	}
	if fields.NextStep != nil {
		deltas = append(deltas, FieldDelta{Target: id, Field: "next", New: *fields.NextStep})
	}
	if fields.Blocked != nil {
		deltas = append(deltas, FieldDelta{Target: id, Field: "blockedBy", New: strings.Join(*fields.Blocked, ",")})
	}
	if fields.Labels != nil {
		deltas = append(deltas, FieldDelta{Target: id, Field: "labels", New: strings.Join(*fields.Labels, ",")})
	}
	return deltas
}

const (
	HumanCarriedChain = "human-carried"
	defaultCarryLife  = 2 * time.Hour
	maximumCarryLife  = 4 * time.Hour
)

// CarryWord is the durable human word extracted from one goal-history row.
type CarryWord struct {
	Goal       string
	History    HistoryLine
	Workspace  string
	Past       string
	Expires    time.Time
	Supersedes string
}

// CarryReservation is the newest reservation row for one carry word and its
// derived state.
type CarryReservation struct {
	Goal    string
	History HistoryLine
	State   string
}

// CarryConsumption is the first durable fact that consumed a word.
type CarryConsumption struct {
	Kind string
	ID   string
}

// CarryDebt is one unpaid carry found before a new word or reservation.
type CarryDebt struct {
	Kind, Goal, ID, Detail string
}

// CarryAskError is the question-shaped outcome shared by the command and
// landing boundaries.
type CarryAskError struct {
	Code string
	Text string
}

func (e *CarryAskError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Text
}

func carryAsk(code, text string) error { return &CarryAskError{Code: code, Text: text} }

func carryAskFromRejectedDetail(detail string) error {
	code, text, ok := strings.Cut(detail, ": ")
	if !ok || text == "" || (code != "goal-item-not-held" && !strings.HasPrefix(code, "carry-")) {
		return nil
	}
	return carryAsk(code, text)
}

var carryTokenPattern = regexp.MustCompile(`(^|\s)carry workspace=([0-9a-f]{40}) goal=([a-z0-9][a-z0-9-]*) past=([^\s]+)(\s|$)`)
var exactCarryTokenPattern = regexp.MustCompile(`^carry workspace=[0-9a-f]{40} goal=[a-z0-9][a-z0-9-]* past=[^\s]+$`)

func ValidCarryToken(value string) bool { return exactCarryTokenPattern.MatchString(value) }

// ParseCarryWord accepts exactly one contiguous four-field token. Terminal
// words carry their explicit expiry; channel words derive the fixed lifetime.
func ParseCarryWord(goalID string, history HistoryLine) (CarryWord, error) {
	reasonBeforeWhy, reasonAfterWhy, err := splitCarryReasonAtWhy(history.Reason)
	if err != nil {
		return CarryWord{}, err
	}
	matches := carryTokenPattern.FindAllStringSubmatch(reasonBeforeWhy, -1)
	if len(matches) != 1 || matches[0][3] != goalID {
		return CarryWord{}, fmt.Errorf("carry word must contain exactly one token for goal %s", goalID)
	}
	at, err := time.Parse(time.RFC3339, history.At)
	if err != nil {
		return CarryWord{}, fmt.Errorf("carry word has invalid time: %w", err)
	}
	word := CarryWord{Goal: goalID, History: history, Workspace: matches[0][2], Past: matches[0][4]}
	if history.Verb == "carry" {
		expires := regexp.MustCompile(`(^|\s)expires=([^\s]+)`).FindStringSubmatch(reasonBeforeWhy)
		if len(expires) != 3 {
			return CarryWord{}, fmt.Errorf("terminal carry word has no expires field")
		}
		word.Expires, err = time.Parse(time.RFC3339, expires[2])
		if err != nil {
			return CarryWord{}, fmt.Errorf("terminal carry word has invalid expires field: %w", err)
		}
		if supersedes := regexp.MustCompile(`(^|\s)supersedes=([^\s]+)`).FindStringSubmatch(reasonAfterWhy); len(supersedes) == 3 {
			word.Supersedes = supersedes[2]
		}
	} else if history.Verb == "answer" {
		word.Expires = at.Add(defaultCarryLife)
	} else {
		return CarryWord{}, fmt.Errorf("history verb %s is not a carry word", history.Verb)
	}
	return word, nil
}

// splitCarryReasonAtWhy keeps keyed fields outside the quoted human reason.
// A token-looking substring inside why is prose, never ledger grammar.
func splitCarryReasonAtWhy(reason string) (string, string, error) {
	marker := strings.Index(reason, " why=")
	if marker < 0 {
		return reason, "", nil
	}
	quoted := reason[marker+len(" why="):]
	if quoted == "" || quoted[0] != '"' {
		return "", "", fmt.Errorf("carry word has an invalid quoted why field")
	}
	escaped := false
	for i := 1; i < len(quoted); i++ {
		switch {
		case escaped:
			escaped = false
		case quoted[i] == '\\':
			escaped = true
		case quoted[i] == '"':
			if _, err := strconv.Unquote(quoted[:i+1]); err != nil {
				return "", "", fmt.Errorf("carry word has an invalid quoted why field: %w", err)
			}
			return reason[:marker], quoted[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("carry word has an unterminated quoted why field")
}

// CarryWordAt finds a terminal or channel carry word on the named goal.
func CarryWordAt(tree *TreeGoals, goalID, opid string) (CarryWord, error) {
	if tree == nil {
		return CarryWord{}, fmt.Errorf("the ledger tree is absent")
	}
	file := tree.Live[goalID]
	if file == nil {
		file = tree.Done[goalID]
	}
	if file == nil {
		return CarryWord{}, fmt.Errorf("goal %s is absent", goalID)
	}
	for _, history := range file.History {
		terminal := history.Verb == "carry" && history.AuthorityOutcome == AuthorityOutcomeHumanAuthorityProven
		channel := history.Verb == "answer" && history.AuthorityOutcome == AuthorityOutcomeAuthenticatedChannelWord
		if history.Opid == opid && (terminal || channel) {
			return ParseCarryWord(goalID, history)
		}
	}
	return CarryWord{}, fmt.Errorf("carry word %s is missing on goal %s", opid, goalID)
}

func carryWords(tree *TreeGoals) []CarryWord {
	if tree == nil {
		return nil
	}
	var words []CarryWord
	collect := func(files map[string]*GoalFile) {
		for _, id := range sortedGoalIds(files) {
			for _, history := range files[id].History {
				terminal := history.Verb == "carry" && history.AuthorityOutcome == AuthorityOutcomeHumanAuthorityProven
				channel := history.Verb == "answer" && history.AuthorityOutcome == AuthorityOutcomeAuthenticatedChannelWord
				if !terminal && !channel {
					continue
				}
				if word, err := ParseCarryWord(id, history); err == nil {
					words = append(words, word)
				}
			}
		}
	}
	collect(tree.Live)
	collect(tree.Done)
	return words
}

// OpidMachine derives the seat from the operation identifier's middle field.
func OpidMachine(opid string) (string, error) {
	if !validOpidShape(opid) {
		return "", fmt.Errorf("operation id %q is not valid", opid)
	}
	last := strings.LastIndex(opid, "-")
	return opid[27:last], nil
}

// CarryWordProven applies the production generation fence while allowing the
// explicit fake-runtime fixture authority.
func CarryWordProven(root string, word CarryWord) bool {
	return word.History.Verb == "answer" || word.History.AuthorityGeneration > 0 || fixtureauth.FixtureModeRoot(root)
}

func CarryableName(root, name string) bool {
	if strings.HasPrefix(name, "group:") {
		contract, err := testpolicy.Load(filepath.Join(root, "testing.json"))
		if err != nil {
			return false
		}
		group := strings.TrimPrefix(name, "group:")
		for _, candidate := range contract.Groups {
			if candidate.ID == group {
				return true
			}
		}
		return false
	}
	for _, row := range refusal.Rows {
		if row.Code == name && strings.HasPrefix(row.Override, "land.sh --carried") && row.Pending == "" {
			return true
		}
	}
	return false
}

func historyFiles(tree *TreeGoals) map[string]*GoalFile {
	files := make(map[string]*GoalFile, len(tree.Live)+len(tree.Done))
	for id, file := range tree.Live {
		files[id] = file
	}
	for id, file := range tree.Done {
		files[id] = file
	}
	return files
}

func carriedRow(tree *TreeGoals, approvedRef string) (string, HistoryLine, bool) {
	for _, id := range sortedGoalIds(historyFiles(tree)) {
		for _, history := range historyFiles(tree)[id].History {
			if history.Verb == "carried" && history.ApprovedRef == approvedRef {
				return id, history, true
			}
		}
	}
	return "", HistoryLine{}, false
}

func hasExactTrailer(message, key, value string) bool {
	wanted := key + ": " + value
	for _, line := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
		if line == wanted {
			return true
		}
	}
	return false
}

func commitWithTrailer(root, revision, key, value string) (string, error) {
	out, err := goalGit(root, nil, "log", "--format=%H%x1f%B%x1e", revision)
	if err != nil {
		return "", err
	}
	for _, record := range strings.Split(out, "\x1e") {
		parts := strings.SplitN(strings.TrimSpace(record), "\x1f", 2)
		if len(parts) == 2 && hasExactTrailer(parts[1], key, value) {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	return "", nil
}

func carryAnchorAndCommit(root, codeTip, opid string) (string, string, error) {
	anchor, err := commitWithTrailer(root, codeTip, "Goal-Transaction", opid)
	if err != nil || anchor == "" {
		return anchor, "", err
	}
	commit, err := commitWithTrailer(root, anchor+".."+codeTip, "Carry", opid)
	return anchor, commit, err
}

// CarryConsumptionAt checks ledger rows first and then the anchored code
// range; no timestamp participates in consumption.
func CarryConsumptionAt(root string, tree *TreeGoals, codeTip string, word CarryWord) (CarryConsumption, error) {
	if _, row, ok := carriedRow(tree, word.History.Opid); ok {
		if strings.HasPrefix(row.Reason, "superseded ") {
			match := regexp.MustCompile(`(^|\s)by=([^\s]+)`).FindStringSubmatch(row.Reason)
			if len(match) == 3 {
				return CarryConsumption{Kind: "superseded", ID: match[2]}, nil
			}
		}
		return CarryConsumption{Kind: "ledger", ID: row.Opid}, nil
	}
	for _, successor := range carryWords(tree) {
		if successor.Supersedes == word.History.Opid {
			return CarryConsumption{Kind: "superseded", ID: successor.History.Opid}, nil
		}
	}
	if codeTip == "" {
		return CarryConsumption{Kind: "none"}, nil
	}
	anchor, commit, err := carryAnchorAndCommit(root, codeTip, word.History.Opid)
	if err != nil {
		return CarryConsumption{}, err
	}
	if anchor == "" {
		return CarryConsumption{Kind: "missing-anchor"}, nil
	}
	if commit != "" {
		return CarryConsumption{Kind: "origin", ID: commit}, nil
	}
	return CarryConsumption{Kind: "none"}, nil
}

func carryingRows(file *GoalFile, ref string) []HistoryLine {
	var rows []HistoryLine
	if file == nil {
		return rows
	}
	for _, history := range file.History {
		if history.Verb == "carrying" && history.ApprovedRef == ref {
			rows = append(rows, history)
		}
	}
	return rows
}

// CarryReservationAt derives open, expired, abandoned, or closed from the
// newest opening row and its durable closers.
func CarryReservationAt(tree *TreeGoals, goalID, ref string, now time.Time) CarryReservation {
	file := tree.Live[goalID]
	if file == nil {
		file = tree.Done[goalID]
	}
	rows := carryingRows(file, ref)
	var opening HistoryLine
	for _, row := range rows {
		if strings.HasPrefix(row.Reason, "open ") || row.Reason == "open" {
			opening = row
		}
	}
	if opening.Opid == "" {
		return CarryReservation{Goal: goalID, State: "none"}
	}
	// A durable carried row is the final closer. It wins even if recovery
	// previously abandoned the local reservation: the canonical landing is
	// the debt-bearing fact the reservation exists to serialize.
	if _, _, ok := carriedRow(tree, ref); ok {
		return CarryReservation{Goal: goalID, History: opening, State: "closed"}
	}
	for _, row := range rows {
		if row.Opid != opening.Opid && strings.HasPrefix(row.Reason, "abandoned ") && strings.Contains(row.Reason, "of="+opening.Opid) {
			return CarryReservation{Goal: goalID, History: opening, State: "abandoned"}
		}
	}
	expiry := regexp.MustCompile(`(^|\s)expires=([^\s]+)`).FindStringSubmatch(opening.Reason)
	if len(expiry) == 3 {
		if stamp, err := time.Parse(time.RFC3339, expiry[2]); err == nil && !now.Before(stamp) {
			return CarryReservation{Goal: goalID, History: opening, State: "expired"}
		}
	}
	return CarryReservation{Goal: goalID, History: opening, State: "open"}
}

func carryCodeTip(endpoint Endpoint, capturedTip string) string {
	if capturedTip != "" && !endpoint.LocalMode() {
		return capturedTip
	}
	return "refs/heads/main"
}

func openCarryWords(root string, tree *TreeGoals, codeTip, seat string, now time.Time) ([]CarryWord, error) {
	var open []CarryWord
	for _, word := range carryWords(tree) {
		if !CarryWordProven(root, word) || !now.Before(word.Expires) {
			continue
		}
		wordSeat, err := OpidMachine(word.History.Opid)
		if err != nil || wordSeat != seat {
			continue
		}
		consumption, err := CarryConsumptionAt(root, tree, codeTip, word)
		if err != nil {
			return nil, err
		}
		if consumption.Kind == "none" || consumption.Kind == "missing-anchor" {
			open = append(open, word)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].History.Opid < open[j].History.Opid })
	return open, nil
}

// OpenCarryWords exposes the counter's single definition to the read-only
// landing classifier and steward summary.
func OpenCarryWords(root string, tree *TreeGoals, codeTip, seat string, now time.Time) ([]CarryWord, error) {
	return openCarryWords(root, tree, codeTip, seat, now)
}

type CarryCounts struct{ Today, Open, Inflight, Debt int }

func LandedCarryCount(file *GoalFile) int {
	count := 0
	if file != nil {
		for _, history := range file.History {
			if history.Verb == "carried" && strings.HasPrefix(history.Reason, "landed ") {
				count++
			}
		}
	}
	return count
}

func CountCarries(root string, tree *TreeGoals, codeTip string, now time.Time) (CarryCounts, error) {
	counts := CarryCounts{}
	for _, collection := range []map[string]*GoalFile{tree.Live, tree.Done} {
		for _, file := range collection {
			seenReservations := map[string]bool{}
			for _, history := range file.History {
				if history.Verb == "carried" && strings.HasPrefix(history.Reason, "landed ") {
					if stamp, err := time.Parse(time.RFC3339, history.At); err == nil && stamp.UTC().Format("2006-01-02") == now.UTC().Format("2006-01-02") {
						counts.Today++
					}
				}
				if history.Verb == "carrying" && history.ApprovedRef != "" && !seenReservations[history.ApprovedRef] {
					seenReservations[history.ApprovedRef] = true
					if CarryReservationAt(tree, file.Id, history.ApprovedRef, now).State == "open" {
						counts.Inflight++
					}
				}
			}
			for _, obligation := range file.ReviewObligations {
				if obligation.Chain == HumanCarriedChain && obligation.State == "open" {
					counts.Debt++
				}
			}
		}
	}
	for _, word := range carryWords(tree) {
		if !CarryWordProven(root, word) || !now.Before(word.Expires) {
			continue
		}
		consumption, err := CarryConsumptionAt(root, tree, codeTip, word)
		if err != nil {
			return CarryCounts{}, err
		}
		if consumption.Kind == "none" || consumption.Kind == "missing-anchor" {
			counts.Open++
		}
	}
	return counts, nil
}

func openCarryingDebt(tree *TreeGoals, exceptRef string, now time.Time) (CarryDebt, bool) {
	for _, id := range sortedGoalIds(tree.Live) {
		file := tree.Live[id]
		seen := map[string]bool{}
		for _, history := range file.History {
			if history.Verb != "carrying" || history.ApprovedRef == "" || seen[history.ApprovedRef] || history.ApprovedRef == exceptRef {
				continue
			}
			seen[history.ApprovedRef] = true
			reservation := CarryReservationAt(tree, id, history.ApprovedRef, now)
			if reservation.State == "open" {
				seat, _ := OpidMachine(reservation.History.Opid)
				expires := regexp.MustCompile(`(^|\s)expires=([^\s]+)`).FindStringSubmatch(reservation.History.Reason)
				detail := ""
				if len(expires) == 3 {
					detail = expires[2]
				}
				return CarryDebt{Kind: "inflight", Goal: id, ID: reservation.History.Opid, Detail: "seat=" + seat + " expires=" + detail}, true
			}
		}
	}
	return CarryDebt{}, false
}

// CarryDebtAt finds reviewed-late, in-flight, and landed-without-a-row debt
// in that order.
func CarryDebtAt(root string, tree *TreeGoals, base, exceptRef string, now time.Time) (CarryDebt, bool, error) {
	for _, id := range sortedGoalIds(tree.Live) {
		for _, obligation := range tree.Live[id].ReviewObligations {
			if obligation.Chain != HumanCarriedChain || obligation.State != "open" || !strings.HasPrefix(obligation.Artifact, "commit:") {
				continue
			}
			commit := strings.TrimPrefix(obligation.Artifact, "commit:")
			ancestor, err := IsAncestor(root, commit, base)
			if err != nil {
				return CarryDebt{}, false, err
			}
			if ancestor {
				return CarryDebt{Kind: "obligation", Goal: id, ID: obligation.Finding, Detail: obligation.Artifact}, true, nil
			}
		}
	}
	if debt, ok := openCarryingDebt(tree, exceptRef, now); ok {
		return debt, true, nil
	}
	for _, word := range carryWords(tree) {
		if !CarryWordProven(root, word) {
			continue
		}
		if _, _, exists := carriedRow(tree, word.History.Opid); exists {
			continue
		}
		consumption, err := CarryConsumptionAt(root, tree, base, word)
		if err != nil {
			return CarryDebt{}, false, err
		}
		if consumption.Kind == "origin" {
			return CarryDebt{Kind: "unrecorded", Goal: word.Goal, ID: word.History.Opid, Detail: consumption.ID}, true, nil
		}
	}
	return CarryDebt{}, false, nil
}

func carryDebtText(debt CarryDebt) string {
	switch debt.Kind {
	case "obligation":
		return fmt.Sprintf("carry debt is unpaid: goal %s has open obligation %s at %s; discharge it with a code critic of the commit or goal accept-risk", debt.Goal, debt.ID, debt.Detail)
	case "inflight":
		return fmt.Sprintf("carry debt is unpaid: reservation %s on goal %s is in flight (%s)", debt.ID, debt.Goal, debt.Detail)
	case "unrecorded":
		return fmt.Sprintf("carry debt is unpaid: commit %s carries word %s without its ledger row; close it with land.sh --carried %s", debt.Detail, debt.ID, debt.ID)
	default:
		return "carry debt is unpaid"
	}
}

// CarryArgs is the complete terminal-word input after the command layer has
// verified and projected the supplied tree.
type CarryArgs struct {
	Goal, Workspace, Past, Why, Supersede string
	Expires                               time.Time
	Transfer, RaiseFormat                 bool
}

const carryRemoteRequiredText = "a carried landing needs a code remote: set goal.sync-remote"

func carryRemoteRequired() error { return carryAsk("carry-remote-required", carryRemoteRequiredText) }

func Carry(r VerbRequest, args CarryArgs, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Endpoint.LocalMode() {
		return PublishResult{}, carryRemoteRequired()
	}
	if proof == nil || !proof.ValidFor(r.Endpoint.Root) || r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("goal carry requires the enrolled human terminal; carry takes no relayed word")
	}
	if args.Goal == "" || args.Workspace == "" || args.Past == "" || strings.TrimSpace(args.Why) == "" {
		return PublishResult{}, fmt.Errorf("goal carry requires the goal, workspace tree, named refusal, and why")
	}
	if !args.Expires.After(r.Now) || args.Expires.Sub(r.Now) > maximumCarryLife {
		return PublishResult{}, carryAsk("carry-word-expired", "the carry expiry must be after now and no more than the four-hour ceiling")
	}
	if !CarryableName(r.Endpoint.Root, args.Past) {
		return PublishResult{}, carryAsk("carry-not-carryable", fmt.Sprintf("%s is not a refusal or testing group a carry word may name", args.Past))
	}
	request := carryRequest(r, args, proof.TerminalGeneration)
	result, err := Publish(r.Endpoint, request)
	if err != nil {
		return result, err
	}
	if result.Outcome == OutcomeRejected {
		if ask := carryAskFromRejectedDetail(result.Detail); ask != nil {
			return result, ask
		}
	}
	return result, nil
}

func doneCarryRefusal(root string, tree *TreeGoals, codeTip, id string, file *GoalFile, now time.Time) error {
	goalOnly := &TreeGoals{Root: tree.Root, Live: map[string]*GoalFile{id: file}, Done: map[string]*GoalFile{}}
	for _, word := range carryWords(goalOnly) {
		if !CarryWordProven(root, word) {
			continue
		}
		consumption, err := CarryConsumptionAt(root, tree, codeTip, word)
		if err != nil {
			return err
		}
		if consumption.Kind == "origin" || (consumption.Kind == "none" && now.Before(word.Expires)) {
			return fmt.Errorf("goal %s has open carry word %s; land it, supersede it, or let it expire; a carried commit without its ledger row must be closed with land.sh --carried %s whether the word is expired or not", id, word.History.Opid, word.History.Opid)
		}
	}
	return nil
}

func carryDebtAskAt(root string, tree *TreeGoals, codeTip, exceptRef string, now time.Time) error {
	debt, found, err := CarryDebtAt(root, tree, codeTip, exceptRef, now)
	if err != nil {
		return err
	}
	if found {
		return carryAsk("carry-debt-unpaid", carryDebtText(debt))
	}
	return nil
}

func carryRequest(r VerbRequest, args CarryArgs, generation uint64) PublishRequest {
	return PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "carry", Targets: []string{args.Goal}, Args: map[string]string{
			"workspace": args.Workspace, "past": args.Past, "why": args.Why,
			"expires": args.Expires.UTC().Format(time.RFC3339), "supersede": args.Supersede,
			"transfer": strconv.FormatBool(args.Transfer), "raiseFormat": strconv.FormatBool(args.RaiseFormat),
			"generation": strconv.FormatUint(generation, 10), "by": r.Actor.Human,
		}}, Message: "goal carry " + args.Goal,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			goalFile := tree.Live[args.Goal]
			if goalFile == nil {
				return nil, carryAsk("carry-goal-not-live", fmt.Sprintf("goal %s is not live", args.Goal))
			}
			if opidLanded(goalFile, r) {
				return nil, AlreadyApplied{}
			}
			if tree.Root.FormatVersion == "1" && !args.RaiseFormat {
				return nil, carryAsk("carry-format-required", "the ledger is at format 1; a carry word needs format 2, which every older engine refuses to read: rebuild and re-arm every seat (`metasystem up` or `steward arm`), then run `goal carry --raise-format`")
			}
			if tree.Root.FormatVersion != "1" && args.RaiseFormat {
				return nil, fmt.Errorf("--raise-format is only valid while the ledger is at format 1")
			}
			codeTip := carryCodeTip(r.Endpoint, tip)
			if err := carryDebtAskAt(r.Endpoint.Root, tree, codeTip, "", r.Now); err != nil {
				return nil, err
			}
			seat, _ := OpidMachine(r.opid())
			open, openErr := openCarryWords(r.Endpoint.Root, tree, codeTip, seat, r.Now)
			if openErr != nil {
				return nil, openErr
			}
			maximum, maxErr := config.CarryOpenMax(filepath.Join(r.Endpoint.Root, "metasystem.conf"))
			if maxErr != nil {
				return nil, maxErr
			}
			if args.Supersede == "" && uint64(len(open)) >= maximum {
				return nil, carryAsk("carry-cap-reached", fmt.Sprintf("carry cap reached by %s; supersede it with --supersede or let it expire", carryWordList(open)))
			}

			changesByGoal := map[string]*GoalFile{args.Goal: goalFile}
			if args.Supersede != "" {
				target, targetFile, targetErr := carrySupersedePrecondition(r, tree, codeTip, args)
				if targetErr != nil {
					return nil, targetErr
				}
				changesByGoal[target.Goal] = targetFile
				touch(targetFile, r, "carried", []string{target.Goal})
				closer := &targetFile.History[len(targetFile.History)-1]
				closer.ApprovedRef = target.History.Opid
				closer.Reason = "superseded by=" + r.opid()
			}
			touch(goalFile, r, "carry", []string{args.Goal})
			row := &goalFile.History[len(goalFile.History)-1]
			row.AuthorityOutcome = AuthorityOutcomeHumanAuthorityProven
			row.AuthorityGeneration = generation
			row.Reason = fmt.Sprintf("carry workspace=%s goal=%s past=%s expires=%s why=%s", args.Workspace, args.Goal, args.Past, args.Expires.UTC().Format(time.RFC3339), strconv.Quote(args.Why))
			if args.Supersede != "" {
				row.Reason += " supersedes=" + args.Supersede
			}
			changes := make([]Change, 0, len(changesByGoal)+1)
			for _, id := range sortedGoalIds(changesByGoal) {
				changes = append(changes, Change{Path: livePath(id), Content: RenderFile(changesByGoal[id])})
			}
			if tree.Root.FormatVersion == "1" {
				tree.Root.FormatVersion = "2"
				tree.Root.Revision++
				tree.Root.History = append(tree.Root.History, HistoryLine{At: r.stamp(), Opid: r.opid(), Verb: "carry", Actor: r.Actor.historyActor(), Targets: []string{args.Goal}, Keep: -1, Reason: "FormatVersion=2"})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(tree.Root)})
			}
			return changes, nil
		}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }}
}

func carrySupersedePrecondition(r VerbRequest, tree *TreeGoals, codeTip string, args CarryArgs) (CarryWord, *GoalFile, error) {
	target, err := findCarryWord(tree, args.Supersede)
	if err != nil {
		return CarryWord{}, nil, fmt.Errorf("supersede target %s is missing", args.Supersede)
	}
	targetSeat, _ := OpidMachine(target.History.Opid)
	if targetSeat != r.Actor.Machine && !args.Transfer {
		return CarryWord{}, nil, fmt.Errorf("supersede target belongs to seat %s; pass --transfer from the human on the new seat", targetSeat)
	}
	if !r.Now.Before(target.Expires) {
		return CarryWord{}, nil, fmt.Errorf("supersede target expired at %s", target.Expires.UTC().Format(time.RFC3339))
	}
	reservation := CarryReservationAt(tree, target.Goal, target.History.Opid, r.Now)
	if reservation.State == "open" {
		return CarryWord{}, nil, fmt.Errorf("in flight on %s since %s: goal carrying --abandon %s on that seat, or wait for %s", targetSeat, reservation.History.At, reservation.History.Opid, target.Expires.UTC().Format(time.RFC3339))
	}
	consumption, err := CarryConsumptionAt(r.Endpoint.Root, tree, codeTip, target)
	if err != nil {
		return CarryWord{}, nil, err
	}
	if consumption.Kind != "none" {
		if consumption.Kind == "origin" {
			return CarryWord{}, nil, fmt.Errorf("consumed on origin by %s at %s; the record is incomplete: rerun land.sh --carried %s", consumption.ID, codeTip, target.History.Opid)
		}
		return CarryWord{}, nil, fmt.Errorf("supersede target consumed by %s", consumption.ID)
	}
	targetFile := tree.Live[target.Goal]
	if targetFile == nil {
		return CarryWord{}, nil, fmt.Errorf("supersede target goal %s is not live", target.Goal)
	}
	return target, targetFile, nil
}

func carryWordList(words []CarryWord) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		parts = append(parts, fmt.Sprintf("%s workspace=%s expires=%s", word.History.Opid, word.Workspace, word.Expires.UTC().Format(time.RFC3339)))
	}
	return strings.Join(parts, ", ")
}

func findCarryWord(tree *TreeGoals, opid string) (CarryWord, error) {
	for _, word := range carryWords(tree) {
		if word.History.Opid == opid {
			return word, nil
		}
	}
	return CarryWord{}, fmt.Errorf("carry word %s is missing", opid)
}

// CarryingArgs drives either the fleet-visible reservation or the local
// carried-intent journal entry written immediately before the code push.
type CarryingArgs struct {
	Goal, ApprovedRef, Carrying, Commit, Project, Workspace string
	Past, Battery, Missing, Failing                         string
	Judge, JudgeTree, JudgeDigest, LiveFailure, Ledger, By  string
	OwnerPID                                                int64
}

func validateCarryReservation(r VerbRequest, tree *TreeGoals, tip string, args CarryingArgs) (CarryWord, error) {
	if tree.Root == nil || tree.Root.FormatVersion == "1" {
		return CarryWord{}, carryAsk("carry-format-required", "the ledger is at format 1; a carried landing needs format 2: rebuild and re-arm every seat, then run goal carry --raise-format")
	}
	file := tree.Live[args.Goal]
	if file == nil {
		return CarryWord{}, carryAsk("carry-goal-not-live", fmt.Sprintf("goal %s is not live", args.Goal))
	}
	if file.State != StateClaimed || !ownPair(file.Claimed, r.Actor) {
		return CarryWord{}, carryAsk("goal-item-not-held", fmt.Sprintf("goal %s is not held by %s+%s; use goal steal", args.Goal, r.Actor.Machine, r.Actor.Lineage))
	}
	word, err := CarryWordAt(tree, args.Goal, args.ApprovedRef)
	if err != nil {
		return CarryWord{}, carryAsk("carry-word-missing", err.Error()+"; fetch the ledger")
	}
	if !CarryWordProven(r.Endpoint.Root, word) {
		return CarryWord{}, carryAsk("carry-word-unproven", fmt.Sprintf("carry word %s has no proven terminal generation", args.ApprovedRef))
	}
	seat, _ := OpidMachine(word.History.Opid)
	if seat != r.Actor.Machine {
		return CarryWord{}, carryAsk("carry-seat-mismatch", fmt.Sprintf("word %s belongs to seat %s; run goal carry --supersede %s --transfer on %s", word.History.Opid, seat, word.History.Opid, r.Actor.Machine))
	}
	if word.Workspace != args.Workspace {
		return CarryWord{}, carryAsk("carry-tree-mismatch", fmt.Sprintf("word workspace=%s candidate workspace=%s; issue goal carry --supersede %s", word.Workspace, args.Workspace, word.History.Opid))
	}
	if !CarryableName(r.Endpoint.Root, word.Past) {
		return CarryWord{}, carryAsk("carry-not-carryable", fmt.Sprintf("%s is not carryable", word.Past))
	}
	if !r.Now.Before(word.Expires) {
		return CarryWord{}, carryAsk("carry-word-expired", fmt.Sprintf("word %s expired at %s; issue a fresh goal carry", word.History.Opid, word.Expires.UTC().Format(time.RFC3339)))
	}
	consumption, err := CarryConsumptionAt(r.Endpoint.Root, tree, carryCodeTip(r.Endpoint, tip), word)
	if err != nil {
		return CarryWord{}, err
	}
	if consumption.Kind != "none" {
		return CarryWord{}, carryAsk("carry-word-consumed", fmt.Sprintf("word %s is consumed at %s:%s", word.History.Opid, consumption.Kind, consumption.ID))
	}
	if debt, found, debtErr := CarryDebtAt(r.Endpoint.Root, tree, carryCodeTip(r.Endpoint, tip), word.History.Opid, r.Now); debtErr != nil {
		return CarryWord{}, debtErr
	} else if found {
		return CarryWord{}, carryAsk("carry-debt-unpaid", carryDebtText(debt))
	}
	open, err := openCarryWords(r.Endpoint.Root, tree, carryCodeTip(r.Endpoint, tip), seat, r.Now)
	if err != nil {
		return CarryWord{}, err
	}
	other := make([]CarryWord, 0, len(open))
	for _, candidate := range open {
		if candidate.History.Opid != word.History.Opid {
			other = append(other, candidate)
		}
	}
	maximum, err := config.CarryOpenMax(filepath.Join(r.Endpoint.Root, "metasystem.conf"))
	if err != nil {
		return CarryWord{}, err
	}
	if uint64(len(other)) >= maximum {
		return CarryWord{}, carryAsk("carry-cap-reached", fmt.Sprintf("other open words reach the cap: %s", carryWordList(other)))
	}
	return word, nil
}

// Carrying publishes a reservation. With Commit set, it writes only the
// local journal intent owned by the wrapper process.
func Carrying(r VerbRequest, args CarryingArgs) (PublishResult, string, error) {
	if r.Endpoint.LocalMode() {
		return PublishResult{}, "", carryRemoteRequired()
	}
	if args.Commit != "" {
		projection, err := Project(r.Endpoint, false, r.Now)
		if err != nil {
			return PublishResult{}, "", err
		}
		reservation := CarryReservationAt(projection.Tree, args.Goal, args.ApprovedRef, r.Now)
		if reservation.State != "open" || reservation.History.Opid != args.Carrying {
			return PublishResult{}, "", carryAsk("carry-debt-unpaid", "reserve first with goal carrying")
		}
		workspace := reasonField(reservation.History.Reason, "workspace")
		if workspace != args.Workspace {
			return PublishResult{}, "", fmt.Errorf("reservation workspace differs: row=%s intent=%s", workspace, args.Workspace)
		}
		intent := carriedIntent(args)
		entryOpid := r.opid()
		entries, entriesErr := Entries(r.Endpoint.Root)
		if entriesErr != nil {
			return PublishResult{}, "", entriesErr
		}
		for _, existing := range entries {
			if existing.Intent.Verb == "carried" && existing.Intent.Args["approvedRef"] == args.ApprovedRef && existing.Phase != PhaseTerminal {
				if OwnerAlive(existing) {
					return PublishResult{}, "", fmt.Errorf("carried intent %s is in flight", existing.Opid)
				}
				if err := MarkTerminal(r.Endpoint.Root, existing.Opid, OutcomeAbandoned, "superseded by rebuilt carried intent"); err != nil {
					return PublishResult{}, "", err
				}
			}
		}
		if _, err := CreateCarryingEntry(r.Endpoint.Root, entryOpid, r.Actor.Machine, r.Actor.Lineage, intent, args.OwnerPID); err != nil {
			return PublishResult{}, "", err
		}
		return PublishResult{Outcome: OutcomeConfirmed, Tip: projection.Tip, Detail: "carrying=" + entryOpid}, entryOpid, nil
	}
	projection, err := Project(r.Endpoint, false, r.Now)
	if err != nil {
		return PublishResult{}, "", err
	}
	if reservation := CarryReservationAt(projection.Tree, args.Goal, args.ApprovedRef, r.Now); reservation.State == "open" {
		seat, _ := OpidMachine(reservation.History.Opid)
		if seat == r.Actor.Machine {
			return PublishResult{Outcome: OutcomeConfirmed, Tip: projection.Tip, Detail: "carrying=" + reservation.History.Opid}, reservation.History.Opid, nil
		}
	}
	// Preserve question-shaped refusals at the command boundary. Publish stores
	// mutation refusals as terminal journal outcomes, so decide once against the
	// projected tip before entering the transaction and rehydrate the same typed
	// outcome if a concurrent ledger move makes the mutation re-decide.
	word, err := validateCarryReservation(r, projection.Tree, projection.Tip, args)
	if err != nil {
		return PublishResult{}, "", err
	}
	if strings.TrimSpace(args.By) == "" {
		args.By = word.History.Actor
	}
	if strings.TrimSpace(args.By) == "" {
		return PublishResult{}, "", fmt.Errorf("goal carrying requires --by or a human actor on the carry word")
	}
	request := carryingRequest(r, args)
	result, err := Publish(r.Endpoint, request)
	if err != nil {
		return result, "", err
	}
	row := r.opid()
	if result.Outcome == OutcomeRejected {
		if ask := carryAskFromRejectedDetail(result.Detail); ask != nil {
			return result, "", ask
		}
		return result, "", nil
	}
	return result, row, nil
}

func carryingRequest(r VerbRequest, args CarryingArgs) PublishRequest {
	return PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "carrying", Targets: []string{args.Goal}, Args: map[string]string{"approvedRef": args.ApprovedRef, "workspace": args.Workspace, "tree": args.Project, "by": args.By}},
		Message: "goal carrying " + args.Goal,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			word, err := validateCarryReservation(r, tree, tip, args)
			if err != nil {
				return nil, err
			}
			if reservation := CarryReservationAt(tree, args.Goal, args.ApprovedRef, r.Now); reservation.State == "open" {
				seat, _ := OpidMachine(reservation.History.Opid)
				if seat == r.Actor.Machine {
					return nil, AlreadyApplied{}
				}
			}
			file := tree.Live[args.Goal]
			touch(file, r, "carrying", []string{args.Goal})
			row := &file.History[len(file.History)-1]
			row.ApprovedRef = args.ApprovedRef
			row.Reason = fmt.Sprintf("open workspace=%s project=%s expires=%s by=%s", args.Workspace, args.Project, word.Expires.UTC().Format(time.RFC3339), args.By)
			return []Change{{Path: livePath(args.Goal), Content: RenderFile(file)}}, nil
		}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }}
}

// AbandonCarrying closes a reservation from its own seat after proving that
// no live wrapper still owns a matching carried intent.
func AbandonCarrying(r VerbRequest, goalID, rowOpid, why string) (PublishResult, error) {
	projection, err := Project(r.Endpoint, false, r.Now)
	if err != nil {
		return PublishResult{}, err
	}
	file := projection.Tree.Live[goalID]
	if file == nil {
		return PublishResult{}, fmt.Errorf("goal %s is not live", goalID)
	}
	var opening HistoryLine
	for _, history := range file.History {
		if history.Opid == rowOpid && history.Verb == "carrying" && strings.HasPrefix(history.Reason, "open ") {
			opening = history
		}
	}
	if opening.Opid == "" {
		return PublishResult{}, fmt.Errorf("carrying row %s is missing", rowOpid)
	}
	seat, _ := OpidMachine(opening.Opid)
	if seat != r.Actor.Machine {
		return PublishResult{}, carryAsk("carry-seat-mismatch", fmt.Sprintf("reservation belongs to seat %s until %s", seat, reasonField(opening.Reason, "expires")))
	}
	entries, err := Entries(r.Endpoint.Root)
	if err != nil {
		return PublishResult{}, err
	}
	for _, entry := range entries {
		if entry.Intent.Verb == "carried" && entry.Intent.Args["approvedRef"] == opening.ApprovedRef && entry.Phase != PhaseTerminal {
			if OwnerAlive(entry) {
				return PublishResult{}, carryAsk("carry-debt-unpaid", fmt.Sprintf("carried intent %s is in flight", entry.Opid))
			}
			_ = MarkTerminal(r.Endpoint.Root, entry.Opid, OutcomeAbandoned, "reservation abandoned")
		}
	}
	return Publish(r.Endpoint, abandonCarryingRequest(r, goalID, opening, rowOpid, why))
}

func abandonCarryingRequest(r VerbRequest, goalID string, opening HistoryLine, rowOpid, why string) PublishRequest {
	return PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Intent: Intent{Verb: "carrying", Targets: []string{goalID}, Args: map[string]string{"abandon": rowOpid}}, Message: "goal carrying abandon " + goalID,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			file := tree.Live[goalID]
			if file == nil {
				return nil, fmt.Errorf("goal %s is not live", goalID)
			}
			reservation := CarryReservationAt(tree, goalID, opening.ApprovedRef, r.Now)
			if reservation.State == "abandoned" || reservation.State == "closed" {
				return nil, AlreadyApplied{}
			}
			if reservation.State != "open" || reservation.History.Opid != rowOpid {
				return nil, fmt.Errorf("reservation %s is %s", rowOpid, reservation.State)
			}
			touch(file, r, "carrying", []string{goalID})
			row := &file.History[len(file.History)-1]
			row.ApprovedRef = opening.ApprovedRef
			row.Reason = "abandoned of=" + rowOpid + " why=" + strconv.Quote(why)
			return []Change{{Path: livePath(goalID), Content: RenderFile(file)}}, nil
		}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }}
}

func reasonField(reason, key string) string {
	for _, field := range strings.Fields(reason) {
		if value, ok := strings.CutPrefix(field, key+"="); ok {
			return value
		}
	}
	return ""
}

// CarriedArgs is the durable fourteen-field landing record plus its
// reservation identity.
type CarriedArgs struct {
	Goal, ApprovedRef, Carrying, Commit, Project, Workspace string
	Past, Battery, Missing, Failing                         string
	Judge, JudgeTree, JudgeDigest, LiveFailure, Ledger, By  string
	Outcome                                                 string
}

func carriedIntent(args CarryingArgs) Intent {
	return Intent{Verb: "carried", Targets: []string{args.Goal}, Args: map[string]string{
		"approvedRef": args.ApprovedRef, "carrying": args.Carrying, "commit": args.Commit,
		"tree": args.Project, "workspace": args.Workspace, "past": args.Past,
		"battery": args.Battery, "missing": listOrDash(args.Missing), "failing": listOrDash(args.Failing),
		"judge": args.Judge, "judgeTree": valueOrDash(args.JudgeTree), "judgeDigest": args.JudgeDigest,
		"liveFailure": valueOrDash(args.LiveFailure), "ledger": args.Ledger, "by": args.By, "outcome": "landed",
	}}
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func listOrDash(value string) string { return valueOrDash(strings.TrimSpace(value)) }

var carriedCounselorAppend = func(string, string, HistoryLine, time.Time) error {
	return fmt.Errorf("the carried counselor writer is not bound")
}

// BindCarriedCounselorAppend connects the upward counselor owner without a
// goal-to-counselor import cycle. It is called during counselor package init.
func BindCarriedCounselorAppend(appendLine func(string, string, HistoryLine, time.Time) error) {
	if appendLine != nil {
		carriedCounselorAppend = appendLine
	}
}

func carriedAfterConfirmed(root, approvedRef string, now time.Time) func(string) error {
	return func(tip string) error {
		tree, err := loadTree(root, tip)
		if err != nil {
			return err
		}
		goalID, row, ok := carriedRow(tree, approvedRef)
		if !ok || !strings.HasPrefix(row.Reason, "landed ") {
			return nil
		}
		return carriedCounselorAppend(root, goalID, row, now)
	}
}

// RepairCarriedCounselor replays only the confirmed carried row's idempotent
// local effect; it runs no ledger transaction.
func RepairCarriedCounselor(endpoint Endpoint, approvedRef string, now time.Time) error {
	projection, err := Project(endpoint, false, now)
	if err != nil {
		return err
	}
	goalID, row, ok := carriedRow(projection.Tree, approvedRef)
	if !ok {
		return fmt.Errorf("carried row for %s is absent", approvedRef)
	}
	return carriedCounselorAppend(endpoint.Root, goalID, row, now)
}

// Carried completes an existing local carried intent under its entry.
func Carried(r VerbRequest, entryOpid string) (PublishResult, error) {
	if r.Endpoint.LocalMode() {
		return PublishResult{}, carryRemoteRequired()
	}
	// The format fence precedes even terminal journal replay. Otherwise a
	// format-1 seat could report a carried record as confirmed without ever
	// consulting the accepted ledger.
	if projection, err := Project(r.Endpoint, false, r.Now); err == nil && (projection.Tree.Root == nil || projection.Tree.Root.FormatVersion == "1") {
		return PublishResult{}, carryAsk("carry-format-required", "the ledger is at format 1; a carried landing needs format 2: rebuild and re-arm every seat, then run goal carry --raise-format")
	}
	if existing, err := ReadEntry(r.Endpoint.Root, entryOpid); err == nil && existing.Phase == PhaseTerminal && existing.Outcome == OutcomeConfirmed {
		return PublishResult{Outcome: OutcomeConfirmed, Detail: "idempotent"}, nil
	}
	entry, err := TakeOverForCompletion(r.Endpoint.Root, entryOpid)
	if err != nil {
		return PublishResult{}, err
	}
	if entry.Intent.Verb != "carried" || entry.Phase != PhaseCreated {
		return PublishResult{}, fmt.Errorf("journal entry %s is not a created carried intent", entryOpid)
	}
	request, err := carriedRequestFromIntent(r.Endpoint, entry)
	if err != nil {
		return PublishResult{}, err
	}
	result, err := CompleteEntry(r.Endpoint, request)
	if err == nil && result.Outcome == OutcomeRejected {
		if ask := carryAskFromRejectedDetail(result.Detail); ask != nil {
			return result, ask
		}
	}
	return result, err
}

// CarriedFromCommit publishes a rebuilt record after the local journal was
// lost. The command layer derives every argument from commit trailers.
func CarriedFromCommit(r VerbRequest, args CarriedArgs) (PublishResult, error) {
	if r.Endpoint.LocalMode() {
		return PublishResult{}, carryRemoteRequired()
	}
	result, err := Publish(r.Endpoint, carriedRequest(r, args))
	if err == nil && result.Outcome == OutcomeRejected {
		if ask := carryAskFromRejectedDetail(result.Detail); ask != nil {
			return result, ask
		}
	}
	return result, err
}

func carriedRequestFromIntent(endpoint Endpoint, entry Entry) (PublishRequest, error) {
	return carriedRequestFromIntentMode(endpoint, entry, false)
}

func carriedRequestFromIntentMode(endpoint Endpoint, entry Entry, recovering bool) (PublishRequest, error) {
	if len(entry.Intent.Targets) != 1 || len(entry.Opid) < 26 {
		return PublishRequest{}, fmt.Errorf("carried intent %s has no unique goal", entry.Opid)
	}
	r := VerbRequest{Endpoint: endpoint, Actor: actorFromEntry(entry), Ulid: entry.Opid[:26], Now: timeNowUTC()}
	args := CarriedArgs{Goal: entry.Intent.Targets[0], ApprovedRef: entry.Intent.Args["approvedRef"], Carrying: entry.Intent.Args["carrying"], Commit: entry.Intent.Args["commit"], Project: entry.Intent.Args["tree"], Workspace: entry.Intent.Args["workspace"], Past: entry.Intent.Args["past"], Battery: entry.Intent.Args["battery"], Missing: entry.Intent.Args["missing"], Failing: entry.Intent.Args["failing"], Judge: entry.Intent.Args["judge"], JudgeTree: entry.Intent.Args["judgeTree"], JudgeDigest: entry.Intent.Args["judgeDigest"], LiveFailure: entry.Intent.Args["liveFailure"], Ledger: entry.Intent.Args["ledger"], By: entry.Intent.Args["by"], Outcome: entry.Intent.Args["outcome"]}
	return carriedRequestMode(r, args, recovering), nil
}

func carriedRequest(r VerbRequest, args CarriedArgs) PublishRequest {
	return carriedRequestMode(r, args, false)
}

func carriedRequestMode(r VerbRequest, args CarriedArgs, recovering bool) PublishRequest {
	intent := Intent{Verb: "carried", Targets: []string{args.Goal}, Args: map[string]string{
		"approvedRef": args.ApprovedRef, "carrying": args.Carrying, "commit": args.Commit,
		"tree": args.Project, "workspace": args.Workspace, "past": args.Past,
		"battery": args.Battery, "missing": listOrDash(args.Missing), "failing": listOrDash(args.Failing),
		"judge": args.Judge, "judgeTree": valueOrDash(args.JudgeTree), "judgeDigest": args.JudgeDigest,
		"liveFailure": valueOrDash(args.LiveFailure), "ledger": args.Ledger, "by": args.By, "outcome": valueOrLanded(args.Outcome),
	}}
	return PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Intent: intent, Message: "goal carried " + args.Goal,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if tree.Root == nil || tree.Root.FormatVersion == "1" {
				return nil, carryAsk("carry-format-required", "the ledger is at format 1; a carried landing needs format 2: rebuild and re-arm every seat, then run goal carry --raise-format")
			}
			if rowGoal, existing, ok := carriedRow(tree, args.ApprovedRef); ok {
				if err := compareCarriedReplay(rowGoal, args.Goal, existing, intent.Args); err != nil {
					return nil, err
				}
				return nil, AlreadyApplied{}
			}
			word, wordErr := CarryWordAt(tree, args.Goal, args.ApprovedRef)
			if wordErr != nil {
				return nil, wordErr
			}
			if !CarryWordProven(r.Endpoint.Root, word) {
				return nil, fmt.Errorf("carry word %s is not proven", args.ApprovedRef)
			}
			if word.Workspace != args.Workspace || word.Past != args.Past {
				return nil, fmt.Errorf("carried record differs from its word: workspace=%s/%s past=%s/%s", args.Workspace, word.Workspace, args.Past, word.Past)
			}
			_, landedCommit, scanErr := carryAnchorAndCommit(r.Endpoint.Root, carryCodeTip(r.Endpoint, tip), args.ApprovedRef)
			if scanErr != nil {
				return nil, scanErr
			}
			if landedCommit != args.Commit {
				if !recovering {
					return nil, fmt.Errorf("the carried commit %s is not on origin", args.Commit)
				}
				file := tree.Live[args.Goal]
				if file == nil {
					return nil, NothingToDo{Reason: fmt.Sprintf("the carried commit %s is not on origin; the word %s stays open", args.Commit, args.ApprovedRef)}
				}
				reservation := CarryReservationAt(tree, args.Goal, args.ApprovedRef, r.Now)
				if reservation.State != "open" || (args.Carrying != "" && reservation.History.Opid != args.Carrying) {
					return nil, NothingToDo{Reason: fmt.Sprintf("the carried commit %s is not on origin; the word %s stays open", args.Commit, args.ApprovedRef)}
				}
				touch(file, r, "carrying", []string{args.Goal})
				closer := &file.History[len(file.History)-1]
				closer.ApprovedRef = args.ApprovedRef
				closer.Reason = fmt.Sprintf("abandoned of=%s why=%s", reservation.History.Opid, strconv.Quote("the carried commit "+args.Commit+" is not on origin"))
				return []Change{{Path: livePath(args.Goal), Content: RenderFile(file)}}, nil
			}
			file := tree.Live[args.Goal]
			if file == nil {
				file = tree.Done[args.Goal]
			}
			if file == nil {
				return nil, fmt.Errorf("goal %s is absent", args.Goal)
			}
			if args.Outcome != "" && args.Outcome != "landed" {
				return nil, fmt.Errorf("carried outcome must be landed")
			}
			reservation := CarryReservationAt(tree, args.Goal, args.ApprovedRef, r.Now)
			if reservation.State == "open" {
				rowWorkspace := reasonField(reservation.History.Reason, "workspace")
				if rowWorkspace != args.Workspace {
					return nil, fmt.Errorf("carrying reservation workspace differs: row=%s intent=%s", rowWorkspace, args.Workspace)
				}
			}
			touch(file, r, "carried", []string{args.Goal})
			row := &file.History[len(file.History)-1]
			row.ApprovedRef = args.ApprovedRef
			row.Reason = renderCarriedReason(intent.Args)
			finding := "carried:" + args.Commit
			if args.Battery == "red" {
				finding += ":battery-red"
			}
			found := false
			for _, obligation := range file.ReviewObligations {
				if obligation.Finding == finding && obligation.Chain == HumanCarriedChain {
					found = true
				}
			}
			if !found {
				file.ReviewObligations = append(file.ReviewObligations, ReviewObligation{Finding: finding, Chain: HumanCarriedChain, Artifact: "commit:" + args.Commit, Test: "pending", State: "open"})
				if file.BudgetExceptions < ^uint16(0) {
					file.BudgetExceptions++
				}
			}
			path := livePath(args.Goal)
			if tree.Done[args.Goal] != nil {
				path = donePath(args.Goal)
			}
			return []Change{{Path: path, Content: RenderFile(file)}}, nil
		}, Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) }, AfterConfirmed: carriedAfterConfirmed(r.Endpoint.Root, args.ApprovedRef, r.Now)}
}

func valueOrLanded(value string) string {
	if value == "" {
		return "landed"
	}
	return value
}

var carriedFieldOrder = []string{"commit", "workspace", "project", "past", "battery", "missing", "failing", "judge", "judgeTree", "judgeDigest", "liveFailure", "ledger", "by"}

func renderCarriedReason(args map[string]string) string {
	var fields []string
	for _, key := range carriedFieldOrder {
		fields = append(fields, key+"="+carriedIntentValue(args, key))
	}
	return args["outcome"] + " " + strings.Join(fields, " ")
}

func parseCarriedReason(reason string) (map[string]string, error) {
	parts := strings.Fields(reason)
	if len(parts) != len(carriedFieldOrder)+1 || (parts[0] != "landed" && parts[0] != "superseded") {
		return nil, fmt.Errorf("carried reason has an invalid field count or outcome")
	}
	values := map[string]string{"outcome": parts[0]}
	if parts[0] == "superseded" {
		return values, nil
	}
	for index, key := range carriedFieldOrder {
		value, ok := strings.CutPrefix(parts[index+1], key+"=")
		if !ok || value == "" {
			return nil, fmt.Errorf("carried reason is missing %s", key)
		}
		values[key] = value
	}
	return values, nil
}

func compareCarriedRow(row HistoryLine, intent map[string]string) error {
	values, err := parseCarriedReason(row.Reason)
	if err != nil {
		return err
	}
	for _, key := range append(carriedFieldOrder, "outcome") {
		given := carriedIntentValue(intent, key)
		if values[key] != given {
			return fmt.Errorf("carried replay refused: %s differs: row=%s intent=%s", key, values[key], given)
		}
	}
	return nil
}

func compareCarriedReplay(rowGoal, intentGoal string, row HistoryLine, intent map[string]string) error {
	if rowGoal != intentGoal {
		return fmt.Errorf("carried replay refused: goal differs: row=%s intent=%s", rowGoal, intentGoal)
	}
	return compareCarriedRow(row, intent)
}

func carriedIntentValue(intent map[string]string, key string) string {
	if key == "project" {
		return intent["tree"]
	}
	return intent[key]
}
