package batch

import (
	"fmt"
	"time"
)

// ReopenHeld permits another proof only when the trunk supplies a distinct
// base tree. The surviving units are reassembled from that new base.
func ReopenHeld(store Store, id, newBaseTree, actor string, at time.Time) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateHeldTrunkRed {
		return fmt.Errorf("batch %s is not held-trunk-red", id)
	}
	if newBaseTree == record.BaseTree || record.TrunkRed != nil && newBaseTree == record.TrunkRed.Red.BaseTree {
		return refuseBatch("BATCH_REOPEN_SAME_TREE", "held batch requires a new base tree")
	}
	survivors := joinedUnits(record.Units)
	prefixes, err := assembleUnits(store.root, newBaseTree, survivors)
	if err != nil {
		return err
	}
	return store.Update(id, func(current *Record) error {
		current.BaseTree, current.PrefixTrees, current.TipTree = newBaseTree, prefixes, prefixes[len(prefixes)-1]
		current.SelectedGroups, current.Seal, current.Proof, current.TrunkRed = nil, nil, nil, nil
		current.ClosedReason = ""
		current.Transition(StateOpen, at, "reopen", actor, "new base tree")
		return nil
	})
}
