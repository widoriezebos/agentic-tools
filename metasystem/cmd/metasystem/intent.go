package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The public command surface says what a person or an agent wants done:
// "approve this goal", "give it this budget". Each command is one descriptor
// in intentCommands; the router, every help page and the Project Partner
// catalogue read that one table, so a command that is not in it is neither
// routed nor advertised. The commands call the existing owners; the technical
// families stay callable for existing scripts and hooks but are not listed.

// intentFlag is one option a public command accepts. An empty value names a
// switch; aliases are older spellings whose meaning is identical.
type intentFlag struct {
	name     string
	aliases  []string
	value    string
	repeat   bool
	advanced bool
	// hidden options stay parseable for existing scripts and fixtures but
	// are never shown in help or offered as a correction.
	hidden bool
	usage  string
	// rest ends option parsing: every later word, dashes included, is this
	// option's value, kept as the exact argument vector.
	rest bool
}

// intentCommand is one public command: its grammar, its help and its handler.
type intentCommand struct {
	name     string
	audience string // human, agent, or both
	summary  string
	usage    []string
	// administrationUsage configures or repairs MetaSystem itself. Mixed
	// commands keep application work in usage so discovery can separate them.
	administrationUsage []string
	details             []string
	flags               []intentFlag
	maxArgs             int // -1 is unlimited
	examples            []string
	helpForms           []intentHelpForm
	run                 func(*intentInvocation) int
	// legacy, when set, keeps an existing call of the same name on its own
	// handler: the old flag-only top-level calls and a family's own verbs.
	legacy func([]string) bool
	// group is the help topic the command belongs to: goals, work,
	// questions or operations.
	group string
	// primary commands appear on the root orientation page; the others are
	// listed by their topic and by help all.
	primary bool
}

var (
	intentRepoFlag = intentFlag{name: "repo", aliases: []string{"root"}, value: "PATH",
		usage: "the repository, or any directory or file inside it (default: the current directory)"}
	intentJSONFlag   = intentFlag{name: "json", usage: "print one JSON result instead of text"}
	intentTargetFlag = intentFlag{name: "id", aliases: []string{"goal"}, value: "G",
		usage: "the goal, as an alternative to naming it first", advanced: true}
	intentByFlag = intentFlag{name: "by", value: "NAME", advanced: true,
		usage: "the acting person's name; filled from the enrolled terminal's proof when omitted"}
	intentLineageFlag = intentFlag{name: "lineage", value: "LINEAGE", advanced: true, hidden: true,
		usage: "the acting agent session's lineage (or METASYSTEM_OWNER_LINEAGE)"}
	intentTemporaryWordFlag = intentFlag{name: "temporary-human-word", value: "WORD", advanced: true,
		usage: "recorded relayed words presented as the human's; resumes TEMPORARILY"}
	intentReviewByFlag = intentFlag{name: "review-by", value: "DATE", advanced: true,
		usage: "re-approval date required with --temporary-human-word"}
	intentFixtureFlag = intentFlag{name: "fixture-human-authority", advanced: true, hidden: true,
		usage: "fixture-only enrolled-human proof, accepted only for an exact fake-runtime root"}
	intentApprovedRefFlag = intentFlag{name: "approved-ref", value: "REF", advanced: true, hidden: true,
		usage: "a recorded human approval (for example an authenticated channel answer) for this exact goal and box"}
)

func intentCommands() []intentCommand {
	commands := goalIntentCommands()
	commands = append(commands, processIntentCommands()...)
	commands = append(commands, intentPlanningCommands()...)
	commands = append(commands, intentWorkCommands()...)
	return append(commands, intentDeliveryCommands()...)
}

func goalIntentCommands() []intentCommand {
	return []intentCommand{
		{
			name: "goals", group: "goals", primary: true, audience: "both", summary: "list the open goals",
			usage: []string{"metasystem goals [--all] [--label LABEL]... [--history]", "metasystem goals --ready [--label LABEL]... [--machine NAME]", "metasystem goals --tiers"},
			details: []string{
				"Lists the accepted goals. --all adds the done and abandoned goals.",
				"--fetch checks the latest shared goal history before listing it and may update the local copy; it works with every view.",
				"--ready takes --label and --machine; --tiers takes no filter; --history belongs to the listing. A filter a view cannot honor is refused.",
			},
			flags: []intentFlag{
				{name: "all", aliases: []string{"done"}, usage: "include done and abandoned goals"},
				{name: "label", value: "LABEL", repeat: true, usage: "only goals carrying every named label"},
				{name: "ready", usage: "the ready frontier: the goal this machine continues or claims next"},
				{name: "tiers", usage: "the recorded and derived tiers, and the goals a person may lower"},
				{name: "machine", value: "NAME", advanced: true, usage: "with --ready: the machine whose frontier is read"},
				{name: "history", advanced: true, usage: "include each goal's ledger history"},
				{name: "fetch", advanced: true, usage: "fetch and validate the canonical ledger before reading"},
				{name: "pretty", advanced: true, usage: "with --json: indented JSON (the JSON result is always indented)"},
			},
			maxArgs:  0,
			examples: []string{"metasystem goals", "metasystem goals --all --label ui", "metasystem goals --ready", "metasystem goals --tiers"},
			run:      runIntentGoalViews,
		},
		{
			name: "show", group: "goals", audience: "both", summary: "one goal's record, a project record, or a question",
			usage: []string{"metasystem show G [--history]", "metasystem show designs [--goal G]", "metasystem show decisions", "metasystem show record ID",
				"metasystem show design --goal G [--out FILE] [--attempt N]", "metasystem show question Q", "metasystem show review REF"},
			details: []string{"show G is the goal's record; status G is its live work.",
				"designs, decisions and record ID read the project's own records; design --goal G is one goal's design records.",
				"question Q is one question, from the channel or a mission, and how it is answered."},
			flags: []intentFlag{intentTargetFlag, {name: "history", advanced: true, usage: "include the goal's ledger history"},
				{name: "attempt", value: "N", usage: "show design: one design attempt and its proposal"},
				{name: "out", value: "FILE", usage: "show design: the design document, when the goal has several"}},
			maxArgs:  2,
			examples: []string{"metasystem show verbs-match-intent", "metasystem show verbs-match-intent --history", "metasystem show designs --goal verbs-match-intent", "metasystem show record 01M3EC3QT7M2TC36P7ZVNRF0RW"},
			run:      runIntentShow,
		},
		{
			name: "approve", group: "goals", primary: true, audience: "human", summary: "approve goals for execution",
			usage: []string{"metasystem approve G... [--budget BOX]"},
			details: []string{
				"Without --budget each goal is approved under its own tier's norm box, all goals in one act.",
				"BOX is norm or the complete compact box, for example 1d/10/720m/1/3 (elapsed/attempts/job minutes/active jobs/review rounds).",
				"Run it at the enrolled terminal; --under GRANT is a seat's act under a recorded power of attorney.",
			},
			flags: []intentFlag{
				{name: "id", aliases: []string{"goal"}, value: "G", repeat: true, advanced: true, usage: "a goal to approve (repeatable)"},
				{name: "budget", value: "BOX", usage: "the box: norm or the complete compact box"},
				{name: "under", value: "GRANT", advanced: true, usage: "act under this recorded power of attorney"},
				intentByFlag, intentLineageFlag, intentApprovedRefFlag, intentTemporaryWordFlag, intentReviewByFlag, intentFixtureFlag,
				intentLongBudgetFlags[0], intentLongBudgetFlags[1], intentLongBudgetFlags[2], intentLongBudgetFlags[3], intentLongBudgetFlags[4],
			},
			maxArgs:  -1,
			examples: []string{"metasystem approve verbs-match-intent", "metasystem approve goal-a goal-b", "metasystem approve verbs-match-intent --budget 1d/10/720m/1/3"},
			run:      runIntentApproveWithLimits,
		},
		{
			name: "budget", group: "goals", audience: "human", summary: "read a goal's budget, or give it a box",
			usage: []string{"metasystem budget G", "metasystem budget G BOX"},
			details: []string{
				"Without BOX: the standing box and what has been spent, the same block show prints.",
				"BOX is norm (the tier's box), keep (the standing box) or the complete compact box 1d/10/720m/1/3.",
				"A queued or parked goal is approved with the box; a running goal's box is changed.",
				"A goal stopped by its budget resumes under its standing box first: metasystem resume G.",
			},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "budget", value: "BOX", advanced: true, usage: "the box, as an alternative to naming it after G"},
				{name: "under", value: "GRANT", advanced: true, usage: "act under this recorded power of attorney"},
				intentByFlag, intentLineageFlag, intentApprovedRefFlag, intentTemporaryWordFlag, intentReviewByFlag, intentFixtureFlag,
				intentLongBudgetFlags[0], intentLongBudgetFlags[1], intentLongBudgetFlags[2], intentLongBudgetFlags[3], intentLongBudgetFlags[4],
			},
			maxArgs:  2,
			examples: []string{"metasystem budget verbs-match-intent", "metasystem budget verbs-match-intent norm", "metasystem budget verbs-match-intent 2d/12/900m/2/3"},
			run:      runIntentBudgetWithLimits,
		},
		{
			name: "pause", group: "goals", audience: "both", summary: "park a goal with a reason",
			usage: []string{"metasystem pause G --reason TEXT"},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "reason", aliases: []string{"because"}, value: "TEXT", usage: "why the goal is parked"},
				fileFlag("reason", "read the reason from FILE"),
				intentByFlag, intentLineageFlag, intentFixtureFlag,
			},
			maxArgs:  1,
			examples: []string{"metasystem pause verbs-match-intent --reason 'waits for the design review'"},
			run:      runIntentPause,
		},
		{
			name: "resume", group: "goals", audience: "human", summary: "resume a parked goal, or a stopped goal under its standing box",
			usage: []string{"metasystem resume G", "metasystem resume G --under GRANT --verified TEXT", "metasystem resume mission M"},
			details: []string{
				"A parked goal returns to the queue; approval is not granted by resuming.",
				"A goal stopped by its budget resumes under its standing approved box; change the box afterwards with budget.",
				"--under and --verified are a seat's unpark under a power of attorney, for parked goals only.",
				"resume mission M resumes a parked or interrupted autonomous mission.",
			},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "under", value: "GRANT", advanced: true, usage: "unpark under this recorded power of attorney"},
				{name: "verified", value: "TEXT", advanced: true, usage: "with --under: what the seat verified holds now"},
				fileFlag("verified", "read what the seat verified from FILE"),
				intentByFlag, intentLineageFlag, intentApprovedRefFlag, intentTemporaryWordFlag, intentReviewByFlag, intentFixtureFlag,
			},
			maxArgs:  2,
			examples: []string{"metasystem resume verbs-match-intent"},
			run:      runIntentResume,
		},
		{
			name: "done", group: "goals", audience: "both", summary: "conclude a goal, or complete a finished job's records",
			usage: []string{"metasystem done G --reason TEXT", "metasystem done job J [--dispositions FILE] [--evidence R]"},
			details: []string{"The goal's obligations are checked first; its merged branch is swept afterwards.",
				"--dispositions and --evidence belong only to done job; the goal form refuses them. A goal conclusion needs --reason.",
				"done job J completes the finished job after checking its authority, results and review decisions. Use the original job reference from status.",
				"It concludes no goal, lands nothing and grants no approval.",
				"A review chain needs its author's --dispositions; review job J --dispositions FILE is the route while reviewing.",
				"--evidence R reconciles review R's evidence into the chain before it closes. An already closed chain is reported unchanged."},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "reason", aliases: []string{"conclude"}, value: "TEXT", usage: "the conclusion"},
				fileFlag("reason", "read the reason from FILE"),
				intentByFlag, intentLineageFlag,
				{name: "dispositions", value: "FILE", usage: "done job: the Markdown dispositions table for a review chain"},
				{name: "evidence", value: "R", advanced: true, usage: "done job: a review evidence job reconciled into the chain before it closes"},
			},
			maxArgs:  2,
			examples: []string{"metasystem done verbs-match-intent --reason 'all four slices landed'", "metasystem done job impl-01"},
			run:      runIntentDone,
		},
	}
}

// findIntentCommand finds a public descriptor by name.
func findIntentCommand(name string) (intentCommand, bool) {
	for _, command := range intentCommands() {
		if command.name == name {
			return command, true
		}
	}
	return intentCommand{}, false
}

// allFlags is the command's own options plus the two every command takes.
func (c intentCommand) allFlags() []intentFlag {
	return append(append([]intentFlag(nil), c.flags...), intentRepoFlag, intentJSONFlag)
}

func (c intentCommand) lookupFlag(name string) (intentFlag, bool) {
	for _, candidate := range c.allFlags() {
		if candidate.name == name {
			return candidate, true
		}
		for _, alias := range candidate.aliases {
			if alias == name {
				return candidate, true
			}
		}
	}
	return intentFlag{}, false
}

// intentInput is one invocation's parsed arguments.
type intentInput struct {
	args   []string
	values map[string][]string
	help   bool
	// sequence is every repeatable option in the order given.
	sequence []intentPair
}

type intentPair struct{ name, value string }

func (in intentInput) text(name string) string {
	if values := in.values[name]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func (in intentInput) has(name string) bool { return len(in.values[name]) > 0 }

func (in intentInput) switched(name string) bool { return in.text(name) == "true" }

// intentInputError is a mistake in the command line itself, found before any
// owner is called.
type intentInputError struct {
	summary string
	next    []string
	reason  string
}

// parseIntentArgs accepts options before, between and after the positional
// words, in --name value and --name=value form. `--` ends the options, so a
// literal word may start with a dash; a value that follows an option is taken
// literally even when it starts with a dash. Repeating an option with the same
// value is harmless; different values are refused before anything happens.
func parseIntentArgs(command intentCommand, raw []string) (intentInput, *intentInputError) {
	input := intentInput{values: map[string][]string{}}
	var pairs []intentPair
	for index := 0; index < len(raw); index++ {
		token := raw[index]
		if token == "--" {
			input.args = append(input.args, raw[index+1:]...)
			break
		}
		if token == "-h" || token == "--help" || token == "-help" {
			input.help = true
			continue
		}
		if len(token) < 2 || token[0] != '-' || strings.HasPrefix(token, "---") {
			input.args = append(input.args, token)
			continue
		}
		spelled := strings.TrimPrefix(strings.TrimPrefix(token, "-"), "-")
		name, value, joined := strings.Cut(spelled, "=")
		definition, known := command.lookupFlag(name)
		if !known {
			return input, unknownIntentFlag(command, raw, index, name)
		}
		if definition.rest {
			words := raw[index+1:]
			if joined {
				words = append([]string{value}, words...)
			}
			if len(words) == 0 {
				return input, &intentInputError{
					summary: fmt.Sprintf("--%s needs a command after it (%s); nothing was done", definition.name, definition.value),
					reason:  "see metasystem help " + command.name,
				}
			}
			input.values[definition.name] = append([]string(nil), words...)
			break
		}
		if definition.value == "" {
			if !joined {
				value = "true"
			}
		} else if !joined {
			if index+1 >= len(raw) {
				return input, &intentInputError{
					summary: fmt.Sprintf("--%s needs a value (%s)", definition.name, definition.value),
					reason:  "see metasystem help " + command.name,
				}
			}
			index++
			value = raw[index]
		}
		pairs = append(pairs, intentPair{definition.name, value})
	}
	if input.help {
		return input, nil
	}
	// Go's own flag parsing checks each value against its option's kind.
	set := flag.NewFlagSet(command.name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	for _, definition := range command.allFlags() {
		if definition.value == "" {
			set.Bool(definition.name, false, definition.usage)
		} else {
			set.String(definition.name, "", definition.usage)
		}
	}
	for _, one := range pairs {
		definition, _ := command.lookupFlag(one.name)
		if err := set.Set(one.name, one.value); err != nil {
			return input, &intentInputError{summary: fmt.Sprintf("--%s cannot be %q: %v", one.name, one.value, err), reason: "see metasystem help " + command.name}
		}
		value := set.Lookup(one.name).Value.String()
		previous := input.values[one.name]
		if definition.repeat {
			input.sequence = append(input.sequence, intentPair{one.name, value})
			if !containsIntentValue(previous, value) {
				input.values[one.name] = append(previous, value)
			}
			continue
		}
		if len(previous) > 0 && previous[0] != value {
			return input, &intentInputError{
				summary: fmt.Sprintf("--%s was given twice with different values, %s and %s; nothing was done", one.name, shellCommand([]string{previous[0]}), shellCommand([]string{value})),
				reason:  "give --" + one.name + " once",
			}
		}
		input.values[one.name] = []string{value}
	}
	if command.maxArgs >= 0 && len(input.args) > command.maxArgs {
		extra := input.args[command.maxArgs:]
		return input, &intentInputError{
			summary: fmt.Sprintf("takes at most %d word(s) before its options; unexpected %s", command.maxArgs, shellCommand(extra)),
			reason:  "quote a text value as one argument, or see metasystem help " + command.name,
		}
	}
	return input, nil
}

func containsIntentValue(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

// unknownIntentFlag names the accepted spelling when exactly one option is a
// small typo away, and otherwise lists what the command takes. The corrected
// command is offered, never run.
func unknownIntentFlag(command intentCommand, raw []string, index int, name string) *intentInputError {
	var near []string
	closest := 3
	for _, candidate := range command.allFlags() {
		if candidate.hidden {
			continue
		}
		for _, spelling := range append([]string{candidate.name}, candidate.aliases...) {
			distance := editDistance(spelling, name)
			switch {
			case distance < closest:
				closest, near = distance, []string{candidate.name}
			case distance == closest && !containsIntentValue(near, candidate.name):
				near = append(near, candidate.name)
			}
		}
	}
	if len(near) == 1 {
		corrected := append([]string{"metasystem", command.name}, raw...)
		token := raw[index]
		_, value, joined := strings.Cut(token, "=")
		corrected[2+index] = "--" + near[0]
		if joined {
			corrected[2+index] += "=" + value
		}
		return &intentInputError{
			summary: fmt.Sprintf("does not take --%s; did you mean --%s? Nothing was done", name, near[0]),
			next:    corrected,
			reason:  "the same command with the accepted option",
		}
	}
	accepted := make([]string, 0, len(command.allFlags()))
	for _, candidate := range command.allFlags() {
		if !candidate.hidden {
			accepted = append(accepted, "--"+candidate.name)
		}
	}
	return &intentInputError{
		summary: fmt.Sprintf("does not take --%s; it takes %s. Nothing was done", name, strings.Join(accepted, ", ")),
		reason:  "see metasystem help " + command.name,
	}
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for i := 1; i <= len(left); i++ {
		current := make([]int, len(right)+1)
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(right)]
}

// intentOwners are the existing owners a public command calls. Production
// uses the real ones; tests give each invocation its own fakes.
type intentOwners struct {
	resolver        stateroot.Resolver
	prove           goalAuthorityProver
	commandNow      func(string) (time.Time, error)
	dependencies    syncRequestDependencies
	binding         goalBindingResolver
	parkBranchCheck func(string, goal.Endpoint) func(string, string) (string, error)
	completion      completionInputs
	processes       processIntentOwners
	work            intentWorkOwners
	delivery        *intentDeliveryOwners
	connection      intentConnectionOwners
}

func defaultIntentOwners() intentOwners {
	return intentOwners{
		resolver:        stateroot.NewResolver(stateroot.RepositoryTop, os.Executable),
		prove:           humanauthority.ProveOrTemporaryGoalAuthority,
		commandNow:      goalCommandNow,
		dependencies:    defaultSyncRequestDependencies(),
		binding:         dispatchcore.ResolveGoalBinding,
		parkBranchCheck: goalParkBranchCheck,
		processes:       defaultProcessIntentOwners(),
	}
}

// intentInvocation is one public command run: its own output streams, the
// directory it was started in, and the one state root it selected.
type intentInvocation struct {
	command   intentCommand
	input     intentInput
	raw       []string
	stdout    io.Writer
	stderr    io.Writer
	cwd       string
	owners    intentOwners
	layout    stateroot.Layout
	stateRoot string
	// reviewWork is set while review G examines one work item's subject.
	reviewWork *reviewWorkContext
	// inferredChain is the one examination root inferred from the goal's
	// work records, when the caller named none.
	inferredChain string
}

// runIntent routes one public command. Help needs no repository, identity or
// writes.
func runIntent(command intentCommand, raw []string, stdout, stderr io.Writer, owners intentOwners) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "metasystem %s: cannot read the current directory: %v\n", command.name, err)
		return 1
	}
	return runIntentIn(command, raw, stdout, stderr, cwd, owners)
}

func runIntentIn(command intentCommand, raw []string, stdout, stderr io.Writer, cwd string, owners intentOwners) int {
	inv := &intentInvocation{command: command, raw: raw, stdout: stdout, stderr: stderr, cwd: cwd, owners: owners}
	input, inputErr := parseIntentArgs(command, raw)
	inv.input = input
	if inputErr == nil && input.help {
		writeIntentHelp(stdout, command)
		return 0
	}
	if inputErr != nil {
		inv.input.values = map[string][]string{"json": {fmt.Sprint(argumentHasFlag(raw, "json"))}}
		return inv.render(intentResult{Outcome: intentRefused, Summary: inputErr.summary, next: inputErr.next, nextReason: inputErr.reason, code: 2})
	}
	if problem := inv.resolveTextFiles(); problem != nil {
		return inv.render(*problem)
	}
	return command.run(inv)
}

// selectRoot resolves the repository once: any path inside the repository,
// its installation, a symbolic link to either, or the current directory.
// Goal owners receive the installation's state root; a missing or unreadable
// installation is refused, never read as an empty world.
func (inv *intentInvocation) selectRoot() *intentResult {
	path := inv.cwd
	if inv.input.has("repo") {
		path = inv.input.text("repo")
		if !filepath.IsAbs(path) {
			path = filepath.Join(inv.cwd, path)
		}
	}
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err == nil {
		inv.layout = layout
		inv.stateRoot, err = inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
	}
	if err != nil {
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("%s is not inside one metasystem installation: %v", shellCommand([]string{path}), err),
			Decision: "run this inside the repository, or name it with --repo PATH"}
	}
	if !converted(inv.stateRoot) {
		return &intentResult{Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("the installation at %s has no synced goal ledger", shellCommand([]string{inv.stateRoot})),
			Decision: "a person upgrades the legacy goals file to the synced ledger first: metasystem repair goals --upgrade --by NAME (it shows the digest to review)"}
	}
	return nil
}

// The outcomes a public result can have.
const (
	intentConfirmed  = "confirmed"
	intentUnchanged  = "unchanged"
	intentInProgress = "in-progress"
	intentPartial    = "partial"
	intentRefused    = "refused"
	intentFailed     = "failed"
)

type intentTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type intentNext struct {
	Argv   []string `json:"argv"`
	Reason string   `json:"reason"`
}

// intentResult is the one JSON envelope every public command prints; text
// output renders the same result.
type intentResult struct {
	SchemaVersion int            `json:"schemaVersion"`
	Verb          string         `json:"verb"`
	Targets       []intentTarget `json:"targets"`
	Outcome       string         `json:"outcome"`
	Summary       string         `json:"summary"`
	Data          any            `json:"data,omitempty"`
	Next          *intentNext    `json:"next,omitempty"`
	Decision      string         `json:"decision,omitempty"`

	text       []string
	next       []string
	nextReason string
	code       int
}

func (inv *intentInvocation) render(result intentResult) int {
	result.SchemaVersion = 1
	result.Verb = inv.command.name
	if result.Targets == nil {
		result.Targets = []intentTarget{}
	}
	if len(result.next) > 0 {
		result.Next = &intentNext{Argv: result.next, Reason: result.nextReason}
	}
	code := result.code
	if code == 0 && result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		code = 1
	}
	if inv.input.switched("json") {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(inv.stderr, "metasystem %s: %v\n", inv.command.name, err)
			return 1
		}
		fmt.Fprintln(inv.stdout, string(encoded))
		return code
	}
	if result.Outcome == intentConfirmed || result.Outcome == intentUnchanged {
		if result.Summary != "" {
			fmt.Fprintln(inv.stdout, result.Summary)
		}
		for _, line := range result.text {
			fmt.Fprintln(inv.stdout, line)
		}
		if result.Next != nil {
			fmt.Fprintf(inv.stdout, "next: %s  (%s)\n", shellCommand(result.Next.Argv), result.Next.Reason)
		}
		return code
	}
	fmt.Fprintf(inv.stderr, "metasystem %s: %s\n", inv.command.name, strings.TrimSpace(result.Summary))
	for _, line := range result.text {
		fmt.Fprintln(inv.stderr, line)
	}
	switch {
	case result.Next != nil:
		fmt.Fprintf(inv.stderr, "run: %s\n", shellCommand(result.Next.Argv))
		if result.Next.Reason != "" {
			fmt.Fprintf(inv.stderr, "     (%s)\n", result.Next.Reason)
		}
	case result.Decision != "":
		fmt.Fprintf(inv.stderr, "needed first: %s\n", result.Decision)
	case result.nextReason != "":
		fmt.Fprintf(inv.stderr, "hint: %s\n", result.nextReason)
	}
	return code
}

// ownerReport receives one goal owner's typed outcome for a public command,
// which renders it; the legacy commands leave it unset and print as before.
type ownerReport struct {
	result  *goal.PublishResult
	failure error
	refusal *ownerRefusal
	notes   []string
	written bytes.Buffer
	// secondary are failures after the primary act landed; repair is the
	// owner's command that completes them, when one exists.
	secondary    []error
	repair       []string
	repairReason string
	// value is an owner's committed typed record, kept when a later
	// publication step refuses.
	value any
	// entry is the recorded id an owner hands back beside its publication,
	// such as a power of attorney's entry.
	entry string
}

type ownerRefusal struct {
	code     int
	sentence string
	remedy   humanVerbRemedy
}

func (d syncRequestDependencies) publish(res goal.PublishResult, err error) int {
	if d.report == nil {
		return printSyncResult(res, err)
	}
	if err != nil {
		// A landed act keeps its publication even when later work failed.
		d.report.failure = err
		if publicationLanded(res) {
			d.report.result = &res
		}
		return 1
	}
	d.report.result = &res
	if res.Outcome != goal.OutcomeConfirmed {
		return 1
	}
	return 0
}

// landed records a committed primary act before its proof or other
// follow-up work runs, so a later failure reports a partial outcome.
func (d syncRequestDependencies) landed(res goal.PublishResult) {
	if d.report != nil && publicationLanded(res) {
		d.report.result = &res
	}
}

func publicationLanded(res goal.PublishResult) bool {
	return res.Outcome == goal.OutcomeConfirmed || res.Outcome == goal.OutcomeConfirmedLate
}

func (d syncRequestDependencies) showOutcome(res goal.PublishResult) {
	if d.report == nil {
		printJSON(map[string]any{"outcome": res.Outcome, "tip": res.Tip, "detail": res.Detail})
		return
	}
	d.report.result = &res
}

func (d syncRequestDependencies) fail(code int, err error) int {
	if d.report == nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	d.report.failure = err
	return code
}

func (d syncRequestDependencies) note(stdout bool, line string) {
	switch {
	case d.report != nil:
		d.report.notes = append(d.report.notes, line)
	case stdout:
		fmt.Println(line)
	default:
		fmt.Fprintln(os.Stderr, line)
	}
}

func (d syncRequestDependencies) noteWriter() io.Writer {
	if d.report == nil {
		return os.Stderr
	}
	return &d.report.written
}

// ownerResult turns an owner's typed report into the public result. The
// owner's own refusal sentence and remedy are kept verbatim; a remedy command
// is carried as the argument vector the owner rendered.
func ownerResult(report *ownerReport, code int, confirmed intentResult) intentResult {
	lines := append([]string(nil), report.notes...)
	for _, line := range strings.Split(strings.TrimSpace(report.written.String()), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	landed := report.result != nil && publicationLanded(*report.result)
	unchanged := report.result != nil && report.result.Outcome == goal.OutcomeAbandoned
	if landed && (report.refusal != nil || report.failure != nil || len(report.secondary) > 0) {
		return partialResult(report, code, confirmed, lines)
	}
	switch {
	case report.refusal != nil:
		result := intentResult{Outcome: intentRefused, Summary: report.refusal.sentence, text: lines, code: report.refusal.code}
		if unchanged {
			result.Outcome, result.code = intentUnchanged, 0
		}
		if report.refusal.remedy.command != "" {
			result.next = shellWords(report.refusal.remedy.command)
			result.nextReason = "the goal owner's remedy"
		} else if words := strings.TrimSpace(report.refusal.remedy.words); words != "" {
			result.Decision = words
		}
		if report.result != nil {
			result.Data = map[string]any{"owner": ownerPublication(*report.result)}
		}
		return result
	case report.failure != nil:
		return intentResult{Outcome: intentRefused, Summary: report.failure.Error(), text: lines, code: max(code, 1)}
	case landed && code == 0:
		confirmed.Outcome = intentConfirmed
		confirmed.text = append(lines, confirmed.text...)
		return confirmed
	case unchanged:
		return intentResult{Outcome: intentUnchanged, Summary: report.result.Detail, text: lines, Data: map[string]any{"owner": ownerPublication(*report.result)}}
	case report.result != nil:
		return intentResult{Outcome: intentRefused, Summary: report.result.Detail, text: lines, code: max(code, 1), Data: map[string]any{"owner": ownerPublication(*report.result)}}
	}
	return intentResult{Outcome: intentFailed, code: max(code, 1), text: lines,
		Summary: "the goal owner stopped without a recorded result; its message is on standard error"}
}

// partialResult reports a primary act that landed and later work that did
// not: the publication and every failure are kept, and the act is not to be
// repeated.
func partialResult(report *ownerReport, code int, confirmed intentResult, lines []string) intentResult {
	var failures []string
	if report.refusal != nil {
		failures = append(failures, report.refusal.sentence)
	}
	if report.failure != nil {
		failures = append(failures, report.failure.Error())
	}
	for _, err := range report.secondary {
		failures = append(failures, err.Error())
	}
	data, _ := confirmed.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	data["owner"] = ownerPublication(*report.result)
	data["incomplete"] = failures
	result := intentResult{Outcome: intentPartial, code: max(code, 1), Data: data, text: append(lines, confirmed.text...),
		Summary: fmt.Sprintf("the act landed at tip %s, but its follow-up did not complete: %s", report.result.Tip, strings.Join(failures, "; "))}
	switch {
	case len(report.repair) > 0:
		result.next, result.nextReason = report.repair, report.repairReason
	case report.refusal != nil && strings.TrimSpace(report.refusal.remedy.words) != "":
		result.Decision = strings.TrimSpace(report.refusal.remedy.words)
	default:
		result.Decision = "the act already landed; do not run it again. Recover the named follow-up with the printed recovery command"
	}
	return result
}

func ownerPublication(res goal.PublishResult) map[string]any {
	return map[string]any{"outcome": res.Outcome, "tip": res.Tip, "detail": res.Detail}
}

// shellWords reads back a command line rendered by shellCommand.
func shellWords(command string) []string {
	var words []string
	var current strings.Builder
	inWord, quoted := false, false
	for index := 0; index < len(command); index++ {
		character := command[index]
		switch {
		case quoted:
			if character == '\'' {
				quoted = false
			} else {
				current.WriteByte(character)
			}
		case character == '\'':
			quoted, inWord = true, true
		case character == '"':
			end := strings.IndexByte(command[index+1:], '"')
			if end < 0 {
				end = len(command) - index - 1
			}
			current.WriteString(command[index+1 : index+1+end])
			index += end + 1
			inWord = true
		case character == ' ':
			if inWord {
				words = append(words, current.String())
				current.Reset()
				inWord = false
			}
		default:
			current.WriteByte(character)
			inWord = true
		}
	}
	if inWord {
		words = append(words, current.String())
	}
	return words
}

// Help pages. Each is written from the command table, so help lists exactly
// the commands and options the router accepts. The root page is a short
// orientation; the topics and help all list the complete public surface.

// intentGroups are the help topics that each list one area in full, in the
// order help all prints them.
var intentGroups = []struct{ name, heading string }{
	{"goals", "Plan and steer goals"},
	{"work", "Deliver work on a goal"},
	{"questions", "Questions for a person"},
	{"operations", "Manage work and recovery"},
	{"administration", "MetaSystem administration (tool setup and maintenance)"},
}

func (command intentCommand) allUsage() []string {
	return append(slices.Clone(command.usage), command.administrationUsage...)
}

// publicIntentCommands are the commands help lists: every descriptor.
func publicIntentCommands() []intentCommand {
	return intentCommands()
}

func writeIntentRootHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: metasystem COMMAND [TARGET...] [OPTIONS]")
	fmt.Fprintln(w, "Say what you want done; metasystem prepares, runs, collects and recovers the work.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Start here:")
	for _, group := range intentGroups {
		for _, command := range publicIntentCommands() {
			if command.primary && command.group == group.name {
				fmt.Fprintf(w, "  %-10s %s\n", command.name, command.summary)
			}
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "A typical delivery:")
	fmt.Fprintln(w, "  metasystem build my-goal --brief brief.md --check go test ./...")
	fmt.Fprintln(w, "  metasystem review my-goal")
	fmt.Fprintln(w, "  metasystem land my-goal")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "More:")
	fmt.Fprintln(w, "  metasystem help goals | help work | help questions | help operations   one area in full")
	fmt.Fprintln(w, "  metasystem help administration                                      MetaSystem setup and maintenance")
	fmt.Fprintln(w, "  metasystem help human | help agent | help all                         by audience, or everything")
	fmt.Fprintln(w, "  metasystem help COMMAND                                               one command's options and examples")
	fmt.Fprintln(w, "Options go before or after the goal; --repo PATH selects the repository from any path inside it.")
}

// writeIntentLong lists commands with every usage, the summary and the first
// example.
func writeIntentLong(w io.Writer, commands []intentCommand) {
	for _, command := range commands {
		for _, usage := range command.usage {
			fmt.Fprintf(w, "  %s\n", usage)
		}
		fmt.Fprintf(w, "      %s\n", command.summary)
		if len(command.examples) > 0 {
			fmt.Fprintf(w, "      e.g. %s\n", command.examples[0])
		}
	}
}

func intentCommandsWhere(keep func(intentCommand) bool) []intentCommand {
	var kept []intentCommand
	for _, command := range publicIntentCommands() {
		if keep(command) {
			kept = append(kept, command)
		}
	}
	return kept
}

// intentTopic reports whether a help word is a topic rather than a command.
// goals is both: its command help carries the planning topic after it.
func intentTopic(name string) bool {
	if slices.Contains([]string{"human", "agent", "all"}, name) {
		return true
	}
	for _, group := range intentGroups {
		if group.name == name {
			return true
		}
	}
	return false
}

func writeIntentGroupHelp(w io.Writer, name string) {
	writeIntentGroupHelpFor(w, name, "")
}

func writeIntentGroupHelpFor(w io.Writer, name, audience string) {
	for _, group := range intentGroups {
		if group.name != name {
			continue
		}
		fmt.Fprintf(w, "%s (metasystem help COMMAND for options):\n", group.heading)
		writeIntentLong(w, intentCommandsWhere(func(command intentCommand) bool {
			return command.group == name && (audience != "human" || command.audience != "agent")
		}))
		if name == "administration" {
			for _, command := range publicIntentCommands() {
				if len(command.administrationUsage) == 0 || audience == "human" && command.audience == "agent" {
					continue
				}
				for _, usage := range command.administrationUsage {
					fmt.Fprintf(w, "  %s\n", usage)
				}
				fmt.Fprintf(w, "      MetaSystem administration forms of %s; help %s describes their inputs and authority.\n", command.name, command.name)
			}
		}
	}
}

func writeIntentTopicHelp(w io.Writer, topic string) {
	switch topic {
	case "human":
		fmt.Fprintln(w, "For people (metasystem help COMMAND for options):")
		for _, group := range intentGroups {
			fmt.Fprintln(w)
			writeIntentGroupHelpFor(w, group.name, "human")
		}
	case "agent":
		writeIntentAgentHelp(w)
	case "all":
		for index, group := range intentGroups {
			if index > 0 {
				fmt.Fprintln(w)
			}
			writeIntentGroupHelp(w, group.name)
		}
	default:
		writeIntentGroupHelp(w, topic)
	}
}

// writeIntentHelp is the page help NAME prints: the command's own help, and
// for goals the planning topic after it, so neither shadows the other.
func writeIntentHelp(w io.Writer, command intentCommand) {
	writeIntentCommandHelp(w, command)
	if command.name == "goals" {
		fmt.Fprintln(w)
		writeIntentGroupHelp(w, "goals")
	}
}

func writeIntentCommandHelp(w io.Writer, command intentCommand) {
	fmt.Fprintf(w, "%s - %s\n", command.name, command.summary)
	if command.group == "administration" {
		fmt.Fprintln(w, "MetaSystem administration: this command manages the work system itself.")
	}
	fmt.Fprintln(w, "usage:")
	for _, usage := range command.usage {
		fmt.Fprintf(w, "  %s\n", usage)
	}
	if len(command.administrationUsage) > 0 {
		fmt.Fprintln(w, "MetaSystem administration:")
		for _, usage := range command.administrationUsage {
			fmt.Fprintf(w, "  %s\n", usage)
		}
	}
	for _, detail := range command.details {
		fmt.Fprintf(w, "%s\n", detail)
	}
	writeFlags := func(heading string, advanced bool) {
		var rows []string
		for _, definition := range command.allFlags() {
			if definition.advanced != advanced || definition.hidden {
				continue
			}
			spelling := "--" + definition.name
			if definition.value != "" {
				spelling += " " + definition.value
			}
			if definition.repeat {
				spelling += " (repeatable)"
			}
			line := fmt.Sprintf("  %-34s %s", spelling, definition.usage)
			if len(definition.aliases) > 0 {
				aliases := make([]string, len(definition.aliases))
				for index, alias := range definition.aliases {
					aliases[index] = "--" + alias
				}
				line += " (also " + strings.Join(aliases, ", ") + ")"
			}
			rows = append(rows, line)
		}
		if len(rows) == 0 {
			return
		}
		fmt.Fprintln(w, heading)
		for _, row := range rows {
			fmt.Fprintln(w, row)
		}
	}
	writeFlags("options:", false)
	writeFlags("advanced:", true)
	if len(command.examples) > 0 {
		fmt.Fprintln(w, "examples:")
		for _, example := range command.examples {
			fmt.Fprintf(w, "  %s\n", example)
		}
	}
	fmt.Fprintln(w, "Options go before or after the goal; `--` ends the options.")
}

// suggestIntentCommand names the public command nearest a mistyped word, or
// nothing when none is close.
func suggestIntentCommand(name string) string {
	best, closest := "", 3
	for _, command := range publicIntentCommands() {
		if distance := editDistance(command.name, name); distance < closest {
			best, closest = command.name, distance
		}
	}
	return best
}
