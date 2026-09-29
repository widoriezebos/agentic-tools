package batch

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Proof is the durable launch decision and the evidence returned by the one
// tip proof. Planned is written before the charged runner can start.
type Proof struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
	Tree   string `json:"tree"`
	Window string `json:"window"`
	// Runner is where the proof ran, stamped by the owner when the run it
	// dispatched completes: the lane's measured proof cost is per runner
	// (D14, R22).
	Runner string `json:"runner,omitempty"`
	// Reason is the one line saying why the batch started (D14, R23).
	Reason          string              `json:"reason,omitempty"`
	RequiredMode    testpolicy.Mode     `json:"requiredMode"`
	ExecutedMode    testpolicy.Mode     `json:"executedMode"`
	SelectedGroups  []string            `json:"selectedGroups"`
	CandidateTip    string              `json:"candidateTip,omitempty"`
	AttemptID       string              `json:"attemptId,omitempty"`
	BaseCommit      string              `json:"baseCommit,omitempty"`
	BaseTree        string              `json:"baseTree,omitempty"`
	Sample          proofrun.LoadSample `json:"sample"`
	Launchers       int                 `json:"launchers"`
	Executions      []string            `json:"executions,omitempty"`
	CachedPasses    []string            `json:"cachedPasses,omitempty"`
	Passed          []string            `json:"passed,omitempty"`
	Reuse           map[string]string   `json:"reuse,omitempty"`
	GroupIdentities map[string]string   `json:"groupIdentities,omitempty"`
	InputManifests  map[string][]string `json:"inputManifests,omitempty"`
	RedGroups       []RedGroup          `json:"redGroups,omitempty"`
	PrefixGoal      string              `json:"prefixGoal,omitempty"`
	Failure         string              `json:"failure,omitempty"`
	// Sources is the retained verifier's answer per selected group on the tip
	// (D4). Flakes are the known flakes a composed proof landed on, rechecked
	// immediately before publication (BL3S-01).
	Sources map[string]Source `json:"sources,omitempty"`
	Flakes  []FlakeUse        `json:"flakes,omitempty"`
	// SourcesUnresolved says why an ordinary green proof's Sources were not
	// recorded; the proof still lands, since nothing reads Sources before U5.
	SourcesUnresolved string `json:"sourcesUnresolved,omitempty"`
}

// The kinds of a group's source: executed in the tip attempt, reused by
// identity from another retained attempt, or covered by other tip groups.
const SourceExecuted, SourceReused, SourceCovered = "executed", "reused", "covered"

// Source names the attempt that holds one group's pass.
type Source struct {
	Kind    string `json:"kind"`
	Attempt string `json:"attempt"`
}

// FlakeUse is one known-flake entry a composed proof relied on.
type FlakeUse struct {
	Identity       string    `json:"identity"`
	EntryID        string    `json:"entryId"`
	AllowanceUntil time.Time `json:"allowanceUntil"`
}

// ResolveSources projects the retained verifier's per-group attempt onto
// Sources. A selected group without a source, or a red group of the tip
// attempt cited from that attempt, does not resolve.
func ResolveSources(proof Proof, resolved map[string]string) (map[string]Source, error) {
	sources := map[string]Source{}
	for _, group := range proof.SelectedGroups {
		attempt := resolved[group]
		red := slices.ContainsFunc(proof.RedGroups, func(red RedGroup) bool { return red.ID == group })
		switch {
		case attempt == "" || attempt == proof.AttemptID && red:
			return nil, fmt.Errorf("group %s does not resolve to a passing attempt through the retained verifier on tip %s", group, proof.Tree)
		case attempt != proof.AttemptID:
			sources[group] = Source{Kind: SourceReused, Attempt: attempt}
		case slices.Contains(proof.Executions, group):
			sources[group] = Source{Kind: SourceExecuted, Attempt: attempt}
		default:
			sources[group] = Source{Kind: SourceCovered, Attempt: attempt}
		}
	}
	return sources, nil
}

// RecordSources fills an ordinary green proof's Sources through the retained
// verifier, best effort: a verifier error or a group that does not resolve
// (another batch's fresh base run can block reuse or leave a newer failed
// observation) is recorded as the reason and the proof still lands. Only the
// composed-proof path, which lands on the classification attempt, returns
// every member when a group does not resolve.
func RecordSources(store Store, id, actor string, at time.Time, resolve func(Record) (map[string]string, error)) error {
	record, err := store.Load(id)
	if err != nil || record.State != StateLanding || record.Proof == nil || record.Proof.Status != "green" || record.Proof.Sources != nil {
		return err
	}
	var sources map[string]Source
	resolved, unresolved := resolve(record)
	if unresolved == nil {
		sources, unresolved = ResolveSources(*record.Proof, resolved)
	}
	return store.Update(id, func(current *Record) error {
		if current.Proof == nil || current.Proof.AttemptID != record.Proof.AttemptID {
			return fmt.Errorf("BATCH_PROOF_INPUT_MOVED: batch changed before its sources were recorded")
		}
		current.Proof.Sources, current.Proof.SourcesUnresolved = sources, ""
		if unresolved != nil {
			current.Proof.SourcesUnresolved = unresolved.Error()
		}
		return nil
	})
}

func returnEveryMember(store Store, record Record, actor string, at time.Time, reason string) error {
	var decisions []ReturnDecision
	for _, unit := range joinedUnits(record.Units) {
		decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: reason})
	}
	return ReassembleSurvivorsWithReturns(store, record.BatchID, actor, at, decisions)
}

// flakesStillCarried rechecks every known flake a composed proof used
// against the register at now: closed, reclassified or expired is refused.
func flakesStillCarried(uses []FlakeUse, open []OpenEntry, now time.Time) error {
	for _, use := range uses {
		index := slices.IndexFunc(open, func(entry OpenEntry) bool { return entry.ID == use.EntryID && entry.Identity == use.Identity })
		switch {
		case index < 0 || open[index].Class != ClassKnownFlake:
			return fmt.Errorf("known flake %s is no longer an open known flake", use.EntryID)
		case !open[index].CarriesLanding(now):
			return fmt.Errorf("known flake %s's landing allowance expired at %s; it blocks every batch until fixed at its cause",
				use.EntryID, open[index].AllowanceUntil.UTC().Format(time.RFC3339))
		}
	}
	return nil
}

// RequireProofPlan closes membership and proves that the selected tip plan
// covers every group accumulated from member plans. The owner's minted token
// binds the plan: only a completion carrying it may finish or refuse it.
func RequireProofPlan(store Store, id, actor, window, token string, sample proofrun.LoadSample, plan testpolicy.Plan, at time.Time) (Record, error) {
	if token == "" {
		return Record{}, fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s proof plan has no token", id)
	}
	record, err := store.Load(id)
	if err != nil {
		return Record{}, err
	}
	if record.State != StateSealed || len(record.Units) == 0 {
		return Record{}, fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s is not a non-empty sealed batch", id)
	}
	selected := slices.Clone(plan.SelectedGroups)
	slices.Sort(selected)
	selected = slices.Compact(selected)
	for _, required := range record.SelectedGroups {
		if _, present := slices.BinarySearch(selected, required); !present {
			return Record{}, fmt.Errorf("BATCH_PROOF_UNION_UNCOVERED: selected tip plan omits %s", required)
		}
	}
	err = store.Update(id, func(current *Record) error {
		if current.State != StateSealed || current.TipTree != record.TipTree || !slices.Equal(current.SelectedGroups, record.SelectedGroups) {
			return fmt.Errorf("BATCH_PROOF_INPUT_MOVED: batch changed before proof admission")
		}
		current.ClosedReason = "proof-admitted"
		candidateTip := ""
		if current.Landing != nil {
			candidateTip = current.Landing.candidateTip()
		}
		current.Proof = &Proof{Status: "planned", Token: token, Tree: current.TipTree, CandidateTip: candidateTip, Window: window, RequiredMode: plan.RequiredMode,
			ExecutedMode: plan.ExecutedMode, SelectedGroups: selected, Sample: sample, Launchers: sample.OverlappingHost,
			Reason: current.StartReason}
		current.Transition(StateProving, at, "prove", actor, "planned")
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return store.Load(id)
}

// RecordUnionRefusal makes an uncovered sealed union terminal for its current
// tree. A new tree clears this decision during survivor reassembly.
func RecordUnionRefusal(store Store, id, actor, reason string, at time.Time) error {
	return store.Update(id, func(record *Record) error {
		if record.State != StateSealed {
			return fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s is not sealed", id)
		}
		record.Proof = &Proof{Status: "union-uncovered", Tree: record.TipTree, Failure: reason}
		record.Transition(StateSealed, at, "prove-refused", actor, reason)
		return nil
	})
}

// RefuseProofAdmission records a pre-run refusal without scheduling red
// diagnosis. Retryable capacity refusals return to the sealed admission edge.
func RefuseProofAdmission(store Store, id, actor, token, status, reason string, at time.Time) error {
	return store.Update(id, func(record *Record) error {
		if err := staleCompletion(*record, token); err != nil {
			return err
		}
		if record.State != StateProving || record.Proof == nil || record.Proof.Status != "planned" {
			return fmt.Errorf("BATCH_PROOF_NOT_ADMITTED: batch %s has no planned proof", id)
		}
		record.Proof.Status, record.Proof.Failure = status, reason
		record.Transition(StateSealed, at, "prove-refused", actor, reason)
		return nil
	})
}

// FinishProof attaches the exact execution and reuse evidence to the planned
// attempt. A failed launch remains durable and is not silently retried.
func FinishProof(store Store, id, actor, token string, result proofrun.TestResult, launchErr error, at time.Time) error {
	return store.Update(id, func(record *Record) error {
		if err := staleCompletion(*record, token); err != nil {
			return err
		}
		if record.State != StateProving || record.Proof == nil || record.Proof.Status != "planned" {
			return fmt.Errorf("BATCH_PROOF_NOT_ADMITTED: batch %s has no planned proof", id)
		}
		record.Proof.AttemptID = result.AttemptID
		record.Proof.BaseCommit = result.BaseCommit
		record.Proof.BaseTree = result.CandidateTree
		record.Proof.Launchers = result.LaunchCounts.Test + result.LaunchCounts.Build + result.LaunchCounts.Other
		for _, group := range result.Groups {
			if len(group.InputManifest) != 0 {
				if record.Proof.InputManifests == nil {
					record.Proof.InputManifests = map[string][]string{}
				}
				record.Proof.InputManifests[group.ID] = slices.Clone(group.InputManifest)
			}
			if group.ExecutionIdentity != "" {
				if record.Proof.GroupIdentities == nil {
					record.Proof.GroupIdentities = map[string]string{}
				}
				record.Proof.GroupIdentities[group.ID] = group.ExecutionIdentity
			}
			if group.NativeLaunched && group.PassedByGoTestCache() {
				record.Proof.CachedPasses = append(record.Proof.CachedPasses, group.ID)
				record.Proof.Passed = append(record.Proof.Passed, group.ID)
			} else if group.NativeLaunched {
				record.Proof.Executions = append(record.Proof.Executions, group.ID)
				if group.Status == "passed" {
					record.Proof.Passed = append(record.Proof.Passed, group.ID)
				}
			} else if proofrun.CoveredTestPass(result, group) {
				// Other groups of this proof ran these tests natively; the
				// group passed here without a launch of its own.
				record.Proof.Passed = append(record.Proof.Passed, group.ID)
			}
			if group.ReuseAttempt != "" {
				if record.Proof.Reuse == nil {
					record.Proof.Reuse = map[string]string{}
				}
				record.Proof.Reuse[group.ID] = group.ReuseAttempt
			}
			if group.Status != "passed" && group.Status != "reused" {
				record.Proof.RedGroups = append(record.Proof.RedGroups, RedGroupFromResult(group))
			}
		}
		if launchErr != nil {
			record.Proof.Status, record.Proof.Failure = "failed", launchErr.Error()
			record.Transition(StateDiagnosing, at, "prove", actor, "red")
		} else if !result.Delivery.Sufficient {
			record.Proof.Status, record.Proof.Failure = "failed", "delivery evidence is insufficient"
			record.Transition(StateDiagnosing, at, "prove", actor, "red")
		} else {
			record.Proof.Status = "green"
			record.Transition(StateLanding, at, "prove", actor, "green")
		}
		return nil
	})
}

// staleCompletion refuses a completion whose plan was cleared or replaced,
// as a reopen or a survivor reassembly does, while its run was in flight.
func staleCompletion(record Record, token string) error {
	if record.Proof == nil || record.Proof.Token != token {
		return fmt.Errorf("BATCH_PROOF_STALE_COMPLETION: batch %s proof plan %q is not the one this completion ran", record.BatchID, token)
	}
	return nil
}

// WithdrawBudgetMember returns the authority member when P2 cannot preserve
// enough budget for both the tip proof and its mandatory diagnostic.
func WithdrawBudgetMember(store Store, id, goalID, actor, reason string, at time.Time) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateProving || record.Proof == nil || record.Proof.Status != "planned" {
		return fmt.Errorf("budget withdrawal requires an admitted proof")
	}
	return ReassembleSurvivorsWithReturns(store, id, actor, at, []ReturnDecision{{GoalID: goalID, Outcome: UnitWithdrawnBudget, Reason: reason}})
}

// ReassembleSurvivors derives a new candidate from the still-joined units in
// original join order. Selection and seal data are intentionally discarded;
// the new tree must cross seal and proof admission again.
func ReassembleSurvivors(store Store, id, actor string, at time.Time) error {
	return ReassembleSurvivorsWithReturns(store, id, actor, at, nil)
}

// ReturnDecision is a member outcome established before survivor composition.
// Its return request and the resulting candidate are recorded in one update.
type ReturnDecision struct {
	GoalID, Outcome, Reason string
}

func ReassembleSurvivorsWithReturns(store Store, id, actor string, at time.Time, decisions []ReturnDecision) error {
	return reassembleSurvivorsOnBase(store, id, actor, at, decisions, "", "", "")
}

// reassembleSurvivorsOnBase is the one owner for both returned-member and
// moved-base composition. A moved base can reveal the first typed conflict;
// subsequent conflicts are closed in original join order.
func reassembleSurvivorsOnBase(store Store, id, actor string, at time.Time, decisions []ReturnDecision, newBaseTree, detail, landedBy string) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	original := record
	original.Units, original.History = slices.Clone(record.Units), slices.Clone(record.History)
	movedBase := newBaseTree != ""
	if movedBase {
		record.BaseTree = newBaseTree
	}
	decisions = slices.Clone(decisions)
	for _, decision := range decisions {
		if err := requestUnitReturn(&record, decision.GoalID, decision.Outcome, decision.Reason, actor, at); err != nil {
			return err
		}
	}
	// A change stacked on one that leaves leaves with it (U11b B-1b).
	stacked, err := stackedReturns(&record, actor, at)
	if err != nil {
		return err
	}
	decisions = append(decisions, stacked...)
	knownRemoved := func() string {
		ids := make([]string, 0, len(record.Units))
		for _, unit := range record.Units {
			if unit.State == UnitReturnPending || terminalUnitState(unit.State) {
				ids = append(ids, unit.GoalID)
			}
		}
		return strings.Join(ids, ",")
	}
	hold := func(reason string) error {
		if len(decisions) == 0 && knownRemoved() == "" {
			return errors.New(reason)
		}
		next := "inspect ordered member composition before returning the named member"
		return store.Update(id, func(current *Record) error {
			if !reflect.DeepEqual(*current, original) {
				return fmt.Errorf("BATCH_REASSEMBLE_MOVED: batch changed before held decision")
			}
			if current.Proof == nil {
				current.Proof = &Proof{}
			}
			current.Proof.Status, current.Proof.Failure = "held-unclassified", reason+"; "+next
			current.Transition(StateHeldUnclassified, at, "reassemble", actor, reason+"; "+next)
			return nil
		})
	}
	var survivors []Unit
	var prefixes []string
	for tries := 0; tries <= len(record.Units); tries++ {
		survivors = slices.DeleteFunc(slices.Clone(record.Units), func(unit Unit) bool { return unit.State != UnitJoined })
		if len(survivors) == 0 {
			break
		}
		prefixes, err = store.reassembly.assemble(record.BaseTree, survivors)
		if err == nil {
			break
		}
		var conflict *assemblyConflict
		if !errors.As(err, &conflict) || (knownRemoved() == "" && !movedBase) {
			return hold("survivor composition unclassified: " + err.Error())
		}
		found := false
		for _, unit := range survivors {
			if unit.GoalID == conflict.GoalID {
				found = true
			}
		}
		if !found {
			return hold("survivor composition has no joined owner: " + err.Error())
		}
		reason := "cannot apply after returning " + knownRemoved() + ": " + err.Error()
		if knownRemoved() == "" {
			reason = movedBaseConflictLine(conflict, landedBy)
		}
		decision := ReturnDecision{GoalID: conflict.GoalID, Outcome: UnitEjected, Reason: reason}
		if err := requestUnitReturn(&record, decision.GoalID, decision.Outcome, decision.Reason, actor, at); err != nil {
			return hold("survivor return unavailable: " + err.Error())
		}
		decisions = append(decisions, decision)
		stacked, err := stackedReturns(&record, actor, at)
		if err != nil {
			return hold("survivor return unavailable: " + err.Error())
		}
		decisions = append(decisions, stacked...)
	}
	if len(survivors) != 0 && err != nil {
		return hold("survivor composition exhausted bounded closure: " + err.Error())
	}
	applyReturns := func(current *Record) error {
		if !reflect.DeepEqual(*current, original) {
			return fmt.Errorf("BATCH_REASSEMBLE_MOVED: batch changed before survivor decision")
		}
		for _, decision := range decisions {
			if err := requestUnitReturn(current, decision.GoalID, decision.Outcome, decision.Reason, actor, at); err != nil {
				return err
			}
		}
		return nil
	}
	// Prepare the complete record before touching the private ref. The store
	// lock then covers the full-record comparison, ref lease, and durable write.
	next := original
	next.Units, next.History = slices.Clone(original.Units), slices.Clone(original.History)
	if err := applyReturns(&next); err != nil {
		return err
	}
	if movedBase {
		forgetEarly(&next, at, actor, "base moved to "+newBaseTree)
	}
	next.BaseTree = record.BaseTree
	next.PrefixTrees, next.SelectedGroups, next.Seal, next.Proof, next.Landing, next.Receipts, next.CostForecast = prefixes, nil, nil, nil, nil, nil, nil
	if len(survivors) == 0 {
		next.PrefixTrees = nil
		next.TipTree = next.BaseTree
		next.Transition(StateDissolved, at, "reassemble", actor, "no survivors")
	} else {
		next.TipTree = prefixes[len(prefixes)-1]
		next.ClosedReason = ""
		verb, transitionDetail := "reassemble", "survivors"
		if movedBase && len(decisions) == 0 {
			verb, transitionDetail = "trunk-moved", detail
		}
		next.Transition(StateOpen, at, verb, actor, transitionDetail)
	}
	branchFailure := ""
	err = store.locked(func() error {
		current, err := store.Load(id)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, original) {
			return fmt.Errorf("BATCH_REASSEMBLE_MOVED: batch changed before survivor decision")
		}
		if current.Landing != nil && current.Landing.publishedTip() != "" {
			if len(survivors) == 0 || !slices.ContainsFunc(survivors, func(unit Unit) bool { return len(unit.Builds) != 0 }) {
				if err := store.reassembly.delete(id, current.Landing.publishedTip()); err != nil {
					branchFailure = "survivor branch cleanup unavailable: " + err.Error()
					return err
				}
			} else {
				branchTip, err := store.reassembly.rebuild(id, next.BaseTree, current.Landing.publishedTip(), actor, survivors)
				if err != nil {
					branchFailure = "survivor branch unavailable: " + err.Error()
					return err
				}
				next.Landing = &LandingProgress{Base: next.BaseTree, BranchTip: branchTip, CandidateTip: branchTip}
			}
		}
		return store.updateLocked(id, func(current *Record) error {
			if !reflect.DeepEqual(*current, original) {
				return fmt.Errorf("BATCH_REASSEMBLE_MOVED: batch changed before survivor publication")
			}
			*current = next
			return nil
		})
	})
	if branchFailure != "" {
		return hold(branchFailure)
	}
	return err
}

// HoldUnclassified preserves a post-P2 diagnostic refusal without assigning
// blame to a member or allowing another automatic attempt.
func HoldUnclassified(store Store, id, actor, status, nextEvidence string, at time.Time) error {
	if status == "" || nextEvidence == "" {
		return fmt.Errorf("held-unclassified requires status and goal Next evidence")
	}
	return store.Update(id, func(record *Record) error {
		if record.State != StateDiagnosing || record.Proof == nil {
			return fmt.Errorf("held-unclassified requires a diagnosing batch with a recorded proof")
		}
		record.Proof.Status, record.Proof.Failure = "held-unclassified", status+"; "+nextEvidence
		record.Transition(StateHeldUnclassified, at, "diagnose", actor, status+"; "+nextEvidence)
		return nil
	})
}

// stackedReturns returns every live change stacked (transitively, by its
// parent commit) on a change that is leaving or has left the batch, with the
// parent's reason, so reassembly never keeps a child without its parent
// (U11b B-1b).
func stackedReturns(record *Record, actor string, at time.Time) ([]ReturnDecision, error) {
	var decisions []ReturnDecision
	for changed := true; changed; {
		changed = false
		for _, child := range record.Units {
			if !child.IsChange() || (child.State != UnitJoining && child.State != UnitJoined) {
				continue
			}
			for _, parent := range record.Units {
				if !parent.IsChange() || parent.Change.Commit != child.Change.Parent || (parent.State != UnitReturnPending && !terminalUnitState(parent.State)) || parent.State == UnitLanded {
					continue
				}
				decision := ReturnDecision{GoalID: child.GoalID, Outcome: UnitEjected,
					Reason: "its parent change " + parent.GoalID + " left the batch: " + parent.Failure}
				if err := requestUnitReturn(record, decision.GoalID, decision.Outcome, decision.Reason, actor, at); err != nil {
					return nil, err
				}
				decisions, changed = append(decisions, decision), true
				break
			}
		}
	}
	return decisions, nil
}
