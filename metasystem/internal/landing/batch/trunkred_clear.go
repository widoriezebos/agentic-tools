package batch

import (
	"errors"
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
	refs, discoveryErr := heldTrunkRedEntries(id, record.TrunkRed.Entries, seams.ledger)
	groups := heldTrunkRedGroups(refs)
	result, err := seams.run(DiagnosticRequest{Tree: newBaseTree, GoalID: joined[len(joined)-1].GoalID, Groups: groups, NeverReuse: true}, joined[len(joined)-1].Claim)
	if err != nil {
		return errors.Join(discoveryErr, err)
	}
	if !result.Green() {
		passedRefs := slices.DeleteFunc(slices.Clone(refs), func(ref EntryRef) bool {
			return slices.ContainsFunc(result.Groups, func(group RedGroup) bool { return group.ID == ref.Group })
		})
		_, clearErr := clearHeldTrunkRedEntries(passedRefs, result, newBaseTree, newBaseCommit, seams)
		clearErr = errors.Join(discoveryErr, clearErr)
		opid, err := seams.mint()
		if err != nil {
			return errors.Join(clearErr, err)
		}
		red := TrunkRed{AttemptID: result.AttemptID, BaseCommit: newBaseCommit, BaseTree: newBaseTree, Groups: result.Groups}
		if err := store.reholdTrunkRed(id, record.TrunkRed.Opid, red, opid, at, actor); err != nil {
			return errors.Join(clearErr, err)
		}
		_, recordErr := store.WithLedgerOwner(seams.ledger).EnsureTrunkRedRecorded(id, seams.mint, at, actor)
		return errors.Join(clearErr, recordErr)
	}
	prepared, err := prepareHeldReopen(store, id, newBaseTree)
	if err != nil {
		return err
	}
	cleared, err := clearHeldTrunkRedEntries(refs, result, newBaseTree, newBaseCommit, seams)
	if clearErr := errors.Join(discoveryErr, err); clearErr != nil {
		return clearErr
	}
	if !cleared {
		return store.Update(id, func(current *Record) error {
			if current.State != StateHeldTrunkRed || current.TrunkRed == nil || current.TrunkRed.Opid != record.TrunkRed.Opid {
				return fmt.Errorf("batch %s moved before its clearing tree was saved", id)
			}
			current.TrunkRed.CheckedTree = newBaseTree
			return nil
		})
	}
	return applyHeldReopen(store, id, newBaseTree, actor, at, prepared)
}

func heldTrunkRedEntries(batchID string, recorded []EntryRef, ledger LedgerOwner) ([]EntryRef, error) {
	refs := slices.Clone(recorded)
	open, err := ledger.Open()
	if err != nil {
		return refs, err
	}
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		seen[ref.ID] = true
	}
	for _, entry := range open {
		if !seen[entry.ID] && slices.Contains(entry.Holds, batchID) {
			refs = append(refs, EntryRef{ID: entry.ID, Group: entry.Group})
			seen[entry.ID] = true
		}
	}
	return refs, nil
}

func heldTrunkRedGroups(refs []EntryRef) []string {
	groups := make([]string, 0, len(refs))
	for _, ref := range refs {
		if !slices.Contains(groups, ref.Group) {
			groups = append(groups, ref.Group)
		}
	}
	return groups
}

func clearHeldTrunkRedEntries(refs []EntryRef, result DiagnosticResult, newBaseTree, newBaseCommit string, seams trunkRedClearSeams) (bool, error) {
	if len(refs) == 0 {
		return true, nil
	}
	open, err := seams.ledger.Open()
	if err != nil {
		return false, err
	}
	byID := make(map[string]OpenEntry, len(open))
	for _, entry := range open {
		byID[entry.ID] = entry
	}
	for _, ref := range refs {
		entry, found := byID[ref.ID]
		if !found {
			continue
		}
		if entry.Group != ref.Group {
			return false, fmt.Errorf("held trunk-red entry %s has group %s, want %s", ref.ID, entry.Group, ref.Group)
		}
		if entry.LastBaseCommit == newBaseCommit {
			return false, nil
		}
		descends, err := seams.descendsFrom(newBaseCommit, entry.LastBaseCommit)
		if err != nil {
			return false, err
		}
		if !descends {
			return false, nil
		}
	}
	for _, ref := range refs {
		if _, found := byID[ref.ID]; !found {
			continue
		}
		opid, err := seams.mint()
		if err != nil {
			return false, err
		}
		if err := seams.ledger.Clear(opid, ref, Green{AttemptID: result.AttemptID, BaseCommit: newBaseCommit, BaseTree: newBaseTree, Group: ref.Group}); err != nil {
			if !IsTrunkRedClosed(err) {
				return false, err
			}
		}
	}
	return true, nil
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
		if len(entry.Holds) != 0 || !slices.Contains(proof.Passed, entry.Group) || entry.LastBaseCommit == proof.BaseCommit {
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
			AttemptID: proof.AttemptID, BaseCommit: proof.BaseCommit, Group: entry.Group,
		}); err != nil {
			return err
		}
	}
	return nil
}
