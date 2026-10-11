package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/rehearsal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

func landingRehearseCommand() intentCommand {
	return intentCommand{object: "landing", action: "rehearse", audience: "both", summary: "rehearse the pinned candidate on an isolated scratch landing lane",
		usage: []string{"metasystem landing rehearse [--keep]"}, examples: []string{"metasystem landing rehearse --keep"}, flags: []intentFlag{{name: "keep", usage: "keep the scratch lane after success"}}, maxArgs: 0, run: runIntentLandingRehearse}
}

func runIntentLandingRehearse(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	module := inv.layout.InstallationRoot.Path()
	start := time.Now()
	s, err := rehearsal.RunGreen(context.Background(), module, os.Environ())
	if s == nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "rehearsal scratch could not be created: " + err.Error()})
	}
	s.PublishCheckoutRecord(module)
	err = s.Finish(module, start, inv.input.switched("keep"), err)
	slowest, duration := s.Breakdown.Slowest()
	result := intentResult{Outcome: intentConfirmed, Data: s.Record, Summary: fmt.Sprintf("green rehearsal %s in %s; slowest: %s (%s)", s.Outcome, time.Duration(s.WallMS)*time.Millisecond, slowest, duration)}
	if err != nil {
		result.Outcome, result.code = intentFailed, 1
		result.Details = []string{err.Error()}
	}
	if err != nil || inv.input.switched("keep") {
		result.Details = append(result.Details, "scratch kept at "+s.Scratch)
	}
	return inv.render(result)
}

// The candidate's public verbs keep their real registration and keeper wiring.
// Only the person, supervision readiness and model launch are scripted, inside
// the controller's accepted scratch store; ordinary lane calls gain no authority.
func (inv *intentInvocation) rehearsalLaneEffects(owners *laneVerbOwners) {
	if inv.command.object != "landing" || (inv.command.action != "set" && inv.command.action != "run") {
		return
	}
	lookup := inv.owners.lookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	scratch, _ := lookup("METASYSTEM_REHEARSAL_SCRATCH")
	if scratch == "" || inv.resolveLayout() != nil {
		return
	}
	registry, _ := lookup("METASYSTEM_SUPERVISION_REGISTRY_HOME")
	executable, err := os.Executable()
	module := inv.layout.InstallationRoot.Path()
	if err != nil || !rehearsal.ChildScope(scratch, module, executable, registry) {
		return
	}
	if inv.command.action == "set" {
		if len(inv.input.args) != 1 || realpath.Resolve(inv.callerPath(inv.input.args[0])) != filepath.Dir(module) {
			return
		}
	} else {
		record, found, err := lane.Read(filepath.Join(registry, ".metasystem"))
		if err != nil || !found || record.Root != filepath.Dir(module) || record.Install != module {
			return
		}
	}
	owners.person = func(root string) (string, error) {
		if root != module {
			return "", fmt.Errorf("registration escaped the scratch module")
		}
		return "rehearsal", nil
	}
	owners.ready = func(root string) error { _, err := goal.ResolveMachine(root); return err }
	owners.keeper = func(home, root string) lane.AgentKeeper {
		keeper := newLandingAgentKeeper(root, home, newLandingAgent())
		keeper.Start = func(string, lane.Wake) (string, error) { return rehearsal.GreenGoal, nil }
		return keeper
	}
}
