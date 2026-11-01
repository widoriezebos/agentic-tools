package main

import (
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type attorneyGoalSubject struct {
	flakeFix         bool
	machine, lineage string
}

type attorneySubject map[string]attorneyGoalSubject

func attorneyActClass(args []string, subject func([]string) (attorneySubject, error), machine, lineage string) (string, bool) {
	if len(args) < 2 {
		return "", false
	}
	command, ok := findIntentAction(args[0], args[1])
	if !ok {
		return "", false
	}
	input, problem := parseIntentArgs(command, args[2:])
	if problem != nil || input.help {
		return "", false
	}
	switch command.name {
	case "goal prioritize", "goal pin", "goal group":
		return "ordering", true
	case "alert ack", "question withdraw":
		return "acks", true
	case "goal approve", "goal release", "work stop":
	default:
		return "", false
	}
	allowed := []string{"json", "verbose"}
	if command.name == "goal release" {
		allowed = append(allowed, "reason")
	}
	for flag := range input.values {
		if !slices.Contains(allowed, flag) {
			return "", false
		}
	}
	if len(input.args) == 0 {
		return "", false
	}
	if command.name == "work stop" {
		return "recovery", len(input.args) == 1 && !strings.HasPrefix(input.args[0], "run:")
	}
	if command.name == "goal release" && len(input.args) != 1 {
		return "", false
	}
	facts, err := subject(input.args)
	if err != nil {
		return "", false
	}
	if command.name == "goal release" {
		claim := facts[input.args[0]]
		return "stale-releases", claim.machine == machine && claim.lineage != lineage
	}
	for _, id := range input.args {
		if !facts[id].flakeFix {
			return "", false
		}
	}
	return "recovery", true
}

func readAttorneySubject(end goal.Endpoint, ids []string, now time.Time) (attorneySubject, error) {
	projection, err := goal.Project(end, false, now)
	if err != nil {
		return nil, err
	}
	flakes := map[string]bool{}
	for _, fact := range goal.FlakeFacts(projection.Tree.TrunkRed) {
		flakes[fact.FixGoal] = true
	}
	out := attorneySubject{}
	for _, id := range ids {
		s := attorneyGoalSubject{flakeFix: flakes[id]}
		if file := projection.Tree.Live[id]; file != nil && file.Claimed != nil {
			s.machine, s.lineage = file.Claimed.Machine, file.Claimed.Lineage
		}
		out[id] = s
	}
	return out, nil
}
