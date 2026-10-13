package main

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingResumeCommand() intentCommand {
	return laneCommand(intentCommand{object: "landing", action: "resume", audience: "both", maxArgs: 0,
		summary:  "recover the landing lane with its waiting goals and start its agent",
		usage:    []string{"metasystem landing resume [--by NAME] [--json]"},
		flags:    []intentFlag{{name: "by", value: "NAME", usage: "the enrolled person's name"}},
		examples: []string{"metasystem landing resume", "metasystem landing resume --by Wido"},
	}, func(inv *intentInvocation, admitted laneAdmitted) int {
		person, problem := inv.landingPersonCheck("resume the landing lane", admitted.record.Root)
		if problem != nil {
			return inv.render(*problem)
		}
		if problem := inv.laneResumable(admitted.owners, admitted.home, admitted.record); problem != nil {
			return inv.render(*problem)
		}
		s := inv.laneBatchSeams(admitted.home, admitted.record, admitted.owners.proveSeams(admitted.installation))
		s.CommandForCommit = func(commit string) (string, error) {
			return landingProofCommand(admitted.installation, admitted.record.Root, commit, proveCommandKey, s.Git)
		}
		keeper := inv.resumeKeeper(admitted)
		s.ResumeKeeper = &keeper
		s.AgentRunning = func() (bool, error) { _, live, err := keeper.Running(); return live, err }
		provenance := plain.ActProvenance{Person: person.Name, Root: person.Root, TerminalGeneration: person.Proof.TerminalGeneration, TerminalRef: person.Proof.TerminalRef}
		run, err := plain.Resume(admitted.home, provenance, admitted.owners.now(), s)
		if err != nil {
			return inv.render(landingLaneFailure(laneTargets(admitted.record.Root), "the lane's resume could not finish", err))
		}
		outcome := intentConfirmed
		if !run.Changed {
			outcome = intentUnchanged
		}
		return inv.render(intentResult{Outcome: outcome, Summary: strings.Join(run.Lines, "; "), Data: run, Targets: laneTargets(admitted.record.Root)})
	})
}

func (inv *intentInvocation) resumeKeeper(admitted laneAdmitted) lane.AgentKeeper {
	keeper := admitted.owners.keeper(admitted.home, admitted.record.Root)
	keeper.Holds = append(keeper.Holds, func(string) (string, error) {
		if problem := inv.laneNotReady(admitted.owners, admitted.record.Root); problem != nil {
			return problem.Summary, nil
		}
		return "", nil
	})
	return keeper
}
