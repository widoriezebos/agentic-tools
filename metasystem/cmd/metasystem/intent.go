package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adopt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hookswitch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// The public command surface is object then action: "goal approve G",
// "work land G". Each (object, action) pair is one descriptor in
// intentCommands; the router, every help page, the JSON help and the Project
// Partner catalogue read that one table, so a pair that is not in it is
// neither routed nor advertised. The actions call the existing owners. Hidden
// rows are process entrypoints whose first word is also an object: they keep
// their existing argument vector and are listed only by metasystem internal.

// intentFlag is one option a public command accepts. An empty value names a
// switch; aliases are other spellings whose meaning is identical.
type intentFlag struct {
	name     string
	aliases  []string
	value    string
	repeat   bool
	advanced bool
	// hidden options stay parseable for machinery callers and fixtures but
	// are never shown in help or offered as a correction.
	hidden bool
	usage  string
	// rest ends option parsing: every later word, dashes included, is this
	// option's value, kept as the exact argument vector.
	rest bool
}

// intentCommand is one (object, action) pair: its grammar, its help and its
// handler. The top-level status is the one row with an empty action.
type intentCommand struct {
	object string
	action string
	// name is "object action" (or "status"); it labels results and help.
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
	// passthrough hands the words after the action, unchanged, to a handler
	// with its own parser, output and exit codes: a machinery verb given a
	// public home, or a process entrypoint.
	passthrough command
	// owner is the handler a passthrough runs, once its public options
	// (--repo, the installation found) are applied.
	owner command
	// hidden rows are process entrypoints: routed, never listed in public
	// help; launcher names the code that starts them.
	hidden   bool
	launcher string
	// accepts are the reference kinds a work reference may name here
	// (goal, j1, j2, run, read, wait, proof); empty takes none.
	accepts []string
	// group is the help area the object belongs to: plan, deliver, run or
	// practice.
	group string
	// primary actions appear on the root orientation page.
	primary bool
	// laidOut commands print through textui (output-style): their refusals
	// wrap to the width as their pages do.
	laidOut bool
}

var (
	intentRepoFlag = intentFlag{name: "repo", aliases: []string{"root"}, value: "PATH",
		usage: "the repository, or any directory or file inside it (default: the current directory)"}
	intentJSONFlag = intentFlag{name: "json", usage: "print one JSON result instead of text"}
	// intentVerboseFlag is the one spelling of "every item, one line each"
	// for a command whose text groups repeated findings; a command opts in
	// by listing it.
	intentVerboseFlag = intentFlag{name: "verbose", usage: "print every item on its own line instead of the grouped summary"}
	intentTargetFlag  = intentFlag{name: "id", aliases: []string{"goal"}, value: "G",
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
	var commands []intentCommand
	for _, part := range [][]intentCommand{
		goalIntentCommands(), intentPlanningCommands(), goalReviewIntentCommands(), goalLandWithoutSittingIntentCommands(), designIntentCommands(), intentWorkCommands(), workspaceIntentCommands(),
		intentDeliveryCommands(), agentIntentCommands(), helmIntentCommands(), processIntentCommands(), landingIntentCommands(), deployIntentCommands(), alertIntentCommands(), rosterIntentCommands(), diskIntentCommands(), evidenceIntentCommands(), appIntentCommands(), practiceIntentCommands(), hiddenIntentEntries(), {topLevelStatus()},
	} {
		commands = append(commands, part...)
	}
	for index := range commands {
		command := &commands[index]
		command.name = strings.TrimSpace(command.object + " " + command.action)
		if command.group == "" {
			command.group = intentObjectGroup(command.object)
		}
		// --lineage is shown where an agent may act: it is the remedy an
		// actor refusal names. A person's act keeps it out of sight.
		if command.audience == "agent" || command.audience == "both" {
			flags := slices.Clone(command.flags)
			for i := range flags {
				if flags[i].name == "lineage" {
					flags[i].hidden = false
					flags[i].usage = "the acting agent session, as its launcher named it in METASYSTEM_OWNER_LINEAGE (default: that variable)"
				}
			}
			command.flags = flags
		}
	}
	return commands
}

func goalIntentCommands() []intentCommand {
	return []intentCommand{
		{
			object: "goal", action: "list", primary: true, audience: "both", laidOut: true, summary: "list the open goals",
			usage: []string{"metasystem goal list [--all] [--label LABEL]... [--history]", "metasystem goal list --ready [--label LABEL]... [--machine NAME]", "metasystem goal list --tiers"},
			details: []string{
				"Lists the active goals: claimed, approved, and queued at priority 1. --all lists every goal, the done and abandoned ones too; --label lists every open goal carrying the labels.",
				"--fetch checks the latest shared goal history before listing it and may update the local copy; it works with every view.",
				"--ready takes --label and --machine; --tiers takes no filter; --history belongs to the listing. A filter a view cannot honor is refused.",
			},
			flags: []intentFlag{
				{name: "all", aliases: []string{"done"}, usage: "every goal: queued and parked ones too, and the done and abandoned"},
				{name: "label", value: "LABEL", repeat: true, usage: "only goals carrying every named label"},
				{name: "ready", usage: "the ready frontier: the goal this machine continues or claims next"},
				{name: "tiers", usage: "the recorded and derived tiers, and the goals a person may lower"},
				{name: "machine", value: "NAME", advanced: true, usage: "with --ready: the machine whose frontier is read"},
				{name: "history", advanced: true, usage: "include each goal's ledger history"},
				{name: "fetch", advanced: true, usage: "fetch and validate the canonical ledger before reading"},
				{name: "pretty", advanced: true, usage: "with --json: indented JSON (the JSON result is always indented)"},
			},
			maxArgs:  0,
			examples: []string{"metasystem goal list", "metasystem goal list --all --label ui", "metasystem goal list --ready", "metasystem goal list --tiers"},
			run:      runIntentGoalViews,
		},
		{
			object: "goal", action: "show", audience: "both", laidOut: true, summary: "one goal's record: intent, next step, budget and designs",
			usage:    []string{"metasystem goal show G [--history]"},
			details:  []string{"goal show G is the goal's record; status G is its live work."},
			flags:    []intentFlag{intentTargetFlag, {name: "history", advanced: true, usage: "include the goal's ledger history"}},
			maxArgs:  1,
			examples: []string{"metasystem goal show verbs-match-intent", "metasystem goal show verbs-match-intent --history"},
			run:      runIntentShow,
		},
		{
			object: "goal", action: "approve", primary: true, audience: "human", laidOut: true, summary: "approve goals for execution",
			usage: []string{"metasystem goal approve G... [--budget BOX]"},
			details: []string{
				"Without --budget each goal is approved under its own tier's norm box, all goals in one act.",
				"BOX is norm or the complete compact box, for example 1d/10/720m/1/3 (elapsed/attempts/job minutes/active jobs/review rounds).",
				"Run it at your enrolled terminal (metasystem system enroll --name NAME enrolls one); --under GRANT is a seat's act under a recorded power of attorney.",
			},
			flags: []intentFlag{
				{name: "id", aliases: []string{"goal"}, value: "G", repeat: true, advanced: true, usage: "a goal to approve (repeatable)"},
				{name: "budget", value: "BOX", usage: "the box: norm or the complete compact box"},
				{name: "under", value: "GRANT", advanced: true, usage: "act under this recorded power of attorney"},
				intentByFlag, intentLineageFlag, intentApprovedRefFlag, intentTemporaryWordFlag, intentReviewByFlag, intentFixtureFlag,
				intentLongBudgetFlags[0], intentLongBudgetFlags[1], intentLongBudgetFlags[2], intentLongBudgetFlags[3], intentLongBudgetFlags[4],
			},
			maxArgs:  -1,
			examples: []string{"metasystem goal approve verbs-match-intent", "metasystem goal approve goal-a goal-b", "metasystem goal approve verbs-match-intent --budget 1d/10/720m/1/3"},
			run:      runIntentApproveWithLimits,
		},
		{
			object: "goal", action: "budget", audience: "human", summary: "read a goal's budget, or give it a box",
			usage: []string{"metasystem goal budget G", "metasystem goal budget G BOX"},
			details: []string{
				"Without BOX: the standing box and what has been spent, the same block goal show prints.",
				"BOX is norm (the tier's box), keep (the standing box) or the complete compact box 1d/10/720m/1/3.",
				"A queued or parked goal is approved with the box; a running goal's box is changed.",
				"A goal stopped by its budget resumes under its standing box first: metasystem goal resume G.",
			},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "budget", value: "BOX", advanced: true, usage: "the box, as an alternative to naming it after G"},
				{name: "under", value: "GRANT", advanced: true, usage: "act under this recorded power of attorney"},
				intentByFlag, intentLineageFlag, intentApprovedRefFlag, intentTemporaryWordFlag, intentReviewByFlag, intentFixtureFlag,
				intentLongBudgetFlags[0], intentLongBudgetFlags[1], intentLongBudgetFlags[2], intentLongBudgetFlags[3], intentLongBudgetFlags[4],
			},
			maxArgs:  2,
			examples: []string{"metasystem goal budget verbs-match-intent", "metasystem goal budget verbs-match-intent norm", "metasystem goal budget verbs-match-intent 2d/12/900m/2/3"},
			run:      runIntentBudgetWithLimits,
		},
		{
			object: "goal", action: "pause", audience: "both", laidOut: true, summary: "park a goal with a reason",
			usage: []string{"metasystem goal pause G --reason TEXT"},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "reason", aliases: []string{"because"}, value: "TEXT", usage: "why the goal is parked"},
				fileFlag("reason", "read the reason from FILE"),
				intentByFlag, intentLineageFlag, intentFixtureFlag,
			},
			maxArgs:  1,
			examples: []string{"metasystem goal pause verbs-match-intent --reason 'waits for the design review'"},
			run:      runIntentPause,
		},
		{
			object: "goal", action: "resume", audience: "human", summary: "resume a parked goal, or a stopped goal under its standing box",
			usage: []string{"metasystem goal resume G", "metasystem goal resume G --under GRANT --verified TEXT"},
			details: []string{
				"A parked goal returns to the queue; approval is not granted by resuming.",
				"A goal stopped by its budget resumes under its standing approved box; change the box afterwards with goal budget.",
				"--under and --verified are a seat's unpark under a power of attorney, for parked goals only.",
			},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "under", value: "GRANT", advanced: true, usage: "unpark under this recorded power of attorney"},
				{name: "verified", value: "TEXT", advanced: true, usage: "with --under: what the seat verified holds now"},
				fileFlag("verified", "read what the seat verified from FILE"),
				intentByFlag, intentLineageFlag, intentApprovedRefFlag, intentTemporaryWordFlag, intentReviewByFlag, intentFixtureFlag,
			},
			maxArgs:  1,
			examples: []string{"metasystem goal resume verbs-match-intent"},
			run:      runIntentResume,
		},
		{
			object: "goal", action: "done", audience: "both", summary: "conclude a goal",
			usage:   []string{"metasystem goal done G --reason TEXT [--force]"},
			details: []string{"The goal's obligations are checked first; its merged branch is swept afterwards. A goal conclusion needs --reason."},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "reason", aliases: []string{"conclude"}, value: "TEXT", usage: "the conclusion"},
				fileFlag("reason", "read the reason from FILE"),
				{name: "force", usage: "at the helm: conclude despite open read items, review obligations, a carry word or a blocked dependency; each is recorded as overridden"},
				intentByFlag, intentLineageFlag,
			},
			maxArgs:  1,
			examples: []string{"metasystem goal done verbs-match-intent --reason 'all four slices landed'"},
			run:      runIntentDone,
		},
	}
}

// findIntentCommand finds a public or hidden descriptor by its name, "object
// action" or "status".
func findIntentCommand(name string) (intentCommand, bool) {
	for _, command := range intentCommands() {
		if command.name == name {
			return command, true
		}
	}
	return intentCommand{}, false
}

// findIntentAction finds the descriptor of one (object, action) pair.
func findIntentAction(object, action string) (intentCommand, bool) {
	return findIntentCommand(strings.TrimSpace(object + " " + action))
}

// words is the command's own spelling as argument-vector words.
func (c intentCommand) words() []string {
	return strings.Fields(c.name)
}

// allFlags is the command's own options plus those every command takes:
// --repo, --json, and --verbose, which shows a result's details ("Messages a
// Person Reads"); a command that groups findings documents its own --verbose.
func (c intentCommand) allFlags() []intentFlag {
	flags := append(append([]intentFlag(nil), c.flags...), intentRepoFlag, intentJSONFlag)
	if !slices.ContainsFunc(c.flags, func(flag intentFlag) bool { return flag.name == intentVerboseFlag.name }) {
		details := intentVerboseFlag
		details.hidden, details.usage = true, "also print the details behind the result"
		flags = append(flags, details)
	}
	return flags
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
	set := newFlagSet(command.name, io.Discard, io.Discard)
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
			summary := fmt.Sprintf("--%s cannot be %q: %v", one.name, one.value, err)
			if definition.value == "" {
				summary = fmt.Sprintf("--%s takes true or false, not %q", one.name, one.value)
			}
			return input, &intentInputError{summary: summary, reason: "see metasystem help " + command.name}
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
		takes := map[int]string{0: "no target", 1: "one target"}[command.maxArgs]
		if takes == "" {
			takes = fmt.Sprintf("at most %d targets", command.maxArgs)
		}
		return input, &intentInputError{
			summary: fmt.Sprintf("%s takes %s; unexpected %s", command.name, takes, shellCommand(extra)),
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
	// A guess is offered only when it is close for the name's length: a
	// far one names an unrelated option.
	if closest*3 > len(name) {
		near = nil
	}
	if len(near) == 1 {
		corrected := append(append([]string{"metasystem"}, command.words()...), raw...)
		token := raw[index]
		_, value, joined := strings.Cut(token, "=")
		corrected[1+len(command.words())+index] = "--" + near[0]
		if joined {
			corrected[1+len(command.words())+index] += "=" + value
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
	resolver   stateroot.Resolver
	prove      goalAuthorityProver
	commandNow func(string) (time.Time, error)
	// lookupEnv answers the environment app runs resolve their evidence
	// root under; nil is os.LookupEnv.
	lookupEnv func(string) (string, bool)
	// appSupervisorWait is how long an app start waits for its supervisor's
	// answer; zero is production's contract wait plus ten seconds, and
	// applaunch.WaitForReport waits for the answer or the supervisor's exit.
	appSupervisorWait time.Duration
	dependencies      syncRequestDependencies
	binding           goalBindingResolver
	parkBranchCheck   func(string, goal.Endpoint) func(string, string) (string, error)
	completion        completionInputs
	processes         processIntentOwners
	work              intentWorkOwners
	delivery          *intentDeliveryOwners
	connection        intentConnectionOwners
	// adopt is the adoption owner system adopt calls; nil selects
	// internal/adopt.
	adopt func(adopt.Options) (adopt.Result, error)
	helm  helmOwners
	// attorney are the general power of attorney's seams; the zero value is
	// production.
	attorney attorneyIntentOwners
	// agent are the agent verbs' seams; the zero value is production.
	agent agentOwners
	// hookSwitch adjusts system setup's seams; nil keeps production.
	hookSwitch func(hookswitch.Deps) hookswitch.Deps
	// sentBackRevise runs the one work revise a sent-back goal's holder
	// performs on the published brief; nil runs the public verb in-process.
	sentBackRevise func(inv *intentInvocation, raw []string) intentResult
	// stopMovesDeclare writes a Stop decision move declaration; nil selects
	// audit.DeclareStopDecisionSurface.
	stopMovesDeclare func(string, audit.StopSurfaceOptions, string, string) (string, error)
	// disk are the disk verbs' seams; the zero value is production.
	disk diskOwners
	// appEngine names the engine an app start launches as its supervisor;
	// nil is this executable. It is an owner of the invocation, never a
	// process-wide variable, so parallel invocations cannot swap it.
	appEngine func() (string, error)
	// landing are the landing verbs' seams; the zero value is production.
	landing laneVerbOwners
	// deploy are the deploy verbs' seams; the zero value is production.
	deploy deployOwners
	// alerts are the alert verbs' seams; the zero value is production.
	alerts alertOwners
	// machines are the machine verbs' seams; the zero value is production.
	machines machineOwners
	// rosters are the roster verbs' seams; the zero value is production.
	rosters rosterOwners
	// textEnv is the text layout of one output stream; nil detects it
	// (inv.textEnv).
	textEnv func(stream io.Writer) textui.Env
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
	// entrants are the registered stores this verb is inside, each held
	// shared until the verb ends (Part B 3.1 "Entrants").
	entrants []*diskstore.Entrant
	// notices holds the helm and grant admission notices until the result
	// says whether the act proceeded; nil prints them at once.
	notices *admissionNotices
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
	// An owner that prints does so on this invocation's streams, unless the
	// caller gave the owners streams of their own.
	if owners.dependencies.stdout == nil {
		owners.dependencies.stdout = stdout
	}
	if owners.dependencies.stderr == nil {
		owners.dependencies.stderr = stderr
	}
	inv := &intentInvocation{command: command, raw: raw, stdout: stdout, stderr: stderr, cwd: cwd, owners: owners}
	if release, held := processAdmissionNotices.hold(); held {
		inv.notices = processAdmissionNotices
		defer release()
	}
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
	defer inv.leaveStores()
	return command.run(inv)
}

// leaveStores drops every shared hold the verb took on a store it entered.
func (inv *intentInvocation) leaveStores() {
	for _, entrant := range inv.entrants {
		_ = entrant.Leave()
	}
	inv.entrants = nil
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
		var root stateroot.State
		root, err = inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
		inv.stateRoot = root.Path()
	}
	if err != nil {
		return inv.notARepository(path, err)
	}
	if !converted(inv.stateRoot) {
		return &intentResult{Outcome: intentRefused, code: 1,
			Summary: "this repository's goals are still in the old goals file, so nothing was done",
			next:    []string{"metasystem", "goal", "sync", "--upgrade", "--by", inv.knownPerson()}, nextReason: "a person converts it; it shows the file to review",
			Details: []string{"the installation at " + shellCommand([]string{inv.stateRoot}) + " has no synced goal ledger"}}
	}
	return nil
}

// notARepository refuses a command run outside a repository with a
// metasystem installation: line 2 is the command again, naming the
// repository, whose path the engine cannot know.
func (inv *intentInvocation) notARepository(path string, err error) *intentResult {
	return &intentResult{Outcome: intentRefused, code: 2, Summary: notAnInstallation(path, err),
		next: append(withoutOption(inv.typedArgv(), "repo"), "--repo", "PATH"), nextReason: "inside the repository, or naming its path",
		Details: []string{"not an installation because: " + err.Error()}}
}

// retryWith is this command as the person typed it, less the options drop
// names (a switch, or an option with its value), plus extra words: the line 2
// of a refusal whose fix is a changed option.
func (inv *intentInvocation) retryWith(drop []string, extra ...string) []string {
	argv := append([]string{"metasystem"}, inv.command.words()...)
	for index := 0; index < len(inv.raw); index++ {
		token := inv.raw[index]
		name, _, joined := strings.Cut(strings.TrimLeft(token, "-"), "=")
		if strings.HasPrefix(token, "-") && slices.Contains(drop, name) {
			if definition, known := inv.command.lookupFlag(name); known && definition.value != "" && !joined {
				index++
			}
			continue
		}
		argv = append(argv, token)
	}
	return append(argv, extra...)
}

// knownPerson is the person a remedy names: the one at this seat's helm,
// else the enrolled person, else NAME when nobody is known.
func (inv *intentInvocation) knownPerson() string {
	if name := inv.personName(""); name != "" {
		return name
	}
	if inv.stateRoot != "" {
		if enrollment, err := humanauthority.ReadEnrollment(inv.stateRoot); err == nil && enrollment.Human != "" {
			return enrollment.Human
		}
	}
	return "NAME"
}

// notAnInstallation says why path is not a metasystem installation in plain
// words: it does not exist, it is not inside a Git repository, or the
// repository has no installation. The resolver's own wording is kept only
// for a cause none of these names. The cause is err's type, never its words.
func notAnInstallation(path string, err error) string {
	shown := shellCommand([]string{path})
	if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
		return shown + " does not exist; nothing was done"
	}
	if errors.Is(err, stateroot.ErrNotInRepository) {
		return shown + " is not inside a Git repository; nothing was done"
	}
	return shown + " is not inside a repository with a metasystem installation; nothing was done"
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
	// Details are what only --verbose prints: refusal codes, paths the fix
	// does not need, background ("Messages a Person Reads").
	Details []string `json:"details,omitempty"`

	text       []string
	next       []string
	nextReason string
	// retry, when no next is set, makes line 2 this command as the person
	// typed it, with retry as its reason: the remedy of a failure whose
	// cause may pass (an unreadable record, a lost race).
	retry string
	code  int
	// view draws a converted verb's text page; nil renders the legacy
	// shape. --json never reads it.
	view func(*textui.Page)
	// viewsRefusal lets view draw a refusal or failure as well: its words
	// differ from --json's (a path shortened), and it draws both lines.
	viewsRefusal bool
	// attention is the banner above the headline (P12): the standing
	// conditions that change what the person may do.
	attention func(textui.Env) []textui.Attention
	// headline is the text headline when --json's Summary carries another
	// line (withHelm keeps the helm line there).
	headline *string
}

func (inv *intentInvocation) render(result intentResult) int {
	result.SchemaVersion = 1
	result.Verb = inv.command.name
	if result.Targets == nil {
		result.Targets = []intentTarget{}
	}
	if len(result.next) == 0 && result.retry != "" && result.Decision == "" && inv.command.name != "" {
		result.next, result.nextReason = inv.typedArgv(), result.retry
	}
	if len(result.next) > 0 {
		result.Next = &intentNext{Argv: result.next, Reason: result.nextReason}
	}
	code := result.code
	proceeded := result.Outcome == intentConfirmed || result.Outcome == intentUnchanged || result.Outcome == intentPartial || result.Outcome == intentInProgress
	if code == 0 && result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		code = 1
	}
	verbose := inv.input.switched("verbose")
	if inv.notices != nil {
		result.Details = append(result.Details, inv.notices.settle(!proceeded, verbose && !inv.input.switched("json"))...)
	}
	if inv.input.switched("json") {
		// Line 2 that is a reason without a command ("nothing to do; why")
		// is next with an empty argv, so a reader shows both lines too.
		if result.Next == nil && result.nextReason != "" {
			result.Next = &intentNext{Argv: []string{}, Reason: result.nextReason}
		}
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(inv.stderr, "metasystem %s: %v\n", inv.command.name, err)
			return 1
		}
		fmt.Fprintln(inv.stdout, string(encoded))
		return code
	}
	succeeded := result.Outcome == intentConfirmed || result.Outcome == intentUnchanged
	stream := inv.stdout
	if !succeeded {
		stream = inv.stderr
	}
	env := inv.textEnv(stream)
	viewed := result.view != nil && (succeeded || result.viewsRefusal)
	page := textui.NewLegacy(env)
	if viewed || inv.command.laidOut {
		page = textui.New(env)
	}
	if result.attention != nil {
		page.Banner(result.attention(env)...)
	}
	switch {
	case viewed:
		// A refusal's view draws its own two lines, the hint among them.
		if result.Next != nil && succeeded {
			page.Hint(inv.hintFor(result.Next))
		}
		result.view(page)
	case succeeded:
		inv.legacyConfirmed(page, result)
	default:
		inv.legacyRefused(page, result)
	}
	if verbose {
		page.Legacy(detailLines(result.Details)...)
	}
	_, _ = io.WriteString(stream, page.String())
	return code
}

// legacyConfirmed is an unconverted verb's result (§4 step 4): its Summary
// is the headline, its lines print as they are, its next step is the hint.
func (inv *intentInvocation) legacyConfirmed(page *textui.Page, result intentResult) {
	summary := result.Summary
	if result.headline != nil {
		summary = *result.headline
	}
	if summary != "" {
		page.Headline(summary)
	}
	page.Legacy(result.text...)
	if result.Next != nil {
		page.Hint(inv.hintFor(result.Next))
	}
}

// legacyRefused is an unconverted verb's refusal or failure: its sentence
// behind ✗ (D2: the verb's name only with --verbose), its lines, and the
// remedy as the indented hint.
func (inv *intentInvocation) legacyRefused(page *textui.Page, result intentResult) {
	summary := strings.TrimSpace(result.Summary)
	if result.headline != nil {
		summary = strings.TrimSpace(*result.headline)
	}
	if inv.input.switched("verbose") {
		summary = "metasystem " + inv.command.name + ": " + summary
	}
	var hint textui.Hint
	// An owner's message brings its own line 2 ("Messages a Person Reads");
	// it is the hint unless the verb named a remedy of its own.
	retried := result.retry != "" && result.Next != nil && slices.Equal(result.Next.Argv, inv.typedArgv())
	var middle []string
	if first, between, owned, ok := ownerRemedy(summary); ok && result.Decision == "" && (result.Next == nil || retried) {
		summary, middle, hint = first, between, owned
	}
	switch {
	case len(hint.Argv) > 0 || hint.Reason != "":
	case result.Next != nil:
		hint = inv.hintFor(result.Next)
	case result.Decision != "":
		hint = textui.Hint{Reason: result.Decision}
	case result.nextReason != "":
		hint = textui.Hint{Reason: result.nextReason}
	}
	switch {
	case result.Outcome == intentPartial:
		page.Mark(textui.Alert, summary)
	case result.Outcome == intentInProgress:
		page.Mark(textui.Running, summary)
	case len(result.text) == 0 && (len(middle) == 0 || !inv.input.switched("verbose")):
		page.Refusal(summary, hint)
		return
	default:
		page.Refusal(summary, textui.Hint{})
	}
	if inv.input.switched("verbose") {
		page.Legacy(middle...)
	}
	page.Legacy(result.text...)
	page.Hint(hint)
}

// ownerRemedy splits an owner's message that ends in its own line 2 (a
// "run: C" line, or a "nothing to do; why" line) into its line 1, the lines
// between (which only --verbose prints) and that line 2 as a hint.
func ownerRemedy(text string) (string, []string, textui.Hint, bool) {
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return text, nil, textui.Hint{}, false
	}
	first, middle, last := lines[0], lines[1:len(lines)-1], lines[len(lines)-1]
	if command, isRun := strings.CutPrefix(last, "run: "); isRun {
		if strings.HasPrefix(command, "metasystem ") && !strings.ContainsAny(command, ",;()'\"") {
			return first, middle, textui.Hint{Argv: strings.Fields(command)}, true
		}
		return first, middle, textui.Hint{Reason: command}, true
	}
	if strings.HasPrefix(last, "nothing to do") {
		return first, middle, textui.Hint{Reason: last}, true
	}
	return text, nil, textui.Hint{}, false
}

// textEnv is the layout of one output stream: its width, colour and
// symbols, the invocation's clock and zone, and the paths it shortens. A
// stream that is not a file (a test's or an in-process caller's buffer) is
// laid out the same everywhere: full width, no colour, the symbols.
func (inv *intentInvocation) textEnv(stream io.Writer) textui.Env {
	var env textui.Env
	if inv.owners.textEnv != nil {
		env = inv.owners.textEnv(stream)
	} else {
		now := time.Now()
		if inv.owners.commandNow != nil && inv.stateRoot != "" {
			if at, err := inv.owners.commandNow(inv.stateRoot); err == nil {
				now = at
			}
		}
		zone := inv.owners.helm.withDefaults().zone
		if file, ok := stream.(*os.File); ok {
			env = textui.Detect(file.Fd(), os.Getenv, now, zone)
		} else {
			env = textui.DetectWith(false, 0, func(string) string { return "" }, now, zone)
		}
		env.Home, _ = os.UserHomeDir()
		env.Repo = inv.layout.GitRoot
		env.InRepo = env.Repo != "" && withinDirectory(inv.cwd, env.Repo)
	}
	env.Verbose = env.Verbose || inv.input.switched("verbose")
	return env
}

// detailLines are a result's details as --verbose prints them, indented
// under the result.
func detailLines(details []string) []string {
	var lines []string
	for _, line := range details {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, "  "+line)
		}
	}
	return lines
}

// withinDirectory reports whether path is dir or lies below it.
// hintFor is a result's next step as the page prints it: a --repo naming
// the checkout the command already runs in is dropped from the text, while
// the result's argv, which --json prints, keeps it.
func (inv *intentInvocation) hintFor(next *intentNext) textui.Hint {
	argv := next.Argv
	if at := slices.Index(argv, "--repo"); at >= 0 && at+1 < len(argv) && inv.layout.GitRoot != "" && inv.cwd != "" {
		checkout := realpath.Resolve(inv.layout.GitRoot)
		if realpath.Resolve(argv[at+1]) == checkout && withinDirectory(realpath.Resolve(inv.cwd), checkout) {
			argv = append(slices.Clone(argv[:at]), argv[at+2:]...)
		}
	}
	return textui.Hint{Argv: argv, Reason: next.Reason}
}

func withinDirectory(path, dir string) bool {
	relative, err := filepath.Rel(dir, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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
	// refusalCode is the owner's refusal code, a detail --verbose and --json
	// show ("Messages a Person Reads").
	refusalCode string
}

// refusalCodeDetails is a refusal code as the detail --verbose prints.
func refusalCodeDetails(codes ...string) []string {
	for _, code := range codes {
		if code != "" {
			return []string{"refusal code: " + code}
		}
	}
	return nil
}

func (d syncRequestDependencies) publish(res goal.PublishResult, err error) int {
	if d.report == nil {
		return writeSyncResult(d.outStream(), d.errStream(), res, err)
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
		writeJSONLine(d.outStream(), d.errStream(), publicationRecord(res))
		return
	}
	d.report.result = &res
}

func (d syncRequestDependencies) fail(code int, err error) int {
	if d.report == nil {
		fmt.Fprintln(d.errStream(), err)
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
		fmt.Fprintln(d.outStream(), line)
	default:
		fmt.Fprintln(d.errStream(), line)
	}
}

func (d syncRequestDependencies) noteWriter() io.Writer {
	if d.report == nil {
		return d.errStream()
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
		result := intentResult{Outcome: intentRefused, Summary: report.refusal.sentence, text: lines, code: report.refusal.code,
			next: shellWords(report.refusal.remedy.command), Decision: strings.TrimSpace(report.refusal.remedy.words)}
		if report.result != nil {
			result.Details = refusalCodeDetails(report.refusal.refusalCode, report.result.Code)
		} else {
			result.Details = refusalCodeDetails(report.refusal.refusalCode)
		}
		if unchanged {
			result.Outcome, result.code = intentUnchanged, 0
		}
		if len(result.next) > 0 {
			result.Decision = ""
		}
		if report.result != nil {
			result.Data = map[string]any{"owner": ownerPublication(*report.result)}
		}
		return result
	case report.failure != nil:
		return intentResult{Outcome: intentRefused, Summary: report.failure.Error(), text: lines, code: max(code, 1), retry: "once the cause above is fixed",
			Details: refusalCodeDetails(goal.RefusalCode(report.failure))}
	case landed && code == 0:
		confirmed.Outcome = intentConfirmed
		confirmed.text = append(lines, confirmed.text...)
		return confirmed
	case unchanged:
		return intentResult{Outcome: intentUnchanged, Summary: report.result.Detail, text: lines, Data: map[string]any{"owner": ownerPublication(*report.result)}}
	case report.result != nil:
		// A rejection is a rule's refusal: the same command is refused again,
		// so its second line reads the goal the rule judged, never the
		// command again. Any other outcome here (a lost race, a passed
		// deadline) has a cause that may pass, and the command is offered
		// again.
		result := intentResult{Outcome: intentRefused, Summary: report.result.Detail, text: lines, code: max(code, 1),
			next: goalShowArgv(confirmed.Targets), nextReason: "shows the goal as the rule above judged it",
			Data: map[string]any{"owner": ownerPublication(*report.result)}, Details: refusalCodeDetails(report.result.Code)}
		if report.result.Outcome != goal.OutcomeRejected {
			result.next, result.nextReason, result.retry = nil, "", "the goals changed meanwhile; try again"
		}
		// A rejection whose second line names the command that clears it
		// ("run: CMD  (why)") carries that command as the next step. A
		// second line in another form (the helm's force proposal appended
		// to a remedy) is printed as the owner wrote it.
		if first, second, found := strings.Cut(report.result.Detail, "\nrun: "); found && !strings.Contains(second, "\n") && strings.HasSuffix(second, ")") {
			if command, why, reasoned := strings.Cut(second, "  ("); reasoned {
				result.Summary, result.next, result.nextReason, result.retry = first, shellWords(command), strings.TrimSuffix(why, ")"), ""
			}
		}
		return result
	}
	return intentResult{Outcome: intentFailed, code: max(code, 1), text: lines,
		Summary: "the command stopped without saying whether it was done", next: []string{"metasystem", "system", "check"},
		nextReason: "names what is wrong here"}
}

// goalShowArgv reads the goal an act targets, or nothing when it names none.
func goalShowArgv(targets []intentTarget) []string {
	for _, target := range targets {
		if target.Kind == "goal" && target.ID != "" {
			return []string{"metasystem", "goal", "show", target.ID}
		}
	}
	return nil
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
	return publicationRecord(res)
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
// the objects, actions and options the router accepts; hidden entries never
// appear.

// intentGroup is one help area: its objects in the order help prints them.
type intentGroup struct {
	name, heading string
	objects       []string
}

var intentGroups = []intentGroup{
	{"plan", "Plan", []string{"goal", "design", "decision", "grant"}},
	{"deliver", "Deliver", []string{"work", "test", "question", "agent", "incident"}},
	{"run", "Run", []string{"status", "helm", "session", "mission", "system", "landing", "deploy", "alert", "machine", "disk", "evidence", "app", "ui", "settings", "roster"}},
	{"practice", "Practice", []string{"receipt", "experiment"}},
}

// intentObjectSummaries say in one line what each object is.
var intentObjectSummaries = map[string]string{
	"goal":       "the backlog: open, approve, budget, claim and conclude goals",
	"design":     "a goal's design: write, review, list and read designs",
	"decision":   "the project's recorded decisions",
	"grant":      "powers of attorney a seat acts under",
	"work":       "a goal's work: brief, build, review, revise, land, finish, wait and stop",
	"test":       "risk-selected tests and their proof",
	"question":   "questions for a person, and their answers",
	"agent":      "messages between the agents on this host: ask, reply and read, never a person",
	"incident":   "failures on main that someone must own",
	"status":     "the overview of this checkout, or one goal's work",
	"helm":       "human at the helm: take the whole seat out of the machinery's hands, and give it back",
	"session":    "this agent session: start, stop, whether it may stop, and its handoff",
	"mission":    "autonomous missions",
	"system":     "MetaSystem for this checkout: set up, start, stop, restart, status, check, enroll, adopt, completion",
	"landing":    "the landing lane on this computer: where every seat's work is proved and pushed",
	"deploy":     "this project's deploy of main: status, now, rollback, pause and resume",
	"alert":      "what the steward reported and a person should see: list, acknowledge and clear",
	"machine":    "the fleet's machines",
	"disk":       "what MetaSystem keeps on this computer's disk, and reclaiming it",
	"evidence":   "evidence outside the checkout: its bound, verified exports, and a person's disposal",
	"app":        "the application this project builds, under its launch contract",
	"ui":         "the browser interface",
	"settings":   "MetaSystem settings and coordination",
	"roster":     "this computer's rosters: which agent, model and effort does each kind of work",
	"receipt":    "task receipts and the retro cadence",
	"experiment": "measured-improvement experiments and their stop-loss",
}

// intentAdministrationObjects configure or repair MetaSystem itself.
var intentAdministrationObjects = []string{"system", "machine", "disk", "evidence", "ui", "settings", "roster"}

func intentObjectGroup(object string) string {
	for _, group := range intentGroups {
		if slices.Contains(group.objects, object) {
			return group.name
		}
	}
	return ""
}

// intentObjects are the public first words that are objects, in help order.
func intentObjects() []string {
	var objects []string
	for _, group := range intentGroups {
		for _, object := range group.objects {
			if object != "status" {
				objects = append(objects, object)
			}
		}
	}
	return objects
}

func isIntentObject(word string) bool { return slices.Contains(intentObjects(), word) }

func (command intentCommand) allUsage() []string {
	return append(slices.Clone(command.usage), command.administrationUsage...)
}

// publicIntentCommands are the rows help lists: every row but the hidden
// entries.
func publicIntentCommands() []intentCommand {
	var public []intentCommand
	for _, command := range intentCommands() {
		if !command.hidden {
			public = append(public, command)
		}
	}
	return public
}

// objectActions are one object's public actions, in table order.
func objectActions(object string) []intentCommand {
	var actions []intentCommand
	for _, command := range publicIntentCommands() {
		if command.object == object {
			actions = append(actions, command)
		}
	}
	return actions
}

// helpEnv is the layout of a help page written to w: a terminal's width,
// colour and symbols, and for anything else the full width without colour.
func helpEnv(w io.Writer) textui.Env {
	if file, ok := w.(*os.File); ok {
		return textui.Detect(file.Fd(), os.Getenv, time.Now(), time.Local)
	}
	return textui.DetectWith(false, 0, func(string) string { return "" }, time.Now(), time.Local)
}

// helpRows are a help list's name and summary rows, the names padded to the
// widest so the summaries align across every section of the page.
type helpRow struct{ name, summary string }

func helpColumn(rows []helpRow) int {
	widest := 0
	for _, row := range rows {
		widest = max(widest, len([]rune(row.name)))
	}
	return widest
}

func addHelpRows(section *textui.Section, column int, rows []helpRow) {
	for _, row := range rows {
		section.KV(row.name+strings.Repeat(" ", column-len([]rune(row.name))), textui.Plain(row.summary))
	}
}

func writeIntentRootHelp(w io.Writer) {
	_, _ = io.WriteString(w, intentRootHelpPage(helpEnv(w)).String())
}

// intentRootHelpPage is the top-level help (output-style §6.13): the
// objects by area, a typical delivery, where the other pages are, and the
// one command to start with.
func intentRootHelpPage(env textui.Env) *textui.Page {
	page := textui.New(env)
	page.Headline("metasystem", "say what you want done to what; it prepares, runs, collects and recovers the work")
	var all []helpRow
	for _, group := range intentGroups {
		for _, object := range group.objects {
			all = append(all, helpRow{object, intentObjectSummaries[object]})
		}
	}
	column := helpColumn(all)
	for _, group := range intentGroups {
		var rows []helpRow
		for _, object := range group.objects {
			rows = append(rows, helpRow{object, intentObjectSummaries[object]})
		}
		addHelpRows(page.Section(group.heading, ""), column, rows)
	}
	delivery := page.Section("A typical delivery", "")
	for _, example := range []string{
		"metasystem work build my-goal --brief brief.md --check go test ./...",
		"metasystem work review my-goal",
		"metasystem work land my-goal",
	} {
		delivery.Text(example)
	}
	more := []helpRow{
		{"metasystem OBJECT", "that object's actions"},
		{"metasystem OBJECT ACTION --help", "one action's forms, options and examples"},
		{"metasystem help [OBJECT [ACTION]]", "the same pages; help agent, help human, help all by audience"},
	}
	addHelpRows(page.Section("More", ""), helpColumn(more), more)
	page.Facts(textui.KV{Key: "usage", Value: []textui.Span{textui.Plain("metasystem OBJECT ACTION [TARGET...] [OPTIONS]")}},
		textui.KV{Value: []textui.Span{textui.Plain("options go before or after the target; --repo PATH takes any path inside the repository")}})
	page.Hint(textui.Hint{Argv: []string{"metasystem", "status"}, Reason: "what is going on in this checkout"})
	return page
}

// intentActionIntents group the actions of an object with more than
// intentHelpGroupAbove of them by what a person does with them (output-style
// D4): read, decide, work, shape. An object with fewer is one list.
const intentHelpGroupAbove = 8

var intentActionIntents = map[string][]struct {
	heading string
	actions []string
}{
	"goal": {
		{"Read", []string{"list", "show", "notes"}},
		{"Decide", []string{"open", "approve", "unapprove", "budget", "prioritize", "pin", "allow", "disallow", "accept-risk", "review", "land-without-sitting"}},
		{"Work", []string{"claim", "release", "pause", "resume", "done", "reopen", "abandon"}},
		{"Shape", []string{"edit", "split", "group", "ungroup", "block", "unblock", "sync"}},
	},
	"work": {
		{"Read", []string{"status", "wait"}},
		{"Work", []string{"brief", "build", "workspace", "review", "revise", "land", "finish", "stop"}},
	},
	"test": {
		{"Read", []string{"plan", "list", "status", "wait"}},
		{"Work", []string{"run", "declare-moves", "baseline"}},
		{"Shape", []string{"add", "remove"}},
	},
	"landing": {
		{"Read", []string{"status"}},
		{"Work", []string{"run", "start", "stop", "prove", "push", "return"}},
		{"Shape", []string{"set", "unset"}},
	},
	"system": {
		{"Read", []string{"status", "check"}},
		{"Decide", []string{"enroll"}},
		{"Work", []string{"start", "stop", "restart"}},
		{"Shape", []string{"setup", "adopt", "completion"}},
	},
}

// writeIntentObjectHelp lists one object's actions.
func writeIntentObjectHelp(w io.Writer, object string) {
	_, _ = io.WriteString(w, intentObjectHelpPage(helpEnv(w), object).String())
}

// intentObjectHelpPage is one object's page (output-style §6.12): its
// actions in one aligned column, grouped by intent when there are many.
func intentObjectHelpPage(env textui.Env, object string) *textui.Page {
	page := textui.New(env)
	page.Headline("metasystem "+object, intentObjectSummaries[object])
	actions := objectActions(object)
	var all []helpRow
	summaries := map[string]string{}
	for _, command := range actions {
		all = append(all, helpRow{command.action, command.summary})
		summaries[command.action] = command.summary
	}
	column := helpColumn(all)
	if len(actions) <= intentHelpGroupAbove || intentActionIntents[object] == nil {
		var rows []textui.KV
		for _, row := range all {
			rows = append(rows, textui.KV{Key: row.name, Value: []textui.Span{textui.Plain(row.summary)}})
		}
		page.Facts(rows...)
	} else {
		listed := map[string]bool{}
		for _, group := range intentActionIntents[object] {
			var rows []helpRow
			for _, action := range group.actions {
				if summary, ok := summaries[action]; ok {
					rows = append(rows, helpRow{action, summary})
					listed[action] = true
				}
			}
			addHelpRows(page.Section(group.heading, ""), column, rows)
		}
		var rest []helpRow
		for _, row := range all {
			if !listed[row.name] {
				rest = append(rest, row)
			}
		}
		addHelpRows(page.Section("More", ""), column, rest)
	}
	page.Hint(textui.Hint{Argv: []string{"metasystem", object, "ACTION", "--help"}, Reason: "one action's forms, options and examples"})
	return page
}

// writeIntentLong lists commands with every usage, the summary and the first
// example.
func writeIntentLong(w io.Writer, commands []intentCommand) {
	for _, command := range commands {
		for _, usage := range command.allUsage() {
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

// intentTopic reports whether a help word is an audience topic rather than
// an object.
func intentTopic(name string) bool {
	return slices.Contains([]string{"human", "agent", "all"}, name)
}

func writeIntentGroupHelpFor(w io.Writer, group intentGroup, audience string) {
	fmt.Fprintf(w, "%s (metasystem OBJECT ACTION --help for options):\n", group.heading)
	for _, object := range group.objects {
		writeIntentLong(w, intentCommandsWhere(func(command intentCommand) bool {
			return command.object == object && (audience != "human" || command.audience != "agent")
		}))
	}
}

func writeIntentTopicHelp(w io.Writer, topic string) {
	switch topic {
	case "human":
		fmt.Fprintln(w, "For people (metasystem OBJECT ACTION --help for options):")
		for _, group := range intentGroups {
			fmt.Fprintln(w)
			writeIntentGroupHelpFor(w, group, "human")
		}
	case "agent":
		writeIntentAgentHelp(w)
	case "all":
		for index, group := range intentGroups {
			if index > 0 {
				fmt.Fprintln(w)
			}
			writeIntentGroupHelpFor(w, group, "")
		}
	}
}

// writeIntentHelp is the page OBJECT ACTION --help and help OBJECT ACTION
// print.
func writeIntentHelp(w io.Writer, command intentCommand) {
	writeIntentCommandHelp(w, command)
}

func writeIntentCommandHelp(w io.Writer, command intentCommand) {
	_, _ = io.WriteString(w, intentCommandHelpPage(helpEnv(w), command).String())
}

// intentCommandHelpPage is one action's page: what it does, its forms, what
// else a person should know, its options and examples.
func intentCommandHelpPage(env textui.Env, command intentCommand) *textui.Page {
	page := textui.New(env)
	page.Headline("metasystem "+command.name, command.summary)
	// A form or an example is a command a person pastes: a table cell,
	// never broken across lines (P9).
	forms := page.Section("", "").Table(textui.Column{}, textui.Column{})
	for index, form := range command.usage {
		key := ""
		if index == 0 {
			key = "  usage"
		}
		forms.Row(textui.Plain(key), textui.Plain(form))
	}
	about := page.Section("", "")
	if slices.Contains(intentAdministrationObjects, command.object) {
		about.Text("MetaSystem administration: this action manages the work system itself.")
	}
	for _, paragraph := range helpParagraphs(command.details) {
		about.Text(paragraph)
	}
	if len(command.administrationUsage) > 0 {
		admin := page.Section("MetaSystem administration", "").Table(textui.Column{})
		for _, form := range command.administrationUsage {
			admin.Row(textui.Plain(form))
		}
	}
	type option struct{ spelling, usage string }
	options := map[bool][]option{}
	widest := 0
	for _, definition := range command.helpFlags() {
		if definition.hidden {
			continue
		}
		spelling := "--" + definition.name
		if definition.value != "" {
			spelling += " " + definition.value
		}
		if definition.repeat {
			spelling += " (repeatable)"
		}
		usage := definition.usage
		if len(definition.aliases) > 0 {
			aliases := make([]string, len(definition.aliases))
			for index, alias := range definition.aliases {
				aliases[index] = "--" + alias
			}
			usage += " (also " + strings.Join(aliases, ", ") + ")"
		}
		options[definition.advanced] = append(options[definition.advanced], option{spelling, usage})
		widest = max(widest, len([]rune(spelling)))
	}
	aside := ""
	if command.passthrough == nil {
		aside = "before or after the target; -- ends them"
	}
	for _, advanced := range []bool{false, true} {
		title := "Options"
		if advanced {
			title = "Advanced"
		}
		var rows []helpRow
		for _, option := range options[advanced] {
			rows = append(rows, helpRow{option.spelling, option.usage})
		}
		if len(rows) > 0 {
			addHelpRows(page.Section(title, aside), widest, rows)
			aside = ""
		}
	}
	if len(command.examples) > 0 {
		examples := page.Section("Examples", "").Table(textui.Column{})
		for _, example := range command.examples {
			examples.Row(textui.Plain(example))
		}
	}
	return page
}

// helpParagraphs joins an action's detail lines into paragraphs: a line
// that does not end a sentence runs on into the next, so the page wraps
// whole sentences at its own width.
func helpParagraphs(details []string) []string {
	var paragraphs []string
	open := false
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		if open {
			paragraphs[len(paragraphs)-1] += " " + detail
		} else {
			paragraphs = append(paragraphs, detail)
		}
		open = !strings.ContainsAny(detail[len(detail)-1:], ".!?:;")
	}
	return paragraphs
}

// helpFlags are the options help shows: a parsed action's own options plus
// --repo and --json; a passthrough action's documented options only.
func (command intentCommand) helpFlags() []intentFlag {
	if command.passthrough != nil {
		return command.flags
	}
	return command.allFlags()
}

// Rule C1 of the design: "did you mean" names the current command that was
// probably meant, computed only from the current table; no removed spelling
// is remembered anywhere.

// suggestIntent names the current commands a first word that is not an
// object probably meant. A word that is a current action of one or more
// objects suggests OBJECT ACTION with the caller's remaining words, one per
// object; otherwise the nearest current object by spelling takes the word's
// place, else the nearest current actions by spelling.
func suggestIntent(word string, rest []string) []string {
	var actions []string
	for _, command := range publicIntentCommands() {
		if command.action != "" && command.action == word {
			actions = append(actions, suggestedCommand(command.words(), rest))
		}
	}
	if len(actions) > 0 {
		return actions
	}
	var near []string
	closest := 3
	for _, object := range append(intentObjects(), "status") {
		distance := editDistance(object, word)
		switch {
		case distance < closest:
			closest, near = distance, []string{suggestedCommand([]string{object}, rest)}
		case distance == closest:
			near = append(near, suggestedCommand([]string{object}, rest))
		}
	}
	if len(near) > 0 {
		return near
	}
	closest = 3
	for _, command := range publicIntentCommands() {
		if command.action == "" {
			continue
		}
		distance := editDistance(command.action, word)
		switch {
		case distance < closest:
			closest, near = distance, []string{suggestedCommand(command.words(), rest)}
		case distance == closest:
			near = append(near, suggestedCommand(command.words(), rest))
		}
	}
	if len(near) > 4 {
		near = near[:4]
	}
	if len(near) == 0 {
		// Nothing is close: the nearest object, however far, so the caller
		// always has a real command to go on.
		closest = -1
		for _, object := range intentObjects() {
			if distance := editDistance(object, word); closest < 0 || distance < closest {
				closest, near = distance, []string{suggestedCommand([]string{object}, nil)}
			}
		}
	}
	return near
}

// suggestIntentAction names the current commands an unknown action of object
// probably meant: the object's nearest actions by spelling, else the other
// objects that have that exact action, each with the caller's remaining
// words.
func suggestIntentAction(object, word string, rest []string) []string {
	var near []string
	closest := 3
	for _, command := range objectActions(object) {
		distance := editDistance(command.action, word)
		switch {
		case distance < closest:
			closest, near = distance, []string{suggestedCommand(command.words(), rest)}
		case distance == closest:
			near = append(near, suggestedCommand(command.words(), rest))
		}
	}
	if len(near) > 0 {
		return near
	}
	for _, command := range publicIntentCommands() {
		if command.action == word {
			near = append(near, suggestedCommand(command.words(), rest))
		}
	}
	if len(near) == 0 {
		// Nothing is close: the object's nearest action, however far.
		closest = -1
		for _, command := range objectActions(object) {
			if distance := editDistance(command.action, word); closest < 0 || distance < closest {
				closest, near = distance, []string{suggestedCommand(command.words(), rest)}
			}
		}
	}
	return near
}

// suggestedCommand is one suggestion as the caller would type it.
func suggestedCommand(words, rest []string) string {
	return strings.TrimSpace("metasystem " + strings.Join(words, " ") + " " + shellCommand(rest))
}
