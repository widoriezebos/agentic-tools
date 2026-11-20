package main

import (
	"errors"
	"fmt"
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
	if len(in.NamedActs) > 0 {
		before, err := channel.NamedActIDs(inv.layout.InstallationRoot.Path(), inv.owners.dependencies.endpoint)
		if err != nil {
			return refuse(err.Error())
		}
		in.ActsBefore = before
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
	var completed channel.ActEvidence
	authorityRoot := checkoutAuthorityRoot(inv.layout)
	actRoot := inv.layout.InstallationRoot.Path()
	ep, err := inv.owners.dependencies.endpoint(actRoot)
	if err == nil {
		machine, e := inv.owners.dependencies.machine(inv.stateRoot)
		err = e
		if err == nil {
			completed, err = channel.RecordNamedAct(authorityRoot, ep, act, person, machine, inv.unitStopNow(), *proof)
		}
	}
	if err == nil {
		err = channel.ReconcileNamedActs(root, inv.owners.dependencies.endpoint)
		if errors.Is(err, channel.ErrChannelBusy) {
			return result
		}
	}
	if err != nil {
		result.Outcome, result.code = intentPartial, 1
		result.Details = append(result.Details, "the act completed, but its question could not be answered: "+err.Error())
		return result
	}
	questions, _ := channel.WalkQuestions(root)
	for _, q := range questions {
		if q.Answer != nil && q.Answer.Act != nil && q.Answer.Act.Act == act && q.Answer.Act.ID == completed.ID {
			line := fmt.Sprintf("Question %s answered: %s.", q.ID, q.Answer.Text)
			result.Details = append(result.Details, line)
			if result.view == nil {
				result.text = append(result.text, line)
			} else {
				prior := result.view
				result.view = func(page *textui.Page) { prior(page); page.Section("", "").Text(line) }
			}
		}
	}
	return result
}
