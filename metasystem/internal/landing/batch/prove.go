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
				return fmt.Errorf("%s: batch changed before held decision", codeReassembleMoved)
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
			return fmt.Errorf("%s: batch changed before survivor decision", codeReassembleMoved)
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
			return fmt.Errorf("%s: batch changed before survivor decision", codeReassembleMoved)
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
				return fmt.Errorf("%s: batch changed before survivor publication", codeReassembleMoved)
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
