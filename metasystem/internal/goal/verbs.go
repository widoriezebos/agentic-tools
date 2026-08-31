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
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
)

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
	// ClaimEpoch is the authenticated checkout lease generation. Only
	// transitions that create a claimed revision consume it.
	ClaimEpoch int64
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
	return &ClaimRecord{Machine: machine, Lineage: lineage, At: at, Revision: revision}
}

func bindClaim(f *GoalFile, machine, lineage, at string, revision uint64, claimEpoch int64) error {
	if claimEpoch < 1 {
		return fmt.Errorf("claim requires the authenticated lease holder's positive claim epoch")
	}
	f.Claimed = newClaimRecord(machine, lineage, at, revision)
	f.StopCapability = &StopCapability{
		Generation: revision, Revision: revision, Machine: machine, ClaimEpoch: claimEpoch,
	}
	f.StopFence = nil
	// A fresh claim or budget revision cannot inherit authority from an older
	// obligation. The human creates a new immutable binding explicitly.
	f.Obligation = nil
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

func suppliedBudget(values []Budget) (*Budget, error) {
	if len(values) > 1 {
		return nil, fmt.Errorf("a claim accepts one complete budget tuple")
	}
	if len(values) == 0 {
		return nil, nil
	}
	if err := values[0].Validate(); err != nil {
		return nil, fmt.Errorf("invalid budget: %v", err)
	}
	budget := values[0]
	return &budget, nil
}

func budgetForClaim(f *GoalFile, supplied *Budget) (Budget, error) {
	if supplied != nil {
		return *supplied, nil
	}
	if f.Budget == nil {
		return Budget{}, fmt.Errorf("goal %s has no structured budget; supply the complete tuple on goal claim or run goal set-budget first", f.Id)
	}
	if err := f.Budget.Validate(); err != nil {
		return Budget{}, fmt.Errorf("goal %s has an invalid structured budget: %v", f.Id, err)
	}
	return *f.Budget, nil
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
	if r.Actor.Human == "" {
		return args
	}
	if args == nil {
		args = map[string]string{}
	}
	args["by"] = r.Actor.Human
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

// Open adds a queued goal. Goal-free clears in the same commit when
// it was declared.
func Open(r VerbRequest, id, intent, origin, nextStep string, labels ...string) (PublishResult, error) {
	req, err := openRequest(r, id, intent, origin, nextStep, labels)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, req)
}

// openRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func openRequest(r VerbRequest, id, intent, origin, nextStep string, labels []string) (PublishRequest, error) {
	canonical, err := canonicalLabels(labels)
	if err != nil {
		return PublishRequest{}, err
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "open", Targets: []string{id}, Args: intentArgs(r, map[string]string{
			"intent": intent, "origin": origin, "next": nextStep, "labels": strings.Join(canonical, ","),
		})},
		Message: "goal open " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
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
				Id: id, State: StateQueued, Intent: intent, Origin: origin,
				NextStep: nextStep, OpenedAt: r.stamp(), Revision: 0, Labels: canonical,
			}
			touch(f, r, "open", []string{id})
			changes := []Change{{Path: livePath(id), Content: RenderFile(f)}}
			// Opening clears a declared Goal-free in the same commit.
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "open",
					Actor: r.Actor.historyActor(), Targets: []string{id}, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}, nil
}

// Claim takes ownership of a queued goal for the actor's pair.
// Claim is AGENT-ONLY: humans direct agents; no human
// lineage exists, so no human claim row.
func Claim(r VerbRequest, id string, budgets ...Budget) (PublishResult, error) {
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("claim is agent-only: humans direct agents; steal reassigns a standing claim under --by")
	}
	budget, err := suppliedBudget(budgets)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, claimRequest(r, id, budget))
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
			if f.State != StateQueued {
				return nil, fmt.Errorf("goal %s is %s; only a queued goal claims (park and done have their own verbs)", id, f.State)
			}
			if f.Pinned != "" && f.Pinned != r.Actor.Machine {
				return nil, fmt.Errorf("goal %s is pinned to machine %s and this machine is %s; only the pinned machine may claim it (a human re-pins with set-pin)", id, f.Pinned, r.Actor.Machine)
			}
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					return nil, fmt.Errorf("goal %s is blocked by %s, which is not done", id, dep)
				}
			}
			budget, err := budgetForClaim(f, supplied)
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

// SetBudget replaces the whole tuple. On claimed work it also starts the
// claim record for the new revision, so elapsed time and job reservations
// have one unambiguous revision boundary.
func SetBudget(r VerbRequest, id string, budget Budget) (PublishResult, error) {
	if err := budget.Validate(); err != nil {
		return PublishResult{}, fmt.Errorf("invalid budget: %v", err)
	}
	return Publish(r.Endpoint, setBudgetRequest(r, id, budget))
}

func setBudgetRequest(r VerbRequest, id string, budget Budget) PublishRequest {
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
			if f.State == StateClaimed && !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is claimed by %s+%s; changing another's budget is a human act", id, f.Claimed.Machine, f.Claimed.Lineage)
			}
			if f.StopFence != nil {
				return nil, fmt.Errorf("goal %s revision %d is breach-stopped by %s; only goal resume with a fresh complete budget may reopen admission", id, f.StopFence.Revision, f.StopFence.StopID)
			}
			if f.State == StateParked && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is parked; changing a parked goal's budget is a human act", id)
			}
			bound := f.State == StateClaimed && f.Claimed != nil && f.Claimed.Revision > 0
			if f.Budget != nil && *f.Budget == budget && (f.State != StateClaimed || bound) {
				return nil, NothingToDo{Reason: "the complete budget tuple already reads exactly that"}
			}
			displaced := ""
			if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
				displaced = pairMarker(f.Claimed)
			}
			f.Budget = &budget
			touchDisplaced(f, r, "set-budget", []string{id}, displaced)
			if f.State == StateClaimed && f.Claimed != nil {
				claimEpoch := r.ClaimEpoch
				if claimEpoch < 1 && r.Actor.Human != "" && f.StopCapability != nil {
					claimEpoch = f.StopCapability.ClaimEpoch
				}
				if err := bindClaim(f, f.Claimed.Machine, f.Claimed.Lineage, r.stamp(), f.Revision, claimEpoch); err != nil {
					return nil, err
				}
			}
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// SetObligation records the human decision that turns an already claimed,
// budgeted goal into a governed recurrence. It replaces the complete record;
// no field-level mutation can rewrite an earlier obligation revision.
func SetObligation(r VerbRequest, id string, proposed GovernedObligation, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" || proof == nil || !proof.AuthorizesSetObligation(r.Endpoint.Root) {
		return PublishResult{}, fmt.Errorf("set-obligation requires freshly observed enrolled-human authority or a recorded temporary human word")
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
			f := t.Live[id]
			if f == nil || f.State != StateClaimed || f.Claimed == nil || f.Budget == nil {
				return nil, fmt.Errorf("goal %s must be claimed with a complete budget before it can own an obligation", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.StopFence != nil {
				return nil, fmt.Errorf("goal %s is breach-stopped; resume with a fresh tuple before creating another obligation", id)
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
			o.AuthorityOutcome, o.AuthorityReviewBy = "", ""
			if temporaryAuthority {
				o.AuthorityOutcome, o.AuthorityReviewBy = AuthorityOutcomeTemporaryHumanWord, proof.ReviewBy
			}
			if err := validateGovernedObligation(&o, o.Revision, f.Claimed, f.Budget); err != nil {
				return nil, err
			}
			touch(f, r, "set-obligation", []string{id})
			f.Obligation = &o
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Release returns the actor's claimed goal to the queue.
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
			f.State = StateQueued
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
			touchDisplaced(f, r, "done", []string{id}, displaced)
			t.Done[id] = f
			delete(t.Live, id)
			return ackDisplacements(t, r, []Change{
				{Path: livePath(id), Delete: true},
				{Path: donePath(id), Content: RenderFile(f)},
			}), nil
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
			if f.State != StateQueued && f.State != StateClaimed {
				return nil, fmt.Errorf("goal %s is %s; only queued or claimed goals park", id, f.State)
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

// Unpark returns a parked goal to the queue. The park's records
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
			f.State = StateQueued
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
			// The member rejoins its arc under the arc's STANDING state
			//: a claimed arc adopts it under the standing
			// claimant (claimant or human only — an outside agent
			// cannot inject work into someone's claim; the member's
			// blockers must be done); a parked arc is human-only
			// and the member lands parked with the arc's record.
			f.State = StateQueued
			f.Conclude = ""
			if f.Arc != "" {
				var standing *GoalFile
				for _, liveId := range sortedGoalIds(t.Live) {
					if t.Live[liveId].Arc == f.Arc {
						standing = t.Live[liveId]
						break
					}
				}
				switch {
				case standing == nil:
					// The arc has no live members; the reopen re-founds it queued.
				case standing.State == StateClaimed && standing.Claimed != nil:
					if !ownPair(standing.Claimed, r.Actor) && r.Actor.Human == "" {
						return nil, fmt.Errorf("goal %s rejoins arc %s, which %s+%s holds; an outside agent cannot inject work into someone's claim", id, f.Arc, standing.Claimed.Machine, standing.Claimed.Lineage)
					}
					for _, dep := range f.Blocked {
						if depState(t, dep) != StateDone {
							return nil, fmt.Errorf("goal %s rejoins a claimed arc but is blocked by %s, which is not done", id, dep)
						}
					}
					if err := pinRefusal(f, standing.Claimed.Machine, "rejoining the claimed arc"); err != nil {
						return nil, err
					}
					if f.Budget == nil {
						return nil, fmt.Errorf("goal %s has no structured budget; run goal set-budget before it rejoins a claimed arc", f.Id)
					}
					f.State = StateClaimed
					claimEpoch := r.ClaimEpoch
					if claimEpoch < 1 && standing.StopCapability != nil {
						claimEpoch = standing.StopCapability.ClaimEpoch
					}
					if err := bindClaim(f, standing.Claimed.Machine, standing.Claimed.Lineage, r.stamp(), f.Revision+1, claimEpoch); err != nil {
						return nil, err
					}
				case standing.State == StateParked && standing.Parked != nil:
					if r.Actor.Human == "" {
						return nil, fmt.Errorf("goal %s rejoins arc %s, which is parked; reopening into a parked arc is a human act", id, f.Arc)
					}
					f.State = StateParked
					f.Parked = &ParkRecord{By: standing.Parked.By, At: standing.Parked.At, Because: standing.Parked.Because}
				}
			}
			touch(f, r, "reopen", []string{id})
			changes := []Change{
				{Path: archivedPath(t, id), Delete: true},
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
	NextStep *string
	Blocked  *[]string
	Labels   *[]string
}

// Edit applies field deltas to one live goal.
func Edit(r VerbRequest, id string, fields EditFields) (PublishResult, error) {
	req, err := editRequest(r, id, fields)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, req)
}

// editRequest builds the verb's complete transaction request — the
// ONE mutation semantics both the live verb and recovery replay
// run (recovery rebuilds through the real verb paths).
func editRequest(r VerbRequest, id string, fields EditFields) (PublishRequest, error) {
	if fields.Labels != nil {
		canonical, err := canonicalLabels(*fields.Labels)
		if err != nil {
			return PublishRequest{}, err
		}
		fields.Labels = &canonical
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "edit", Targets: []string{id}, Deltas: editDeltas(id, fields), Args: intentArgs(r, nil)},
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
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}, nil
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
				case StateQueued, StateClaimed:
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
			// The claim binds the ARC: stealing any member
			// reassigns every live member the standing pair holds, one
			// transaction, one quota slot — a partial steal would split
			// the arc's ownership. Every touched line carries the
			// displaced marker.
			oldPair := f.Claimed
			members := arcMembers(t, id)
			for _, member := range members {
				if member.StopFence != nil {
					return nil, fmt.Errorf("goal %s is breach-stopped by %s; only goal resume may replace its claim authority", member.Id, member.StopFence.StopID)
				}
				if member.Pinned != "" && member.Pinned != r.Actor.Machine {
					return nil, fmt.Errorf("goal %s is pinned to machine %s and this machine is %s; even a steal honors the pin — clear the pin, steal, then re-pin", member.Id, member.Pinned, r.Actor.Machine)
				}
				if member.Budget == nil {
					return nil, fmt.Errorf("goal %s has no structured budget; run goal set-budget before stealing its claim", member.Id)
				}
			}
			targets := make([]string, 0, len(members))
			for _, m := range members {
				targets = append(targets, m.Id)
			}
			var changes []Change
			for _, m := range members {
				if m.State != StateClaimed || !ownPair(m.Claimed, Actor{Machine: oldPair.Machine, Lineage: oldPair.Lineage}) {
					continue // uniformity is validated; ride over strays defensively
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

// OpenClaim opens a goal already claimed by the actor — one commit,
// the claim guards holding trivially on a goal with no blockers.
func OpenClaim(r VerbRequest, id, intent, origin, nextStep string, budget Budget, labels ...string) (PublishResult, error) {
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("open --claim is agent-only: humans direct agents; bare open leaves the goal queued")
	}
	if err := budget.Validate(); err != nil {
		return PublishResult{}, fmt.Errorf("invalid budget: %v", err)
	}
	req, err := openClaimRequest(r, id, intent, origin, nextStep, budget, labels)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, req)
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
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
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
				Id: id, State: StateClaimed, Intent: intent, Origin: origin,
				NextStep: nextStep, OpenedAt: r.stamp(), Revision: 0,
				Labels: canonical, Budget: &budget,
			}
			touch(f, r, "open-claim", []string{id})
			if err := bindClaim(f, r.Actor.Machine, r.Actor.Lineage, r.stamp(), f.Revision, r.ClaimEpoch); err != nil {
				return nil, err
			}
			changes := []Change{{Path: livePath(id), Content: RenderFile(f)}}
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "open-claim",
					Actor: r.Actor.historyActor(), Targets: []string{id}, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return ackDisplacements(t, r, changes), nil
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

// ClaimArc claims a goal AND its arc's live members as one unit
// under one claimant counting once against the quota. Every
// member's blockers must be done; a standing foreign claim on any
// member loses the whole cascade.
func ClaimArc(r VerbRequest, id string, budgets ...Budget) (PublishResult, error) {
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("claim is agent-only: humans direct agents; steal reassigns a standing claim under --by")
	}
	budget, err := suppliedBudget(budgets)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, claimArcRequest(r, id, budget))
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
				if m.State != StateQueued {
					return nil, fmt.Errorf("arc member %s is %s; the cascade claims queued members only", m.Id, m.State)
				}
				for _, dep := range m.Blocked {
					if depState(t, dep) != StateDone {
						return nil, fmt.Errorf("arc member %s is blocked by %s, which is not done", m.Id, dep)
					}
				}
				if err := pinRefusal(m, r.Actor.Machine, "the arc claim"); err != nil {
					return nil, err
				}
				budget, err := budgetForClaim(m, supplied)
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

// ReleaseArc releases the actor's whole claimed arc as one unit.
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
					return nil, fmt.Errorf("arc member %s is claimed by %s+%s; a foreign release is a human act", m.Id, m.Claimed.Machine, m.Claimed.Lineage)
				}
				displaced := ""
				if !ownPair(m.Claimed, r.Actor) {
					displaced = pairMarker(m.Claimed)
				}
				m.State = StateQueued
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

// ParkArc pauses a whole arc as one unit. Parking a foreign claim
// anywhere in the cascade is a human act, and the design pins ONE
// acknowledgment for the displaced pair across the whole cascade
// : the displaced= field rides every touched history line,
// but the pair is recorded once per claimant, not once per member.
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
			// One acknowledgment per displaced PAIR: collect the
			// foreign claimants once before touching anything.
			displacedPair := ""
			for _, m := range members {
				if m.Origin == OriginHuman && r.Actor.Human == "" {
					return nil, fmt.Errorf("arc member %s was opened by the human; an agent cannot silently remove a standing human reservation (park is a human act here)", m.Id)
				}
				if m.State == StateClaimed && m.Claimed != nil && !ownPair(m.Claimed, r.Actor) {
					if r.Actor.Human == "" {
						return nil, fmt.Errorf("arc member %s is claimed by %s+%s; parking another's claim is a human act", m.Id, m.Claimed.Machine, m.Claimed.Lineage)
					}
					pair := pairMarker(m.Claimed)
					if displacedPair == "" {
						displacedPair = pair
					}
				}
			}
			var changes []Change
			for _, m := range members {
				if opidLanded(m, r) {
					return nil, AlreadyApplied{}
				}
				if m.State == StateParked {
					continue // already parked members ride along
				}
				if m.State != StateQueued && m.State != StateClaimed {
					return nil, fmt.Errorf("arc member %s is %s; only queued or claimed goals park", m.Id, m.State)
				}
				memberDisplaced := ""
				if m.State == StateClaimed && m.Claimed != nil && !ownPair(m.Claimed, r.Actor) {
					memberDisplaced = displacedPair
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

// UnparkArc restores a whole parked arc to the queue as one unit.
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
					return nil, fmt.Errorf("arc member %s was parked by %s; lifting a human's pause is a human act", m.Id, m.Parked.By)
				}
				m.State = StateQueued
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
				f.State = StateQueued
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
// claimed destination auto-claims a queued member with done
// blockers under the STANDING claimant — a stranger refuses, and a
// human injecting into another machine's claim leaves the
// displacement signal.
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
	if pin == "" || pin == "-" {
		return false
	}
	for _, r := range pin {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
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
				f.State = StateQueued
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
			// The destination side: the arc's standing state rules the
			// join (an empty destination founds a queued arc).
			var standing *GoalFile
			for _, liveId := range sortedGoalIds(t.Live) {
				m := t.Live[liveId]
				if m.Arc == arc && m.Id != id {
					standing = m
					break
				}
			}
			switch {
			case standing == nil || standing.State == StateQueued:
				if f.State == StateParked {
					return nil, fmt.Errorf("goal %s is parked; a parked member joins a queued arc after unpark", id)
				}
			case standing.State == StateClaimed && standing.Claimed != nil:
				if sourceWasClaimed {
					// TWO displaced pairs in one move — the source
					// claimant losing a member and the destination
					// claimant gaining one — is the composed-move row
					// the design explicitly refuses:
					// release first, then join.
					return nil, fmt.Errorf("goal %s moves from one claimed arc into another; two claimants cannot trade a member in one move — release it first", id)
				}
				if f.State == StateParked {
					return nil, fmt.Errorf("goal %s is parked; a claimed arc admits queued members with done blockers only", id)
				}
				if !ownPair(standing.Claimed, r.Actor) && r.Actor.Human == "" {
					return nil, fmt.Errorf("arc %s is claimed by %s+%s; a stranger cannot move goals into a claimed arc", arc, standing.Claimed.Machine, standing.Claimed.Lineage)
				}
				for _, dep := range f.Blocked {
					if depState(t, dep) != StateDone {
						return nil, fmt.Errorf("goal %s is blocked by %s, which is not done; it cannot join the claimed arc unclaimed-late", id, dep)
					}
				}
				if !ownPair(standing.Claimed, r.Actor) {
					// A human injection into another machine's claim is
					// displacement-bearing: the runner hears its scope
					// changed.
					displaced = pairMarker(standing.Claimed)
				}
				// The join auto-claims under the standing claimant: the
				// arc stays one unit under one pair.
				if err := pinRefusal(f, standing.Claimed.Machine, "joining the claimed arc"); err != nil {
					return nil, err
				}
				if f.Budget == nil {
					return nil, fmt.Errorf("goal %s has no structured budget; run goal set-budget before it joins a claimed arc", f.Id)
				}
				f.State = StateClaimed
				claimEpoch := r.ClaimEpoch
				if claimEpoch < 1 && standing.StopCapability != nil {
					claimEpoch = standing.StopCapability.ClaimEpoch
				}
				if err := bindClaim(f, standing.Claimed.Machine, standing.Claimed.Lineage, r.stamp(), f.Revision+1, claimEpoch); err != nil {
					return nil, err
				}
			case standing.State == StateParked && standing.Parked != nil:
				if r.Actor.Human == "" {
					return nil, fmt.Errorf("arc %s is parked; a parked arc's membership edits are human acts", arc)
				}
				// A queued or parked incoming member parks with the
				// arc's record; the released-on-detach path lands here
				// queued and parks the same way.
				f.State = StateParked
				f.Parked = &ParkRecord{By: standing.Parked.By, At: standing.Parked.At, Because: standing.Parked.Because}
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
