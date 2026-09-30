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
		usage: []string{"metasystem goal review G --record PATH --verdict clear-to-land|send-back [--brief FILE] [--work NAME]",
			"metasystem goal review G --record PATH --hold|--release"},
		details: []string{
			"PATH is the review record in the project's review home. Its Goals line names G, its Outcome opens with the verdict and",
			"names the tip it was drafted for, and that tip is the one its Reviewed line records; the verdict is recorded against it.",
			"The record, and with send-back the brief, are published beside the goal's history line in the ledger's own commit.",
			"A record already published with the same words is a repeat; other words at that path are another review's and are refused.",
			"clear-to-land is the word the landing gate waits for at that tip; the holder lands it on its next turn. send-back carries --brief FILE;",
			"the goal leaves the Review lane and the seat that holds it revises. --work names the work item the holder asked you to name.",
			"--hold records that your review sitting of G stands: nothing lands while it stands, whatever the tier, and every seat reads it.",
			"--release ends it; a verdict of yours releases it in the same line. A hold that is not released stands until you release it.",
		},
		flags: withFlags([]intentFlag{
			intentTargetFlag,
			{name: "record", value: "PATH", usage: "the review record, in the project's review home"},
			{name: "verdict", value: "VERDICT", usage: "clear-to-land or send-back, as the record's Outcome says"},
			{name: "brief", value: "FILE", usage: "with send-back: the correction brief"},
			{name: "work", value: "NAME", usage: "with send-back: the work item the holder asked you to name"},
			{name: "hold", usage: "record that your review sitting of the goal stands; nothing lands while it does"},
			{name: "release", usage: "record that your review sitting of the goal has ended"},
		}, intentHumanActFlags),
		maxArgs: 1,
		examples: []string{
			"metasystem goal review app-launch-contract --record plans/reviews/review-of-app-launch-contract.md --verdict clear-to-land",
			"metasystem goal review app-launch-contract --record plans/reviews/review-of-app-launch-contract.md --verdict send-back --brief fix.md",
			"metasystem goal review app-launch-contract --record plans/reviews/review-of-app-launch-contract.md --hold",
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
	if inv.input.switched("hold") || inv.input.switched("release") {
		return runIntentGoalSitting(inv, id, record)
	}
	if record == "" || verdict == "" {
		retry := withoutOption(withoutOption(inv.typedArgv(), "record"), "verdict")
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "a review needs its record file and its verdict; nothing was done",
			next:    append(retry, "--record", reviewRecordOr(record, id), "--verdict", verdictOr(verdict)), nextReason: "or --verdict send-back --brief FILE"})
	}
	switch verdict {
	case goal.VerdictSendBack:
		if !inv.input.has("brief") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: "a send-back needs its brief, the findings to fix; nothing was done",
				next:    append(inv.typedArgv(), "--brief", "FILE"), nextReason: "the file with the findings answered fix"})
		}
	case goal.VerdictClearToLand:
		if inv.input.has("brief") || inv.input.has("work") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: "clear-to-land takes no brief and no work; nothing was done",
				next:    withoutOption(withoutOption(inv.typedArgv(), "brief"), "work"), nextReason: "or send it back with --verdict send-back"})
		}
	default:
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: fmt.Sprintf("the verdict is clear-to-land or send-back, not %s; nothing was done", shellCommand([]string{verdict})),
			next:    append(withoutOption(inv.typedArgv(), "verdict"), "--verdict", goal.VerdictClearToLand), nextReason: "or --verdict send-back --brief FILE"})
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

// runIntentGoalSitting records a review sitting's hold or its release (g1-s70
// D2): a human's line on the goal's history that every seat reads.
func runIntentGoalSitting(inv *intentInvocation, id, record string) int {
	hold := inv.input.switched("hold")
	if hold && inv.input.switched("release") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "--hold starts a sitting and --release ends it; give one; nothing was done",
			next:    withoutFlag(inv.typedArgv(), "release"), nextReason: "or keep --release and drop --hold"})
	}
	for _, other := range []string{"verdict", "brief", "work"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: "--" + other + " goes with a verdict, not with --hold or --release; nothing was done",
				next:    withoutOption(inv.typedArgv(), other)})
		}
	}
	if record == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "a sitting needs its review record file; nothing was done",
			next:    append(inv.typedArgv(), "--record", reviewRecordOr("", id)), nextReason: "the record file of this review"})
	}
	actor, _, problem := inv.actingAs("review", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	word := "--release"
	if hold {
		word = "--hold"
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--record", inv.callerPath(record), word}, actor...)
	return inv.render(inv.goalAct(id, "review", func(dependencies syncRequestDependencies) int {
		return runGoalSittingWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}))
}

// runGoalSittingWithInputs is the owner of a sitting's hold and release: the
// record resolved to its path in the review home, the human's proof, and the
// one publication.
func runGoalSittingWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	flags := flag.NewFlagSet("goal review", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	by := flags.String("by", "", "the directing human")
	record := flags.String("record", "", "the review record")
	hold := flags.Bool("hold", false, "open the sitting's hold")
	release := flags.Bool("release", false, "release the sitting's hold")
	lineage := flags.String("lineage", "", "this coordinator's lineage")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	if flags.Parse(args) != nil || *hold == *release {
		return 2
	}
	if !converted(*root) {
		dependencies.complain("goal review works only with the synced backlog; migrate this checkout first")
		return 1
	}
	path, err := goal.ReviewRecordPath(*root, *record)
	if err != nil {
		dependencies.complain(err)
		return 1
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
	result, err := goal.Sitting(request, *id, path, *hold, &proof)
	return dependencies.publish(result, err)
}

// reviewRecordOr is the typed review record, else the record's usual place
// for goal id: the value a refusal's run line fills in.
func reviewRecordOr(record, id string) string {
	if record != "" {
		return record
	}
	return "plans/reviews/review-of-" + id + ".md"
}

// verdictOr is the typed verdict, else clear-to-land.
func verdictOr(verdict string) string {
	if verdict != "" {
		return verdict
	}
	return goal.VerdictClearToLand
}

// withoutFlag is argv less every --name switch.
func withoutFlag(argv []string, name string) []string {
	var kept []string
	for _, arg := range argv {
		if arg != "--"+name {
			kept = append(kept, arg)
		}
	}
	return kept
}
