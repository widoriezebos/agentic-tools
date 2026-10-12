package main

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingHealthCommand() intentCommand {
	return laneCommand(intentCommand{object: "landing", action: "health", audience: "both", maxArgs: 0,
		summary:  "show the lane's health facts and the observations that need attention",
		usage:    []string{"metasystem landing health [--json]"},
		examples: []string{"metasystem landing health", "metasystem landing health --json"},
	}, runIntentLandingHealth)
}
func runIntentLandingHealth(inv *intentInvocation, admitted laneAdmitted) int {
	seams := inv.laneBatchSeams(admitted.home, admitted.record, admitted.owners.plainProve)
	seams.Now = admitted.owners.now
	seams.ReadOnly = true
	status := plain.ReadStatus(admitted.home, admitted.record, inv.laneView(admitted.owners, admitted.home), seams)
	if status.Health == nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the lane health cannot be read: " + strings.Join(status.Problems, "; ")})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: status.Health.Text(), Data: status.Health})
}
