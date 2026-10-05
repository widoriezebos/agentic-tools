package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// landRebaseSkip protects commits a lane or a person already holds.
func (inv *intentInvocation) landRebaseSkip(id, through, install string, state intentBranchState) (string, *intentResult) {
	targets := []intentTarget{{Kind: "goal", ID: id}}
	if through != "" {
		return "--through names a commit", nil
	}
	if install != "" {
		entry, ok, err := inv.latestLaneEntry(install, id, state.EndpointTip)
		if err != nil {
			return "", &intentResult{Targets: targets, Outcome: intentFailed, code: 1,
				Summary: "the landing lane's queue can't be read, so nothing was handed in",
				next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
		}
		// The lane may still merge its waiting commit, even when the
		// branch has moved on. Its commits must keep their names.
		if ok && entry.State == plain.StateWaiting {
			return "its hand-in at " + plain.Short(entry.SHA) + " still waits in the lane", nil
		}
		if ok && entry.State == plain.StateLanded && entry.SHA == state.BranchTip {
			return "its hand-in at " + plain.Short(entry.SHA) + " already landed on main", nil
		}
	}
	found, err := inv.hasGoalWorktree(id)
	if err != nil {
		return "", &intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "the goal's worktree can't be found, so nothing was handed in",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if !found {
		return "the goal has no worktree", nil
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return "", problem
	}
	settings, err := landingGateSettings(inv.layout.InstallationRoot.Path(), delegationToolInstallation)
	if err != nil {
		return "", &intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "the landing settings can't be read, so nothing was handed in",
			next:    inv.publicArgv("settings", "check"), nextReason: "shows what needs fixing", Details: []string{err.Error()}}
	}
	file := projection.Tree.Live[id]
	// A person's approval at this tip would be void at a rewritten tip.
	if settings.WaitsForHuman(file) {
		if _, err := goal.Gate(file, state.BranchTip, settings); err == nil {
			return "a person's word stands at this tip", nil
		}
	}
	return "", nil
}
