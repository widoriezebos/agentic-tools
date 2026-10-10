package main

// landing return GOAL --cause CAUSE: the landing agent,
// or a person, gives a goal back to its seat by appending a "returned" line
// to the lane's queue. The seat's claim never left the seat, so nothing
// else is handed back; the seat's next work land G shows the reason.

import (
	"errors"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingReturnCommand() intentCommand {
	return laneCommand(intentCommand{
		object: "landing", action: "return", audience: "both", summary: "give one goal in the landing lane back to its seat, with the reason",
		usage: []string{"metasystem landing return GOAL --cause CAUSE [--reason TEXT] [--by NAME]"},
		details: []string{"The landing agent returns only a goal whose newest check demonstrates its own defect at the waiting commit. A proven person may return any cause. Without --reason, the failed tests and evidence supply the reason.",
			"The seat's work land GOAL then shows it was returned and why; a new commit on its branch is handed in again with work land GOAL.",
			"A goal already returned is left as it is."},
		flags:    []intentFlag{{name: "cause", value: "CAUSE", usage: "own, main, other, flake, environment, or unclassified"}, reasonFlag("because", "why, kept with the return"), {name: "by", value: "NAME", usage: "a proven person returns any cause"}},
		maxArgs:  1,
		examples: []string{"metasystem landing return verbs-match-intent --cause own --reason 'app-standard fails since it joined'"},
	}, runIntentLandingReturn)
}

func runIntentLandingReturn(inv *intentInvocation, admitted laneAdmitted) int {
	targets := laneTargets(admitted.record.Root)
	reason := strings.TrimSpace(inv.input.text("reason"))
	kind := strings.TrimSpace(inv.input.text("cause"))
	if len(inv.input.args) != 1 || !(plain.Cause{Kind: kind}).Valid() {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing return requires one goal and --cause own, main, other, flake, environment, or unclassified; nothing was returned",
			next:    inv.publicArgv("landing", "return", "GOAL", "--cause", "CAUSE"), nextReason: "returns GOAL"})
	}
	goalID := inv.input.args[0]
	targets = append(targets, intentTarget{Kind: "goal", ID: goalID})
	observed, problem := inv.lanePerson("return this goal", admitted.record.Root)
	person := problem == nil
	if !person && inv.input.has("by") {
		return inv.render(*problem)
	}
	seams := inv.laneBatchSeams(admitted.home, admitted.record, admitted.owners.proveSeams(admitted.installation))
	by := ""
	if person {
		by = observed.Name
		seams.Person = &plain.ActProvenance{Kind: "return", Person: observed.Name, Root: observed.Root, CheckedAt: observed.At.UTC(), TerminalGeneration: observed.Proof.TerminalGeneration, TerminalRef: observed.Proof.TerminalRef, Destination: admitted.record}
	}
	entry, changed, err := plain.ReturnProven(admitted.installation, goalID, kind, reason, person, by, admitted.owners.now(), seams)
	_ = plain.SyncPolicyQuestion(admitted.installation, admitted.owners.machine, admitted.owners.now())
	var policyHold *plain.Refusal
	if errors.As(err, &policyHold) {
		return inv.render(landingProveRefusal(inv, targets, err))
	}
	var refused *plain.ReturnRefused
	switch {
	case errors.As(err, &refused):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: refused.Error(), next: inv.publicArgv("landing", "status"), nextReason: "shows the cause"})
	case errors.Is(err, plain.ErrNotWaiting):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "nothing of goal " + goalID + " waits in the landing lane, so nothing was returned",
			next:    inv.publicArgv("landing", "status"), nextReason: "shows the queue", Details: []string{err.Error()}})
	case err != nil:
		return inv.render(landingLaneFailure(targets, "the return of "+goalID+" could not be recorded: "+oneLine(err.Error()), err))
	}
	if !changed {
		summary := goalID + " is already returned: " + entry.Reason
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: entry, Summary: summary, view: landingDone(summary)})
	}
	summary := "returned " + goalID + " at " + shortLandingID(entry.SHA) + " to its seat: " + entry.Reason
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: entry, Summary: summary, Details: inv.writeReturnedCard(goalID), view: landingDone(summary)})
}
