package main

// landing return GOAL --reason TEXT (plain lane step 5): the landing agent,
// or a person, gives a goal back to its seat by appending a "returned" line
// to the lane's queue. The seat's claim never left the seat, so nothing
// else is handed back; the seat's next work land G shows the reason.

import (
	"errors"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingReturnCommand() intentCommand {
	return laneCommand(intentCommand{
		object: "landing", action: "return", audience: "both", summary: "give one goal in the landing lane back to its seat, with the reason",
		usage: []string{"metasystem landing return GOAL --reason TEXT"},
		details: []string{"The landing agent returns the goal it found turned the tree red or does not merge; the reason is kept with its line in the lane's queue.",
			"The seat's work land GOAL then shows it was returned and why; a new commit on its branch is handed in again with work land GOAL.",
			"A goal already returned is left as it is."},
		flags:    []intentFlag{reasonFlag("because", "why, kept with the return")},
		maxArgs:  1,
		examples: []string{"metasystem landing return verbs-match-intent --reason 'app-standard fails since it joined'"},
	}, runIntentLandingReturn)
}

func runIntentLandingReturn(inv *intentInvocation, admitted laneAdmitted) int {
	targets := laneTargets(admitted.record.Root)
	reason := strings.TrimSpace(inv.input.text("reason"))
	if len(inv.input.args) != 1 || reason == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing return names one goal and the reason it goes back; nothing was returned",
			next:    inv.publicArgv("landing", "return", "GOAL", "--reason", "TEXT"), nextReason: "returns GOAL"})
	}
	goalID := inv.input.args[0]
	targets = append(targets, intentTarget{Kind: "goal", ID: goalID})
	entry, changed, err := plain.Return(admitted.installation, goalID, reason, admitted.owners.now())
	switch {
	case errors.Is(err, plain.ErrNotWaiting):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "nothing of goal " + goalID + " waits in the landing lane, so nothing was returned",
			next:    inv.publicArgv("landing", "status"), nextReason: "shows the queue", Details: []string{err.Error()}})
	case err != nil:
		return inv.render(landingLaneFailure(targets, "the return of "+goalID+" could not be recorded: "+oneLine(err.Error()), err))
	}
	if record := admitted.owners.returnClaim; record != nil {
		err = record(admitted.installation, goalID)
	} else {
		endpoint := admitted.owners.mainEndpoint
		if endpoint == nil {
			endpoint = branch.MainEndpoint
		}
		var e goal.Endpoint
		e, err = endpoint(admitted.installation)
		if err == nil {
			var machine, ulid string
			machine, err = admitted.owners.machine(admitted.installation)
			if err == nil {
				ulid, err = goalUlid()
			}
			if err == nil {
				err = recordClaimHandIn(goal.VerbRequest{Endpoint: e, Actor: goal.Actor{Machine: machine, Lineage: lane.AgentLineage}, Ulid: ulid, Now: admitted.owners.now()}, goalID, true)
			}
		}
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Targets: targets,
			Summary: "the lane returned " + goalID + ", but its claim could not be reopened",
			next:    inv.sameCommand(), nextReason: "retries the claim update", Details: []string{err.Error()}})
	}
	if !changed {
		summary := goalID + " is already returned: " + entry.Reason
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: entry, Summary: summary, view: landingDone(summary)})
	}
	summary := "returned " + goalID + " at " + shortLandingID(entry.SHA) + " to its seat: " + reason
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: entry, Summary: summary, view: landingDone(summary)})
}
