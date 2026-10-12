package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func (inv *intentInvocation) runWithLandingStep() int {
	if inv.command.object != "landing" && inv.command.object != "work" {
		return inv.command.run(inv)
	}
	switch inv.command.object + " " + inv.command.action {
	case "landing prove", "landing push", "landing resolve", "landing return", "landing run", "work build", "work review", "work wait", "work land":
	default:
		return inv.command.run(inv)
	}
	owners, _, record, problem := inv.laneContext(false)
	if problem != nil || record.Install == "" {
		return inv.command.run(inv)
	}
	actor := owners.stepActor
	if actor == nil {
		actor = landingFixActor
	}
	if !actor(record.Install, int64(os.Getpid())) {
		return inv.command.run(inv)
	}
	verb := inv.command.action
	if inv.command.object == "work" {
		verb = "work " + verb
	}
	finish, err := plain.BeginAgentStep(record.Install, verb, inv.raw, owners.now())
	if err != nil {
		fmt.Fprintln(inv.stderr, "the landing agent step could not be recorded:", err)
	} else {
		defer func() {
			if err := finish(owners.now()); err != nil {
				fmt.Fprintln(inv.stderr, "the landing agent step exit could not be recorded:", err)
			}
		}()
	}
	return inv.command.run(inv)
}
