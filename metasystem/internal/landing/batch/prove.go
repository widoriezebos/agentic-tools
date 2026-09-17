package batch

import (
	"fmt"
	"slices"
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
	AttemptID       string              `json:"attemptId,omitempty"`
	BaseCommit      string              `json:"baseCommit,omitempty"`
	BaseTree        string              `json:"baseTree,omitempty"`
	Sample          proofrun.LoadSample `json:"sample"`
	Launchers       int                 `json:"launchers"`
	Executions      []string            `json:"executions,omitempty"`
	Reuse           map[string]string   `json:"reuse,omitempty"`
	GroupIdentities map[string]string   `json:"groupIdentities,omitempty"`
	RedGroups       []RedGroup          `json:"redGroups,omitempty"`
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
		current.Proof = &Proof{Status: "planned", Tree: current.TipTree, Window: window, RequiredMode: plan.RequiredMode,
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
			if group.ExecutionIdentity != "" {
				if record.Proof.GroupIdentities == nil {
					record.Proof.GroupIdentities = map[string]string{}
				}
				record.Proof.GroupIdentities[group.ID] = group.ExecutionIdentity
			}
			if group.NativeLaunched {
				record.Proof.Executions = append(record.Proof.Executions, group.ID)
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
	if err := store.Update(id, func(record *Record) error {
		if record.State != StateProving || record.Proof == nil || record.Proof.Status != "planned" {
			return fmt.Errorf("budget withdrawal requires an admitted proof")
		}
		if err := requestUnitReturn(record, goalID, UnitWithdrawnBudget, reason, actor, at); err != nil {
			return err
		}
		record.Proof.Status, record.Proof.Failure = "budget-refused", reason
		record.Transition(StateSealed, at, "prove", actor, "budget-refused "+goalID)
		return nil
	}); err != nil {
		return err
	}
	return ReassembleSurvivors(store, id, actor, at)
}

// ReassembleSurvivors derives a new candidate from the still-joined units in
// original join order. Selection and seal data are intentionally discarded;
// the new tree must cross seal and proof admission again.
func ReassembleSurvivors(store Store, id, actor string, at time.Time) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	survivors := slices.DeleteFunc(slices.Clone(record.Units), func(unit Unit) bool { return unit.State != UnitJoined })
	if len(survivors) == 0 {
		return store.Update(id, func(current *Record) error {
			current.PrefixTrees, current.SelectedGroups, current.Seal, current.Proof, current.Landing, current.Receipts = nil, nil, nil, nil, nil, nil
			current.TipTree = current.BaseTree
			current.Transition(StateDissolved, at, "reassemble", actor, "no survivors")
			return nil
		})
	}
	prefixes, err := assembleUnits(store.root, record.BaseTree, survivors)
	if err != nil {
		return err
	}
	return store.Update(id, func(current *Record) error {
		current.PrefixTrees, current.SelectedGroups, current.Seal, current.Proof, current.Landing, current.Receipts = prefixes, nil, nil, nil, nil, nil
		current.TipTree = prefixes[len(prefixes)-1]
		current.ClosedReason = ""
		current.Transition(StateOpen, at, "reassemble", actor, "survivors")
		return nil
	})
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
