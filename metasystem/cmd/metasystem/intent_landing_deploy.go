package main

import (
	"errors"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

func landingDeployCommand() intentCommand {
	return laneCommand(intentCommand{object: "landing", action: "deploy", audience: "human", maxArgs: 0,
		summary:  "deploy the lane's own engine from origin main and preserve its run state",
		usage:    []string{"metasystem landing deploy [--now] [--by NAME] [--json]"},
		flags:    []intentFlag{{name: "now", usage: "deploy during a proof; the lane owns its restart"}, {name: "by", value: "NAME", usage: "the enrolled person's name"}},
		examples: []string{"metasystem landing deploy", "metasystem landing deploy --now --by Wido --json"},
	}, runIntentLandingDeploy)
}

func runIntentLandingDeploy(inv *intentInvocation, admitted laneAdmitted) int {
	person, problem := inv.lanePerson("deploy the lane's engine", admitted.record.Root)
	if problem != nil {
		return inv.render(*problem)
	}
	provenance := plain.ActProvenance{Kind: "deploy", Person: person.Name, Root: person.Root, CheckedAt: person.At, TerminalGeneration: person.Proof.TerminalGeneration, TerminalRef: person.Proof.TerminalRef, Destination: admitted.record}
	seams := admitted.owners.plainProve
	keeper := admitted.owners.keeper(admitted.home, admitted.record.Root)
	seams.ResumeKeeper = &keeper
	opts := plain.DeployOptions{Now: inv.input.switched("now"), MergeSettled: func(fix *plain.Fix) (bool, error) {
		return landingDeployMergeSettled(admitted.installation, fix, seams)
	}, RestartSupervisor: admitted.owners.restartSupervisor}
	if opts.RestartSupervisor == nil {
		opts.RestartSupervisor = func(install string) error {
			scope, err := resolveProcessScopeWith(admitted.record.Root, install, inv.owners.processes.process.repositoryTop)
			if err != nil {
				return err
			}
			// The arming half of system restart replaces a changed supervisor
			// generation without the aggregate stop that would end the proof.
			_, err = restartLandingSupervisor(scope, defaultProcessArmEffects(), supervise.EnsureArmed)
			return err
		}
	}
	run, err := plain.DeployEngine(admitted.home, provenance, admitted.owners.now(), opts, seams)
	result := intentResult{Outcome: intentConfirmed, Targets: laneTargets(admitted.record.Root), Data: run, Summary: "deployed the lane's engine; its run state is preserved", Details: run.Steps}
	if run.Unchanged {
		result.Outcome, result.Summary = intentUnchanged, "nothing to deploy"
	}
	if err != nil {
		result.Outcome, result.code, result.Summary = intentFailed, 1, "the lane's engine deploy stopped: "+oneLine(err.Error())
		var refusal *plain.Refusal
		if errors.As(err, &refusal) {
			result.refusalCode = refusal.Code
			result.Outcome, result.next = intentRefused, strings.Fields(refusal.Next)
		}
	}
	return inv.render(result)
}

func restartLandingSupervisor(scope processScope, effects processArmEffects, ensure func(supervise.EnsureOptions) (supervise.EnsureResult, error)) (processArmResult, error) {
	arm := effects.up
	effects.up = func(options up.Options) up.Result {
		options.EnsureArmed = func(arming supervise.EnsureOptions) (supervise.EnsureResult, error) {
			arming.OnlyIfDown = false
			return ensure(arming)
		}
		return arm(options)
	}
	return effects.steps(scope, 1000, processArmAuthority{})
}

func landingDeployMergeSettled(install string, fix *plain.Fix, seams plain.ProveSeams) (bool, error) {
	settled, _, err := plain.ResolutionWritersSettled(install, *fix, seams)
	return settled, err
}
