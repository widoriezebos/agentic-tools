package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// A question is named by its id. A channel question lives with the channel
// owner and a mission question with its mission; M/Q names a mission's
// question explicitly. A bare id that matches both is never resolved by
// precedence: the caller is shown both public names.

type questionRef struct {
	kind    string // channel or mission
	id      string
	mission string
	channel channel.Question
	ask     map[string]any
}

func (inv *intentInvocation) missionAsk(mission, id string) (map[string]any, bool) {
	data, err := os.ReadFile(filepath.Join(inv.stateRoot, "artifacts", "agents", "missions", mission, "asks", id+".json"))
	if err != nil {
		return nil, false
	}
	var ask map[string]any
	if json.Unmarshal(data, &ask) != nil {
		return nil, false
	}
	return ask, true
}

// noChannelQuestion is the refusal for a channel question that does not
// exist, or cannot be read.
func (inv *intentInvocation) noChannelQuestion(targets []intentTarget, id string, err error) *intentResult {
	summary := fmt.Sprintf("no channel question %s; nothing was done", shellCommand([]string{id}))
	if !errors.Is(err, fs.ErrNotExist) {
		summary = fmt.Sprintf("channel question %s cannot be read: %v; nothing was done", shellCommand([]string{id}), err)
	}
	return &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: summary,
		next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids"}
}

// resolveQuestion finds the one question a reference names.
func (inv *intentInvocation) resolveQuestion(ref string) (questionRef, *intentResult) {
	targets := []intentTarget{{Kind: "question", ID: ref}}
	if id, explicit := strings.CutPrefix(ref, "channel:"); explicit {
		q, err := inv.owners.processes.question(inv.stateRoot, id)
		if err != nil {
			return questionRef{}, inv.noChannelQuestion(targets, id, err)
		}
		return questionRef{kind: "channel", id: id, channel: q}, nil
	}
	if mission, id, explicit := strings.Cut(ref, "/"); explicit {
		if !missionIDRe.MatchString(mission) || !missionIDRe.MatchString(id) {
			return questionRef{}, &intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "nothing was done: a mission question is named mission/question, in lowercase words and dashes",
				next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids"}
		}
		ask, found := inv.missionAsk(mission, id)
		if !found {
			return questionRef{}, &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf("nothing was done: mission %s has no question %s", mission, id),
				next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids"}
		}
		return questionRef{kind: "mission", id: id, mission: mission, ask: ask}, nil
	}
	var matches []questionRef
	if q, err := inv.owners.processes.question(inv.stateRoot, ref); err == nil {
		matches = append(matches, questionRef{kind: "channel", id: ref, channel: q})
	}
	if missionIDRe.MatchString(ref) {
		missions, _ := os.ReadDir(filepath.Join(inv.stateRoot, "artifacts", "agents", "missions"))
		for _, entry := range missions {
			if ask, found := inv.missionAsk(entry.Name(), ref); found && entry.IsDir() {
				matches = append(matches, questionRef{kind: "mission", id: ref, mission: entry.Name(), ask: ask})
			}
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return questionRef{}, &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf("nothing was done: there is no question %s", shellCommand([]string{ref})),
			next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids"}
	}
	names, lines := []string{}, []string{}
	for _, match := range matches {
		name := "channel:" + match.id
		if match.kind == "mission" {
			name = match.mission + "/" + match.id
		}
		names = append(names, name)
		lines = append(lines, fmt.Sprintf("  %s (%s question)", name, match.kind))
	}
	return questionRef{}, &intentResult{Outcome: intentRefused, code: 2, Targets: targets, text: lines, Data: map[string]any{"candidates": names},
		Summary:  fmt.Sprintf("nothing was done: %d questions are named %s", len(matches), shellCommand([]string{ref})),
		Decision: "run the same metasystem command with one of the full names listed below"}
}

func (q questionRef) publicName() string {
	if q.kind == "mission" {
		return q.mission + "/" + q.id
	}
	return q.id
}

// questionView is one question's public state and how it is answered.
func (inv *intentInvocation) questionView(q questionRef) intentResult {
	targets := []intentTarget{{Kind: "question", ID: q.publicName()}}
	if q.kind == "channel" {
		c := q.channel
		data := map[string]any{"kind": "channel", "question": c, "replyInstructions": channel.ReplyInstructions(c)}
		result := intentResult{Outcome: intentConfirmed, Targets: targets, Data: data}
		var headline, fact string
		switch {
		case c.State == "closed":
			result.Summary = fmt.Sprintf("channel question %s is withdrawn", c.ID)
			headline = "Channel question " + c.ID + " is withdrawn"
		case c.Answer != nil:
			result.Summary = fmt.Sprintf("channel question %s is answered through its channel", c.ID)
			headline = "Channel question " + c.ID + " is answered through its channel"
		case c.Thread == nil:
			result.Summary = fmt.Sprintf("channel question %s is not delivered yet (%d failed deliveries)", c.ID, c.Undelivered)
			result.next, result.nextReason = inv.publicArgv("question", "retry", c.ID), "retry delivering exactly this question; check the channel settings first when it keeps failing"
			headline, fact = "Channel question "+c.ID+" is not delivered yet", textui.Count(c.Undelivered, "failed delivery", "failed deliveries")
		default:
			result.Summary = fmt.Sprintf("channel question %s waits for the person's reply in its channel thread", c.ID)
			result.text = []string{channel.ReplyInstructions(c)}
			headline = "Channel question " + c.ID + " waits for the person's reply in its channel thread"
		}
		result.view = func(page *textui.Page) {
			page.Headline(headline, fact)
			rows := []textui.KV{}
			if c.Goal != "" {
				rows = append(rows, textui.KV{Key: "goal", Value: []textui.Span{textui.Plain(c.Goal)}})
			}
			if len(c.Facts) > 0 {
				rows = append(rows, textui.KV{Key: "asks", Value: []textui.Span{textui.Plain(strings.Join(c.Facts, " "))}})
			}
			page.Facts(rows...)
			if c.Thread != nil && c.Answer == nil && c.State != "closed" {
				page.Section("", "").Text(channel.ReplyInstructions(c))
			}
		}
		return result
	}
	answered := q.ask["answeredAt"] != nil
	data := map[string]any{"kind": "mission", "mission": q.mission, "ask": q.ask}
	if answered {
		answer, _ := q.ask["answer"].(string)
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, Summary: fmt.Sprintf("mission %s's question %s is answered", q.mission, q.id),
			view: func(page *textui.Page) {
				page.Headline("Mission " + q.mission + "'s question " + q.id + " is answered")
				if answer != "" {
					page.Facts(textui.KV{Key: "answer", Value: []textui.Span{textui.Plain(answer)}})
				}
			}}
	}
	return intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
		Summary: fmt.Sprintf("mission %s's question %s waits for an answer", q.mission, q.id),
		next:    inv.publicArgv("question", "answer", q.publicName(), "TEXT"), nextReason: "a person answers the mission's question",
		view: func(page *textui.Page) {
			page.Headline("Mission " + q.mission + "'s question " + q.id + " waits for an answer")
		}}
}

func runIntentShowQuestion(inv *intentInvocation, args []string) int {
	if len(args) != 1 || inv.input.has("history") || inv.input.has("id") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "question show takes one question id (or M/Q) and nothing else; nothing was done",
			next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	q, problem := inv.resolveQuestion(args[0])
	if problem != nil {
		return inv.render(*problem)
	}
	return inv.render(inv.questionView(q))
}

// waitQuestion waits for a channel question through the channel wait owner,
// and reads a mission question's state once: a mission's answer is given by
// a person, not waited for by a process.
func (inv *intentInvocation) waitQuestion(ref string) intentResult {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return *problem
	}
	q, problem := inv.resolveQuestion(ref)
	if problem != nil {
		return *problem
	}
	if q.kind == "mission" {
		view := inv.questionView(q)
		if q.ask["answeredAt"] == nil {
			view.Outcome = intentInProgress
		}
		return view
	}
	if q.channel.Answer != nil || q.channel.State == "closed" {
		return inv.questionView(q)
	}
	args := []string{"--root", inv.stateRoot, "--question", q.id}
	if timeout, problem := inv.waitTimeout(); problem != nil {
		return *problem
	} else if timeout > 0 {
		args = append(args, "--timeout", fmt.Sprint(max(int(timeout.Minutes()), 1)))
	}
	// The wait runs in this process (design 6.2): this process is the waiting
	// caller the child registered, and the lineage is this invocation's.
	caller, lineage := ownercall.CurrentProcess(), ""
	if inv.owners.dependencies.ownerLineage != nil {
		lineage = inv.owners.dependencies.ownerLineage()
	}
	ran := ownerCall(func(stdout, stderr io.Writer) int {
		return inv.ownerCalls().channelWait(caller, lineage, stdout, stderr, args)
	})
	result := ownerViewed(ownerVerbResult(ran, []intentTarget{{Kind: "question", ID: q.id}}, "channel question "+q.id+" is answered", nil))
	if result.Outcome != intentConfirmed {
		result.Outcome = intentInProgress
		result.next, result.nextReason = inv.publicArgv("question", "wait", "channel:"+q.id), "the same wait continues; delivery or the answer is still pending"
		if view := inv.questionView(q); view.Next != nil || len(view.next) > 0 {
			result.Decision = view.Summary
		}
	}
	return result
}

// The question verbs of the command table: each needs the question's id.
func runIntentQuestionRetry(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(questionIDMissing(inv, "retry"))
	}
	return runIntentAskRetry(inv, inv.input.args[0])
}

func runIntentQuestionWithdraw(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(questionIDMissing(inv, "withdraw"))
	}
	return runIntentAskWithdraw(inv, inv.input.args[0])
}

func runIntentQuestionShow(inv *intentInvocation) int {
	return runIntentShowQuestion(inv, inv.input.args)
}

func runIntentQuestionWait(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(questionIDMissing(inv, "wait"))
	}
	return inv.render(inv.waitQuestion(inv.input.args[0]))
}

func questionIDMissing(inv *intentInvocation, action string) intentResult {
	return intentResult{Outcome: intentRefused, code: 2, Summary: "question " + action + " needs the question's id, so nothing was done",
		next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids"}
}

// runIntentAskRetry and runIntentAskWithdraw act on one existing channel
// question under the channel's poll lock; neither asks a new question.
func runIntentAskRetry(inv *intentInvocation, id string) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "question", ID: id}}
	provider, destination := inv.owners.processes.channelLink(inv.stateRoot)
	q, outcome, err := channel.RetryDelivery(context.Background(), inv.stateRoot, id, provider, destination)
	data := map[string]any{"outcome": outcome, "question": q}
	switch {
	case errors.Is(err, channel.ErrChannelBusy):
		return inv.render(intentResult{Outcome: intentInProgress, Targets: targets, Summary: err.Error(), next: inv.sameCommand(), nextReason: "the same retry runs once the poll finishes"})
	case errors.Is(err, fs.ErrNotExist):
		return inv.render(*inv.noChannelQuestion(targets, id, err))
	case err != nil:
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: "question " + id + " was not retried: " + err.Error(),
			next: inv.publicArgv("settings", "show"), nextReason: "check the channel settings"})
	}
	done := func(text ...string) func(*textui.Page) {
		return func(page *textui.Page) { page.Done(joinFacts(page, text...)) }
	}
	switch outcome {
	case channel.RetryDelivered:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, Summary: "question " + id + " is delivered to its channel; no other question was touched",
			view: done("Question "+id+" is delivered to its channel", "no other question was touched")})
	case channel.RetryAlreadyDelivered:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, Summary: "question " + id + " was already delivered; nothing was sent again",
			view: done("Question "+id+" was already delivered", "nothing was sent again")})
	case channel.RetryNotOpen:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, Summary: "question " + id + " is " + q.State + "; there is nothing to deliver",
			view: done("Question "+id+" is "+q.State, "there is nothing to deliver")})
	}
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
		Summary: fmt.Sprintf("question %s is still not delivered (%d failed deliveries); it stays open", id, q.Undelivered),
		next:    inv.publicArgv("settings", "show", "channel.provider"), nextReason: "check the channel provider settings, then retry"})
}

func runIntentAskWithdraw(inv *intentInvocation, id string) int {
	reason := strings.TrimSpace(inv.input.text("reason"))
	if reason == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "nothing was done: withdrawing a question needs the reason",
			next: append(inv.typedArgv(), "--reason", "<why>")})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "question", ID: id}}
	before, err := inv.owners.processes.question(inv.stateRoot, id)
	if err != nil {
		return inv.render(*inv.noChannelQuestion(targets, id, err))
	}
	if before.State == "closed" {
		// Already closed (R-129-ui): success, and nothing is posted or
		// written again; the channel owner's Close makes the same no-op.
		summary := "question " + id + " is already withdrawn; nothing was changed"
		headline := []string{"Question " + id + " is already withdrawn", "nothing was changed"}
		if before.Answer != nil {
			summary = "question " + id + " is already closed with its answer recorded; nothing was changed"
			headline = []string{"Question " + id + " is already closed with its answer recorded", "nothing was changed"}
		}
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Summary: summary,
			view: func(page *textui.Page) { page.Done(joinFacts(page, headline...)) }})
	}
	provider, destination := inv.owners.processes.channelLink(inv.stateRoot)
	q, err := channel.Withdraw(inv.stateRoot, id, reason, provider, destination)
	switch {
	case errors.Is(err, channel.ErrChannelBusy):
		return inv.render(intentResult{Outcome: intentInProgress, Targets: targets, Summary: err.Error(), next: inv.sameCommand(), nextReason: "the same withdrawal runs once the poll finishes"})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "question " + id + " was not withdrawn: " + err.Error(),
			next: inv.sameCommand(), nextReason: "once the cause is fixed"})
	}
	summary := "question " + id + " is withdrawn"
	headline := []string{"Question " + id + " is withdrawn"}
	if before.Answer != nil {
		summary += "; it had already been answered, and that answer stays recorded"
		headline = append(headline, "its earlier answer stays recorded")
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: map[string]any{"question": q}, Summary: summary,
		view: func(page *textui.Page) { page.Done(joinFacts(page, headline...)) }})
}
