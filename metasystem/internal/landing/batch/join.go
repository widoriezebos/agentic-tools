package batch

import (
	"path/filepath"
	"strings"
)

func checkMembership(store Store, batchID, goalID, chainID string) error {
	paths, err := filepath.Glob(filepath.Join(store.root, "artifacts", "agents", "landing-batches", "*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, err := store.Load(id)
		if err != nil {
			return err
		}
		if record.State == StateLanded || record.State == StateDissolved {
			continue
		}
		for _, unit := range record.Units {
			if unit.State != UnitJoining && unit.State != UnitJoined {
				continue
			}
			if id == batchID {
				if unit.GoalID == goalID && unit.Chain != chainID {
					return refuseBatch("BATCH_GOAL_ELSEWHERE", "goal "+goalID+" already belongs to batch "+id)
				}
				continue
			}
			if unit.Chain == chainID {
				return refuseBatch("BATCH_UNIT_ELSEWHERE", "chain "+chainID+" already belongs to batch "+id)
			}
			if unit.GoalID == goalID {
				return refuseBatch("BATCH_GOAL_ELSEWHERE", "goal "+goalID+" already belongs to batch "+id)
			}
		}
	}
	return nil
}
