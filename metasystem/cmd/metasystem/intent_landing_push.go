package main

// landing push (simple lane, rail 1): the one way the landing agent puts
// work on main. It pushes the landing checkout's HEAD only when landing
// prove recorded a green result for HEAD's exact tree, and only as a
// fast-forward of main, leased at the main it read; then every waiting
// member main now contains is recorded landed.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func landingPushCommand() intentCommand {
	return laneKernelCommand(intentCommand{
		object: "landing", action: "push", audience: "agent", summary: "put the landing checkout's proven HEAD on main",
		usage: []string{"metasystem landing push"},
		details: []string{"Pushes the landing checkout's HEAD to main only when landing prove recorded a green result for HEAD's exact tree, and only when HEAD contains main: main is never rewritten.",
			"Every member waiting in the lane whose work main then contains is recorded landed, and its goal goes back to its seat as landed.",
			"Refused while the lane is stopped. A HEAD already on main pushes nothing."},
		maxArgs:  0,
		examples: []string{"metasystem landing push"},
	}, runIntentLandingPush)
}

func runIntentLandingPush(inv *intentInvocation, admitted laneKernel) int {
	root := admitted.record.Root
	targets := laneTargets(root)
	actor, err := laneClaimActor(admitted)
	if err != nil {
		return inv.render(landingKernelFailure(targets, "the landing lane's machine can't be named, so nothing was pushed", err))
	}
	outcome, err := admitted.owners.push(kernel.PushRequest{Home: admitted.home, Layout: admitted.layout, Actor: actor})
	status := inv.publicArgv("landing", "status", "--verbose")
	var refused *lane.PublishError
	switch {
	case err == nil:
	case errors.As(err, &refused) && refused.Code == lane.CodeBaseMoved:
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: outcome,
			Summary: fmt.Sprintf("main moved to %s while HEAD was pushed, so nothing was pushed", shortLandingID(refused.Current)),
			next:    []string{"git", "-C", root, "fetch", "origin"}, nextReason: "then merge origin/main, prove and push again",
			Details: []string{"refused because: " + lane.CodeBaseMoved, refused.Error()}})
	case errors.As(err, &refused):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: outcome,
			Summary: strings.TrimSuffix(refused.Message, ", so nothing was published") + ", so nothing was pushed",
			next:    status, nextReason: "shows the lane", Details: []string{"refused because: " + refused.Code, refused.Error()}})
	case outcome.Changed || outcome.Old == outcome.Commit && outcome.Commit != "":
		// Main holds HEAD; what failed is recording its members landed.
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: outcome,
			Summary: "main is " + shortLandingID(outcome.Commit) + ", but its members could not all be recorded landed: " + oneLine(err.Error()),
			retry:   "records them again; main is not pushed twice", Details: []string{err.Error()}})
	default:
		return inv.render(landingKernelRefusal(inv, targets, err))
	}
	landed := "nothing waiting landed with it"
	if len(outcome.Landed) != 0 {
		landed = "landed " + strings.Join(outcome.Landed, ", ")
	}
	if !outcome.Changed {
		summary := "main already is " + shortLandingID(outcome.Commit) + "; " + landed
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: outcome, Summary: summary, view: landingDone(summary, root)})
	}
	summary := "pushed " + shortLandingID(outcome.Commit) + " to main (from " + shortLandingID(outcome.Old) + "); " + landed
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: outcome, Summary: summary, view: landingDone(summary, root)})
}
