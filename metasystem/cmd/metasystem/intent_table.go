package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
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
			usage:     []string{reviewDesignUsage, reviewDesignCheck},
			helpForms: designReviewHelpForms(),
			details: []string{
				"An independent critique of a design document; the goal comes from the document or --goal.",
				"Findings stop at the author's decision: decide each finding and repeat with --dispositions FILE [--after N].",
				"An examination round that fails without findings is retried with --retry N under the same round limit.",
				"The review checks the page's moved-effect inventory against the code; --check-only runs that check alone, without a critic.",
			},
			flags: []intentFlag{
				{name: "goal", value: "G", usage: "the goal the design serves (default: the one the document names)"},
				{name: "dispositions", value: "FILE", usage: "the author's decisions on the examined design"},
				{name: "after", value: "N", usage: "the examination the decisions answer"},
				{name: "retry", value: "N", usage: "examine the design once more after examination N failed without findings"},
				{name: "tool-calls", value: "N", usage: "the reader's maximum tool calls, stated in its brief"},
				{name: "effort", value: "VALUE", hidden: true, usage: "refused: every review's reasoning effort is set by its hazard class's configuration obligations"},
				{name: "model", value: "MODEL", hidden: true, usage: "refused: a design critique's critic comes from the roster"},
				{name: "check-only", usage: "check the page's moved-effect inventory against the code; no critic is asked"},
			},
			maxArgs:  1,
			examples: []string{"metasystem design review plans/designs/intent.md --tool-calls 60", "metasystem design review plans/designs/intent.md --check-only"},
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
	if inv.input.switched("check-only") {
		return runIntentDesignCheckOnly(inv, inv.input.args[0])
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	result := inv.reviewDesign(inv.input.args[0])
	inv.attachMovedEffects(&result, inv.input.args[0])
	return inv.render(result)
}

// runIntentDesignCheckOnly checks a design page's moved-effect inventory
// against the repository's code, without asking a critic.
func runIntentDesignCheckOnly(inv *intentInvocation, file string) int {
	for _, other := range []string{"goal", "dispositions", "after", "retry", "tool-calls"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--check-only asks no critic; --%s belongs to the critique; nothing was done", other)})
		}
	}
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	path := inv.textPath(file)
	page, err := os.ReadFile(path)
	target := []intentTarget{{Kind: "design", ID: file}}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: target, Summary: fmt.Sprintf("the design page %s is unreadable: %v; nothing was checked", shellCommand([]string{file}), err)})
	}
	lines, problems := movedEffectsReport(page, inv.layout.GitRoot)
	data := map[string]any{"problems": problems, "lines": lines}
	if problems > 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: target, Data: data, text: lines,
			Summary: fmt.Sprintf("the moved-effect inventory of %s has %d problem(s)", file, problems)})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: target, Data: data, text: lines,
		Summary: fmt.Sprintf("the moved-effect inventory of %s checks against the code", file)})
}

// attachMovedEffects adds the page's moved-effect check to a design review's
// result, so every review carries it; a result that is not a plain record
// is left as it is.
func (inv *intentInvocation) attachMovedEffects(result *intentResult, file string) {
	page, err := os.ReadFile(inv.textPath(file))
	if err != nil || inv.layout.GitRoot == "" {
		return
	}
	lines, problems := movedEffectsReport(page, inv.layout.GitRoot)
	switch data := result.Data.(type) {
	case nil:
		result.Data = map[string]any{"movedEffects": map[string]any{"problems": problems, "verdict": lines[len(lines)-1]}}
	case map[string]any:
		data["movedEffects"] = map[string]any{"problems": problems, "verdict": lines[len(lines)-1]}
	}
}

func practiceIntentCommands() []intentCommand {
	receiptFlags := []intentFlag{documented("root", "CHECKOUT", "the checkout root"), documented("type", "TYPE", "the receipt type"),
		documented("outcome", "OUTCOME", "the outcome"), documented("goal", "G", "the goal the receipt belongs to"),
		documented("corrects", "EPOCH:SHA1", "correct the receipt line with this epoch and SHA-1 instead of adding one"),
		documented("field", "FIELD", "with --corrects: the corrected field"), documented("was", "VALUE", "with --corrects: the field's recorded value"),
		documented("now", "VALUE", "with --corrects: the field's correct value"), documented("reason", "TEXT", "with --corrects: why")}
	frontierFlags := []intentFlag{documented("file", "FILE", "the frontier file (default plans/frontier)"), documented("score", "SCORE", "the candidate score"),
		documented("eval", "COMMAND", "the evaluation command that produced the score"), documented("artifact", "PATH", "the run artifact"),
		documented("min-delta", "N", "the noise floor"), documented("direction", "max|min", "which way is better"), {name: "force", usage: "re-baseline after an evaluation change"}}
	contextRoot := documented("root", "ROOT", "the installation or containing template root")
	return []intentCommand{
		passthroughAction("receipt", "add", "agent", "append one task receipt at completion, or a correction of one",
			[]string{"metasystem receipt add --type TYPE --outcome OUTCOME [--goal G] [--skills LIST] [--verify RESULT]",
				"metasystem receipt add --corrects EPOCH:SHA1 --field FIELD --was OLD --now NEW --reason TEXT"}, receiptFlags,
			[]string{"metasystem receipt add --type implementation --outcome done --goal verbs-match-intent",
				"metasystem receipt add --corrects 1788441779:3f2a9c1e0d4b5a6978695a4b3c2d1e0f98765432 --field outcome --was done --now rework --reason 'reopened'"}, runReceiptAdd),
		passthroughAction("receipt", "status", "both", "whether a metasystem retro is due, and the period's numbers; exit 1 when due",
			[]string{"metasystem receipt status [--all] [--root CHECKOUT]"},
			[]intentFlag{receiptFlags[0], {name: "all", usage: "count the whole ledger, not only the period since the last retro"}},
			[]string{"metasystem receipt status"}, runReceiptStatus),
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
		passthroughAction("session", "status", "agent", "why this session may or may not stop: one exact Stop report",
			[]string{"metasystem session status --id ID [--root INSTALLATION]"},
			[]intentFlag{documented("id", "ID", "the Stop report's short alias, as the Stop line printed it"), documented("root", "INSTALLATION", "the installation")},
			[]string{"metasystem session status --id r-7f3a"}, runReportStopStatus),
		passthroughAction("session", "handoff", "agent", "hand this session's work to a successor, cancel a handoff, or read the session's context budget",
			[]string{"metasystem session handoff --root ROOT --note FILE [--no-delegates]", "metasystem session handoff --root ROOT --cancel NONCE",
				"metasystem session handoff --status --root ROOT [--json]", "metasystem session handoff --verify NONCE --root ROOT"},
			[]intentFlag{contextRoot, documented("note", "FILE", "the lessons note"), documented("cancel", "NONCE", "the live handoff to cancel"),
				{name: "no-delegates", usage: "no background task remains in flight"},
				{name: "status", usage: "the session's recorded context-budget evidence; nothing is handed off"},
				documented("verify", "NONCE", "a successor verifies the immutable handoff it continues")},
			[]string{"metasystem session handoff --root . --note memory/handoff.md", "metasystem session handoff --status --root .", "metasystem session handoff --verify 3f2a9c --root ."}, runSessionHandoff),
		passthroughAction("session", "isolate", "both", "create an isolated writer worktree for a second session in this checkout",
			[]string{"metasystem session isolate [--root INSTALLATION] [NAME]"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation whose checkout the session isolates from")},
			[]string{"metasystem session isolate"}, runSessionIsolate),
		passthroughAction("test", "plan", "both", "preview the risk-selected tests and their reasons without running them",
			[]string{"metasystem test plan --root INSTALLATION [--goal G] --json"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation"), documented("goal", "G", "the goal owning the delivery")},
			[]string{"metasystem test plan --root . --json"}, runTestPlan),
		passthroughAction("test", "list", "both", "every group in the committed testing contract",
			[]string{"metasystem test list [--root INSTALLATION] [--json]"}, []intentFlag{documented("root", "INSTALLATION", "the installation")},
			[]string{"metasystem test list --root ."}, runTestList),
		passthroughAction("test", "status", "both", "whether an exact tree is already proven, and what its tests cost; nothing is run",
			[]string{"metasystem test status --tree TREE [--goal G] [--root INSTALLATION] [--json]", "metasystem test status --result FILE --expensive-ms N"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation"), documented("tree", "TREE", "the exact tree"), documented("goal", "G", "the goal"),
				documented("result", "FILE", "a recorded test result whose measured cost is summarized"), documented("expensive-ms", "N", "with --result: the positive threshold for expensive tests")},
			[]string{"metasystem test status --tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904 --root . --json", "metasystem test status --result result.json --expensive-ms 5000"}, runTestStatus),
	}
}

// takeIntentFlag removes one option from a passthrough's words: its value
// (when it takes one) and whether it was given, in --name value, --name=value
// or single-dash form.
func takeIntentFlag(args []string, name string, takesValue bool) (string, bool, []string) {
	rest := make([]string, 0, len(args))
	value, found := "", false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			rest = append(rest, args[index:]...)
			break
		}
		spelled, joinedValue, joined := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		if !strings.HasPrefix(arg, "-") || spelled != name {
			rest = append(rest, arg)
			continue
		}
		found = true
		switch {
		case joined:
			value = joinedValue
		case takesValue && index+1 < len(args):
			index++
			value = args[index]
		case !takesValue:
			value = "true"
		}
	}
	return value, found, rest
}

// runReceiptAdd appends a receipt, or with --corrects EPOCH:SHA1 a
// correction of the receipt line with that epoch and SHA-1.
func runReceiptAdd(args []string) int {
	words, problem := receiptAddWords(args)
	if problem != "" {
		fmt.Fprintln(os.Stderr, "metasystem receipt add: "+problem)
		return 2
	}
	return runReceipt(words)
}

// receiptAddWords are the receipt owner's words for a receipt add: an add,
// or with --corrects EPOCH:SHA1 a correction of that line.
func receiptAddWords(args []string) ([]string, string) {
	line, corrects, rest := takeIntentFlag(args, "corrects", true)
	if !corrects {
		return append([]string{"add"}, args...), ""
	}
	epoch, sha, ok := strings.Cut(line, ":")
	if !ok || epoch == "" || sha == "" {
		return nil, "--corrects takes the corrected line's EPOCH:SHA1; nothing was recorded"
	}
	return append([]string{"correct", "--ref-epoch", epoch, "--ref-sha1", sha}, rest...), ""
}

// runReceiptStatus says whether a retro is due and prints the period's
// numbers; its exit code is the due check's (1 when a retro is due).
func runReceiptStatus(args []string) int {
	all, _, checkArgs := takeIntentFlag(args, "all", false)
	if _, named, _ := takeIntentFlag(checkArgs, "file", true); !named {
		// Both reads use the one ledger, resolved once.
		root, err := stateroot.StateRoot(stateroot.Receipts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "receipt:", err)
			return 1
		}
		checkArgs = append(slices.Clone(checkArgs), "--file", filepath.Join(root, "receipts.log"))
	}
	due := runReceipt(append([]string{"check"}, checkArgs...))
	if due > 1 {
		return due
	}
	statsArgs := checkArgs
	if all == "true" {
		statsArgs = append(slices.Clone(checkArgs), "--all")
	}
	if stats := runReceipt(append([]string{"stats"}, statsArgs...)); stats != 0 {
		return stats
	}
	return due
}

// runSessionHandoff hands off, cancels, reads the context budget (--status)
// or verifies a handoff (--verify NONCE) through the context owners.
func runSessionHandoff(args []string) int {
	handler, rest := sessionHandoffRoute(args)
	return handler(rest)
}

// sessionHandoffRoute is the context owner a session handoff's words reach,
// with the words that owner takes.
func sessionHandoffRoute(args []string) (func([]string) int, []string) {
	if _, status, rest := takeIntentFlag(args, "status", false); status {
		return runContextStatus, rest
	}
	if nonce, verify, rest := takeIntentFlag(args, "verify", true); verify {
		return runContextVerify, append(rest, "--nonce", nonce)
	}
	return runContextHandoff, args
}

// runTestStatus reads whether retained proof covers an exact tree, or with
// --result the measured cost of one recorded result; neither runs a test.
func runTestStatus(args []string) int {
	return testStatusRoute(args)(args)
}

// testStatusRoute is the owner a test status's words reach: the result
// report for --result, else the retained-proof verifier.
func testStatusRoute(args []string) func([]string) int {
	if _, result, _ := takeIntentFlag(args, "result", true); result {
		return runTestReport
	}
	return runTestVerify
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
