package main

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingGiveBackCommand() intentCommand {
	return intentCommand{object: "landing", action: "give-back", laidOut: true, audience: "human", maxArgs: 1,
		summary:  "give a goal back to its seat in any lane state",
		usage:    []string{"metasystem landing give-back G --reason TEXT [--by NAME] [--json]"},
		flags:    []intentFlag{reasonFlag("because", "why the goal is returned"), intentByFlag},
		details:  []string{"A person's running proof continues; its goal is returned when the proof ends. A repeated return changes nothing."},
		examples: []string{"metasystem landing give-back my-goal --reason 'Continue at the seat'"},
		run: func(inv *intentInvocation) int {
			if len(inv.input.args) != 1 || strings.TrimSpace(inv.input.text("reason")) == "" {
				return inv.render(intentResult{Outcome: intentFailed, code: 2, Summary: "landing give-back needs one goal and --reason TEXT"})
			}
			admitted, problem := inv.admitLane()
			if problem != nil {
				if problem.Outcome == intentRefused && inv.laneRecord != nil && inv.laneRecord.Root == "" {
					return inv.render(intentResult{Outcome: intentUnchanged, Summary: "nothing to give back: " + inv.input.args[0] + " is not in the lane"})
				}
				return inv.render(*problem)
			}
			person, problem := inv.lanePerson("give this goal back", admitted.record.Root)
			if problem != nil {
				return inv.render(*problem)
			}
			provenance := plain.ActProvenance{Kind: "return", Person: person.Name, Root: person.Root, CheckedAt: person.At, TerminalGeneration: person.Proof.TerminalGeneration, TerminalRef: person.Proof.TerminalRef, Destination: admitted.record}
			seams := admitted.owners.proveSeams(admitted.installation)
			seams.Now = admitted.owners.now
			entry, message, err := plain.GiveBack(admitted.installation, inv.input.args[0], strings.TrimSpace(inv.input.text("reason")), provenance, seams)
			if err != nil {
				return inv.render(landingLaneFailure(laneTargets(admitted.record.Root), "the give-back could not finish: "+oneLine(err.Error()), err))
			}
			outcome := intentConfirmed
			if strings.HasPrefix(message, entry.Goal+" is already returned:") || strings.HasPrefix(message, "give-back already pending") || strings.HasPrefix(message, "nothing to give back") {
				outcome = intentUnchanged
			}
			details := []string{}
			if outcome == intentConfirmed && entry.State == plain.StateReturned {
				details = inv.writeReturnedCard(entry.Goal)
			}
			return inv.render(intentResult{Outcome: outcome, Summary: message, Data: entry, Details: details})
		}}
}
