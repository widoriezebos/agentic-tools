package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
	result, warning, refusal := inv.rebaseGoal(id)
	if refusal != nil {
		return inv.render(*refusal)
	}
	targets := []intentTarget{{Kind: "goal", ID: id}}
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
	lines := append(inv.rebaseReviewLines(id, result), warning...)
	return inv.render(intentResult{Targets: targets, Outcome: outcome, Summary: summary, Data: result, text: lines})
}

// rebaseGoal is the shared branch operation for rebase and land.
func (inv *intentInvocation) rebaseGoal(id string) (branch.RebaseResult, []string, *intentResult) {
	targets := []intentTarget{{Kind: "goal", ID: id}}
	refused := func(err error) (branch.RebaseResult, []string, *intentResult) {
		result := intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: err.Error(), next: inv.publicArgv("work", "status", id), nextReason: "shows the goal's branch"}
		if first, paths, hint, ok := ownerRemedy(err.Error()); ok {
			result.Summary, result.text = first, paths
			result.next, result.nextReason = hint.Argv, hint.Reason
		}
		var operation *branch.OpError
		if inv.command.action == "land" && errors.As(err, &operation) {
			result.Data = map[string]any{"code": operation.Code}
		}
		return branch.RebaseResult{}, nil, &result
	}
	found, err := inv.hasGoalWorktree(id)
	if err != nil {
		return refused(err)
	}
	if !found {
		return branch.RebaseResult{}, nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: "goal " + id + " needs a worktree before its branch can be rebased",
			next:    inv.publicArgv("work", "build", id), nextReason: "prepares the goal's worktree"}
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
	if result.State != "held" {
		record := conn.recordRebase
		if record == nil {
			record = productionRecordRebase
		}
		// The branch has already been published; a missing history line
		// cannot undo that publication or keep its landing from proceeding.
		if err := record(inv, id, result); err != nil {
			return result, []string{"the rebase history line was not written; run: metasystem goal sync"}, nil
		}
	}
	return result, nil, nil
}

func (inv *intentInvocation) hasGoalWorktree(id string) (bool, error) {
	trees, err := inv.registeredWorktrees()
	if err != nil {
		return false, err
	}
	for path, tree := range trees {
		if tree.ref == "refs/heads/goal/"+id {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				return true, nil
			}
		}
	}
	return false, nil
}

func (inv *intentInvocation) rebaseReviewLines(id string, result branch.RebaseResult) []string {
	lines := []string{}
	for _, unit := range result.Carried {
		lines = append(lines, "review carried: "+unit, "review: "+shellCommand(inv.publicArgv("work", "review", id, "--work", unit)))
	}
	for _, unit := range result.NeedsReview {
		lines = append(lines, "needs review: "+unit, "run: "+shellCommand(inv.publicArgv("work", "review", id, "--work", unit)))
	}
	return lines
}

func productionRecordRebase(inv *intentInvocation, id string, rebase branch.RebaseResult) error {
	reason := "on main " + shortCommit(rebase.MainTip)
	if rebase.State == "rebased" {
		reason = "rebased " + shortCommit(rebase.OldTip) + " onto main " + shortCommit(rebase.MainTip)
	}
	if len(rebase.Carried) != 0 {
		reason += "; reviews carried: " + strings.Join(rebase.Carried, ", ")
	}
	if len(rebase.NeedsReview) != 0 {
		reason += "; needs review: " + strings.Join(rebase.NeedsReview, ", ")
	}
	args := []string{"--root", inv.stateRoot, "--id", id, "--reason", reason}
	result := inv.goalAct(id, "rebase", func(dependencies syncRequestDependencies) int {
		return runGoalRecordRebase(args, inv.owners.commandNow, dependencies)
	})
	if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		return fmt.Errorf("%s", result.Summary)
	}
	return nil
}
