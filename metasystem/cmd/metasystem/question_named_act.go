package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func (inv *intentInvocation) declareQuestionActs(in *channelAskInput) *intentResult {
	in.NamedActs = map[string]channel.NamedAct{}
	for _, raw := range inv.input.values["option-act"] {
		label, command, ok := strings.Cut(raw, ":")
		label = strings.TrimSpace(label)
		if !ok {
			return inv.refuseQuestionAct("name an option of a question on a goal")
		}
		if _, exists := in.NamedActs[label]; exists {
			return inv.refuseQuestionAct("declare an option's command once")
		}
		act, problem := inv.declarableQuestionAct(in, label, command)
		if problem != nil {
			return problem
		}
		in.NamedActs[label] = act
	}
	return inv.requireQuestionActDeclarations(in)
}

func (inv *intentInvocation) refuseQuestionAct(why string) *intentResult {
	return &intentResult{Outcome: intentRefused, code: 2, Summary: "the option's command is incomplete; nothing was asked", Details: []string{why}, next: inv.retryWith([]string{"option-act"}), nextReason: "ask without that command, or name its exact finding or run"}
}

// Both explicit declarations and the agent's missing-declaration check use
// this decision. An option cannot require a declaration the question cannot carry.
func (inv *intentInvocation) declarableQuestionAct(in *channelAskInput, label, command string) (channel.NamedAct, *intentResult) {
	refuse := func(why string) (channel.NamedAct, *intentResult) {
		return channel.NamedAct{}, inv.refuseQuestionAct(why)
	}
	if in.Wants != "" || inv.input.has("wants") {
		return channel.NamedAct{}, &intentResult{Outcome: intentRefused, code: 2,
			Summary: "--wants cannot accompany --option-act; nothing was asked",
			next:    inv.retryWith([]string{"wants"}), nextReason: "the completed command answers only its option"}
	}
	if in.Goal == "" || in.Kind != "other" {
		return refuse("name an option of an ordinary question on a goal")
	}
	found := false
	for _, raw := range in.Options {
		l, _, _ := strings.Cut(raw, ":")
		if strings.TrimSpace(l) == label {
			found = true
		}
	}
	if !found {
		return refuse("name an option of this question")
	}
	act, problem := inv.resolveQuestionAct(in.Goal, command)
	if problem != nil {
		return channel.NamedAct{}, problem
	}
	for other, prior := range in.NamedActs {
		if other != label && prior == act {
			return refuse("two options cannot name the same command")
		}
	}
	for _, raw := range in.Options {
		other, consequence, _ := strings.Cut(raw, ":")
		if strings.TrimSpace(other) == label {
			continue
		}
		if command := optionActCommand(consequence); command != "" {
			prior, problem := inv.resolveQuestionAct(in.Goal, command)
			if problem == nil && prior == act {
				return refuse("two options cannot name the same command")
			}
		}
	}
	return act, nil
}

func (inv *intentInvocation) resolveQuestionAct(goalID, command string) (channel.NamedAct, *intentResult) {
	refuse := inv.refuseQuestionAct
	quoted := byte(0)
	commandBytes := []byte(command)
	for i := 0; i < len(command); i++ {
		if quoted == 0 && strings.ContainsRune("\t\r\n\v\f", rune(command[i])) {
			commandBytes[i] = ' '
		}
		if command[i] == '\'' || command[i] == '"' {
			if quoted == command[i] {
				quoted = 0
			} else if quoted == 0 {
				quoted = command[i]
			}
		}
	}
	if quoted != 0 {
		return channel.NamedAct{}, refuse("close the command quotation")
	}
	words := shellWords(string(commandBytes))
	if len(words) < 4 || words[0] != "metasystem" {
		return channel.NamedAct{}, refuse("use a complete metasystem command")
	}
	verb, ok := findIntentAction(words[1], words[2])
	if !ok {
		return channel.NamedAct{}, refuse("that command is not available")
	}
	args, problem := parseIntentArgs(verb, words[3:])
	if problem != nil || len(args.args) != 1 {
		return channel.NamedAct{}, refuse("name exactly one target")
	}
	for key := range args.values {
		if key != "review" && key != "finding" && key != "reason" && key != "json" && key != "verbose" {
			return channel.NamedAct{}, refuse("the declaration may name its subject and reason only")
		}
	}
	act := channel.NamedAct{Verb: verb.name, Goal: goalID}
	switch verb.name {
	case "goal accept-risk":
		if strings.TrimPrefix(args.args[0], "goal:") != goalID || args.text("finding") == "" {
			return channel.NamedAct{}, refuse("the finding must belong to this goal")
		}
		review := args.text("review")
		if review == "" {
			var p *intentResult
			review, p = inv.uniqueExamination(goalID, "accept-risk")
			if p != nil {
				return channel.NamedAct{}, p
			}
		}
		chain, p := inv.reviewRoot(goalID, review)
		if p != nil {
			return channel.NamedAct{}, p
		}
		if chain != goal.HumanCarriedChain {
			if _, err := dispatchcore.CritiqueRegisterDecisionFinding(branch.CriticStore(inv.layout.InstallationRoot.Path(), chain), chain, args.text("finding"), goalID); err != nil {
				return channel.NamedAct{}, refuse(err.Error())
			}
		}
		act.Target, act.Finding = chain, args.text("finding")
	case "work stop":
		if !strings.HasPrefix(args.args[0], "run:") {
			return channel.NamedAct{}, refuse("name a single run to stop")
		}
		record, err := inv.unitRunner().Status(strings.TrimPrefix(args.args[0], "run:"))
		if err != nil || record.Goal != goalID || record.ID == "" {
			return channel.NamedAct{}, refuse("the run must belong to this goal")
		}
		act.Target = record.ID
	default:
		return channel.NamedAct{}, refuse("declare accepting a risk or stopping a run")
	}
	return act, nil
}

// Recognize one target and the declaration's flags, stopping before prose or
// surrounding punctuation. Quoted arguments may contain spaces and punctuation.
const questionActArgument = `(?:'[^']*'|"[^"]*"|[^[:space:]'"` + "`" + `,;()!?]+)`
const questionActFlag = `--(?:(?:review|finding|reason)(?:=|[[:space:]]+)` + questionActArgument + `|json|verbose)`

var questionActCommand = regexp.MustCompile(`(?:^|[^[:alnum:]_-])(metasystem[[:space:]]+(?:goal[[:space:]]+accept-risk|work[[:space:]]+stop)[[:space:]]+` +
	`(?:` + questionActFlag + `[[:space:]]+)*` + questionActArgument + `(?:[[:space:]]+` + questionActFlag + `)*)`)

func optionActCommand(consequence string) string {
	match := questionActCommand.FindStringSubmatch(consequence)
	if match == nil {
		return ""
	}
	return strings.TrimRight(match[1], ".")
}

func (inv *intentInvocation) requireQuestionActDeclarations(in *channelAskInput) *intentResult {
	for _, raw := range in.Options {
		label, consequence, ok := strings.Cut(raw, ":")
		label = strings.TrimSpace(label)
		command := optionActCommand(consequence)
		if !ok || command == "" {
			continue
		}
		if _, declared := in.NamedActs[label]; declared {
			continue
		}
		namedSession := inv.owners.dependencies.ownerLineage != nil && inv.owners.dependencies.ownerLineage() != ""
		if !namedSession && !inv.agentSessionCaller() {
			return nil
		}
		act, problem := inv.declarableQuestionAct(in, label, command)
		if problem != nil {
			continue
		}
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary:    "the agent's option names a person's command without declaring it; nothing was asked",
			next:       inv.retryWith(nil, "--option-act", label+": "+shellCommand(act.Command())),
			nextReason: "declare this option's exact command on its goal so the person's act answers the question"}
	}
	return nil
}

func (inv *intentInvocation) completeQuestionAct(result intentResult, root string, act channel.NamedAct, proof *humanauthority.Proof, person string) intentResult {
	if proof == nil || result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		return result
	}
	if proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) && !proof.ChannelWordFor(inv.stateRoot) {
		return result
	}
	machine, err := inv.owners.dependencies.machine(inv.stateRoot)
	if err != nil {
		machine = ""
	}
	// The question's installation and the caller's authority root can differ
	// in an adopted layout; proof is always checked where the person stands.
	for _, closed := range channel.CompleteNamedAct(root, inv.stateRoot, act, person, machine, inv.unitStopNow(), *proof, inv.owners.dependencies.endpoint) {
		line := fmt.Sprintf("Question %s answered: %s.", closed.Question, closed.Option)
		if closed.Err != nil {
			line = fmt.Sprintf("Question %s stays open; repeat the act or withdraw the question.", closed.Question)
		}
		result.Details = append(result.Details, line)
		if result.view == nil {
			result.text = append(result.text, line)
		} else {
			prior := result.view
			result.view = func(page *textui.Page) { prior(page); page.Section("", "").Text(line) }
		}
	}
	return result
}
