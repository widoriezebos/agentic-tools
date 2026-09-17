package batch

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// LandingProgress is the durable boundary between local construction, the
// single push, and trailer-based recovery.
type LandingProgress struct {
	Base          string            `json:"base"`
	Commits       map[string]string `json:"commits,omitempty"`
	BranchTip     string            `json:"branchTip,omitempty"`
	HeldChecked   bool              `json:"heldChecked,omitempty"`
	PushComplete  bool              `json:"pushComplete,omitempty"`
	PushedTip     string            `json:"pushedTip,omitempty"`
	CleanupDone   bool              `json:"cleanupDone,omitempty"`
	RefusedOrigin string            `json:"refusedOrigin,omitempty"`
	RefusedBase   string            `json:"refusedBase,omitempty"`
	PushRounds    int               `json:"pushRounds,omitempty"`
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
	Origin        func() (string, error)
	OriginTree    func(string) (string, error)
	Abandon       func(tip, detachAt string) error
	LeaseBase     string
	RecoverPush   func(refusedOrigin, base, tip string) (PushRecovery, error)
}

// PushRecovery is the one bounded decision after an endpoint lease refusal.
// A changed proof input reopens the batch; otherwise the rebased, identity-
// verified series is reported as one completed endpoint push.
type PushRecovery struct {
	Origin, BaseTree, Tip string
	Reopen, Pushed        bool
}

const maxRecoveryPushRounds = 3

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
	origin := record.BaseTree
	if seams.Origin != nil {
		origin, err = seams.Origin()
		if err != nil {
			return err
		}
	}
	originTree := func(commit string) (string, error) {
		if seams.OriginTree != nil {
			return seams.OriginTree(commit)
		}
		return record.BaseTree, nil
	}
	abandonAndReopen := func(candidateTip, detachAt, baseTree string) error {
		if seams.Abandon != nil {
			if err := seams.Abandon(candidateTip, detachAt); err != nil {
				return err
			}
		}
		return ReopenRefusedPush(store, id, baseTree, actor, at)
	}
	recoverNow := func(recoveryOrigin, candidateTip string, pushErr error) (bool, error) {
		if seams.RecoverPush == nil {
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s refused the complete series: %w", recoveryOrigin, pushErr)
		}
		recovery, recoveryErr := seams.RecoverPush(recoveryOrigin, record.BaseTree, candidateTip)
		if recovery.Origin == "" {
			recovery.Origin = recoveryOrigin
		}
		progress.RefusedOrigin, progress.RefusedBase = recovery.Origin, recoveryOrigin
		if recovery.Tip != "" {
			// PublishLandingBranch completed. Preserve its lease tip even
			// when the following endpoint transaction was refused.
			progress.BranchTip = recovery.Tip
		}
		if recoveryErr != nil && recovery.Tip != "" {
			progress.PushRounds++
		}
		if storeErr := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); storeErr != nil {
			return false, errors.Join(pushErr, recoveryErr, storeErr)
		}
		if recoveryErr != nil {
			if progress.PushRounds >= maxRecoveryPushRounds {
				baseTree := recovery.BaseTree
				if baseTree == "" {
					baseTree, err = originTree(recovery.Origin)
					if err != nil {
						return false, err
					}
				}
				return false, abandonAndReopen(progress.BranchTip, recovery.Origin, baseTree)
			}
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s recovery failed: %w", recoveryOrigin, errors.Join(pushErr, recoveryErr))
		}
		if recovery.Reopen {
			return false, ReopenMovedTrunk(store, id, recovery.BaseTree, actor, at)
		}
		if !recovery.Pushed || recovery.Tip == "" {
			return false, fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s is unchanged after the refused push: %w", recoveryOrigin, pushErr)
		}
		progress.PushComplete, progress.PushedTip, progress.BranchTip = true, recovery.Tip, recovery.Tip
		progress.RefusedOrigin, progress.RefusedBase, progress.PushRounds = "", "", 0
		if storeErr := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); storeErr != nil {
			return false, storeErr
		}
		return true, nil
	}
	if !progress.PushComplete && progress.RefusedBase != "" {
		candidateTip := progress.BranchTip
		if candidateTip == "" {
			candidateTip = progress.Commits[units[len(units)-1].GoalID]
		}
		if candidateTip == "" {
			return fmt.Errorf("BATCH_LAND_PUSH_REFUSED: refused landing has no candidate tip")
		}
		if progress.RefusedBase == origin {
			baseTree, treeErr := originTree(origin)
			if treeErr != nil {
				return treeErr
			}
			return abandonAndReopen(candidateTip, origin, baseTree)
		}
		pushed, recoveryErr := recoverNow(origin, candidateTip, errors.New("prior endpoint refusal"))
		if recoveryErr != nil {
			return recoveryErr
		}
		if !pushed {
			return nil
		}
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
		if seams.Held == nil {
			return fmt.Errorf("BATCH_LAND_HELD_REFUSED: held helper is absent")
		}
		if heldErr := seams.Held(record.BaseTree, tip); heldErr != nil {
			return fmt.Errorf("BATCH_LAND_HELD_REFUSED: complete series did not pass held: %w", heldErr)
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
		if pushErr := seams.Push(record.BaseTree, tip); pushErr != nil {
			// RefusedBase is the lease base used by this endpoint attempt. A
			// later tick compares the freshly fetched endpoint to this base,
			// never to a candidate tip that was merely published for recovery.
			progress.RefusedBase = seams.LeaseBase
			if progress.RefusedBase == "" {
				progress.RefusedBase = record.BaseTree
			}
			progress.RefusedOrigin = origin
			if storeErr := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); storeErr != nil {
				return errors.Join(pushErr, storeErr)
			}
			if seams.Origin != nil {
				origin, err = seams.Origin()
				if err != nil {
					return errors.Join(pushErr, err)
				}
				progress.RefusedOrigin = origin
				if storeErr := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); storeErr != nil {
					return errors.Join(pushErr, storeErr)
				}
			}
			if origin == progress.RefusedBase {
				return fmt.Errorf("BATCH_LAND_PUSH_REFUSED: origin %s is unchanged after the refused push: %w", origin, pushErr)
			}
			pushed, recoveryErr := recoverNow(origin, tip, pushErr)
			if recoveryErr != nil {
				return recoveryErr
			}
			if !pushed {
				return nil
			}
			tip = progress.PushedTip
		} else {
			progress.PushComplete, progress.PushedTip = true, tip
			progress.RefusedOrigin, progress.RefusedBase, progress.PushRounds = "", "", 0
			if err := store.Update(id, func(current *Record) error { current.Landing = &progress; return nil }); err != nil {
				return err
			}
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

// ReopenMovedTrunk makes a proof-input-changing origin move cross seal and
// proof again on the new base. It never retries the old landing candidate.
func ReopenMovedTrunk(store Store, id, newBaseTree, actor string, at time.Time) error {
	return reopenLandingCandidate(store, id, newBaseTree, actor, at, false, "selected proof input moved")
}

// ReopenRefusedPush gives up a bounded endpoint transaction. Unlike a moved-
// input reopen, the endpoint may still be the batch's recorded base.
func ReopenRefusedPush(store Store, id, newBaseTree, actor string, at time.Time) error {
	return reopenLandingCandidate(store, id, newBaseTree, actor, at, true, "endpoint push refused")
}

func reopenLandingCandidate(store Store, id, newBaseTree, actor string, at time.Time, allowSameBase bool, detail string) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateLanding || newBaseTree == "" || (!allowSameBase && newBaseTree == record.BaseTree) {
		return fmt.Errorf("BATCH_LAND_PUSH_REFUSED: moved trunk did not supply a new landing base")
	}
	units := joinedUnits(record.Units)
	if len(units) == 0 {
		return fmt.Errorf("BATCH_LAND_STATE_REFUSED: batch %s has no joined units", id)
	}
	prefixes, err := assembleUnits(store.root, newBaseTree, units)
	if err != nil {
		var conflict *assemblyConflict
		if !errors.As(err, &conflict) {
			return err
		}
		if updateErr := store.Update(id, func(current *Record) error {
			current.BaseTree = newBaseTree
			return nil
		}); updateErr != nil {
			return updateErr
		}
		if returnErr := RequestReturn(store, id, conflict.GoalID, UnitEjected, conflict.Error(), actor, at); returnErr != nil {
			return returnErr
		}
		return ReassembleSurvivors(store, id, actor, at)
	}
	return store.Update(id, func(current *Record) error {
		current.BaseTree, current.PrefixTrees, current.TipTree = newBaseTree, prefixes, prefixes[len(prefixes)-1]
		current.SelectedGroups, current.Seal, current.Proof, current.Receipts, current.Landing = nil, nil, nil, nil, nil
		current.ClosedReason = ""
		current.Transition(StateOpen, at, "trunk-moved", actor, detail)
		return nil
	})
}

func cloneStrings(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
