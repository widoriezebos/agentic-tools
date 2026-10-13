package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingIncidentCommand() intentCommand {
	return intentCommand{object: "landing", action: "incident", audience: "human", summary: "clear an engine incident after inspecting its stack",
		usage: []string{"metasystem landing incident clear ID", "metasystem landing incident clear --all"}, maxArgs: 2,
		flags:    []intentFlag{{name: "all", usage: "clear all open engine incidents"}},
		examples: []string{"metasystem landing incident clear 20261013T180300.000000000-merge", "metasystem landing incident clear --all"}, run: func(inv *intentInvocation) int {
			owners, _, record, problem := inv.laneContext(true)
			if problem != nil {
				return inv.render(*problem)
			}
			if _, err := owners.person(record.Install); err != nil {
				return inv.render(landingLaneFailure(nil, "only a person clears an engine incident", err))
			}
			all := inv.input.switched("all")
			if len(inv.input.args) == 0 || inv.input.args[0] != "clear" || all && len(inv.input.args) != 1 || !all && len(inv.input.args) != 2 {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "use landing incident clear ID or landing incident clear --all"})
			}
			id := "all open incidents"
			if !all {
				id = inv.input.args[1]
			}
			if err := plain.ClearEngineIncidents(record.Install, id, all); err != nil {
				return inv.render(landingLaneFailure(nil, "the incident could not be cleared", err))
			}
			return inv.render(intentResult{Outcome: intentConfirmed, Summary: "Engine incident cleared: " + id})
		}}
}
