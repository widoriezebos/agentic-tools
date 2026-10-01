package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type humanVerbValues struct {
	verb, root, lineage, id, by, approvedRef, temporaryWord, reviewBy string
	boxTyped                                                          string
	box                                                               *goal.Budget
	standingBox                                                       *goal.Budget
	tierBox                                                           *goal.Budget
	state                                                             string
	fixtureHumanAuthority, stopFence, byTyped                         bool
	// tierless is a goal without a tier. The goal family reads its norm as
	// the tier-three box; the public goal budget refuses norm for it until a
	// person classifies the goal, so a remedy names the box itself.
	tierless bool
	rawArgs  []string
	// refusalCode is the owner refusal's code, kept for --verbose and --json.
	refusalCode string
	// report, when set, receives the refusal for the public intent commands
	// to render; the legacy calls print it below exactly as before.
	report *ownerReport
	// stdout and stderr are the caller's streams (syncRequestDependencies'),
	// nil meaning the process's own: a refusal printed below never reaches a
	// parallel test's capture of the process streams.
	stdout, stderr io.Writer
}

// bindDependencies takes the owner's report and printing streams from the
// caller's dependencies.
func (values *humanVerbValues) bindDependencies(dependencies syncRequestDependencies) {
	values.report = dependencies.report
	values.stdout, values.stderr = dependencies.stdout, dependencies.stderr
}

func (values *humanVerbValues) outStream() io.Writer {
	if values.stdout != nil {
		return values.stdout
	}
	return os.Stdout
}

func (values *humanVerbValues) errStream() io.Writer {
	if values.stderr != nil {
		return values.stderr
	}
	return os.Stderr
}

type humanVerbRemedy struct {
	command string
	words   string
}

func humanProofRemedy(values *humanVerbValues, fixture bool, temporaryWord, reviewBy string, cause error) humanVerbRemedy {
	if fixture && (temporaryWord != "" || reviewBy != "") {
		return humanVerbRemedy{command: values.sameCommandWithout("temporary-human-word", "review-by")}
	}
	if temporaryWord != "" || reviewBy != "" {
		return humanVerbRemedy{words: "nothing to do until the relayed word and its review date are given together"}
	}
	return personRemedy(values, cause)
}

// personRemedy is the one command that resolves a refused person act
// ("Messages a Person Reads"): enrolling this terminal with the name filled in
// (the typed --by, else the enrolled person), or the same act in a terminal
// the person opened themselves. cause is the proof's refusal.
func personRemedy(values *humanVerbValues, cause error) humanVerbRemedy {
	retry := shellWords(values.sameCommandWithout())
	remedy := humanauthority.RemedyFor(values.root, cause, values.by, retry)
	if remedy.Kind == "" {
		// A cause the remedy cannot read is still this shell not proven to
		// be the person: enrolling it is what resolves that.
		remedy = humanauthority.RemedyFor(values.root, humanauthority.Refused(humanauthority.OutcomeTerminalMissing, cause), values.by, retry)
	}
	if len(remedy.Argv) > 0 {
		return humanVerbRemedy{command: shellCommand(remedy.Argv)}
	}
	return humanVerbRemedy{words: "nothing to do from this shell; " + remedy.Reason + ", and a person runs it in a terminal they opened"}
}

func newHumanVerbValues(verb string, args []string) *humanVerbValues {
	return &humanVerbValues{verb: verb, root: ".", byTyped: argumentHasFlag(args, "by"), rawArgs: append([]string(nil), args...)}
}

func (values *humanVerbValues) bindSyncFlags(flags *syncFlags) {
	if flags == nil {
		return
	}
	values.root, values.lineage, values.id, values.by = flags.root, flags.lineage, flags.id, flags.by
	values.approvedRef, values.temporaryWord, values.reviewBy = flags.approvedRef, flags.temporaryWord, flags.reviewBy
	values.fixtureHumanAuthority = flags.fixtureHumanAuthority
}

func (values *humanVerbValues) bindGoalView(file *goal.GoalFile, tierBox goal.Budget) {
	values.tierBox = &tierBox
	if file == nil {
		return
	}
	values.state, values.stopFence, values.tierless = file.State, file.StopFence != nil, file.Tier == 0
	if file.Budget != nil {
		standing := *file.Budget
		values.standingBox = &standing
	}
}

// alreadyCarriesBox is the remedy that says a budget request completes to the
// box the goal already carries.
// nothingToDo is the second line when nothing needs doing: it says why.
func nothingToDo(why string) humanVerbRemedy { return humanVerbRemedy{words: "nothing to do; " + why} }

// runRemedy is the second line that runs argv.
func runRemedy(argv ...string) humanVerbRemedy { return humanVerbRemedy{command: shellCommand(argv)} }

// retryRemedy repeats the act in its public form; why stands in when the act
// has none.
func (values *humanVerbValues) retryRemedy(why string) humanVerbRemedy {
	if command := values.sameCommandWithout(); command != "" {
		return humanVerbRemedy{command: command}
	}
	return humanVerbRemedy{words: why}
}

// showRemedy shows the goal the act names, or the goal list when it names none.
func (values *humanVerbValues) showRemedy() humanVerbRemedy {
	if values.id != "" {
		return runRemedy("metasystem", "goal", "show", values.id)
	}
	return runRemedy("metasystem", "goal", "list")
}

const alreadyCarriesBox = "the goal already carries that box, so there is no new act to record"

// cause is an owner error as a refusal's sentence: its first line (the
// caller's remedy is line 2), with its refusal code kept for --verbose and
// --json when it has one.
func (values *humanVerbValues) cause(err error) string {
	if err == nil {
		return ""
	}
	if code := goal.RefusalCode(err); code != "" && values.refusalCode == "" {
		values.refusalCode = code
	}
	return ownerLineOne(err.Error())
}

// ownerLineOne is an owner's message without the line 2 it brings: the
// public verb prints its own remedy, never two.
func ownerLineOne(text string) string {
	for _, marker := range []string{"\nrun: ", "\nnothing to do"} {
		if index := strings.Index(text, marker); index >= 0 {
			return text[:index]
		}
	}
	return text
}

func refuseHumanVerb(values *humanVerbValues, code int, sentence string, remedy humanVerbRemedy) int {
	// A budget request that completes to the box the goal already carries
	// asks for an effect that holds: success with no record (R-129-ui,
	// U-idem), not a refusal with no way forward.
	if remedy.command == "" && remedy.words == alreadyCarriesBox && values.box != nil {
		detail := fmt.Sprintf("goal %s already carries the box %s; nothing new was recorded", values.id, goalbudget.FormatBox(*values.box))
		if values.report != nil {
			values.report.result = &goal.PublishResult{Outcome: goal.OutcomeAbandoned, Unchanged: true, Detail: detail}
			return 0
		}
		fmt.Fprintln(values.outStream(), detail)
		return 0
	}
	sentence = strings.Join(strings.Fields(strings.TrimSpace(ownerLineOne(sentence))), " ")
	sentence = strings.TrimSuffix(sentence, ".") + "."
	if values.report != nil {
		values.report.refusal = &ownerRefusal{code: code, sentence: sentence, remedy: remedy, refusalCode: values.refusalCode}
		return code
	}
	stderr := values.errStream()
	fmt.Fprintf(stderr, "goal %s: %s\n", values.verb, sentence)
	if remedy.command != "" {
		fmt.Fprintln(stderr, "run:", remedy.command)
	} else {
		fmt.Fprintln(stderr, "no command completes this:", strings.TrimSpace(remedy.words))
	}
	return code
}

func (values *humanVerbValues) budgetCommand(box string) string {
	return values.renderBudgetCommand(box, true)
}

func (values *humanVerbValues) budgetCommandWithoutApprovedRef(box string) string {
	return values.renderBudgetCommand(box, false)
}

func (values *humanVerbValues) renderBudgetCommand(box string, includeApprovedRef bool) string {
	// The public goal budget G [options] BOX; --root is its alias of --repo.
	args := []string{"metasystem", "goal", "budget"}
	if values.id != "" {
		args = append(args, values.id)
	}
	if values.root != "" && values.root != "." {
		args = append(args, "--root", values.root)
	}
	if values.lineage != "" {
		args = append(args, "--lineage", values.lineage)
	}
	if values.fixtureHumanAuthority {
		args = append(args, "--fixture-human-authority")
	}
	if values.byTyped && values.by != "" {
		args = append(args, "--by", values.by)
	}
	if values.temporaryWord != "" {
		args = append(args, "--temporary-human-word", values.temporaryWord)
	}
	if values.reviewBy != "" {
		args = append(args, "--review-by", values.reviewBy)
	}
	if includeApprovedRef && values.approvedRef != "" {
		args = append(args, "--approved-ref", values.approvedRef)
	}
	if box == "norm" && values.tierless && values.tierBox != nil {
		// The public goal budget refuses norm for a tierless goal, where the
		// family recorded the tier-three box; the remedy names that box, so
		// running it records exactly what the family's norm recorded.
		box = goalbudget.FormatBox(*values.tierBox)
	}
	if box != "" {
		args = append(args, box)
	}
	return shellCommand(args)
}

func (values *humanVerbValues) suggestedBudgetBox() string {
	if values.box != nil {
		return goalbudget.FormatBox(*values.box)
	}
	if values.standingBox != nil {
		return goalbudget.FormatBox(*values.standingBox)
	}
	if values.tierBox != nil {
		return goalbudget.FormatBox(*values.tierBox)
	}
	return ""
}

func (values *humanVerbValues) sameCommandWithout(drop ...string) string {
	dropped := map[string]bool{}
	for _, name := range drop {
		dropped[strings.TrimPrefix(name, "--")] = true
	}
	// The remedy is the public form of the same act, with the options it
	// takes; an option the public form does not take is left out.
	action, public := publicGoalActions[values.verb]
	if !public {
		action = values.verb
	}
	command, found := findIntentAction("goal", action)
	if !found || command.hidden {
		return ""
	}
	accepted := map[string]bool{"repo": true, "root": true, "json": true}
	for _, flag := range command.allFlags() {
		accepted[flag.name] = true
		for _, alias := range flag.aliases {
			accepted[alias] = true
		}
	}
	args := []string{"metasystem", "goal", action}
	for index := 0; index < len(values.rawArgs); index++ {
		token := values.rawArgs[index]
		if !strings.HasPrefix(token, "--") {
			args = append(args, token)
			continue
		}
		nameValue := strings.TrimPrefix(token, "--")
		name, _, joined := strings.Cut(nameValue, "=")
		keep := !dropped[name] && accepted[name]
		if keep {
			args = append(args, token)
		}
		if joined || goalBooleanFlag(name) || index+1 >= len(values.rawArgs) {
			continue
		}
		index++
		if keep {
			args = append(args, values.rawArgs[index])
		}
	}
	return shellCommand(args)
}

// publicGoalActions are the public goal actions of the goal family's verbs
// whose remedies repeat the caller's command.
var publicGoalActions = map[string]string{
	"accept-risk": "accept-risk", "budget": "budget", "set-budget": "budget", "approve": "approve",
	"unapprove": "unapprove", "resume": "resume", "unpark": "resume",
}

func shellCommand(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		if arg != "" && strings.IndexFunc(arg, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("_@%+=:,./-", r))
		}) == -1 {
			quoted[index] = arg
		} else {
			quoted[index] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
	}
	return strings.Join(quoted, " ")
}

func argumentHasFlag(args []string, wanted string) bool {
	prefix := "--" + wanted
	for _, arg := range args {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}
	return false
}

func goalBooleanFlag(name string) bool {
	switch name {
	case "claim", "refresh-only", "sweep", "preview", "fixture-human-authority":
		return true
	default:
		return false
	}
}
