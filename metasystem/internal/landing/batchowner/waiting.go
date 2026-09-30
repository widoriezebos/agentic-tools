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

// RearmLaneAtTrunk re-arms the lane checkout whose module root is
// controlRoot at the fetched trunk commit, the way a landing re-arms at its
// pushed tip, holding the lane checkout like every checkout-moving step. The
// cadence asks it when the lane's engine is behind the trunk it validates.
func RearmLaneAtTrunk(controlRoot, commit string) error {
	return rearmLaneAtTrunkWith(controlRoot, commit, RearmBatchTip)
}

func rearmLaneAtTrunkWith(controlRoot, commit string, rearmTip func(root, tip string) error) error {
	root := controlRoot
	if parent := filepath.Dir(controlRoot); batch.ModuleRoot(parent) == controlRoot {
		root = parent
	}
	return WithLaneCheckout(root, func() error { return rearmTip(root, commit) })
}
