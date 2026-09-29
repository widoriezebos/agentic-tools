package goal_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func init() {
	goal.LeaseAnnounceForTest = func(root, session string, pid, start, startTicks int64, bootID, tag, runtime, ownerLineage string) error {
		_, err := lease.AnnounceWithPair(root, session, pid, start, startTicks, bootID, tag, runtime, ownerLineage)
		return err
	}
	goal.GoalRevisionAdmissionForTest = func(root, id string, revision uint64, now time.Time) error {
		verdict, err := dispatch.EvaluateGoalRevisionAdmissionForDispatchWithReads(root, id, revision, 1, now, "implementer", "fresh",
			dispatch.ConcreteProofAdmissionReads(), dispatch.HazardMechanical)
		if err != nil {
			return fmt.Errorf("EvaluateGoalRevisionAdmission refused revision %d: %w", revision, err)
		}
		if verdict.Refused() {
			return fmt.Errorf("EvaluateGoalRevisionAdmission refused revision %d: %s %s", revision, verdict.PolicyRefusal,
				strings.Join(dispatch.FormatGoalRevisionAdmission(verdict), "; "))
		}
		return nil
	}
}
