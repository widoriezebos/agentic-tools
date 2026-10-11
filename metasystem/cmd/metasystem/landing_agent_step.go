package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func (inv *intentInvocation) runWithLandingStep() int {
	if inv.command.object != "landing" {
		return inv.command.run(inv)
	}
	switch inv.command.action {
	case "prove", "push", "resolve", "return", "run":
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
	finish, err := plain.BeginAgentStep(record.Install, inv.command.action, inv.raw, owners.now())
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
