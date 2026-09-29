package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// goal review: a human's verdict on a goal waiting to land, recorded on the
// goal through the ledger and bound to its review record (g1-s69 D1, D2).

func goalReviewIntentCommands() []intentCommand {
	return []intentCommand{{
		object: "goal", action: "review", audience: "human", summary: "record your verdict on a goal waiting to land",
		usage: []string{"metasystem goal review G --record PATH --verdict clear-to-land|send-back [--brief FILE] [--work NAME]"},
		details: []string{
			"PATH is the review record in the project's review home. Its Goals line names G, its Outcome opens with the verdict and",
			"names the tip it was drafted for, and that tip is the one its Reviewed line records; the verdict is recorded against it.",
			"The record, and with send-back the brief, are published beside the goal's history line in the ledger's own commit.",
			"A record already published with the same words is a repeat; other words at that path are another review's and are refused.",
			"clear-to-land records your word only: nothing lands because of it. send-back carries --brief FILE, the findings answered fix;",
			"the goal leaves the Review lane and the seat that holds it revises. --work names the work item the holder asked you to name.",
		},
		flags: withFlags([]intentFlag{
			intentTargetFlag,
			{name: "record", value: "PATH", usage: "the review record, in the project's review home"},
			{name: "verdict", value: "VERDICT", usage: "clear-to-land or send-back, as the record's Outcome says"},
			{name: "brief", value: "FILE", usage: "with send-back: the correction brief"},
			{name: "work", value: "NAME", usage: "with send-back: the work item the holder asked you to name"},
		}, intentHumanActFlags),
		maxArgs: 1,
		examples: []string{
			"metasystem goal review app-launch-contract --record plans/reviews/review-of-app-launch-contract.md --verdict clear-to-land",
			"metasystem goal review app-launch-contract --record plans/reviews/review-of-app-launch-contract.md --verdict send-back --brief fix.md",
		},
		run: runIntentGoalReview,
	}}
}

func runIntentGoalReview(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	record, verdict := inv.input.text("record"), inv.input.text("verdict")
	if record == "" || verdict == "" {
		return inv.refuse(id, "needs the review record and the verdict; nothing was done",
			"metasystem goal review "+id+" --record PATH --verdict clear-to-land|send-back")
	}
	switch verdict {
	case goal.VerdictSendBack:
		if !inv.input.has("brief") {
			return inv.refuse(id, "a send-back carries its correction brief; nothing was done", "give the findings answered fix with --brief FILE")
		}
	case goal.VerdictClearToLand:
		if inv.input.has("brief") || inv.input.has("work") {
			return inv.refuse(id, "clear-to-land carries no brief and names no work; nothing was done", "drop --brief and --work, or send it back with --verdict send-back")
		}
	default:
		return inv.refuse(id, fmt.Sprintf("the verdict is clear-to-land or send-back, not %s; a review that ends without a verdict records nothing on the goal; nothing was done", shellCommand([]string{verdict})), "")
	}
	actor, _, problem := inv.actingAs("review", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := []string{"--root", inv.stateRoot, "--id", id, "--record", inv.callerPath(record), "--verdict", verdict}
	if inv.input.has("brief") {
		args = append(args, "--brief", inv.callerPath(inv.input.text("brief")))
	}
	if work := inv.input.text("work"); work != "" {
		args = append(args, "--work", work)
	}
	args = append(args, actor...)
	return inv.render(inv.goalAct(id, "review", func(dependencies syncRequestDependencies) int {
		return runGoalReviewWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}))
}

// runGoalReviewWithInputs is the owner of goal review: the record resolved in
// its home, the human's proof, and the one publication.
func runGoalReviewWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	flags := flag.NewFlagSet("goal review", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	by := flags.String("by", "", "the directing human")
	record := flags.String("record", "", "the review record")
	verdict := flags.String("verdict", "", "clear-to-land or send-back")
	brief := flags.String("brief", "", "the correction brief")
	work := flags.String("work", "", "the work item the holder asked to be named")
	lineage := flags.String("lineage", "", "this coordinator's lineage")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		dependencies.complain("goal review takes no positional arguments")
		return 2
	}
	if !converted(*root) {
		dependencies.complain("goal review works only with the synced backlog; migrate this checkout first")
		return 1
	}
	path, content, err := goal.ResolveReviewRecord(*root, *record)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	act := goal.ReviewAct{Record: path, Content: content, Verdict: *verdict, Work: *work}
	if *brief != "" {
		act.Brief, err = os.ReadFile(*brief)
		if err != nil {
			dependencies.complain(fmt.Errorf("cannot read the correction brief: %w", err))
			return 1
		}
	}
	shared := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	proof, err := proveGoalHumanAuthorityAt("review", shared, prove, commandNow)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	request, err := syncReqWithProofAtWithDependencies("review", *root, *by, *lineage, &proof, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	result, err := goal.Review(request, *id, act, &proof)
	return dependencies.publish(result, err)
}
