package batch

import (
	"fmt"
	"slices"
	"time"
)

type trunkRedClearSeams struct {
	run          func(DiagnosticRequest, Claim) (DiagnosticResult, error)
	mint         func() (string, error)
	ledger       LedgerOwner
	descendsFrom func(string, string) (bool, error)
}

func reopenHeldAfterDiagnostic(store Store, id, newBaseTree, newBaseCommit, actor string, at time.Time, seams trunkRedClearSeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateHeldTrunkRed || record.TrunkRed == nil {
		return fmt.Errorf("batch %s is not held-trunk-red", id)
	}
	if newBaseTree == record.BaseTree || newBaseTree == record.TrunkRed.Red.BaseTree {
		return refuseBatch("BATCH_REOPEN_SAME_TREE", "held batch requires a new base tree")
	}
	joined := joinedUnits(record.Units)
	if len(joined) == 0 || seams.run == nil || seams.mint == nil || seams.ledger == nil || seams.descendsFrom == nil {
		return fmt.Errorf("batch %s has incomplete trunk-red clearing seams", id)
	}
	groups := make([]string, len(record.TrunkRed.Entries))
	for index, entry := range record.TrunkRed.Entries {
		groups[index] = entry.Group
	}
	result, err := seams.run(DiagnosticRequest{Tree: newBaseTree, GoalID: joined[len(joined)-1].GoalID, Groups: groups, NeverReuse: true}, joined[len(joined)-1].Claim)
	if err != nil {
		return err
	}
	if !result.Green() {
		opid, err := seams.mint()
		if err != nil {
			return err
		}
		if err := store.Update(id, func(current *Record) error {
			if current.State != StateHeldTrunkRed || current.TrunkRed == nil || current.TrunkRed.Opid != record.TrunkRed.Opid {
				return fmt.Errorf("batch %s moved before the new base red was recorded", id)
			}
			current.Transition(StateDiagnosing, at, "diagnose", actor, "new base tree")
			return nil
		}); err != nil {
			return err
		}
		red := TrunkRed{AttemptID: result.AttemptID, BaseCommit: newBaseCommit, BaseTree: newBaseTree, Groups: result.Groups}
		if err := store.HoldTrunkRed(id, red, opid, at, actor); err != nil {
			return err
		}
		_, err = store.WithLedgerOwner(seams.ledger).EnsureTrunkRedRecorded(id, seams.mint, at, actor)
		return err
	}
	prepared, err := prepareHeldReopen(store, id, newBaseTree)
	if err != nil {
		return err
	}
	open, err := seams.ledger.Open()
	if err != nil {
		return err
	}
	byID := make(map[string]OpenEntry, len(open))
	for _, entry := range open {
		byID[entry.ID] = entry
	}
	for _, ref := range record.TrunkRed.Entries {
		entry, found := byID[ref.ID]
		if !found || entry.Group != ref.Group {
			return fmt.Errorf("held trunk-red entry %s is not open", ref.ID)
		}
		if entry.LastBaseCommit == newBaseCommit {
			return nil
		}
		descends, err := seams.descendsFrom(newBaseCommit, entry.LastBaseCommit)
		if err != nil {
			return err
		}
		if !descends {
			return nil
		}
	}
	for _, ref := range record.TrunkRed.Entries {
		opid, err := seams.mint()
		if err != nil {
			return err
		}
		if err := seams.ledger.Clear(opid, ref, Green{AttemptID: result.AttemptID, BaseCommit: newBaseCommit, BaseTree: newBaseTree, Group: ref.Group}); err != nil {
			return err
		}
	}
	return applyHeldReopen(store, id, newBaseTree, actor, at, prepared)
}

func clearGreenTipEntries(proof *Proof, seams trunkRedClearSeams) error {
	if proof == nil || proof.Status != "green" || proof.BaseCommit == "" || seams.mint == nil || seams.ledger == nil || seams.descendsFrom == nil {
		return nil
	}
	open, err := seams.ledger.Open()
	if err != nil {
		return err
	}
	for _, entry := range open {
		if len(entry.Holds) != 0 || !slices.Contains(proof.Executions, entry.Group) || entry.LastBaseCommit == proof.BaseCommit {
			continue
		}
		descends, err := seams.descendsFrom(proof.BaseCommit, entry.LastBaseCommit)
		if err != nil {
			return err
		}
		if !descends {
			continue
		}
		opid, err := seams.mint()
		if err != nil {
			return err
		}
		if err := seams.ledger.Clear(opid, EntryRef{ID: entry.ID, Group: entry.Group}, Green{
			AttemptID: proof.AttemptID, BaseCommit: proof.BaseCommit, BaseTree: proof.BaseTree, Group: entry.Group,
		}); err != nil {
			return err
		}
	}
	return nil
}
