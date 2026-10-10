package main

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func intentWorkCommitCommand() intentCommand {
	return intentCommand{object: "work", action: "commit", laidOut: true, audience: "both",
		summary:  "commit staged hand changes as a named unit on the goal branch",
		usage:    []string{"metasystem work commit G --work U [--amend]"},
		examples: []string{"metasystem work commit verbs-match-intent --work discovery"}, maxArgs: 1, accepts: []string{refGoal},
		flags:   []intentFlag{{name: "work", value: "U", usage: "the unit whose staged changes are committed"}, {name: "amend", usage: "correct the existing unit, preserving later work"}},
		details: []string{"Uses the current goal worktree's staged changes, supplies its Goal-Unit trailer and runs the static and committed cheap checks on the commit candidate. Publish and read it with work review --commit SHA --goal G."},
		run:     runIntentWorkCommit}
}

func runIntentWorkCommit(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" || !validManualWorkName(inv.input.text("work")) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "commit needs a goal and a work name; nothing was committed", next: inv.publicArgv("work", "commit", "G", "--work", "U")})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	worktree, problem := inv.goalWorktree(id)
	if problem != nil {
		return inv.render(*problem)
	}
	root, conn, runner := inv.goalWorktreeInstallation(worktree), inv.connection(), inv.unitRunner()
	refused := func(err error) int {
		return inv.render(intentResult{Targets: inv.targets(id), Outcome: intentRefused, code: 1, Summary: err.Error(), next: inv.sameCommand(), nextReason: "once the staged change or its checks are corrected"})
	}
	endpoint, err := conn.endpoint(root)
	if err != nil {
		return refused(err)
	}
	check := conn.claimCheck(inv.layout.InstallationRoot.Path(), id, endpoint)
	if err := branch.CheckCommitAccess(id, check); err != nil {
		if holder, next, reason, held := inv.heldElsewhere(id, err); held {
			return inv.render(intentResult{Targets: inv.targets(id), Outcome: intentRefused, code: 1,
				Summary: fmt.Sprintf("seat %s holds goal %s and writes its branch, so this work wasn't committed", holder, id),
				next:    next, nextReason: reason, Details: []string{err.Error()}})
		}
		return inv.render(intentResult{Targets: inv.targets(id), Outcome: intentRefused, code: 1,
			Summary: claimRefusalSummary(err, fmt.Sprintf("this session's hold on goal %s can't be confirmed, so this work wasn't committed", id)),
			next:    inv.claimRemedy(id, err), nextReason: claimRemedyReason(err, "a person takes it over; or commit from the session that holds it"),
			Details: []string{err.Error()}})
	}
	release, err := runner.ReserveMutation(worktree, id, "commit")
	if err != nil {
		return inv.render(inv.treeFailure(err))
	}
	defer release()
	base, err := conn.endpointTip(root, endpoint)
	if err != nil {
		return refused(err)
	}
	operation, err := conn.operationID()
	if err != nil {
		return refused(err)
	}
	var commit string
	err = runner.MutationSection(worktree, func(bound *launch.UnitRunner) error {
		return conn.section(root, func(token func(func() error) error) error {
			return token(func() error {
				var err error
				commit, err = conn.commit(branch.CommitRequest{Repo: root, Remote: endpoint.Remote, EndpointTip: base, GoalID: id,
					Unit: inv.input.text("work"), OpID: operation, Kind: branch.Unit, Amend: inv.input.switched("amend"), CheckClaim: check, Transport: conn.transport,
					BeforeCommit: func(candidate, parent, tree string) error {
						prefix, err := goalBranchGit(root, "rev-parse", "--show-prefix")
						if err != nil {
							return err
						}
						installation := filepath.Join(candidate, prefix)
						beforeHead, before, err := bound.WorktreeResult(candidate)
						if err != nil {
							return err
						}
						beforeIndex, err := goalBranchGit(candidate, "write-tree")
						if err != nil {
							return err
						}
						if _, err := conn.rebaseGate(installation); err != nil {
							return err
						}
						top, err := goalBranchGit(candidate, "rev-parse", "--show-toplevel")
						if err != nil {
							return err
						}
						command, err := landingProofCommand(installation, top, parent, "proof.cheap", goalBranchGit)
						if err != nil {
							return err
						}
						proof := exec.Command("/bin/sh", "-c", command)
						proof.Env = append(proof.Environ(), "LANDING_PROOF_BASE="+parent)
						proof.Dir, proof.Stdout, proof.Stderr = installation, inv.stderr, inv.stderr
						if err := proof.Run(); err != nil {
							return fmt.Errorf("the cheap check failed; nothing was committed: %w", err)
						}
						afterHead, after, err := bound.WorktreeResult(candidate)
						if err != nil {
							return err
						}
						afterIndex, indexErr := goalBranchGit(candidate, "write-tree")
						if beforeHead != afterHead || before != after || beforeIndex != afterIndex || indexErr != nil {
							return fmt.Errorf("the checks changed the commit candidate; nothing was committed")
						}
						return nil
					}})
				return err
			})
		})
	})
	if err != nil {
		return refused(err)
	}

	err = runner.MutationSection(worktree, func(bound *launch.UnitRunner) error {
		return bound.CommandWait(func() error {
			_, err := conn.push(branch.PushRequest{Repo: root, Remote: endpoint.Remote, EndpointTip: base, GoalID: id, OpID: operation + "-push", CheckClaim: check, Transport: conn.transport})
			return err
		})
	})
	if err != nil {
		return inv.render(intentResult{Targets: inv.targets(id), Outcome: intentPartial, code: 1, Summary: "the work is committed but publication is pending: " + err.Error(), next: inv.publicArgv("work", "review", id, "--changes", "--work", inv.input.text("work"), "--brief", "FILE"), nextReason: "continues the committed work"})
	}
	return inv.render(intentResult{Targets: inv.targets(id), Outcome: intentConfirmed, Summary: "committed the staged work as " + shortSHA(commit),
		Data: map[string]any{"commit": commit, "work": inv.input.text("work")}, next: inv.publicArgv("work", "review", "--commit", commit, "--goal", id), nextReason: "publishes its independent read"})
}
