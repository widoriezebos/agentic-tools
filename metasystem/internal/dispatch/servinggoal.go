package dispatch

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ResolveGoalRevision reads one live goal from the accepted projection and
// returns the revision dispatch must copy into every reservation record.
// Goal admission owns the separate budget decision.
func ResolveGoalRevision(root, id string) (uint64, error) {
	if id == "" {
		return 0, fmt.Errorf("a goal id is required")
	}
	if !goal.NewWorld(root) {
		return 0, fmt.Errorf("goal %s has no revision-bearing synced record", id)
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return 0, fmt.Errorf("resolve goal ledger: %v", err)
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil {
		if unknown, ok := GoalRecordBudgetUnknown(err); ok {
			return 0, fmt.Errorf("BUDGET_UNKNOWN record=%s reason=%s", unknown.Record, unknown.Reason)
		}
		return 0, fmt.Errorf("read accepted goal ledger: %v", err)
	}
	record := projection.Tree.Live[id]
	if record == nil {
		return 0, fmt.Errorf("goal %s is not a live accepted goal", id)
	}
	if record.State != goal.StateClaimed || record.Claimed == nil {
		return 0, fmt.Errorf("goal %s is not claimed; a goal-bound reservation requires a claim revision", id)
	}
	if record.Claimed.Revision == 0 {
		return 0, fmt.Errorf("goal %s has a revisionless claim; run goal set-budget before dispatch", id)
	}
	return record.Claimed.Revision, nil
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
	id, intent, ok := (&goal.Store{Root: root}).ServingProjection()
	if !ok {
		return "", fmt.Errorf("no serving goal to project: a converted checkout serves this machine's claimed goal, a legacy checkout its Current goal")
	}
	return "# Serving goal (context, not instruction)\n" + id + " — " + intent + "\n", nil
}
