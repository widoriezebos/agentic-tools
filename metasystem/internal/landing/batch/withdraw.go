package batch

import (
	"fmt"
	"slices"
	"sort"
	"time"
)

// RequestWithdrawal records a pre-seal request from the source that joined
// the unit. The return owner remains responsible for changing custody and
// settling the terminal withdrawn state.
func RequestWithdrawal(store Store, goalID, machine, lineage, seatRoot, actor string, at time.Time) (Record, error) {
	var result Record
	err := store.locked(func() error {
		records, err := store.Records()
		if err != nil {
			return err
		}
		type match struct {
			record Record
			index  int
		}
		var active, historical []match
		for _, record := range records {
			for index, unit := range record.Units {
				if unit.GoalID != goalID {
					continue
				}
				found := match{record: record, index: index}
				if record.State == StateLanded || record.State == StateDissolved || terminalUnitState(unit.State) {
					historical = append(historical, found)
				} else {
					active = append(active, found)
				}
			}
		}
		if len(active) == 0 {
			if len(historical) != 0 {
				unit := historical[len(historical)-1].record.Units[historical[len(historical)-1].index]
				return refuseBatch("BATCH_WITHDRAW_REFUSED", fmt.Sprintf("goal %s is already %s; nothing to do", goalID, unit.State))
			}
			return refuseBatch("BATCH_WITHDRAW_REFUSED", "goal "+goalID+" is not in a landing batch; nothing to do")
		}
		if len(active) != 1 {
			return refuseBatch("BATCH_WITHDRAW_REFUSED", fmt.Sprintf("goal %s is in %d batches at once; metasystem landing status --verbose shows them", goalID, len(active)))
		}
		selected := active[0]
		if selected.record.State != StateOpen {
			switch selected.record.State {
			case StateSealed, StateProving, StateDiagnosing, StateLanding:
				return refuseBatch("BATCH_SEALED", fmt.Sprintf("batch %s is %s, too late to leave; metasystem work wait %s --for landing waits for it", selected.record.BatchID, selected.record.State, goalID))
			default:
				return refuseBatch("BATCH_WITHDRAW_REFUSED", fmt.Sprintf("goal %s can't leave batch %s while it is %s; metasystem landing status shows it", goalID, selected.record.BatchID, selected.record.State))
			}
		}
		unit := selected.record.Units[selected.index]
		if unit.State != UnitJoined {
			return refuseBatch("BATCH_WITHDRAW_REFUSED", fmt.Sprintf("goal %s can't leave its batch while it is %s; metasystem landing status shows it", goalID, unit.State))
		}
		if unit.Claim.Machine != machine || unit.Claim.Lineage != lineage || unit.SeatRoot != seatRoot {
			return refuseBatch("BATCH_WITHDRAW_REFUSED", fmt.Sprintf("only machine %s, which queued goal %s from %s, may take it out; nothing to do here", unit.Claim.Machine, goalID, unit.SeatRoot))
		}
		if err := store.updateLocked(selected.record.BatchID, func(record *Record) error {
			current := &record.Units[selected.index]
			if record.State != StateOpen || current.State != UnitJoined {
				return refuseBatch("BATCH_WITHDRAW_REFUSED", "batch "+selected.record.BatchID+" changed meanwhile; metasystem landing status shows where goal "+goalID+" is")
			}
			current.State, current.Outcome = UnitReturnPending, UnitWithdrawn
			current.Failure, current.ReturnDisposition = "withdraw requested by the recorded joiner", ""
			appendUnitHistory(record, at, "withdraw-requested", actor, goalID, UnitJoined, UnitReturnPending)
			forgetEarly(record, at, actor, leaveCause(goalID, UnitWithdrawn))
			survivors := joinedUnits(record.Units)
			record.PrefixTrees, record.SelectedGroups = nil, nil
			record.TipTree = record.BaseTree
			if len(survivors) != 0 {
				prefixes, err := store.reassembly.assemble(record.BaseTree, survivors)
				if err != nil {
					return err
				}
				record.PrefixTrees, record.TipTree = prefixes, prefixes[len(prefixes)-1]
				for _, survivor := range survivors {
					record.SelectedGroups = append(record.SelectedGroups, survivor.SelectedGroups...)
				}
				sort.Strings(record.SelectedGroups)
				record.SelectedGroups = slices.Compact(record.SelectedGroups)
			}
			return nil
		}); err != nil {
			return err
		}
		result, err = store.Load(selected.record.BatchID)
		return err
	})
	return result, err
}
