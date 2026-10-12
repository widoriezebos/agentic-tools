package main

import (
	"errors"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
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
	var status plain.Status
	for try := 0; try < 3; try++ {
		seams := inv.laneBatchSeams(admitted.home, admitted.record, admitted.owners.plainProve)
		seams.Now = admitted.owners.now
		seams.ReadOnly = true
		seams.AgentProbe = admitted.owners.probe
		seams.Machine = admitted.owners.machine
		status = plain.ObserveStatus(admitted.home, admitted.record, seams, func(readonly plain.ProveSeams) lane.View {
			return inv.laneObservationView(admitted.owners, admitted.home, readonly)
		})
		var changed *plain.Reregistered
		if !errors.As(status.ObservationError, &changed) {
			break
		}
		var problem *intentResult
		admitted, problem = inv.admitLane()
		if problem != nil {
			return inv.render(*problem)
		}
	}
	if status.Health == nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the lane health cannot be read: " + strings.Join(status.Problems, "; ")})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: strings.Join(append([]string{status.Health.Text()}, status.Findings...), "\n"), Data: status.Health})
}
