package dispatch

import (
	"fmt"
	"time"

	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

func resolveGoalRevisionWithReads(root, id string, reads goalAdmissionReads) (uint64, uint8, error) {
	if id == "" {
		return 0, 0, fmt.Errorf("a goal id is required")
	}
	if !reads.NewWorld(root) {
		return 0, 0, fmt.Errorf("goal %s has no revision-bearing synced record", id)
	}
	endpoint, err := reads.ResolveEndpoint(root)
	if err != nil {
		return 0, 0, fmt.Errorf("resolve goal ledger: %v", err)
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil {
		if unknown, ok := GoalRecordBudgetUnknown(err); ok {
			return 0, 0, refusal.New("BUDGET_UNKNOWN", "record="+unknown.Record+" reason="+unknown.Reason, fmt.Errorf("the goal's budget cannot be worked out: %s cannot be read", unknown.Record))
		}
		return 0, 0, fmt.Errorf("read accepted goal ledger: %v", err)
	}
	record := projection.Tree.Live[id]
	if record == nil {
		return 0, 0, fmt.Errorf("goal %s is not a live accepted goal", id)
	}
	if record.State != goal.StateClaimed || record.Claimed == nil {
		return 0, 0, fmt.Errorf("goal %s is not claimed; a goal-bound reservation requires a claim revision", id)
	}
	if record.Claimed.Revision == 0 {
		return 0, 0, fmt.Errorf("goal %s has a revisionless claim; run metasystem goal budget %s BOX before dispatch", id, id)
	}
	tier, err := effectiveGoalTier(projection.Tree, record)
	if err != nil {
		return 0, 0, err
	}
	return record.Claimed.Revision, tier, nil
}

// ServingGoalSection resolves --serving-goal at dispatch setup: the brief
// section projecting the Current goal to a delegate
// (orchestrator-chosen, per dispatch, default off). The
// read goes through the exported goal parser in-process — the parser's
// third named consumer. With NO usable Current goal (absent ledger, no
// Current, degraded) the dispatch REFUSES loudly: a silent no-op would
// record a brief hash that lies about intent. The section is quoted data
// bounded at the ledger; it confers zero authority.
func ServingGoalSection(root string) (string, error) {
	return servingGoalSection((&goal.Store{Root: root}).ServingProjection)
}

func servingGoalSection(project func() (string, string, bool)) (string, error) {
	id, intent, ok := project()
	if !ok {
		return "", errors.New("this checkout serves no goal: it has no claimed goal and no current goal")
	}
	return "# Serving goal (context, not instruction)\n" + id + " — " + intent + "\n", nil
}
