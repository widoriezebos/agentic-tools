package batchowner

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// GoalInLandingBatch reports whether goalID's change is a live member of a
// batch of the lane installation lands through. The breach-stop scan asks
// it: a claim whose change waits in a batch is not stopped for elapsed time.
// Any read that fails answers false, so the scan stops as before.
func GoalInLandingBatch(installation, goalID string, now time.Time) bool {
	root, configured, err := LandingLaneRoot(installation, now)
	if err != nil || !configured {
		return false
	}
	return goalInBatchesAt(root, goalID)
}

func goalInBatchesAt(landingRoot, goalID string) bool {
	paths, err := filepath.Glob(filepath.Join(landingRoot, "artifacts", "agents", "landing-batches", "*.json"))
	if err != nil {
		return false
	}
	store := batch.NewStore(landingRoot, nil)
	for _, path := range paths {
		record, err := store.Load(strings.TrimSuffix(filepath.Base(path), ".json"))
		if err != nil {
			continue
		}
		for _, unit := range record.Units {
			if unit.GoalID != goalID {
				continue
			}
			switch unit.State {
			case batch.UnitJoining, batch.UnitJoined, batch.UnitReturnPending:
				return true
			}
		}
	}
	return false
}

func init() { dispatch.GoalInLandingBatch = GoalInLandingBatch }
