package main

import (
	"flag"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// goal land-without-sitting: a person's decision that a goal at or above the
// landing gate's tier lands without a review sitting, with the reason, bound
// to the goal branch's current tip (g1-s70 D4). The seat that holds the goal
// lands it on its next turn; a moved tip needs the word again.

func goalLandWithoutSittingIntentCommands() []intentCommand {
	return []intentCommand{{
		object: "goal", action: "land-without-sitting", audience: "human", summary: "let a goal that waits for your review land without a sitting",
		usage: []string{"metasystem goal land-without-sitting G --reason TEXT"},
		details: []string{
			"A goal at or above landing.review.human-from-tier waits for a person before its work lands: a review sitting that ends",
			"clear to land, or this decision. It is recorded on the goal's history against the tip goal/G has at origin now, with your",
			"reason; the seat that holds the goal lands it on its next turn. If the branch moves, the decision no longer stands.",
		},
		flags: withFlags([]intentFlag{
			intentTargetFlag,
			{name: "reason", value: "TEXT", usage: "why this goal needs no sitting, in one line"},
		}, intentHumanActFlags),
		maxArgs:  1,
		examples: []string{"metasystem goal land-without-sitting backlog-ordered-by-priority --reason \"one-line doc fix, read the diff on the card\""},
		run:      runIntentGoalLandWithoutSitting,
	}}
}

func runIntentGoalLandWithoutSitting(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	reason := strings.TrimSpace(inv.input.text("reason"))
	if reason == "" {
		return inv.refuse(id, "landing without a sitting carries your reason; nothing was done",
			"metasystem goal land-without-sitting "+id+" --reason TEXT")
	}
	tip := inv.intentBranchTip(id)
	if tip == "" {
		return inv.refuse(id, "goal/"+id+" has no tip at origin, so there is no work to let land; nothing was done", "")
	}
	actor, _, problem := inv.actingAs("land-without-sitting", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--tip", tip, "--reason", reason}, actor...)
	return inv.render(inv.goalAct(id, "land-without-sitting", func(dependencies syncRequestDependencies) int {
		return runGoalLandWithoutSittingWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}))
}

// runGoalLandWithoutSittingWithInputs is the owner of the decision: the
// human's proof and the one publication.
func runGoalLandWithoutSittingWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	flags := flag.NewFlagSet("goal land-without-sitting", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	by := flags.String("by", "", "the directing human")
	tip := flags.String("tip", "", "the goal branch's tip")
	reason := flags.String("reason", "", "why no sitting")
	lineage := flags.String("lineage", "", "this coordinator's lineage")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if !converted(*root) {
		dependencies.complain("goal land-without-sitting works only with the synced backlog; migrate this checkout first")
		return 1
	}
	shared := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	proof, err := proveGoalHumanAuthorityAt("land-without-sitting", shared, prove, commandNow)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	request, err := syncReqWithProofAtWithDependencies("land-without-sitting", *root, *by, *lineage, &proof, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	result, err := goal.LandWithoutSitting(request, *id, *tip, *reason, &proof)
	return dependencies.publish(result, err)
}
