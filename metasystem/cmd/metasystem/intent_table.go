package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The rows of the object-action table that are not goal, work or process
// actions: the design object, the practice objects whose actions run a
// machinery verb's own parser, the top-level status, and the hidden process
// entrypoints whose first word is also an object.

// passthroughAction is a public action whose words go, unchanged, to an
// existing handler with its own parser, output and exit codes.
func passthroughAction(object, action, audience, summary string, usage []string, flags []intentFlag, examples []string, run func([]string) int) intentCommand {
	return intentCommand{object: object, action: action, audience: audience, summary: summary, usage: usage,
		flags: flags, examples: examples, maxArgs: -1, passthrough: run}
}

// withLead gives a handler that takes its action word first the same words.
func withLead(lead string, run func([]string) int) func([]string) int {
	return func(args []string) int { return run(append([]string{lead}, args...)) }
}

func documented(name, value, usage string) intentFlag {
	return intentFlag{name: name, value: value, usage: usage}
}

func designIntentCommands() []intentCommand {
	return []intentCommand{
		designCommand(),
		{
			object: "design", action: "show", audience: "both", summary: "one goal's design attempts and proposal",
			usage:    []string{"metasystem design show --goal G [--out FILE] [--attempt N]"},
			details:  []string{"Shows the goal's design attempts, or with --attempt N one attempt and its proposal."},
			flags:    []intentFlag{{name: "goal", value: "G", usage: "the goal whose design records are shown"}, {name: "attempt", value: "N", usage: "one design attempt and its proposal"}, {name: "out", value: "FILE", usage: "the design document, when the goal has several"}},
			maxArgs:  0,
			examples: []string{"metasystem design show --goal verbs-match-intent"},
			run:      func(inv *intentInvocation) int { return runIntentShowRecords(inv, "design", inv.input.args) },
		},
		{
			object: "design", action: "list", audience: "both", summary: "the project's design records",
			usage:    []string{"metasystem design list [--goal G]"},
			flags:    []intentFlag{{name: "goal", value: "G", usage: "only the designs naming this goal"}},
			maxArgs:  0,
			examples: []string{"metasystem design list", "metasystem design list --goal verbs-match-intent"},
			run:      func(inv *intentInvocation) int { return runIntentShowRecords(inv, "designs", inv.input.args) },
		},
		{
			object: "design", action: "review", audience: "both", summary: "independently critique an existing project design",
			usage:     []string{reviewDesignUsage},
			helpForms: designReviewHelpForms(),
			details: []string{
				"An independent critique of a design document; the goal comes from the document or --goal.",
				"Findings stop at the author's decision: decide each finding and repeat with --dispositions FILE [--after N].",
				"An examination round that fails without findings is retried with --retry N under the same round limit.",
			},
			flags: []intentFlag{
				{name: "goal", value: "G", usage: "the goal the design serves (default: the one the document names)"},
				{name: "dispositions", value: "FILE", usage: "the author's decisions on the examined design"},
				{name: "after", value: "N", usage: "the examination the decisions answer"},
				{name: "retry", value: "N", usage: "examine the design once more after examination N failed without findings"},
				{name: "tool-calls", value: "N", usage: "the reader's maximum tool calls, stated in its brief"},
				{name: "effort", value: "VALUE", hidden: true, usage: "refused: every review's reasoning effort is set by its hazard class's configuration obligations"},
				{name: "model", value: "MODEL", hidden: true, usage: "refused: a design critique's critic comes from the roster"},
			},
			maxArgs:  1,
			examples: []string{"metasystem design review plans/designs/intent.md --tool-calls 60"},
			run:      runIntentDesignReview,
		},
		{
			object: "design", action: "stop", audience: "both", summary: "stop a goal's running design attempt",
			usage:    []string{"metasystem design stop G [--out FILE] [--attempt N]"},
			flags:    []intentFlag{{name: "out", value: "FILE", usage: "the design document, when the goal has several"}, {name: "attempt", value: "N", usage: "this attempt instead of the newest"}},
			maxArgs:  1,
			examples: []string{"metasystem design stop verbs-match-intent"},
			run: func(inv *intentInvocation) int {
				if len(inv.input.args) != 1 {
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "design stop needs the goal: metasystem design stop G; nothing was done"})
				}
				return runIntentStopDesign(inv, inv.input.args[0])
			},
		},
		{
			object: "decision", action: "list", audience: "both", summary: "the project's recorded decisions",
			usage:    []string{"metasystem decision list"},
			maxArgs:  0,
			examples: []string{"metasystem decision list"},
			run:      func(inv *intentInvocation) int { return runIntentShowRecords(inv, "decisions", inv.input.args) },
		},
		{
			object: "decision", action: "show", audience: "both", summary: "one project record by id, its references and what references it",
			usage:    []string{"metasystem decision show ID"},
			maxArgs:  1,
			examples: []string{"metasystem decision show 01M3EC3QT7M2TC36P7ZVNRF0RW"},
			run:      func(inv *intentInvocation) int { return runIntentShowRecords(inv, "record", inv.input.args) },
		},
		passthroughAction("design", "find", "both", "the design records that name one goal",
			[]string{"metasystem design find --root CHECKOUT --goal G"},
			[]intentFlag{documented("root", "CHECKOUT", "the checkout whose design records are searched"), documented("goal", "G", "the goal")},
			[]string{"metasystem design find --root . --goal verbs-match-intent"}, runProjectDesignOf),
		passthroughAction("design", "check-moves", "both", "check a design page's moved-effect inventory against the code",
			[]string{"metasystem design check-moves --file PAGE [--root REPOSITORY]"},
			[]intentFlag{documented("file", "PAGE", "the design page"), documented("root", "REPOSITORY", "the repository root containing the metasystem root")},
			[]string{"metasystem design check-moves --file plans/designs/intent.md"}, runValidateMovedEffects),
	}
}

// runIntentDesignReview requests an independent critique of one design file.
func runIntentDesignReview(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "design review needs the design file: " + reviewDesignUsage + "; nothing was done"})
	}
	for _, number := range []string{"retry", "after"} {
		if value, err := strconv.Atoi(inv.input.text(number)); inv.input.has(number) && (err != nil || value < 1) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s takes an examination number such as 1; nothing was done", number)})
		}
	}
	if inv.input.has("effort") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  "no review owner accepts a reasoning-effort override: dispatch sets it from the hazard class's configuration obligations",
			Decision: "omit --effort; the roster and the destructive-reach class decide the critic's effort"})
	}
	if inv.input.has("model") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  "design review takes no --model: the delegate accepts a critic model override only for a run or commit read's code critic",
			Decision: "omit --model; the roster decides the design critic"})
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	return inv.render(inv.reviewDesign(inv.input.args[0]))
}

func practiceIntentCommands() []intentCommand {
	receiptFlags := []intentFlag{documented("root", "CHECKOUT", "the checkout root"), documented("type", "TYPE", "add: the receipt type"),
		documented("outcome", "OUTCOME", "add: the outcome"), documented("goal", "G", "add: the goal the receipt belongs to")}
	frontierFlags := []intentFlag{documented("file", "FILE", "the frontier file (default plans/frontier)"), documented("score", "SCORE", "the candidate score"),
		documented("eval", "COMMAND", "the evaluation command that produced the score"), documented("artifact", "PATH", "the run artifact"),
		documented("min-delta", "N", "the noise floor"), documented("direction", "max|min", "which way is better"), {name: "force", usage: "re-baseline after an evaluation change"}}
	rootJob := []intentFlag{documented("repo", "CHECKOUT", "the checkout root"), documented("root-job", "JOB", "the critic register's root job")}
	contextRoot := documented("root", "ROOT", "the installation or containing template root")
	return []intentCommand{
		passthroughAction("receipt", "add", "agent", "append one task receipt at completion",
			[]string{"metasystem receipt add --type TYPE --outcome OUTCOME [--goal G] [--skills LIST] [--verify RESULT]"}, receiptFlags,
			[]string{"metasystem receipt add --type implementation --outcome done --goal verbs-match-intent"}, withLead("add", runReceipt)),
		passthroughAction("receipt", "check", "both", "exit 1 when a metasystem retro is due",
			[]string{"metasystem receipt check [--root CHECKOUT]"}, receiptFlags[:1], []string{"metasystem receipt check"}, withLead("check", runReceipt)),
		passthroughAction("receipt", "stats", "both", "the retro period's numbers as key=value lines",
			[]string{"metasystem receipt stats [--root CHECKOUT]"}, receiptFlags[:1], []string{"metasystem receipt stats"}, withLead("stats", runReceipt)),
		passthroughAction("receipt", "correct", "agent", "append a correction referencing an existing receipt line",
			[]string{"metasystem receipt correct --line N [...]"}, receiptFlags[:1], []string{"metasystem receipt correct --line 12 --outcome rework"}, withLead("correct", runReceipt)),
		passthroughAction("receipt", "retro", "agent", "record that a retro ran and reset the cadence",
			[]string{"metasystem receipt retro SUMMARY [--root CHECKOUT]"}, receiptFlags[:1], []string{"metasystem receipt retro 'kept 3, reverted 1'"}, withLead("retro", runReceipt)),
		passthroughAction("experiment", "record", "agent", "record the measured-improvement frontier",
			[]string{"metasystem experiment record --score SCORE --eval COMMAND --artifact PATH [--direction max|min] [--force]"}, frontierFlags,
			[]string{"metasystem experiment record --score 0.82 --eval 'go test ./bench/...' --artifact runs/1.json"}, withLead("record", runReportFrontier)),
		passthroughAction("experiment", "challenge", "agent", "test a run against the recorded frontier and its noise floor",
			[]string{"metasystem experiment challenge --score SCORE --eval COMMAND --artifact PATH --min-delta N"}, frontierFlags,
			[]string{"metasystem experiment challenge --score 0.85 --eval 'go test ./bench/...' --artifact runs/2.json --min-delta 0.01"}, withLead("challenge", runReportFrontier)),
		passthroughAction("experiment", "status", "both", "the recorded frontier",
			[]string{"metasystem experiment status [--file FILE]"}, frontierFlags[:1], []string{"metasystem experiment status"}, withLead("status", runReportFrontier)),
		passthroughAction("experiment", "check", "agent", "block another investigation cycle when the ledger's stop-loss fired",
			[]string{"metasystem experiment check --file LEDGER"}, []intentFlag{documented("file", "LEDGER", "the investigation ledger")},
			[]string{"metasystem experiment check --file plans/investigation.md"}, runValidateStopLoss),
		passthroughAction("critique", "rebind-budget", "agent", "copy the goal's raised review-round limit onto an open critic register",
			[]string{"metasystem critique rebind-budget --root-job JOB [--repo CHECKOUT]"}, rootJob,
			[]string{"metasystem critique rebind-budget --root-job crit-01"}, runDispatchCritiqueBudgetRebind),
		passthroughAction("critique", "close-register", "agent", "close or defer an exhausted critic register",
			[]string{"metasystem critique close-register --root-job JOB [--repo CHECKOUT]"}, rootJob,
			[]string{"metasystem critique close-register --root-job crit-01"}, runDispatchCritiqueRegisterClose),
		passthroughAction("covenant", "check", "both", "check the covenant document's shape",
			[]string{"metasystem covenant check [--root DIR]"}, []intentFlag{documented("root", "DIR", "the repository root holding covenant.json")},
			[]string{"metasystem covenant check"}, runCovenantValidate),
		passthroughAction("session", "report", "agent", "read one exact Stop report",
			[]string{"metasystem session report --id ID [--root INSTALLATION]"},
			[]intentFlag{documented("id", "ID", "the Stop report's short alias"), documented("root", "INSTALLATION", "the installation")},
			[]string{"metasystem session report --id r-7f3a"}, runReportStopStatus),
		passthroughAction("session", "handoff", "agent", "record a noted, task-aware context handoff, or cancel one",
			[]string{"metasystem session handoff --root ROOT --note FILE [--no-delegates]", "metasystem session handoff --root ROOT --cancel NONCE"},
			[]intentFlag{contextRoot, documented("note", "FILE", "the lessons note"), documented("cancel", "NONCE", "the live handoff to cancel"), {name: "no-delegates", usage: "no background task remains in flight"}},
			[]string{"metasystem session handoff --root . --note memory/handoff.md"}, runContextHandoff),
		passthroughAction("session", "context", "agent", "the session's recorded context-budget evidence",
			[]string{"metasystem session context --root ROOT [--json]"}, []intentFlag{contextRoot},
			[]string{"metasystem session context --root ."}, runContextStatus),
		passthroughAction("session", "verify", "agent", "verify an immutable context handoff",
			[]string{"metasystem session verify --root ROOT --nonce NONCE"}, []intentFlag{contextRoot, documented("nonce", "NONCE", "the handoff nonce")},
			[]string{"metasystem session verify --root . --nonce 3f2a9c"}, runContextVerify),
		passthroughAction("test", "plan", "both", "preview the risk-selected tests and their reasons without running them",
			[]string{"metasystem test plan --root INSTALLATION [--goal G] --json"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation"), documented("goal", "G", "the goal owning the delivery")},
			[]string{"metasystem test plan --root . --json"}, runTestPlan),
		passthroughAction("test", "list", "both", "every group in the committed testing contract",
			[]string{"metasystem test list [--root INSTALLATION] [--json]"}, []intentFlag{documented("root", "INSTALLATION", "the installation")},
			[]string{"metasystem test list --root ."}, runTestList),
		passthroughAction("test", "check", "both", "validate the testing contract and its declared tools without running tests",
			[]string{"metasystem test check [--root INSTALLATION]"}, []intentFlag{documented("root", "INSTALLATION", "the installation")},
			[]string{"metasystem test check --root ."}, runTestCheck),
		passthroughAction("test", "verify", "both", "check whether retained proof covers an exact tree, without rerunning tests",
			[]string{"metasystem test verify --root INSTALLATION --tree TREE [--goal G] --json"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation"), documented("tree", "TREE", "the exact tree"), documented("goal", "G", "the goal")},
			[]string{"metasystem test verify --root . --tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904 --json"}, runTestVerify),
		passthroughAction("test", "report", "both", "summarize a recorded test result's measured cost",
			[]string{"metasystem test report --result FILE --expensive-ms N"},
			[]intentFlag{documented("result", "FILE", "the recorded test result"), documented("expensive-ms", "N", "the positive threshold for expensive tests")},
			[]string{"metasystem test report --result result.json --expensive-ms 5000"}, runTestReport),
		passthroughAction("work", "watch", "agent", "block until a delegate job or a tracked run is terminal; exit with its pinned code",
			[]string{"metasystem work watch --root CHECKOUT --job J [--caller-pid PID]", "metasystem work watch --root CHECKOUT --run RUN"},
			[]intentFlag{documented("root", "CHECKOUT", "the checkout root"), documented("job", "J", "the delegate job"), documented("run", "RUN", "the tracked run"), documented("caller-pid", "PID", "the caller whose exit ends the watch")},
			[]string{"metasystem work watch --root . --job impl-01"}, runWorkWatch),
		passthroughAction("work", "report", "both", "summarize launch outcomes and refusals",
			[]string{"metasystem work report [--id ID | --goal G [--since RFC3339]] [--json]"},
			[]intentFlag{documented("id", "ID", "one launch"), documented("goal", "G", "one goal's launches"), documented("since", "RFC3339", "activity at or after this instant")},
			[]string{"metasystem work report --goal verbs-match-intent"}, runLaunchReport),
		passthroughAction("work", "check", "agent", "check an implementer job's conformance, or that a critic round's findings are all dispositioned",
			[]string{"metasystem work check --stage review|recertify|merge --job J [--test-command COMMAND]", "metasystem work check --findings RETURN --dispositions FILE [--repo CHECKOUT --root-job JOB]"},
			[]intentFlag{documented("stage", "STAGE", "conformance: review, recertify or merge"), documented("job", "J", "conformance: the implementer job"),
				documented("findings", "RETURN", "critique: the critic return"), documented("dispositions", "FILE", "critique: the dispositions table")},
			[]string{"metasystem work check --stage review --job impl-01", "metasystem work check --findings return.json --dispositions dispositions.md"}, runWorkCheck),
	}
}

// runWorkWatch waits on a delegate job (--job) or a tracked run (--run),
// each through its own waiter.
func runWorkWatch(args []string) int {
	for index, arg := range args {
		name, value, joined := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") || name != "run" {
			continue
		}
		rewritten := slices.Clone(args)
		if joined {
			rewritten[index] = "--id=" + value
		} else {
			rewritten[index] = "--id"
		}
		return runRunWatch(rewritten)
	}
	return runJobWatchVerb(args)
}

// runWorkCheck runs the conformance check for --stage and otherwise the
// critique-closed join.
func runWorkCheck(args []string) int {
	for _, arg := range args {
		if name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "="); strings.HasPrefix(arg, "-") && name == "stage" {
			return runValidateConformance(args)
		}
	}
	return runValidateCritiqueClosed(args)
}

// hiddenIntentEntries are the process entrypoints whose first word is also a
// public object. They keep their existing argument vector exactly: the
// router hands them to their family verb as the internal form does. They are
// listed only by metasystem internal and never named to a person or agent.
func hiddenIntentEntries() []intentCommand {
	entry := func(object, action, launcher string) intentCommand {
		return intentCommand{object: object, action: action, hidden: true, launcher: launcher, maxArgs: -1,
			summary: "process entrypoint (internal)"}
	}
	return []intentCommand{
		entry("goal", "fetch", "internal/seat/launch/sequence.go"),
		entry("goal", "next", "internal/seat/launch/sequence.go"),
		entry("test", "worker", "cmd/metasystem/test.go"),
		entry("test", "worker-capabilities", "cmd/metasystem/test_protection.go"),
		entry("mission", "run-loop", "internal/missionrunner/launch.go"),
		entry("ui", "serve", "internal/ui/lifecycle/launch.go"),
		entry("ui", "tools", "internal/ui/partner/runtime.go"),
	}
}

// topLevelStatus is the one top-level form besides help: the overview of
// this checkout, or one goal's work.
func topLevelStatus() intentCommand {
	return intentCommand{
		object: "status", audience: "both", primary: true, summary: "the overview of this checkout, or one goal's live work",
		usage: []string{"metasystem status", "metasystem status G [--work NAME]"},
		details: []string{
			"Without G: this checkout's MetaSystem, its running work, open questions and what needs attention.",
			"With G: every named work item of the goal with its stage and the command that continues it; goal show G is the goal's record.",
			"Unknown and stale readings are reported as such; a read failure is never shown as stopped or healthy.",
		},
		flags:    []intentFlag{intentInstallationFlag, {name: "work", value: "NAME", usage: "with G: only this named work"}},
		maxArgs:  -1,
		examples: []string{"metasystem status", "metasystem status verbs-match-intent"},
		run:      runIntentTopStatus,
	}
}

func runIntentTopStatus(inv *intentInvocation) int {
	if len(inv.input.args) > 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("status takes one goal at most; unexpected %s; nothing was done", shellCommand(inv.input.args[1:])),
			Decision: "one goal's work is metasystem status G; a job, run or read is metasystem work status REF; every other status is metasystem OBJECT status"})
	}
	if len(inv.input.args) == 1 {
		if inv.input.has("installation") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "status G reads the goal's work; --installation belongs to the overview; nothing was done"})
		}
		return runIntentStatusGoal(inv, inv.input.args[0])
	}
	if inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work names a goal's work: status G --work NAME; nothing was done"})
	}
	return runIntentCheckoutStatus(inv)
}
