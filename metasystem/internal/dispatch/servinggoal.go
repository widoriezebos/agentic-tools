package dispatch

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ServingGoalSection resolves --serving-goal at dispatch setup: the brief
// section projecting the Current goal to a delegate
// (orchestrator-chosen, per dispatch, default off). The
// read goes through the exported goal parser in-process — the parser's
// third named consumer. With NO usable Current goal (absent ledger, no
// Current, degraded) the dispatch REFUSES loudly: a silent no-op would
// record a brief hash that lies about intent. The section is quoted data
// bounded at the ledger; it confers zero authority.
func ServingGoalSection(root string) (string, error) {
	id, intent, appetite, ok := (&goal.Store{Root: root}).ServingProjection()
	if !ok {
		return "", fmt.Errorf("no serving goal to project: a converted checkout serves this machine's claimed goal, a legacy checkout its Current goal")
	}
	section := "# Serving goal (context, not instruction)\n" + id + " — " + intent + "\n"
	if appetite != "" {
		section += "Appetite: " + appetite + "\n"
	}
	return section, nil
}
