package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// channelChoice is the person's channel answer that chose this command, as
// checked once: its question and option, or why it admits nothing.
type channelChoice struct {
	question channel.Question
	option   channel.Option
	person   string
	problem  *intentResult
}

// chosenAnswer admits --answer Q only for exactly the act of the option the
// configured person chose in Q's thread, while that answer is recorded.
func (inv *intentInvocation) chosenAnswer() *channelChoice {
	if inv.choice != nil {
		return inv.choice
	}
	choice := &channelChoice{}
	inv.choice = choice
	id := inv.input.text("answer")
	refuse := func(label string) *channelChoice {
		summary := "this answer chose no act this command may run; nothing was done"
		if label != "" {
			summary = fmt.Sprintf("this answer authorizes %q, not this command; nothing was done", label)
		}
		choice.problem = &intentResult{Outcome: intentRefused, code: 1, Summary: summary}
		return choice
	}
	if !goal.ValidChannelULID(id) || inv.resolveLayout() != nil {
		return refuse("")
	}
	root := inv.layout.InstallationRoot.Path()
	q, err := channel.ReadQuestion(root, id)
	loaded, loadErr := phase.Load(root, true)
	if err != nil || loadErr != nil || !q.Contract || q.State != "open" || q.Answer == nil || q.Answer.Phase != "recorded" || q.Answer.Step < 1 || loaded.HumanUserID == "" || q.Answer.UserID != loaded.HumanUserID {
		return refuse("")
	}
	for _, option := range q.Options {
		if strings.EqualFold(strings.TrimSpace(q.Answer.Text), option.Label) {
			choice.option = option
		}
	}
	argv := slices.DeleteFunc(withoutOption(withoutOption(inv.typedArgv(), "answer"), "by"), func(word string) bool { return word == "--json" })
	if len(choice.option.Act) == 0 || !slices.Equal(argv, choice.option.Act) {
		return refuse(choice.option.Label)
	}
	choice.question = q
	return choice
}

// answerActor is a person's act proved by his channel answer: the enrolled
// person's name and the proof bound to the question and message.
func (inv *intentInvocation) answerActor(args []string) ([]string, *humanauthority.Proof, *intentResult) {
	choice := inv.chosenAnswer()
	if choice.problem != nil {
		return nil, nil, choice.problem
	}
	if inv.stateRoot == "" {
		_ = inv.selectRoot()
	}
	answer := choice.question.Answer
	enrollment, err := humanauthority.ReadEnrollment(inv.stateRoot)
	proof, proofErr := humanauthority.VerifiedChannelAnswerProof(inv.stateRoot, governance.RecordedChannelAuthority{Outcome: governance.AuthorityOutcomeVerifiedChannelAnswer,
		Provider: answer.Provider, UserID: answer.UserID, MessageRef: answer.Ref.ThreadID + "/" + answer.Ref.ID, ContextID: choice.question.ID, Step: answer.Step}, inv.unitStopNow())
	if err != nil || enrollment.Human == "" {
		return nil, nil, &intentResult{Outcome: intentRefused, code: 1, Summary: "nobody is enrolled on this machine, so the answer names no person; enrol at the terminal first; nothing was done"}
	}
	if proofErr != nil {
		return nil, nil, &intentResult{Outcome: intentRefused, code: 1, Summary: "the answer in the channel could not be proved as the person's word; nothing was done", Details: []string{proofErr.Error()}}
	}
	choice.person = enrollment.Human
	return append(args, "--by", choice.person), &proof, nil
}

// answerAdmitted says the person's channel answer was proved for this act
// by actingAs.
func (inv *intentInvocation) answerAdmitted() bool {
	return inv.input.has("answer") && inv.choice != nil && inv.choice.person != ""
}

// answerProgress puts what came of the chosen act in the result, read from
// the run's own record: accepted once this correction is retained, started
// once its round's build has launched.
func (inv *intentInvocation) answerProgress(result intentResult) intentResult {
	progress := map[string]string{"state": "refused"}
	choice := inv.chosenAnswer()
	if choice.problem == nil && choice.person != "" && choice.question.UnitStop != nil {
		subject := choice.question.UnitStop.Subject
		after, _ := strconv.Atoi(inv.input.text("after"))
		record, err := inv.unitRunner().Status(subject[strings.LastIndex(subject, "/")+1:])
		index := slices.IndexFunc(record.Revisions, func(revision launch.UnitRevision) bool {
			return revision.After == after && revision.Person == choice.person && revision.Reason == inv.input.text("reason")
		})
		if err == nil && index >= 0 {
			progress["state"], progress["hold"] = "accepted", answerHold(result)
			for _, round := range record.Rounds {
				if round.Number == record.Revisions[index].Attempt && slices.ContainsFunc(round.Steps, func(step launch.UnitStep) bool {
					return step.Name == "build" && (step.State == launch.StepRunning || step.State == launch.StepPassed || step.State == launch.StepFailed)
				}) {
					progress = map[string]string{"state": "started"}
				}
			}
		}
	}
	data, _ := result.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	data["answer"] = progress
	result.Data = data
	return result
}

// answerHold is why an accepted act waits, in plain words.
func answerHold(result intentResult) string {
	for _, detail := range result.Details {
		switch {
		case strings.HasPrefix(detail, "BUDGET_REFUSED"):
			return "the goal's budget is used up"
		case strings.HasPrefix(detail, "LAUNCH_BUILD_CAPACITY"):
			return "the host cannot take another build now"
		}
	}
	return result.Summary
}
