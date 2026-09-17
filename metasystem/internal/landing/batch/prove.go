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
	Status         string              `json:"status"`
	Window         string              `json:"window"`
	RequiredMode   testpolicy.Mode     `json:"requiredMode"`
	ExecutedMode   testpolicy.Mode     `json:"executedMode"`
	SelectedGroups []string            `json:"selectedGroups"`
	AttemptID      string              `json:"attemptId,omitempty"`
	Sample         proofrun.LoadSample `json:"sample"`
	Launchers      int                 `json:"launchers"`
	Executions     []string            `json:"executions,omitempty"`
	Reuse          map[string]string   `json:"reuse,omitempty"`
	Failure        string              `json:"failure,omitempty"`
}

// RequireProofPlan closes membership and proves that the selected tip plan
// covers every group accumulated from member plans.
func RequireProofPlan(store Store, id, actor, window string, sample proofrun.LoadSample, plan testpolicy.Plan, at time.Time) (Record, error) {
	record, err := store.Load(id)
	if err != nil {
		return Record{}, err
	}
	if record.State != StateOpen || len(record.Units) == 0 {
		return Record{}, fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s is not a non-empty open batch", id)
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
		if current.State != StateOpen || current.TipTree != record.TipTree || !slices.Equal(current.SelectedGroups, record.SelectedGroups) {
			return fmt.Errorf("BATCH_PROOF_INPUT_MOVED: batch changed before proof admission")
		}
		current.ClosedReason = "proof-admitted"
		current.Proof = &Proof{Status: "planned", Window: window, RequiredMode: plan.RequiredMode,
			ExecutedMode: plan.ExecutedMode, SelectedGroups: selected, Sample: sample, Launchers: sample.OverlappingHost}
		current.Transition(StateProving, at, "prove", actor, "planned")
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return store.Load(id)
}

// FinishProof attaches the exact execution and reuse evidence to the planned
// attempt. A failed launch remains durable and is not silently retried.
func FinishProof(store Store, id, actor string, result proofrun.TestResult, launchErr error, at time.Time) error {
	return store.Update(id, func(record *Record) error {
		if record.State != StateProving || record.Proof == nil || record.Proof.Status != "planned" {
			return fmt.Errorf("BATCH_PROOF_NOT_ADMITTED: batch %s has no planned proof", id)
		}
		record.Proof.AttemptID = result.AttemptID
		for _, group := range result.Groups {
			if group.NativeLaunched {
				record.Proof.Executions = append(record.Proof.Executions, group.ID)
			}
			if group.ReuseAttempt != "" {
				if record.Proof.Reuse == nil {
					record.Proof.Reuse = map[string]string{}
				}
				record.Proof.Reuse[group.ID] = group.ReuseAttempt
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
	return store.Update(id, func(record *Record) error {
		if record.State != StateProving || record.Proof == nil || record.Proof.Status != "planned" {
			return fmt.Errorf("budget withdrawal requires an admitted proof")
		}
		if err := requestUnitReturn(record, goalID, UnitWithdrawnBudget, reason, actor, at); err != nil {
			return err
		}
		record.Proof.Status, record.Proof.Failure = "budget-refused", reason
		record.Transition(StateOpen, at, "prove", actor, "budget-refused "+goalID)
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
		if record.Proof == nil {
			return fmt.Errorf("held-unclassified requires a recorded proof")
		}
		record.Proof.Status, record.Proof.Failure = "held-unclassified", status+"; "+nextEvidence
		record.Transition(StateHeldUnclassified, at, "diagnose", actor, status+"; "+nextEvidence)
		return nil
	})
}
