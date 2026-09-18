package batch

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// PrefixReceipt records how one exact prefix obtained every selected group.
type PrefixReceipt struct {
	GoalID, Tree, AttemptID, ResultPath string
	CommitIDs                           []string `json:"CommitIDs,omitempty"`
	Units                               []string `json:"Units,omitempty"`
	LastUnit                            string   `json:"LastUnit,omitempty"`
	Reused                              map[string]string
	Executed                            []string
}

type PrefixRunResult struct {
	AttemptID, ResultPath string
	Reused                map[string]string
	Executed              []string
	Red                   []RedGroup
}

type PrefixReceiptSeams struct {
	Execute func(goalID, tree string, groups []string) (PrefixRunResult, error)
}

type PrefixRedError struct {
	GoalID string
	Groups []RedGroup
}

func (red *PrefixRedError) Error() string { return "prefix proof red for " + red.GoalID }

type PrefixBudgetRefusal struct{ Reason string }

func (refusal *PrefixBudgetRefusal) Error() string { return refusal.Reason }

type PrefixRevisionRefusal struct{ Reason string }

func (refusal *PrefixRevisionRefusal) Error() string { return refusal.Reason }

// PrefixAdmissionRefusal preserves a non-budget admission decision so receipt
// composition can retry it without changing the member's lifecycle.
type PrefixAdmissionRefusal struct {
	Code, Reason string
}

func (refusal *PrefixAdmissionRefusal) Error() string { return refusal.Reason }

// ComposePrefixReceipts reuses only identity-equal terminal evidence. A
// prefix with no identity differences creates no attempt.
func ComposePrefixReceipts(store Store, id, actor string, at time.Time, seams PrefixReceiptSeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateLanding || record.Proof == nil || record.Proof.Status != "green" {
		return fmt.Errorf("batch %s has no green tip proof", id)
	}
	joined := joinedUnits(record.Units)
	if len(joined) == 0 || len(record.PrefixTrees) < len(joined) {
		return fmt.Errorf("batch %s has no complete prefix tree list", id)
	}
	for index, unit := range joined[:len(joined)-1] {
		tree := record.PrefixTrees[index]
		if existing, ok := record.Receipts[unit.GoalID]; ok && existing.Tree == tree {
			continue
		}
		receipt := PrefixReceipt{GoalID: unit.GoalID, Tree: tree, CommitIDs: slices.Clone(unit.CommitIDs), LastUnit: unit.LastUnit, Reused: map[string]string{}}
		for _, build := range unit.Builds {
			receipt.Units = append(receipt.Units, build.Units...)
		}
		if seams.Execute == nil {
			return fmt.Errorf("batch %s has no prefix receipt runner", id)
		}
		result, runErr := seams.Execute(unit.GoalID, tree, slices.Clone(record.Proof.SelectedGroups))
		if runErr != nil {
			var revision *PrefixRevisionRefusal
			if errors.As(runErr, &revision) {
				if err := RequestReturn(store, id, unit.GoalID, UnitEjected, revision.Error(), actor, at); err != nil {
					return err
				}
				return ReassembleSurvivors(store, id, actor, at)
			}
			var budget *PrefixBudgetRefusal
			if errors.As(runErr, &budget) {
				if err := RequestReturn(store, id, unit.GoalID, UnitWithdrawnBudget, budget.Error(), actor, at); err != nil {
					return err
				}
				return ReassembleSurvivors(store, id, actor, at)
			}
			return runErr
		}
		if len(result.Red) != 0 {
			if err := store.Update(id, func(current *Record) error {
				current.Proof.Status, current.Proof.Failure = "prefix-red", unit.GoalID
				current.Proof.RedGroups = slices.Clone(result.Red)
				current.Proof.PrefixGoal = unit.GoalID
				current.Transition(StateDiagnosing, at, "prefix-proof", actor, unit.GoalID)
				return nil
			}); err != nil {
				return fmt.Errorf("persist prefix red for %s: %w", unit.GoalID, err)
			}
			return &PrefixRedError{GoalID: unit.GoalID, Groups: result.Red}
		}
		receipt.AttemptID, receipt.ResultPath, receipt.Executed, receipt.Reused = result.AttemptID, result.ResultPath, slices.Clone(result.Executed), result.Reused
		if receipt.Reused == nil {
			receipt.Reused = map[string]string{}
		}
		if err := store.Update(id, func(current *Record) error {
			if current.Receipts == nil {
				current.Receipts = map[string]PrefixReceipt{}
			}
			current.Receipts[unit.GoalID] = receipt
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
