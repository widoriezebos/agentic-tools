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
	Status          string              `json:"status"`
	Tree            string              `json:"tree"`
	Window          string              `json:"window"`
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
	Passed          []string            `json:"passed,omitempty"`
	Reuse           map[string]string   `json:"reuse,omitempty"`
	GroupIdentities map[string]string   `json:"groupIdentities,omitempty"`
	InputManifests  map[string][]string `json:"inputManifests,omitempty"`
	RedGroups       []RedGroup          `json:"redGroups,omitempty"`
	PrefixGoal      string              `json:"prefixGoal,omitempty"`
	Failure         string              `json:"failure,omitempty"`
}

// RequireProofPlan closes membership and proves that the selected tip plan
// covers every group accumulated from member plans.
func RequireProofPlan(store Store, id, actor, window string, sample proofrun.LoadSample, plan testpolicy.Plan, at time.Time) (Record, error) {
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
		current.Proof = &Proof{Status: "planned", Tree: current.TipTree, CandidateTip: candidateTip, Window: window, RequiredMode: plan.RequiredMode,
			ExecutedMode: plan.ExecutedMode, SelectedGroups: selected, Sample: sample, Launchers: sample.OverlappingHost}
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
func RefuseProofAdmission(store Store, id, actor, status, reason string, at time.Time) error {
	return store.Update(id, func(record *Record) error {
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
func FinishProof(store Store, id, actor string, result proofrun.TestResult, launchErr error, at time.Time) error {
	return store.Update(id, func(record *Record) error {
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
			if group.NativeLaunched {
				record.Proof.Executions = append(record.Proof.Executions, group.ID)
				if group.Status == "passed" {
					record.Proof.Passed = append(record.Proof.Passed, group.ID)
				}
			}
			if group.ReuseAttempt != "" {
				if record.Proof.Reuse == nil {
					record.Proof.Reuse = map[string]string{}
				}
				record.Proof.Reuse[group.ID] = group.ReuseAttempt
			}
			if group.Status != "passed" && group.Status != "reused" {
				red := RedGroup{ID: group.ID, Status: group.Status, NotRunReason: group.NotRunReason, LogPath: group.LogPath,
					LogDigest: group.LogDigest, InputManifest: slices.Clone(group.InputManifest)}
				for _, observed := range group.Observed {
					if observed.Status == "failed" {
						red.Failures = append(red.Failures, Failure{Report: observed.Report, Classname: observed.Classname, Name: observed.Name, Status: observed.Status, Reason: observed.Reason})
					}
				}
				record.Proof.RedGroups = append(record.Proof.RedGroups, red)
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
	return reassembleSurvivorsOnBase(store, id, actor, at, decisions, "", "")
}

// reassembleSurvivorsOnBase is the one owner for both returned-member and
// moved-base composition. A moved base can reveal the first typed conflict;
// subsequent conflicts are closed in original join order.
func reassembleSurvivorsOnBase(store Store, id, actor string, at time.Time, decisions []ReturnDecision, newBaseTree, detail string) error {
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
			reason = "cannot apply on moved base: " + err.Error()
		}
		decision := ReturnDecision{GoalID: conflict.GoalID, Outcome: UnitEjected, Reason: reason}
		if err := requestUnitReturn(&record, decision.GoalID, decision.Outcome, decision.Reason, actor, at); err != nil {
			return hold("survivor return unavailable: " + err.Error())
		}
		decisions = append(decisions, decision)
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
