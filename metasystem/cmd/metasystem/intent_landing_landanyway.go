package main

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func landingLandAnywayCommand() intentCommand {
	return intentCommand{object: "landing", action: "land-anyway", laidOut: true, audience: "human", maxArgs: 1, accepts: []string{refGoal},
		summary:  "admit a goal despite main's open incidents, recording the person's decision",
		usage:    []string{"metasystem landing land-anyway G --reason TEXT [--by NAME] [--json]"},
		flags:    []intentFlag{reasonFlag("because", "why to land despite the incidents"), intentByFlag},
		details:  []string{"Also spelled work land G --exception GOAL_LAND_TRUNK_RED --reason TEXT --by NAME. The decision stays on the hand-in and its push."},
		examples: []string{"metasystem landing land-anyway my-goal --reason 'Ship the independent fix'"},
		run: func(inv *intentInvocation) int {
			reason := strings.TrimSpace(inv.input.text("reason"))
			if len(inv.input.args) != 1 || reason == "" {
				return inv.render(intentResult{Outcome: intentFailed, code: 2, Summary: "landing land-anyway needs one goal and --reason TEXT"})
			}
			inv.input.values["reason"] = []string{reason}
			if problem := inv.selectRoot(); problem != nil {
				return inv.render(*problem)
			}
			person, problem := inv.landingPersonCheck("land this goal anyway", inv.layout.InstallationRoot.Path())
			if problem != nil {
				return inv.render(*problem)
			}
			inv.input.values["by"] = []string{person.Name}
			inv.input.values["exception"] = []string{goal.LandTrunkRedCode}
			return runIntentLand(inv)
		},
	}
}
