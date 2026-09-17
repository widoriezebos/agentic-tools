package batch

import (
	"fmt"
	"time"
)

const ReturnTargetLive, ReturnTargetDead, ReturnTargetRestarted, ReturnTargetUnknown = "live-same-instance", "dead", "restarted", "unknown"

type ReturnLedgerGoal struct {
	Claimed                       bool
	Machine, Lineage, Batch, Next string
}
type ReturnTarget struct {
	State string
	Epoch uint64
}
type ReturnSeams struct {
	Read     func(root, tree, goalID string) (ReturnLedgerGoal, error)
	Target   func(Claim) ReturnTarget
	HandBack func(goalID string, source Claim, reboundEpoch uint64) error
	Release  func(goalID, next string) error
}

func RequestReturn(store Store, batchID, goalID, outcome, reason, actor string, at time.Time) error {
	return store.Update(batchID, func(record *Record) error { return requestUnitReturn(record, goalID, outcome, reason, actor, at) })
}
func requestUnitReturn(record *Record, goalID, outcome, reason, actor string, at time.Time) error {
	for index := range record.Units {
		unit := &record.Units[index]
		if unit.GoalID == goalID {
			if (unit.State == UnitReturnPending || terminalUnitState(unit.State)) && unit.Outcome == outcome && unit.Failure == reason {
				return nil
			}
			if unit.State != UnitJoining && unit.State != UnitJoined {
				return fmt.Errorf("unit %s cannot request return from %s", goalID, unit.State)
			}
			from := unit.State
			unit.State, unit.Outcome, unit.Failure, unit.ReturnDisposition = UnitReturnPending, outcome, reason, ""
			appendUnitHistory(record, at, "return-request", actor, goalID, from, UnitReturnPending)
			return nil
		}
	}
	return fmt.Errorf("batch unit %s is absent", goalID)
}
func ReturnUnits(store Store, batchID, tree, actor string, at time.Time, seams ReturnSeams) error {
	return store.locked(func() error {
		record, err := store.Load(batchID)
		if err != nil {
			return err
		}
		for _, unit := range record.Units {
			if unit.State != UnitReturnPending {
				continue
			}
			if err := store.seams.publish("before-return"); err != nil {
				return err
			}
			ledger, readErr := seams.Read(store.root, tree, unit.GoalID)
			if readErr != nil {
				continue
			}
			if !ledger.Claimed || ledger.Batch != batchID || ledger.Machine == unit.Claim.Machine && ledger.Lineage == unit.Claim.Lineage {
				if err := settleReturn(store, batchID, unit.GoalID, ReturnAlreadyReturned, actor, at); err != nil {
					return err
				}
				continue
			}
			target := seams.Target(unit.Claim)
			switch target.State {
			case ReturnTargetLive:
				err = seams.HandBack(unit.GoalID, unit.Claim, target.Epoch)
			case ReturnTargetDead, ReturnTargetRestarted:
				next := unit.Outcome + ": " + unit.Failure
				if ledger.Next != "" {
					next += "; " + ledger.Next
				}
				err = seams.Release(unit.GoalID, next)
			default:
				continue
			}
			if err != nil {
				return err
			}
			if err := store.seams.publish("after-return"); err != nil {
				return err
			}
			disposition := ReturnHandedBack
			if target.State != ReturnTargetLive {
				disposition = ReturnReleased
			}
			if err := settleReturn(store, batchID, unit.GoalID, disposition, actor, at); err != nil {
				return err
			}
		}
		return nil
	})
}

func settleReturn(store Store, batchID, goalID, disposition, actor string, at time.Time) error {
	return store.updateLocked(batchID, func(record *Record) error {
		for index := range record.Units {
			unit := &record.Units[index]
			if unit.GoalID == goalID && unit.State == UnitReturnPending {
				unit.State, unit.ReturnDisposition = unit.Outcome, disposition
				appendUnitHistory(record, at, "return", actor, goalID, UnitReturnPending, unit.State)
				return nil
			}
		}
		return fmt.Errorf("return-pending unit %s is absent", goalID)
	})
}
