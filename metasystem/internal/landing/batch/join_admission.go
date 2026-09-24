package batch

import (
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// JoinAdmissionRun uses the shared testing owner after the claim reaches the
// landing checkout. It must verify its exact-tree result before returning.
type JoinAdmissionRun func(string, Unit) (JoinAdmission, error)

type JoinAdmissionRed struct{ Reason string }

func (red *JoinAdmissionRed) Error() string { return red.Reason }

// PublishJoinWithAdmission keeps a joining member out of seal/proof until the
// handed-over claim has produced structured admission evidence. No batch lock
// is held while the testing owner waits for capacity or runs native checks.
func PublishJoinWithAdmission(store Store, batchID string, unit Unit, actor string, at time.Time,
	plan func(string, string, string) (testpolicy.Plan, error), handover func() error, run JoinAdmissionRun) error {
	return publishJoinWithAdmission(store, batchID, unit, actor, at, plan, handover, run, nil)
}

// PublishJoinWithAdmissionForecast consumes an outside-lock snapshot before
// handover. A changed membership, claim, selection, base or prefix refuses the
// handover; the forecast itself does not reserve budget.
func PublishJoinWithAdmissionForecast(store Store, batchID string, unit Unit, actor string, at time.Time,
	plan func(string, string, string) (testpolicy.Plan, error), handover func() error, run JoinAdmissionRun,
	forecast CostForecast) error {
	return publishJoinWithAdmission(store, batchID, unit, actor, at, plan, handover, run, &forecast)
}

func publishJoinWithAdmission(store Store, batchID string, unit Unit, actor string, at time.Time,
	plan func(string, string, string) (testpolicy.Plan, error), handover func() error, run JoinAdmissionRun,
	forecast *CostForecast) error {
	if run == nil {
		return fmt.Errorf("BATCH_JOIN_TEST_DROPPED: shared admission runner is unavailable")
	}
	err := store.locked(func() error {
		if err := store.updateLocked(batchID, func(record *Record) error {
			if err := joinRefusal(*record); err != nil {
				return err
			}
			if err := checkMembership(store, batchID, unit.GoalID, unit.Chain); err != nil {
				return err
			}
			for _, existing := range record.Units {
				if existing.State == UnitJoining {
					return refuseBatch("BATCH_JOIN_PENDING", "another member's admission is pending")
				}
			}
			live := slices.DeleteFunc(slices.Clone(record.Units), func(existing Unit) bool { return existing.State != UnitJoined })
			unit.State = UnitJoining
			live = append(live, unit)
			if forecast != nil {
				if len(forecast.Binding.PrefixTrees) != len(live) ||
					!slices.Equal(record.PrefixTrees, forecast.Binding.PrefixTrees[:len(live)-1]) ||
					!reflect.DeepEqual(CostBinding(*record, live, forecast.Binding.PrefixTrees), forecast.Binding) {
					return fmt.Errorf("BATCH_COST_INPUT_MOVED: batch or member changed before handover")
				}
			}
			prefixes, err := store.reassembly.assemble(record.BaseTree, live)
			if err != nil {
				return err
			}
			if forecast != nil && !slices.Equal(prefixes, forecast.Binding.PrefixTrees) {
				return fmt.Errorf("BATCH_COST_INPUT_MOVED: cumulative prefix trees changed before handover")
			}
			unitPrefixes, err := store.reassembly.assemble(record.BaseTree, []Unit{unit})
			if err != nil || len(unitPrefixes) != 1 {
				return fmt.Errorf("select joined unit %s: prefixes=%d: %w", unit.GoalID, len(unitPrefixes), err)
			}
			selection, err := store.planJoinedUnit(record.BaseTree, unit, unitPrefixes[0], plan)
			if err != nil {
				return err
			}
			unit.SelectedGroups = slices.Clone(selection.SelectedGroups)
			if forecast != nil && !slices.Equal(unit.SelectedGroups, forecast.Binding.Members[len(live)-1].SelectedGroups) {
				return fmt.Errorf("BATCH_COST_INPUT_MOVED: admission selection changed before handover")
			}
			unit.Admission = &JoinAdmission{Tree: unitPrefixes[0], Status: "pending"}
			record.PrefixTrees, record.TipTree = prefixes, prefixes[len(prefixes)-1]
			recordSelection(record, selection)
			record.Units = append(record.Units, unit)
			appendUnitHistory(record, at, "join", actor, unit.GoalID, "", UnitJoining)
			return nil
		}); err != nil {
			return err
		}
		if err := store.seams.publish(UnitJoining); err != nil {
			return err
		}
		if err := handover(); err != nil {
			return err
		}
		if err := store.seams.publish("handover"); err != nil {
			return err
		}
		return store.updateLocked(batchID, func(record *Record) error {
			for index := range record.Units {
				if record.Units[index].GoalID == unit.GoalID && record.Units[index].State == UnitJoining && record.Units[index].Admission != nil {
					record.Units[index].Admission.Status = "handed-over"
					return nil
				}
			}
			return fmt.Errorf("BATCH_JOIN_PENDING: member %s changed during handover", unit.GoalID)
		})
	})
	if err != nil {
		return err
	}
	return ResumeJoinAdmission(store, batchID, unit.GoalID, actor, at, run)
}

// ResumeJoinAdmission is idempotent across a crash after handover or after a
// terminal shared attempt. The runner first consumes retained evidence, then
// executes only missing work under the testing owner's reservation rules.
func ResumeJoinAdmission(store Store, batchID, goalID, actor string, at time.Time, run JoinAdmissionRun) error {
	record, err := store.Load(batchID)
	if err != nil {
		return err
	}
	var unit Unit
	found := false
	for _, candidate := range record.Units {
		if candidate.GoalID == goalID {
			unit, found = candidate, true
			break
		}
	}
	if !found {
		return fmt.Errorf("BATCH_JOIN_PENDING: member %s is absent", goalID)
	}
	if unit.State == UnitJoined {
		return nil
	}
	if unit.State != UnitJoining || unit.Admission == nil || unit.Admission.Tree == "" || unit.Admission.Status != "handed-over" || run == nil {
		return fmt.Errorf("BATCH_JOIN_PENDING: member %s has no resumable admission", goalID)
	}
	result, runErr := run(batchID, unit)
	if runErr != nil {
		if err := HandlePrefixMemberRefusal(store, batchID, goalID, actor, at, runErr); err != nil {
			return err
		}
		return runErr
	}
	if result.Status != "verified" || result.Tree != unit.Admission.Tree {
		return fmt.Errorf("BATCH_JOIN_TEST_DROPPED: member %s has no verified admission on %s", goalID, unit.Admission.Tree)
	}
	return store.locked(func() error {
		if err := store.updateLocked(batchID, func(current *Record) error {
			if current.State != StateOpen {
				return refuseBatch("BATCH_SEALED", "batch sealed during admission")
			}
			for index := range current.Units {
				candidate := &current.Units[index]
				if candidate.GoalID != goalID {
					continue
				}
				if candidate.State == UnitJoined && candidate.Admission != nil && candidate.Admission.Tree == result.Tree {
					return nil
				}
				if candidate.State != UnitJoining || candidate.Admission == nil || candidate.Admission.Tree != result.Tree || candidate.Claim != unit.Claim {
					return fmt.Errorf("BATCH_JOIN_PENDING: member %s changed during admission", goalID)
				}
				candidate.Admission = &result
				if result.AttemptID != "" {
					candidate.Gate = append(candidate.Gate, result.AttemptID)
				}
				candidate.State = UnitJoined
				appendUnitHistory(current, at, "join", actor, goalID, UnitJoining, UnitJoined)
				return nil
			}
			return fmt.Errorf("BATCH_JOIN_PENDING: member %s disappeared", goalID)
		}); err != nil {
			return err
		}
		return store.seams.publish(UnitJoined)
	})
}
