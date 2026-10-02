package main

// landing push (plain lane step 4): the one way the landing agent puts work
// on main. HEAD goes on main only when results.jsonl says green for exactly
// HEAD's tree and origin's main is an ancestor of HEAD, leased at that main.
// Nothing else: a hand-in main then contains reads landed, and its seat
// concludes its goal.

import (
	"errors"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// landedGoals are the waiting hand-ins a push from old to head put on
// main: head contains them and old did not.
func landedGoals(install, checkout, old, head string) []string {
	waiting, _ := plain.Waiting(install)
	inHead, inOld := plain.ContainedIn(checkout, head), plain.ContainedIn(checkout, old)
	var goals []string
	for _, entry := range waiting {
		now, _ := inHead(entry.SHA)
		before, _ := inOld(entry.SHA)
		if now && !before {
			goals = append(goals, entry.Goal)
		}
	}
	return goals
}

func landingPushCommand() intentCommand {
	return laneCommand(intentCommand{
		object: "landing", action: "push", audience: "agent", summary: "put the landing checkout's proven HEAD on main",
		usage: []string{"metasystem landing push"},
		details: []string{"Pushes the landing checkout's HEAD to main only when landing prove recorded green for HEAD's exact tree, and only when HEAD contains origin's main: main is never rewritten.",
			"Each hand-in whose commit main then contains reads landed; its seat sees it in work land and concludes the goal.",
			"Refused while the lane is stopped. A HEAD already on main pushes nothing."},
		maxArgs:  0,
		examples: []string{"metasystem landing push"},
	}, runIntentLandingPush)
}

func runIntentLandingPush(inv *intentInvocation, admitted laneAdmitted) int {
	root := admitted.record.Root
	targets := laneTargets(root)
	if refused := inv.lanePaused(admitted, "pushed"); refused != nil {
		return inv.render(*refused)
	}
	checkout := string(admitted.layout.Checkout)
	outcome, err := plain.Push(admitted.installation, checkout, admitted.owners.now())
	var told []string
	if outcome.Changed {
		// A landing on main is the one piece of news the channel carries
		// (Decision 7); a failed post is kept for a retry and fails nothing.
		if problem := postLanded(admitted.installation, landedGoals(admitted.installation, checkout, outcome.Old, outcome.Commit), outcome.Commit, admitted.owners.now()); problem != nil {
			told = []string{"the channel was not told of the landing; the next landing or tick retries once: " + problem.Error()}
		}
	}
	var refusal *plain.Refusal
	switch {
	case errors.As(err, &refusal):
		result := intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: outcome, Summary: refusal.Reason,
			Details: []string{"refused because: " + refusal.Code}}
		switch refusal.Code {
		case plain.CodeUnproven:
			result.next, result.nextReason = inv.publicArgv("landing", "prove"), "proves HEAD's tree; then push again"
		case plain.CodeRed:
			result.next, result.nextReason = inv.publicArgv("landing", "return", "GOAL", "--reason", "TEXT"), "gives the goal that broke it back to its seat"
		default:
			result.next, result.nextReason = []string{"git", "-C", root, "merge", "origin/main"}, "then prove and push again"
		}
		return inv.render(result)
	case err != nil && outcome.Changed:
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: outcome,
			Summary: "pushed " + shortLandingID(outcome.Commit) + " to main, but the push could not be recorded for landing status: " + oneLine(err.Error()),
			Details: append([]string{err.Error()}, told...)})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: outcome,
			Summary: "the push could not be made: " + oneLine(err.Error()), retry: "tries again", Details: []string{err.Error()}})
	}
	if !outcome.Changed {
		summary := "main already is " + shortLandingID(outcome.Commit) + "; nothing was pushed"
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: outcome, Summary: summary, view: landingDone(summary, root)})
	}
	summary := "pushed " + shortLandingID(outcome.Commit) + " to main (from " + shortLandingID(outcome.Old) + ")"
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: outcome, Summary: summary, view: landingDone(summary, root), Details: told})
}
