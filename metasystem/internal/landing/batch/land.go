package batch

import (
	"fmt"
	"slices"
	"time"
)

// LandingProgress is the durable boundary between local construction, the
// single push, and trailer-based recovery.
type LandingProgress struct {
	Base         string            `json:"base"`
	Commits      map[string]string `json:"commits,omitempty"`
	BranchTip    string            `json:"branchTip,omitempty"`
	HeldChecked  bool              `json:"heldChecked,omitempty"`
	PushComplete bool              `json:"pushComplete,omitempty"`
	PushedTip    string            `json:"pushedTip,omitempty"`
	CleanupDone  bool              `json:"cleanupDone,omitempty"`
}

type LandSeams struct {
	Prepare       func(base string) error
	Apply         func(Unit) error
	AppendReceipt func(Unit, PrefixReceipt) error
	Commit        func(Unit, PrefixReceipt) (string, error)
	Held          func(base, tip string) error
	PublishBranch func(expected, tip string) error
	Push          func(base, tip string) error
	Reset         func(base string) error
	Cleanup       func() error
}

// LandSeries builds every commit locally in original join order, checks the
// whole range once, and pushes the complete series once.
func LandSeries(store Store, id, actor string, at time.Time, seams LandSeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateLanding || record.Proof == nil || record.Proof.Status != "green" {
		return fmt.Errorf("BATCH_LAND_STATE_REFUSED: batch %s has no green landing candidate", id)
	}
	proofOwner := ""
	for index := len(record.History) - 1; index >= 0; index-- {
		entry := record.History[index]
		if entry.To != StateLanding {
			continue
		}
		proofOwner = entry.Actor
		break
	}
	if proofOwner == "" || proofOwner != actor {
		return fmt.Errorf("BATCH_LAND_DELEGATION_REFUSED: landing actor %s is not green-proof owner %s", actor, proofOwner)
	}
	units := joinedUnits(record.Units)
	if len(units) == 0 {
		return fmt.Errorf("BATCH_LAND_STATE_REFUSED: batch %s has no joined units", id)
	}
	progress := LandingProgress{Base: record.BaseTree, Commits: map[string]string{}}
	if record.Landing != nil {
		progress = *record.Landing
		progress.Commits = cloneStrings(record.Landing.Commits)
	}
	if !progress.PushComplete {
		// A process may die after any local commit. Rebuild the complete local
		// stack from the recorded base so recovery never trusts an unproved
		// worktree shape or pushes a recorded prefix.
		progress.Commits = map[string]string{}
		progress.HeldChecked = false
		if seams.Prepare != nil {
			if err := seams.Prepare(record.BaseTree); err != nil {
				return err
			}
		}
	}
	for index, unit := range units {
		if progress.Commits[unit.GoalID] != "" {
			continue
		}
		receipt, ok := record.Receipts[unit.GoalID]
		if index == len(units)-1 && !ok {
			receipt = PrefixReceipt{GoalID: unit.GoalID, Tree: record.TipTree, AttemptID: record.Proof.AttemptID,
				Reused: cloneStrings(record.Proof.Reuse), Executed: slices.Clone(record.Proof.Executions)}
			ok = true
		}
		if !ok || receipt.Tree != record.PrefixTrees[index] {
			return fmt.Errorf("BATCH_LAND_RECEIPT_REFUSED: unit %s has no exact prefix receipt", unit.GoalID)
		}
		if seams.Apply == nil || seams.AppendReceipt == nil || seams.Commit == nil {
			return fmt.Errorf("BATCH_LAND_UNWIRED: local series helpers are incomplete")
		}
		if err = seams.Apply(unit); err == nil {
			err = seams.AppendReceipt(unit, receipt)
		}
		var commit string
		if err == nil {
			commit, err = seams.Commit(unit, receipt)
		}
		if err != nil {
			if seams.Reset != nil {
				err = fmt.Errorf("commit %s refused: %w", unit.GoalID, err)
				if resetErr := seams.Reset(record.BaseTree); resetErr != nil {
					return fmt.Errorf("%v; reset: %w", err, resetErr)
				}
			}
			if returnErr := RequestReturn(store, id, unit.GoalID, UnitEjected, err.Error(), actor, at); returnErr != nil {
				return returnErr
			}
			return ReassembleSurvivors(store, id, actor, at)
		}
		if commit == "" {
			return fmt.Errorf("BATCH_LAND_COMMIT_REFUSED: unit %s returned no commit", unit.GoalID)
		}
		progress.Commits[unit.GoalID] = commit
		if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
			return err
		}
	}
	tip := progress.Commits[units[len(units)-1].GoalID]
	if !progress.PushComplete && seams.PublishBranch != nil {
		if err := seams.PublishBranch(progress.BranchTip, tip); err != nil {
			return err
		}
		progress.BranchTip = tip
		if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
			return err
		}
	}
	if !progress.HeldChecked {
		if seams.Held == nil || seams.Held(record.BaseTree, tip) != nil {
			return fmt.Errorf("BATCH_LAND_HELD_REFUSED: complete series did not pass held")
		}
		progress.HeldChecked = true
		if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
			return err
		}
	}
	if !progress.PushComplete {
		if seams.Push == nil {
			return fmt.Errorf("BATCH_LAND_UNWIRED: push helper is absent")
		}
		if err := seams.Push(record.BaseTree, tip); err != nil {
			return err
		}
		progress.PushComplete, progress.PushedTip = true, tip
		if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
			return err
		}
	}
	if seams.Cleanup != nil {
		if err := seams.Cleanup(); err != nil {
			return err
		}
		progress.CleanupDone = true
		return store.Update(id, func(current *Record) error { current.Landing = &progress; return nil })
	}
	return nil
}

func cloneStrings(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
