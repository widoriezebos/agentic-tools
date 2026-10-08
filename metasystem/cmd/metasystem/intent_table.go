package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The rows of the object-action table that are not goal, work or process
// actions: the design object, the practice objects whose actions run a
// machinery verb's own parser, the top-level status, and the hidden process
// entrypoints whose first word is also an object.

// passthroughAction is a public action whose words go, unchanged, to an
// existing handler with its own parser, output and exit codes.
func passthroughAction(object, action, audience, summary string, usage []string, flags []intentFlag, examples []string, run command) intentCommand {
	// An owner's --root is the repository every public command takes as
	// --repo (its --root spelling still parses): the help says so, and the
	// installation is found from the current directory when neither is given.
	shown := make([]intentFlag, 0, len(flags))
	for _, flag := range flags {
		if flag.name == "root" {
			flag = intentRepoFlag
		}
		shown = append(shown, flag)
	}
	optionalRoot := regexp.MustCompile(`\[--root [A-Z]+\]`)
	requiredRoot := regexp.MustCompile(` --root [A-Z.]+`)
	lines := make([]string, len(usage))
	for index, line := range usage {
		lines[index] = requiredRoot.ReplaceAllString(optionalRoot.ReplaceAllString(line, "[--repo PATH]"), "")
	}
	shownExamples := make([]string, len(examples))
	for index, example := range examples {
		shownExamples[index] = strings.ReplaceAll(example, " --root .", "")
	}
	command := intentCommand{object: object, action: action, audience: audience, summary: summary, usage: lines,
		flags: shown, examples: shownExamples, maxArgs: -1, owner: run}
	command.passthrough = func(args []string, stdout, stderr io.Writer) int {
		return runPassthrough(command, run, args, stdout, stderr)
	}
	return command
}

// runPassthrough gives a passthrough action what every public command has:
// --repo, and the installation found from the current directory or --repo
// when its help shows --root as optional (its owner takes it as --root; a
// receipt action also takes the ledger it writes). What the owner answers
// is its own.
func runPassthrough(command intentCommand, run command, args []string, stdout, stderr io.Writer) int {
	label := "metasystem " + command.object + " " + command.action
	repo, repoGiven, rest := takeIntentFlag(args, "repo", true)
	if repoGiven && repo == "" {
		retry := append(strings.Fields(label), args...)
		if cwd, err := os.Getwd(); err == nil {
			retry = append(append(strings.Fields(label), withoutOption(args, "repo")...), "--repo", cwd)
		}
		fmt.Fprintf(stderr, "--repo needs the repository's directory; nothing was done\nrun: %s\n", shellCommand(retry))
		return 2
	}
	documentsRoot := slices.ContainsFunc(command.flags, func(flag intentFlag) bool { return flag.name == "repo" })
	_, rootGiven, _ := takeIntentFlag(rest, "root", true)
	_, fileGiven, _ := takeIntentFlag(rest, "file", true)
	if repoGiven && !documentsRoot {
		fmt.Fprintf(stderr, "%s: works on the files it names and takes no --repo; nothing was done\n", label)
		return 2
	}
	if command.object == "test" && command.action == "groups" {
		if repoGiven {
			rest = append([]string{"--root", repo}, rest...)
		}
		return run(rest, stdout, stderr)
	}
	receipts := command.object == "receipt" && !fileGiven
	if !documentsRoot || rootGiven && !receipts {
		return run(rest, stdout, stderr)
	}
	path := repo
	if path == "" {
		path = "."
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	resolver := stateroot.NewResolver(stateroot.RepositoryTop, os.Executable)
	layout, err := resolver.ResolveLayout(path)
	if err != nil {
		if repoGiven {
			retry := append(append(strings.Fields(label), withoutOption(args, "repo")...), "--repo", "REPOSITORY")
			fmt.Fprintf(stderr, "%s isn't inside a checkout MetaSystem is set up in; nothing was done\nrun: %s  (REPOSITORY is such a checkout)\n", path, shellCommand(retry))
			return 2
		}
		// Outside a repository the owner answers for its own --root.
		return run(rest, stdout, stderr)
	}
	var extra []string
	if !rootGiven {
		extra = append(extra, "--root", layout.InstallationRoot.Path())
	}
	if receipts {
		stateRoot, err := resolver.RootForInstallation(layout.InstallationRoot)
		relative, relErr := stateroot.RelativeRoot(stateroot.Receipts)
		if err == nil && relErr == nil {
			extra = append(extra, "--file", stateRoot.Path(relative, "receipts.log"))
		}
	}
	// The options go after the leading words the usage shows before them
	// (receipt retro SUMMARY), before any other word.
	leading := 0
	if len(command.usage) > 0 {
		words := strings.Fields(command.usage[0])
		for _, word := range words[min(3, len(words)):] {
			if strings.HasPrefix(word, "[") || strings.HasPrefix(word, "-") {
				break
			}
			leading++
		}
	}
	at := 0
	for at < len(rest) && at < leading && !strings.HasPrefix(rest[at], "-") {
		at++
	}
	return run(slices.Concat(rest[:at], extra, rest[at:]), stdout, stderr)
}

func documented(name, value, usage string) intentFlag {
	return intentFlag{name: name, value: value, usage: usage}
}

func designIntentCommands() []intentCommand {
	return []intentCommand{
		designCommand(),
		{
			object: "design", action: "show", audience: "both", summary: "one goal's design attempts and proposal",
			usage:    []string{"metasystem design show G [--out FILE] [--attempt N]"},
			details:  []string{"Shows the goal's design attempts, or with --attempt N one attempt and its proposal."},
			flags:    []intentFlag{{name: "goal", value: "G", advanced: true, usage: "the goal, as an alternative to naming it first"}, {name: "attempt", value: "N", usage: "one design attempt and its proposal"}, {name: "out", value: "FILE", usage: "the design document, when the goal has several"}},
			maxArgs:  1,
			examples: []string{"metasystem design show verbs-match-intent"},
			run:      runIntentDesignShow,
		},
		{
			object: "design", action: "list", audience: "both", summary: "the project's design records",
			usage:    []string{"metasystem design list [--goal G]"},
			flags:    []intentFlag{{name: "goal", value: "G", usage: "only the designs naming this goal"}},
			maxArgs:  0,
			examples: []string{"metasystem design list", "metasystem design list --goal verbs-match-intent"},
			run:      runIntentDesignList,
		},
		{
			object: "design", action: "check", audience: "both", summary: "check a plan's obligation matrix: is every critical or high obligation proven",
			usage: []string{"metasystem design check FILE... [--complete]"},
			details: []string{
				"Checks the structure and declared state of each file's design-obligation matrix (docs/design/design-obligation-gate.md); it reads, never changes, the files.",
				"By default a critical or high obligation may still await its one named runtime proof (READY_FOR_RUNTIME); --complete, the completion gate, requires every one DONE.",
				"Proof and owner cells on critical and high rows must name something concrete; a passing check does not prove the named tests are truthful.",
			},
			flags:    []intentFlag{{name: "complete", usage: "the completion gate: every critical or high obligation is DONE"}},
			maxArgs:  -1,
			examples: []string{"metasystem design check plans/rate-limit.md", "metasystem design check plans/rate-limit.md --complete"},
			run:      runIntentDesignCheck,
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
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "design stop needs the goal whose design author to stop; nothing was done",
						next: inv.typedArgvFor("GOAL"), nextReason: "names the goal"})
				}
				return runIntentStopDesign(inv, inv.input.args[0])
			},
		},
		{
			object: "decision", action: "list", audience: "both", summary: "the project's recorded decisions",
			usage:    []string{"metasystem decision list"},
			maxArgs:  0,
			examples: []string{"metasystem decision list"},
			run:      runIntentDecisionList,
		},
		{
			object: "decision", action: "show", audience: "both", summary: "one project record by id, its references and what references it",
			usage:    []string{"metasystem decision show ID"},
			maxArgs:  1,
			examples: []string{"metasystem decision show 01M3EC3QT7M2TC36P7ZVNRF0RW"},
			run:      runIntentDecisionShow,
		},
	}
}

// runIntentDesignReview requests an independent critique of one design file.
func runIntentDesignReview(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "design review needs the one design file to review; nothing was done",
			next: inv.typedArgvFor("FILE"), nextReason: "FILE is the design page", Details: []string{"usage: " + reviewDesignUsage}})
	}
	for _, number := range []string{"retry", "after"} {
		if value, err := strconv.Atoi(inv.input.text(number)); inv.input.has(number) && (err != nil || value < 1) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s takes a review number such as 1; nothing was done", number),
				next: append(inv.typedArgvLess(number), "--"+number, "1"), nextReason: "the review number that metasystem design show lists"})
		}
	}
	if inv.input.has("effort") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "a critic's effort is set by the project's configuration, not by --effort; nothing was done",
			next:    inv.typedArgvLess("effort"), nextReason: "the roster and the change's reach decide the effort"})
	}
	if inv.input.has("model") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "a design critic is chosen by the roster, not by --model; nothing was done",
			next:    inv.typedArgvLess("model"), nextReason: "--model is only for a code review's critic"})
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
				Summary: fmt.Sprintf("--check-only asks no critic, so --%s doesn't apply; nothing was done", other),
				next:    inv.typedArgvLess(other), nextReason: "checks the page without --" + other})
		}
	}
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	path := inv.textPath(file)
	page, err := os.ReadFile(path)
	target := []intentTarget{{Kind: "design", ID: file}}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: target, Summary: fileProblem("design page", path, err) + "; nothing was checked",
			next: inv.sameCommand(), nextReason: "once it names a readable page"})
	}
	lines, problems := movedEffectsReport(page, inv.layout.GitRoot)
	data := map[string]any{"problems": problems, "lines": lines}
	if problems > 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: target, Data: data, text: lines,
			Summary: fmt.Sprintf("the moved-effect inventory of %s has %d problem(s), listed above", file, problems),
			next:    inv.sameCommand(), nextReason: "after fixing them in the page"})
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
		passthroughAction("receipt", "status", "both", "whether a metasystem retro is due, and the period's numbers; exit 1 when due; with --uncovered the lines no retro has read",
			[]string{"metasystem receipt status [--all] [--root CHECKOUT]", "metasystem receipt status --uncovered [--json] [--verbose] [--root CHECKOUT]"},
			[]intentFlag{receiptFlags[0], {name: "all", usage: "count the whole ledger, not only the period since the last retro"},
				{name: "uncovered", usage: "print exactly the lines no retro has covered, in ledger order, and the token the retro marker takes"},
				{name: "json", usage: "with --uncovered: print the lines, their digests and the token as JSON"},
				{name: "verbose", usage: "with --uncovered: the ledger lines exactly as they stand"}},
			[]string{"metasystem receipt status", "metasystem receipt status --uncovered"}, runReceiptStatus),
		passthroughAction("receipt", "retro", "agent", "record that a retro ran and reset the cadence, and which lines it read",
			[]string{"metasystem receipt retro SUMMARY [--covered TOKEN] [--root CHECKOUT]"},
			[]intentFlag{receiptFlags[0], documented("covered", "TOKEN", "the token receipt status --uncovered printed: the lines this retro read")},
			[]string{"metasystem receipt retro 'kept 3, reverted 1' --covered 12:4f1a2b3c4d5e"}, runReceiptRetro),
		passthroughAction("experiment", "record", "agent", "record the measured-improvement frontier",
			[]string{"metasystem experiment record --score SCORE --eval COMMAND --artifact PATH [--direction max|min] [--force]"}, frontierFlags,
			[]string{"metasystem experiment record --score 0.82 --eval 'go test ./bench/...' --artifact runs/1.json"}, runExperimentRecord),
		passthroughAction("experiment", "challenge", "agent", "test a run against the recorded frontier and its noise floor",
			[]string{"metasystem experiment challenge --score SCORE --eval COMMAND --artifact PATH --min-delta N"}, frontierFlags,
			[]string{"metasystem experiment challenge --score 0.85 --eval 'go test ./bench/...' --artifact runs/2.json --min-delta 0.01"}, runExperimentChallenge),
		passthroughAction("experiment", "status", "both", "the recorded frontier",
			[]string{"metasystem experiment status [--file FILE]"}, frontierFlags[:1], []string{"metasystem experiment status"}, runExperimentStatus),
		passthroughAction("experiment", "check", "agent", "block another investigation cycle when the ledger's stop-loss fired",
			[]string{"metasystem experiment check --file LEDGER"}, []intentFlag{documented("file", "LEDGER", "the investigation ledger")},
			[]string{"metasystem experiment check --file plans/investigation.md"}, runValidateStopLoss),
		passthroughAction("session", "status", "agent", "why this session may or may not stop: one exact Stop report",
			[]string{"metasystem session status --id ID [--root INSTALLATION]"},
			[]intentFlag{documented("id", "ID", "the Stop report's short alias, as the Stop line printed it"), documented("root", "INSTALLATION", "the installation"),
				{name: "verbose", usage: "also the plan lines older than a week, every health role and where the whole report is"}},
			[]string{"metasystem session status --id 7f3a"}, runReportStopStatus),
		passthroughAction("session", "handoff", "agent", "hand this session's work to a successor, cancel a handoff, or read the session's context budget",
			[]string{"metasystem session handoff --root ROOT --note FILE [--no-delegates]", "metasystem session handoff --root ROOT --cancel NONCE",
				"metasystem session handoff --status --root ROOT [--json]", "metasystem session handoff --verify NONCE --root ROOT"},
			[]intentFlag{contextRoot, documented("note", "FILE", "the lessons note"), documented("cancel", "NONCE", "the live handoff to cancel"),
				{name: "no-delegates", usage: "no background task remains in flight"},
				{name: "status", usage: "the session's recorded context-budget evidence; nothing is handed off"},
				documented("verify", "NONCE", "a successor verifies the immutable handoff it continues"),
				{name: "verbose", usage: "also a refusal's code and facts"}},
			[]string{"metasystem session handoff --root . --note memory/handoff.md", "metasystem session handoff --status --root .", "metasystem session handoff --verify 3f2a9c --root ."}, runSessionHandoff),
		passthroughAction("session", "isolate", "both", "create an isolated writer worktree for a second session in this checkout",
			[]string{"metasystem session isolate [--root INSTALLATION] [NAME]"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation whose checkout the session isolates from")},
			[]string{"metasystem session isolate"}, runSessionIsolate),
		passthroughAction("test", "plan", "both", "preview the risk-selected tests and their reasons without running them",
			[]string{"metasystem test plan [--goal G] [--root INSTALLATION] [--json] [--verbose]"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation"), documented("goal", "G", "the goal owning the delivery"), testVerboseFlag},
			[]string{"metasystem test plan", "metasystem test plan --goal verbs-match-intent --json"}, runTestPlan),
		passthroughAction("test", "add", "agent", "add verified Go tests, group inputs or surface paths to the testing contract",
			[]string{"metasystem test add --file FILE --group ID --tests NAME,NAME", "metasystem test add --file FILE --group ID --inputs PATH,PATH",
				"metasystem test add --file FILE --surface ID --paths PATH,PATH"},
			[]intentFlag{documented("file", "FILE", "the testing contract"), documented("group", "ID", "the group the tests or inputs join"),
				documented("tests", "NAME,NAME", "Go test names, each verified in the group's packages"),
				documented("inputs", "PATH,PATH", "input paths the group gains; one it already lists is already added"),
				documented("surface", "ID", "the surface the paths join"),
				documented("paths", "PATH,PATH", "paths the surface gains; one it already lists is already added")},
			[]string{"metasystem test add --file testing.json --group verb-ratchet --tests TestEveryInternalVerbHasALauncherThatStartsIt",
				"metasystem test add --file testing.json --group goal-full-coverage --inputs metasystem/testing-coverage-floors.json"}, runTestingAddTests),
		passthroughAction("test", "remove", "agent", "take deleted Go tests, group inputs, surface paths, a group or a surface out of the testing contract",
			[]string{"metasystem test remove --file FILE --group ID --tests NAME,NAME", "metasystem test remove --file FILE --group ID --inputs PATH,PATH",
				"metasystem test remove --file FILE --group ID", "metasystem test remove --file FILE --surface ID --paths PATH,PATH",
				"metasystem test remove --file FILE --surface ID"},
			[]intentFlag{documented("file", "FILE", "the testing contract"), documented("group", "ID", "the group the tests or inputs leave; alone, the group itself and every reference to it"),
				documented("tests", "NAME,NAME", "Go test names the group lists; a name it no longer lists is already removed"),
				documented("inputs", "PATH,PATH", "input paths the group lists; a path it no longer lists is already removed"),
				documented("surface", "ID", "the surface the paths leave; alone, the surface itself and every dependsOn naming it"),
				documented("paths", "PATH,PATH", "paths the surface lists; a path it no longer lists is already removed")},
			[]string{"metasystem test remove --file testing.json --group verb-ratchet --tests TestLauncherRuleRefusesAVerbNothingStarts",
				"metasystem test remove --file testing.json --group go-affected --inputs 'metasystem/scripts/**'"}, runTestingRemoveTests),
		passthroughAction("test", "baseline", "agent", "record or check the trusted baseline a refactor proceeds from",
			[]string{"metasystem test baseline --gate COMMAND [--file FILE] [--root INSTALLATION]",
				"metasystem test baseline --check [--file FILE] [--max-age-minutes N] [--max-commits N] [--root INSTALLATION]"},
			[]intentFlag{documented("gate", "COMMAND", "the acceptance gate that passed at this clean HEAD; records it as the trusted baseline"),
				{name: "check", usage: "may another edit batch start: clean worktree, the baseline an ancestor, the cadence not exceeded"},
				documented("file", "FILE", "the baseline file (default plans/refactor-baseline)"),
				documented("max-age-minutes", "N", "with --check: the cadence's age limit"), documented("max-commits", "N", "with --check: the cadence's commit limit"),
				documented("root", "INSTALLATION", "the installation whose metasystem.conf supplies the cadence")},
			[]string{"metasystem test baseline --gate 'go test ./...'", "metasystem test baseline --check"}, runTestBaseline),
		passthroughAction("session", "wait", "agent", "say this session waits for a running process or a person's answer, so it may stop",
			[]string{"metasystem session wait --pid PID --label TEXT [--job J] [--timeout DURATION]",
				"metasystem session wait --label TEXT --timeout DURATION",
				"metasystem session wait --question TEXT --timeout DURATION",
				"metasystem session wait --end WAIT-ID"},
			[]intentFlag{documented("pid", "PID", "the running process this session waits for"), documented("label", "TEXT", "what that process is doing"),
				documented("job", "J", "the delegate job the process belongs to"), documented("question", "TEXT", "the question a person is to answer"),
				documented("timeout", "DURATION", "how long the wait lasts, at most 24h; required with --question (default 4h)"),
				documented("end", "WAIT-ID", "the wait is over"), documented("root", "PATH", "the checkout (default: the current directory)"),
				{name: "json", usage: "print the recorded wait as JSON"}},
			[]string{"metasystem session wait --pid 4242 --label 'release build'", "metasystem session wait --question 'Ship it?' --timeout 2h", "metasystem session wait --end 0123456789abcdef0123456789abcdef"}, runSessionWait),
		passthroughAction("test", "groups", "both", "run named contract groups and their prerequisites for the proof command, without recording proof",
			[]string{"metasystem test groups ID... [--environment] [--root INSTALLATION]", "metasystem test groups --environment [--root INSTALLATION]"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation"), {name: "environment", usage: "with no ids, print only the landing environment"}},
			[]string{"metasystem test groups fast-static-build", "metasystem test groups --environment"}, runTestGroups),
		passthroughAction("test", "list", "both", "every group in the committed testing contract",
			[]string{"metasystem test list [--root INSTALLATION] [--json]"}, []intentFlag{documented("root", "INSTALLATION", "the installation")},
			[]string{"metasystem test list --root ."}, runTestList),
		passthroughAction("test", "status", "both", "whether an exact tree is already proven, and what its tests cost; nothing is run",
			[]string{"metasystem test status --tree TREE [--goal G] [--root INSTALLATION] [--json] [--verbose]", "metasystem test status --result FILE --expensive-ms N"},
			[]intentFlag{documented("root", "INSTALLATION", "the installation"), documented("tree", "TREE", "the exact tree"), documented("goal", "G", "the goal"),
				documented("result", "FILE", "a recorded test result whose measured cost is summarized"), documented("expensive-ms", "N", "with --result: the positive threshold for expensive tests"),
				testVerboseFlag},
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
func runReceiptAdd(args []string, stdout, stderr io.Writer) int {
	words, problem := receiptAddWords(args)
	if problem != "" {
		return refusePassthrough(stderr, 2, problem, textui.Hint{Argv: []string{"metasystem", "receipt", "add", "--help"}, Reason: "its forms"})
	}
	return runReceipt(words, stdout, stderr)
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
func runReceiptStatus(args []string, stdout, stderr io.Writer) int {
	return receiptStatus(args, stdout, stderr)
}

// runSessionHandoff hands off, cancels, reads the context budget (--status)
// or verifies a handoff (--verify NONCE) through the context owners.
func runSessionHandoff(args []string, stdout, stderr io.Writer) int {
	handler, rest := sessionHandoffRoute(args)
	return handler(rest, stdout, stderr)
}

// sessionHandoffRoute is the context owner a session handoff's words reach,
// with the words that owner takes.
func sessionHandoffRoute(args []string) (command, []string) {
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
func runTestStatus(args []string, stdout, stderr io.Writer) int {
	return runTestStatusWithInputs(args, stdout, stderr, testStatusInputs{})
}

func runTestStatusWithInputs(args []string, stdout, stderr io.Writer, inputs testStatusInputs) int {
	route := testStatusRoute(args)
	if _, result, _ := takeIntentFlag(args, "result", true); result {
		// A recorded result is read on its own; the installation found for
		// it is not an option the report takes.
		_, _, args = takeIntentFlag(args, "root", true)
		return route(args, stdout, stderr)
	}
	return runTestVerifyAsWithStatus("test status", args, stdout, stderr, inputs)
}

// testStatusRoute is the owner a test status's words reach: the result
// report for --result, else the retained-proof verifier.
func testStatusRoute(args []string) command {
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
		entry("test", "worker", "cmd/metasystem/test_protection.go"),
		entry("test", "worker-capabilities", "internal/testrun/worker.go"),
		entry("mission", "run-loop", "internal/missionrunner/launch.go"),
		entry("app", "serve", "internal/applaunch/launch.go"),
		entry("ui", "serve", "internal/ui/lifecycle/launch.go"),
		entry("ui", "tools", "internal/ui/partner/runtime.go"),
		entry("landing", "observe", "cmd/metasystem/landing_path.go"),
		entry("landing", "workspace", "cmd/metasystem/landing_path.go"),
	}
}

// topLevelStatus is the one top-level form besides help: the overview of
// this checkout, or one goal's work.
func topLevelStatus() intentCommand {
	return intentCommand{
		object: "status", audience: "both", primary: true, laidOut: true, summary: "the overview of this checkout, or one goal's live work",
		usage: []string{"metasystem status", "metasystem status G [--work NAME]"},
		details: []string{
			"Without G: this checkout's MetaSystem, its running work, open questions and what needs attention.",
			"With G: every named work item of the goal with its stage and the command that continues it; goal show G is the goal's record.",
			"Unknown and stale readings are reported as such; a read failure is never shown as stopped or healthy.",
			"The board block names each seat of this host with what it works on and how far it is, then one line per unfinished landing batch; --verbose adds one line per goal.",
		},
		flags:    []intentFlag{intentInstallationFlag, {name: "work", value: "NAME", usage: "with G: only this named work"}, intentVerboseFlag},
		maxArgs:  -1,
		examples: []string{"metasystem status", "metasystem status verbs-match-intent", "metasystem status --verbose"},
		run:      runIntentTopStatus,
	}
}

func runIntentTopStatus(inv *intentInvocation) int {
	if len(inv.input.args) > 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("status takes one goal at most; unexpected %s; nothing was done", shellCommand(inv.input.args[1:])),
			next:    inv.publicArgv("status", inv.input.args[0]), nextReason: "a job, run or read is metasystem work status REF"})
	}
	if len(inv.input.args) == 1 {
		if inv.input.has("installation") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--installation is for the overview, not one goal's status; nothing was done",
				next: inv.typedArgvLess("installation"), nextReason: "the goal's status"})
		}
		return runIntentStatusGoal(inv, inv.input.args[0])
	}
	if inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work needs the goal whose work it names; nothing was done",
			next: inv.typedArgvFor("GOAL"), nextReason: "names the goal"})
	}
	return runIntentCheckoutStatus(inv)
}

// hostBoardView is the board as this checkout's one-shot views show it. Its
// claim check reads the goal ledger at the state root, where goal list reads
// it: the ledger's files are read relative to the root given, and a template
// checkout's repository top has none, so every card read "not claimed" (F3).
func (inv *intentInvocation) hostBoardView(now time.Time) board.View {
	ledgerRoot := inv.stateRoot
	if ledgerRoot == "" {
		ledgerRoot = inv.layout.GitRoot
	}
	var view board.View
	if inv.owners.delivery != nil && inv.owners.delivery.boardView != nil {
		view = inv.owners.delivery.boardView(ledgerRoot, now)
	} else {
		view = batchowner.ProductionPipeline(batchowner.PipelineStall(inv.layout.InstallationRoot.Path()), batchowner.AcceptedClaims(ledgerRoot)).View(now)
	}
	hasGoals := false
	for _, seat := range view.Seats {
		if len(seat.Goals) > 0 {
			hasGoals = true
			break
		}
	}
	if !hasGoals || inv.owners.dependencies.endpoint == nil {
		return view
	}
	endpoint, err := inv.owners.dependencies.endpoint(ledgerRoot)
	if err != nil {
		return view
	}
	if projection, err := goal.Project(endpoint, false, now); err == nil {
		for i := range view.Seats {
			for j := range view.Seats[i].Goals {
				entry := &view.Seats[i].Goals[j]
				file := projection.Tree.Live[entry.Goal]
				entry.Reserved = file != nil && file.Claimed != nil && file.Claimed.Machine == view.Seats[i].Machine && file.StopCapability != nil && file.StopCapability.ClaimEpoch == 0
			}
		}
	}
	return view
}

func (inv *intentInvocation) boardNow() time.Time {
	if inv.owners.delivery != nil && inv.owners.delivery.now != nil {
		return inv.owners.delivery.now()
	}
	return time.Now().UTC()
}

// runTestBaseline is test baseline: --gate records the trusted refactor
// baseline, --check asks whether another edit batch may start; both reach
// the refactor baseline owner.
func runTestBaseline(args []string, stdout, stderr io.Writer) int {
	// Its options are judged first, in the public style.
	options := newFlagSet("test baseline", stdout, stderr)
	options.Bool("check", false, "")
	options.String("gate", "", "the gate command that passed, e.g. metasystem test baseline --gate 'go test ./...'")
	for _, name := range []string{"file", "max-age-minutes", "max-commits", "root"} {
		options.String(name, "", "")
	}
	if err := options.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	words, err := testBaselineArgs(args)
	if err != nil {
		return printOwnerLines(nil, stderr, "", nil, []string{err.Error()}, 2, textui.Hint{Argv: []string{"metasystem", "test", "baseline", "--help"}, Reason: "its two forms"})
	}
	return runValidateRefactorBaseline(words, stdout, stderr)
}

// testBaselineArgs maps test baseline's options onto the owner's record and
// check modes.
func testBaselineArgs(args []string) ([]string, error) {
	_, check, rest := takeIntentFlag(args, "check", false)
	_, gate, _ := takeIntentFlag(rest, "gate", true)
	switch {
	case check && gate:
		return nil, fmt.Errorf("--check reads the baseline and --gate records one; give one")
	case check:
		return append([]string{"check"}, rest...), nil
	case gate:
		return append([]string{"record"}, rest...), nil
	}
	return nil, errors.New("the baseline needs the gate command that passed; nothing was done\n" +
		"run: metasystem test baseline --gate 'go test ./...'  (your gate command; --check checks the recorded baseline)")
}

// runIntentDesignCheck judges the obligation matrices of the named files
// through the design-obligation owner in this process.
func runIntentDesignCheck(inv *intentInvocation) int {
	if len(inv.input.args) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "design check needs the plan to check; nothing was checked",
			next: inv.typedArgvFor("FILE"), nextReason: "FILE is the plan; several may be named"})
	}
	files := make([]string, 0, len(inv.input.args))
	targets := make([]intentTarget, 0, len(inv.input.args))
	for _, name := range inv.input.args {
		path := inv.callerPath(name)
		files = append(files, path)
		targets = append(targets, intentTarget{Kind: "design", ID: path})
	}
	out, problems, code := validate.DesignObligations(inv.cwd, files, inv.input.switched("complete"))
	data := map[string]any{"files": files, "complete": inv.input.switched("complete"), "lines": nonNilLines(out), "problems": nonNilLines(problems)}
	switch {
	case code == 0:
		gate := map[bool]string{true: "completion", false: "default"}[inv.input.switched("complete")]
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, text: out, Data: data,
			Summary: fmt.Sprintf("%d obligation matrix(es) pass the %s gate", len(files), gate),
			view: func(page *textui.Page) {
				// Each plan is named as the person named it.
				named := inv.input.args
				if len(named) == 1 {
					page.Done(named[0] + " passes the " + gate + " gate")
				} else {
					page.Done(textui.Count(len(named), "plan passes", "plans pass") + " the " + gate + " gate")
					section := page.Section("", "")
					for _, name := range named {
						section.Item(textui.Done, name)
					}
				}
				if page.Verbose() {
					section := page.Section("Checked", "")
					for _, line := range out {
						section.Text(line)
					}
				}
			}})
	case code == 2:
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets, text: problems, Data: data,
			Summary: strings.Join(nonNilLines(problems), "; ") + "; nothing was checked", next: inv.sameCommand(), nextReason: "once that is corrected"})
	}
	summary, rest := "the obligation matrix does not pass", problems
	if len(problems) > 0 {
		// The first problem is the summary; the text is the rest.
		summary, rest = problems[0], problems[1:]
	}
	return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: rest, Data: data, Summary: summary,
		next: inv.sameCommand(), nextReason: "after meeting or re-stating the named obligations in the plan"})
}
