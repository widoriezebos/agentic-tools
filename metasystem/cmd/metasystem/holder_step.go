package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The holder's step on its Stop path (g1-s70 D3, SOL-S70-02).
//
// The seat that holds a claim takes its due step itself when its turn ends: a
// due landing is admitted by the public work land G (the batch join, or the
// hand route's start, which is what that command does before it returns; the
// batch proves and publishes later), with the gate evaluated then against the
// fresh ledger; a send-back is revised by the public work revise G with no
// --brief, which starts the revision from the published brief and records its
// answer on the goal, so it is taken once. Both run in this process from the
// session's checkout, so the seat acts under the identity it has when it types
// the command. The Stop shows what was done or the refusal with its code and
// the verb a person carries past it with; it never blocks for a taken step.

// holderStepTaker takes one holder step through the public command in this
// process and answers the line the Stop shows.
func holderStepTaker(cwd string, owners intentOwners) func(goal.HolderStep) string {
	return func(step goal.HolderStep) string {
		verb := "land"
		if step.Revise {
			verb = "revise"
		}
		command, _ := findIntentAction("work", verb)
		var stdout, stderr bytes.Buffer
		runIntentIn(command, []string{step.Goal, "--json"}, &stdout, &stderr, cwd, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			result = intentResult{Outcome: intentFailed, Summary: "work " + verb + " gave no answer the Stop could read",
				next: []string{"metasystem", "work", verb, step.Goal}, nextReason: "shows its answer",
				Details: []string{strings.TrimSpace(stderr.String())}}
		}
		return holderStepLine(step, result)
	}
}

// holderStepLine is what the Stop says about one taken step.
func holderStepLine(step goal.HolderStep, result intentResult) string {
	taken := result.Outcome != intentRefused && result.Outcome != intentFailed
	data, _ := result.Data.(map[string]any)
	switch {
	case step.Revise && taken:
		return fmt.Sprintf("REVISION STARTED %s, %s: %s", step.Goal, step.Why, result.Summary)
	case step.Revise:
		return fmt.Sprintf("REVISION REFUSED %s, %s: %s", step.Goal, step.Why, result.Summary)
	case taken:
		if joined, _ := data["joinedNow"].(bool); joined {
			return fmt.Sprintf("LANDED %s: joined batch %v", step.Goal, data["batchId"])
		}
		return fmt.Sprintf("LANDED %s: %s", step.Goal, result.Summary)
	}
	line := "LANDING REFUSED " + step.Goal
	if code, _ := data["code"].(string); code != "" {
		line += " [" + code + "]"
	}
	line += ": " + result.Summary
	switch {
	case result.Next != nil && len(result.Next.Argv) > 0:
		line += "; a person carries past it: " + shellCommand(result.Next.Argv)
	case result.Decision != "":
		line += "; a person carries past it: " + result.Decision
	}
	return line
}
