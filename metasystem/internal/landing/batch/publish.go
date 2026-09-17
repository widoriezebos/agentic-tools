package batch

import (
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func PublishJoin(store Store, batchID string, unit Unit, actor string, at time.Time, plan func(string, string, string) (testpolicy.Plan, error), handover func() error) error {
	return store.locked(func() error {
		if err := store.updateLocked(batchID, func(record *Record) error {
			if err := joinRefusal(*record); err != nil {
				return err
			}
			if err := checkMembership(store, batchID, unit.GoalID, unit.Chain); err != nil {
				return err
			}
			live := slices.DeleteFunc(slices.Clone(record.Units), func(existing Unit) bool { return existing.State != UnitJoining && existing.State != UnitJoined })
			unit.State = UnitJoining
			live = append(live, unit)
			prefixes, err := assembleUnits(store.root, record.BaseTree, live)
			if err != nil {
				return err
			}
			tip := prefixes[len(prefixes)-1]
			selection, err := plan(store.root, unit.GoalID, tip)
			if err != nil {
				return err
			}
			record.PrefixTrees, record.TipTree = prefixes, tip
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
		if err := store.updateLocked(batchID, func(record *Record) error {
			index := len(record.Units) - 1
			record.Units[index].State = UnitJoined
			appendUnitHistory(record, at, "join", actor, unit.GoalID, UnitJoining, UnitJoined)
			return nil
		}); err != nil {
			return err
		}
		return store.seams.publish(UnitJoined)
	})
}
func ReconcileJoins(store Store, batchID, tree, actor string, at time.Time, read func(string, string, string, string) (Claim, error)) error {
	if read == nil {
		read = claimAt
	}
	return store.locked(func() error {
		return store.updateLocked(batchID, func(record *Record) error {
			for index := range record.Units {
				unit := &record.Units[index]
				if unit.State != UnitJoining {
					continue
				}
				claim, claimErr := read(store.root, tree, batchID, unit.GoalID)
				unit.State, unit.Outcome, unit.Failure = UnitReturnPending, UnitEjected, "join-incomplete"
				if claimErr == nil && claim.Machine == unit.Claim.Machine && claim.Lineage == unit.Claim.Lineage && claim.Revision == unit.Claim.Revision && claim.AccountingRevision == unit.Claim.AccountingRevision {
					unit.State, unit.Outcome, unit.Failure = UnitJoined, "", ""
				}
				appendUnitHistory(record, at, "reconcile", actor, unit.GoalID, UnitJoining, unit.State)
			}
			return nil
		})
	})
}
func appendUnitHistory(record *Record, at time.Time, verb, actor, goalID, from, to string) {
	record.History = append(record.History, HistoryEntry{At: at.UTC().Format(time.RFC3339Nano), Verb: verb, From: from, To: to, Actor: actor, Detail: goalID + " " + to})
}
