package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func intentWorkRebaseCommand() intentCommand {
	return intentCommand{
		object: "work", action: "rebase", laidOut: true, audience: "both",
		summary: "move a goal's branch onto origin's main, carrying each unchanged unit's review",
		usage:   []string{"metasystem work rebase G"}, maxArgs: 1, accepts: []string{refGoal},
		examples: []string{"metasystem work rebase conflicts-resolve-unattended"}, run: runIntentWorkRebase,
	}
}

func runIntentWorkRebase(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "work rebase needs the goal to rebase; nothing was done",
			next:    inv.publicArgv("work", "status", "--all"), nextReason: "lists the goals with work"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	id := ref.id
	targets := []intentTarget{{Kind: "goal", ID: id}}
	refused := func(err error) int {
		result := intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: err.Error(), next: inv.publicArgv("work", "status", id), nextReason: "shows the goal's branch"}
		if first, paths, hint, ok := ownerRemedy(err.Error()); ok {
			result.Summary, result.text = first, paths
			result.next, result.nextReason = hint.Argv, hint.Reason
		}
		return inv.render(result)
	}
	trees, err := inv.registeredWorktrees()
	if err != nil {
		return refused(err)
	}
	found := false
	for path, tree := range trees {
		if tree.ref == "refs/heads/goal/"+id {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				found = true
			}
		}
	}
	if !found {
		return inv.render(intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: "goal " + id + " needs a worktree before its branch can be rebased",
			next:    inv.publicArgv("work", "build", id), nextReason: "prepares the goal's worktree"})
	}
	root := inv.goalBranchInstallation(id)
	conn := inv.connection()
	endpoint, err := conn.endpoint(root)
	if err != nil {
		return refused(err)
	}
	check := conn.claimCheck(root, id, endpoint)
	if err := branch.CheckHolder(check); err != nil {
		return refused(err)
	}
	mainTip, err := conn.endpointTip(root, endpoint)
	if err != nil {
		return refused(err)
	}
	var result branch.RebaseResult
	err = conn.section(root, func(_ func(func() error) error) error {
		var err error
		result, err = conn.rebase(branch.RebaseRequest{Repo: root, Remote: endpoint.Remote, EndpointTip: mainTip,
			GoalID: id, CheckClaim: check, Gate: conn.rebaseGate, Transport: conn.transport})
		return err
	})
	if err != nil {
		return refused(err)
	}
	outcome := intentConfirmed
	summary := fmt.Sprintf("rebased from %s onto main %s", shortCommit(result.OldTip), shortCommit(result.MainTip))
	if result.State == "held" {
		outcome = intentUnchanged
		summary = "goal " + id + " is already on main; held, nothing was written"
		if len(result.NeedsReview) != 0 {
			summary = "goal " + id + " is already on main; its branch was held"
		}
	} else if result.State != "rebased" {
		summary = "goal " + id + " is already on main; published its branch"
		if len(result.Carried) != 0 {
			summary = "goal " + id + " is on main; carried reviews and published its branch"
		}
	}
	lines := []string{}
	for _, unit := range result.Carried {
		lines = append(lines, "review carried: "+unit, "review: "+shellCommand(inv.publicArgv("work", "review", id, "--work", unit)))
	}
	for _, unit := range result.NeedsReview {
		lines = append(lines, "needs review: "+unit, "run: "+shellCommand(inv.publicArgv("work", "review", id, "--work", unit)))
	}
	return inv.render(intentResult{Targets: targets, Outcome: outcome, Summary: summary, Data: result, text: lines})
}
