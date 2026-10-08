package main

import (
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func (inv *intentInvocation) treeFailure(err error) intentResult {
	var waiting *launch.TreeWaitingError
	if errors.As(err, &waiting) {
		return intentResult{Outcome: intentInProgress, code: 3, Summary: waiting.Error(), next: inv.publicArgv("work", "wait", "run:"+waiting.Run)}
	}
	return intentResult{Outcome: intentFailed, code: 1, Summary: "the worktree ownership cannot be read; nothing was started", Details: []string{err.Error()}, next: inv.sameCommand()}
}

func (inv *intentInvocation) stopUnitRun(id string) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.directPersonProof("cancellation of a unit run"); problem != nil {
		return inv.render(*problem)
	}
	record, err := inv.unitRunner().CancelRun(id)
	targets := []intentTarget{{Kind: "unit", ID: id}}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the run could not be fully stopped; its worktree remains reserved", Details: []string{err.Error()}, next: inv.sameCommand(), Data: record})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "run " + id + " cancelled; its children ended and its worktree is released", Data: record})
}
