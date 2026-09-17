package batch

import (
	"fmt"
	"time"
)

// ReopenHeld permits another proof only when the trunk supplies a distinct
// base tree. The surviving units are reassembled from that new base.
func ReopenHeld(store Store, id, newBaseTree, actor string, at time.Time) error {
	prepared, err := prepareHeldReopen(store, id, newBaseTree)
	if err != nil {
		return err
	}
	return applyHeldReopen(store, id, newBaseTree, actor, at, prepared)
}

type heldReopen struct {
	opid     string
	prefixes []string
}

func prepareHeldReopen(store Store, id, newBaseTree string) (heldReopen, error) {
	record, err := store.Load(id)
	if err != nil {
		return heldReopen{}, err
	}
	if record.State != StateHeldTrunkRed {
		return heldReopen{}, fmt.Errorf("batch %s is not held-trunk-red", id)
	}
	if newBaseTree == record.BaseTree || record.TrunkRed != nil && newBaseTree == record.TrunkRed.Red.BaseTree {
		return heldReopen{}, refuseBatch("BATCH_REOPEN_SAME_TREE", "held batch requires a new base tree")
	}
	survivors := joinedUnits(record.Units)
	prefixes, err := assembleUnits(store.root, newBaseTree, survivors)
	if err != nil {
		return heldReopen{}, err
	}
	return heldReopen{opid: record.TrunkRed.Opid, prefixes: prefixes}, nil
}

func applyHeldReopen(store Store, id, newBaseTree, actor string, at time.Time, prepared heldReopen) error {
	return store.Update(id, func(current *Record) error {
		if current.State != StateHeldTrunkRed || current.TrunkRed == nil || current.TrunkRed.Opid != prepared.opid {
			return fmt.Errorf("batch %s moved before it could reopen", id)
		}
		current.BaseTree, current.PrefixTrees, current.TipTree = newBaseTree, prepared.prefixes, prepared.prefixes[len(prepared.prefixes)-1]
		current.SelectedGroups, current.Seal, current.Proof, current.TrunkRed = nil, nil, nil, nil
		current.ClosedReason = ""
		current.Transition(StateOpen, at, "reopen", actor, "new base tree")
		return nil
	})
}
