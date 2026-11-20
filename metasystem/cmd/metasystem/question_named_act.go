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
	refuse := func(why string) *intentResult {
		return &intentResult{Outcome: intentRefused, code: 2, Summary: "the option's command is incomplete; nothing was asked", Details: []string{why}, next: inv.retryWith([]string{"option-act"}), nextReason: "ask without that command, or name its exact finding or run"}
	}
	if inv.input.has("option-act") && inv.input.has("wants") {
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary: "--wants cannot accompany --option-act; nothing was asked",
			next:    inv.retryWith([]string{"wants"}), nextReason: "the completed command answers only its option"}
	}
	for _, raw := range inv.input.values["option-act"] {
		label, command, ok := strings.Cut(raw, ":")
		label = strings.TrimSpace(label)
		foundLabel := false
		for _, opt := range in.Options {
			l, _, _ := strings.Cut(opt, ":")
			if strings.TrimSpace(l) == label {
				foundLabel = true
			}
		}
		if !ok || !foundLabel || in.Goal == "" {
			return refuse("name an option of a question on a goal")
		}
		if _, exists := in.NamedActs[label]; exists {
			return refuse("declare an option's command once")
		}
		quoted := byte(0)
		for i := 0; i < len(command); i++ {
			if command[i] == '\'' || command[i] == '"' {
				if quoted == command[i] {
					quoted = 0
				} else if quoted == 0 {
					quoted = command[i]
				}
			}
		}
		if quoted != 0 {
			return refuse("close the command quotation")
		}
		words := shellWords(command)
		if len(words) < 4 || words[0] != "metasystem" {
			return refuse("use a complete metasystem command")
		}
		verb, ok := findIntentAction(words[1], words[2])
		if !ok {
			return refuse("that command is not available")
		}
		args, problem := parseIntentArgs(verb, words[3:])
		if problem != nil || len(args.args) != 1 {
			return refuse("name exactly one target")
		}
		for key := range args.values {
			if key != "review" && key != "finding" && key != "reason" && key != "json" && key != "verbose" {
				return refuse("the declaration may name its subject and reason only")
			}
		}
		act := channel.NamedAct{Verb: verb.name, Goal: in.Goal}
		switch verb.name {
		case "goal accept-risk":
			if strings.TrimPrefix(args.args[0], "goal:") != in.Goal || args.text("finding") == "" {
				return refuse("the finding must belong to this goal")
			}
			review := args.text("review")
			if review == "" {
				var p *intentResult
				review, p = inv.uniqueExamination(in.Goal, "accept-risk")
				if p != nil {
					return p
				}
			}
			chain, p := inv.reviewRoot(in.Goal, review)
			if p != nil {
				return p
			}
			if chain != goal.HumanCarriedChain {
				if _, err := dispatchcore.CritiqueRegisterDecisionFinding(branch.CriticStore(inv.layout.InstallationRoot.Path(), chain), chain, args.text("finding"), in.Goal); err != nil {
					return refuse(err.Error())
				}
			}
			act.Target, act.Finding = chain, args.text("finding")
		case "work stop":
			if !strings.HasPrefix(args.args[0], "run:") {
				return refuse("name a single run to stop")
			}
			record, err := inv.unitRunner().Status(strings.TrimPrefix(args.args[0], "run:"))
			if err != nil || record.Goal != in.Goal {
				return refuse("the run must belong to this goal")
			}
			act.Target = record.ID
		default:
			return refuse("declare accepting a risk or stopping a run")
		}
		for _, prior := range in.NamedActs {
			if prior == act {
				return refuse("two options cannot name the same command")
			}
		}
		in.NamedActs[label] = act
	}

	return inv.requireQuestionActDeclarations(in)
}

// Only command prefixes in option consequences are recognized. Other prose
// and commands outside the completed-act allowlist remain ordinary choices.
var questionActCommand = regexp.MustCompile(`(?:^|[^[:alnum:]_-])metasystem[[:space:]]+(goal[[:space:]]+accept-risk(?:[[:space:]]|$)|work[[:space:]]+stop[[:space:]]+run:)`)

func (inv *intentInvocation) requireQuestionActDeclarations(in *channelAskInput) *intentResult {
	for _, raw := range in.Options {
		label, consequence, ok := strings.Cut(raw, ":")
		label = strings.TrimSpace(label)
		match := questionActCommand.FindStringSubmatch(consequence)
		if !ok || match == nil {
			continue
		}
		if _, declared := in.NamedActs[label]; declared {
			continue
		}
		namedSession := inv.owners.dependencies.ownerLineage != nil && inv.owners.dependencies.ownerLineage() != ""
		if !namedSession && !inv.agentSessionCaller() {
			return nil
		}
		command := "metasystem goal accept-risk " + in.Goal + " --review REVIEW --finding FINDING"
		if strings.HasPrefix(match[1], "work") {
			command = "metasystem work stop run:RUN"
		}
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary:    "the agent's option names a person's command without declaring it; nothing was asked",
			next:       inv.retryWith(nil, "--option-act", label+": "+command),
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
