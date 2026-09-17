package batch

import (
	"fmt"
	"time"
)

type RecoverySeams struct {
	// OriginCommit resolves Landing-Provenance: chain=<unit.Chain> on origin.
	OriginCommit func(Unit) (string, bool, error)
	// OriginSource resolves the endpoint commit carrying one Goal-Source id.
	OriginSource func(Unit, string) (string, bool, error)
	// Finalize performs the idempotent P6 re-arm, Next edit and cleanup.
	Finalize func(Unit, string) error
	// Cleanup removes the detached/local assembly after trailer recognition.
	Cleanup func() error
}

// RecoverPushedSeries recognizes units by individual origin trailers. It never
// infers membership from tip equality and records completion per unit.
func RecoverPushedSeries(store Store, id, actor string, at time.Time, seams RecoverySeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.Landing == nil || !record.Landing.PushComplete {
		return fmt.Errorf("BATCH_RECOVERY_NOT_PUSHED: batch %s has no completed push", id)
	}
	for _, snapshot := range record.Units {
		if snapshot.P6Done || snapshot.State != UnitJoined && snapshot.Outcome != UnitLanded {
			continue
		}
		commit, found := "", false
		if len(snapshot.CommitIDs) != 0 {
			found = true
			for _, source := range snapshot.CommitIDs {
				if seams.OriginSource == nil {
					return fmt.Errorf("BATCH_P6_REFUSED: member %s has no Goal-Source resolver", snapshot.GoalID)
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
				return fmt.Errorf("BATCH_P6_REFUSED: unit %s has no provenance resolver", snapshot.GoalID)
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
		if seams.Finalize == nil {
			return fmt.Errorf("BATCH_P6_REFUSED: unit %s finalization helper is absent", snapshot.GoalID)
		}
		if finalizeErr := seams.Finalize(unit, commit); finalizeErr != nil {
			return fmt.Errorf("BATCH_P6_REFUSED: unit %s finalization failed: %w", snapshot.GoalID, finalizeErr)
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
	if complete && record.Landing != nil && !record.Landing.CleanupDone {
		if seams.Cleanup == nil {
			return fmt.Errorf("BATCH_P6_REFUSED: landing cleanup helper is absent")
		}
		if cleanupErr := seams.Cleanup(); cleanupErr != nil {
			return fmt.Errorf("BATCH_P6_REFUSED: landing cleanup failed: %w", cleanupErr)
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
