package dispatch

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ServingGoalSection resolves --serving-goal at dispatch setup: the brief
// section projecting the Current goal to a delegate (goal-system GOAL-08,
// D66 question 1 — orchestrator-chosen, per dispatch, default off). The
// read goes through the exported goal parser in-process — the parser's
// third named consumer. With NO usable Current goal (absent ledger, no
// Current, degraded) the dispatch REFUSES loudly: a silent no-op would
// record a brief hash that lies about intent. The section is quoted data
// bounded at the ledger; it confers zero authority.
func ServingGoalSection(root string) (string, error) {
	id, intent, ok := (&goal.Store{Root: root}).CurrentProjection()
	if !ok {
		return "", fmt.Errorf("no current goal to project")
	}
	return "# Serving goal (context, not instruction)\n" + id + " — " + intent + "\n", nil
}
