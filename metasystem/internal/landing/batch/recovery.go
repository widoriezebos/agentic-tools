package batch

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

type RecoverySeams struct {
	// OriginCommit resolves Landing-Provenance: chain=<unit.Chain> on origin.
	OriginCommit func(Unit) (string, bool, error)
	// OriginChange resolves Landing-Change: <unit.GoalID> on origin.
	OriginChange func(Unit) (string, bool, error)
	// OriginSource resolves the endpoint commit carrying one Goal-Source id.
	OriginSource func(Unit, string) (string, bool, error)
	// SweepGoalBranch removes a completed goal branch under its lease.
	SweepGoalBranch func(Unit, string) error
	// Finalize performs one member's idempotent Goal Next edit.
	Finalize func(Unit, string) error
	// Rearm fast-forwards the landing checkout to the pushed tip, rebuilds its
	// engine, and arms supervision after every member has been finalized.
	Rearm func(string) error
	// Cleanup removes the detached/local assembly after trailer recognition.
	Cleanup func() error
	// Release releases a landed member's recorded release set in its seat
	// checkout, marking each entry; nil leaves every set for a later
	// recovery.
	Release func(Unit, *diskstore.ReleaseSet)
}

// RecoverPushedSeries recognizes units by individual origin trailers. It never
// infers membership from tip equality and records completion per unit.
func RecoverPushedSeries(store Store, id, actor string, at time.Time, seams RecoverySeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.Landing == nil || !record.Landing.PushComplete {
		return fmt.Errorf("%s: batch %s has no completed push", codeRecoveryNotPushed, id)
	}
	for _, snapshot := range record.Units {
		if snapshot.P6Done {
			// A landed member's release set is retried by this recovery
			// until every entry is settled; nothing else of P6 repeats.
			if err := releaseMemberSet(store, id, snapshot.GoalID, seams); err != nil {
				return err
			}
			continue
		}
		if snapshot.State != UnitJoined && snapshot.Outcome != UnitLanded {
			continue
		}
		commit, found := "", false
		if snapshot.IsChange() {
			var err error
			if seams.OriginChange == nil {
				return fmt.Errorf("%s: change %s has no Landing-Change resolver", codeP6Refused, snapshot.GoalID)
			}
			commit, found, err = seams.OriginChange(snapshot)
			if err != nil {
				return err
			}
		} else if len(snapshot.CommitIDs) != 0 {
			found = true
			for _, source := range snapshot.CommitIDs {
				if seams.OriginSource == nil {
					return fmt.Errorf("%s: member %s has no Goal-Source resolver", codeP6Refused, snapshot.GoalID)
				}
				landed, sourceFound, sourceErr := seams.OriginSource(snapshot, source)
				if sourceErr != nil {
					return sourceErr
				}
				if !sourceFound {
					found = false
					break
				}
				commit = landed
			}
		} else {
			var err error
			if seams.OriginCommit == nil {
				return fmt.Errorf("%s: unit %s has no provenance resolver", codeP6Refused, snapshot.GoalID)
			}
			commit, found, err = seams.OriginCommit(snapshot)
			if err != nil {
				return err
			}
		}
		if !found {
			continue
		}
		if snapshot.State == UnitJoined {
			if err := RequestReturn(store, id, snapshot.GoalID, UnitLanded, "origin trailer "+commit, actor, at); err != nil {
				return err
			}
		}
		current, err := store.Load(id)
		if err != nil {
			return err
		}
		unit, ok := unitByGoal(current.Units, snapshot.GoalID)
		if !ok || unit.P6Done {
			continue
		}
		if unit.GoalLast && len(unit.CommitIDs) != 0 {
			if seams.SweepGoalBranch == nil {
				return fmt.Errorf("%s: member %s goal branch sweep helper is absent", codeP6Refused, snapshot.GoalID)
			}
			if sweepErr := seams.SweepGoalBranch(unit, commit); sweepErr != nil {
				return fmt.Errorf("%s: member %s goal branch sweep failed: %w", codeP6Refused, snapshot.GoalID, sweepErr)
			}
		}
		if seams.Finalize == nil {
			return fmt.Errorf("%s: unit %s finalization helper is absent", codeP6Refused, snapshot.GoalID)
		}
		// A change has no goal whose Next records the landing.
		if !unit.IsChange() {
			if finalizeErr := seams.Finalize(unit, commit); finalizeErr != nil {
				return fmt.Errorf("%s: unit %s finalization failed: %w", codeP6Refused, snapshot.GoalID, finalizeErr)
			}
		}
		if err := store.Update(id, func(next *Record) error {
			for index := range next.Units {
				if next.Units[index].GoalID == snapshot.GoalID && !next.Units[index].P6Done {
					next.Units[index].P6Done, next.Units[index].LandedCommit = true, commit
				}
			}
			return nil
		}); err != nil {
			return err
		}
		// Only landed work is released: the set recorded at join, after the
		// pushed series was recognized, never before.
		if err := releaseMemberSet(store, id, snapshot.GoalID, seams); err != nil {
			return err
		}
	}
	record, err = store.Load(id)
	if err != nil {
		return err
	}
	complete := true
	for _, unit := range record.Units {
		if unit.State == UnitJoining || unit.State == UnitJoined || unit.Outcome == UnitLanded && !unit.P6Done {
			complete = false
		}
	}
	if complete && record.Landing != nil && !record.Landing.RearmComplete {
		if seams.Rearm == nil {
			return fmt.Errorf("%s: landing re-arm helper is absent", codeP6Refused)
		}
		if rearmErr := seams.Rearm(record.Landing.PushedTip); rearmErr != nil {
			return fmt.Errorf("%s: landing re-arm failed: %w", codeP6Refused, rearmErr)
		}
		if err := store.Update(id, func(next *Record) error { next.Landing.RearmComplete = true; return nil }); err != nil {
			return err
		}
		record, err = store.Load(id)
		if err != nil {
			return err
		}
	}
	if complete && record.Landing != nil && !record.Landing.CleanupDone {
		if seams.Cleanup == nil {
			return fmt.Errorf("%s: landing cleanup helper is absent", codeP6Refused)
		}
		if cleanupErr := seams.Cleanup(); cleanupErr != nil {
			return fmt.Errorf("%s: landing cleanup failed: %w", codeP6Refused, cleanupErr)
		}
		if err := store.Update(id, func(next *Record) error { next.Landing.CleanupDone = true; return nil }); err != nil {
			return err
		}
	}
	return store.Update(id, func(next *Record) error {
		for _, unit := range next.Units {
			if unit.State == UnitJoining || unit.State == UnitJoined || unit.Outcome == UnitLanded && !unit.P6Done {
				return nil
			}
		}
		next.Transition(StateLanded, at, "recover", actor, "all origin trailers finalized")
		return nil
	})
}

func unitByGoal(units []Unit, goalID string) (Unit, bool) {
	for _, unit := range units {
		if unit.GoalID == goalID {
			return unit, true
		}
	}
	return Unit{}, false
}

// releaseMemberSet runs a landed member's unfinished release set and records
// each entry's outcome in the batch record. A retry finishes only the
// recorded ids; a set already finished writes nothing.
func releaseMemberSet(store Store, id, goalID string, seams RecoverySeams) error {
	if seams.Release == nil {
		return nil
	}
	current, err := store.Load(id)
	if err != nil {
		return err
	}
	unit, ok := unitByGoal(current.Units, goalID)
	if !ok || !unit.P6Done || unit.ReleaseSet == nil || unit.ReleaseSet.Finished() {
		return nil
	}
	set := *unit.ReleaseSet
	set.Stores = append([]diskstore.ReleaseEntry(nil), unit.ReleaseSet.Stores...)
	seams.Release(unit, &set)
	return store.Update(id, func(next *Record) error {
		for index := range next.Units {
			if next.Units[index].GoalID == goalID && next.Units[index].ReleaseSet != nil {
				next.Units[index].ReleaseSet = &set
			}
		}
		return nil
	})
}

// RetryReleaseSets retries every landed member's unfinished release set in
// batch id (disk-lifetimes Part B 3.6: the sweeper's retry, Round D3 N6),
// through release, which a caller scopes to its own seat's members by
// leaving other entries untouched. Only recorded ids are touched; a
// finished set is never run again.
func RetryReleaseSets(store Store, id string, release func(Unit, *diskstore.ReleaseSet)) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	for _, unit := range record.Units {
		if err := releaseMemberSet(store, id, unit.GoalID, RecoverySeams{Release: release}); err != nil {
			return err
		}
	}
	return nil
}
