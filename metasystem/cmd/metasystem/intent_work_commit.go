package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
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
	reverting := inv.command.action == "revert"
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" || !reverting && !validManualWorkName(inv.input.text("work")) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "commit needs a goal and a work name; nothing was committed", next: inv.publicArgv("work", "commit", "G", "--work", "U")})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	worktree, problem := inv.goalWorktree(id)
	if problem != nil {
		return inv.render(*problem)
	}
	var inverse *processchange.ProcessAct
	if reverting {
		prepared, code := inv.prepareDeclarationInverse(id, worktree)
		if code != 0 {
			return code
		}
		inverse = &prepared
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
			Summary: fmt.Sprintf("this session's hold on goal %s can't be confirmed, so this work wasn't committed", id),
			next:    inv.publicArgv("goal", "claim", id, "--take-over", "--reason", "TEXT"), nextReason: "a person takes it over; or commit from the session that holds it",
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
	operation, unit, publish, patch := "", inv.input.text("work"), conn.commit, []byte(nil)
	if inverse == nil {
		operation, err = conn.operationID()
	} else {
		operation, unit, publish, patch = inverse.Operation, "inverse-"+inverse.ID, conn.commitPatch, inverse.Patch
	}
	if err != nil {
		return refused(err)
	}
	var commit string
	err = runner.MutationSection(worktree, func(bound *launch.UnitRunner) error {
		return conn.section(root, func(token func(func() error) error) error {
			return token(func() error {
				var err error
				commit, err = publish(branch.CommitRequest{Repo: root, Remote: endpoint.Remote, EndpointTip: base, GoalID: id,
					Unit: unit, FrozenPatch: patch, OpID: operation, Kind: branch.Unit, Amend: inv.input.switched("amend"), CheckClaim: check, Transport: conn.transport,
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
		if inverse != nil {
			return refused(err)
		}
		return inv.render(intentResult{Targets: inv.targets(id), Outcome: intentPartial, code: 1, Summary: "the work is committed but publication is pending: " + err.Error(), next: inv.publicArgv("work", "review", id, "--changes", "--work", inv.input.text("work"), "--brief", "FILE"), nextReason: "continues the committed work"})
	}
	if inverse != nil {
		act, err := processchange.CompleteInverse(inv.stateRoot, inv.layout.InstallationRoot, *inverse, commit, runner.Manager.Now())
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error(), Data: *inverse, next: append(inv.typedArgvLess("patch", "json"), "--patch", "FILE"), nextReason: "repeat with the retained inverse patch at your enrolled terminal"})
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: "the declaration delta was published; its intervention hold is resolved", Data: act})
	}
	return inv.render(intentResult{Targets: inv.targets(id), Outcome: intentConfirmed, Summary: "committed the staged work as " + shortSHA(commit),
		Data: map[string]any{"commit": commit, "work": inv.input.text("work")}, next: inv.publicArgv("work", "review", "--commit", commit, "--goal", id), nextReason: "publishes its independent read"})
}

func intentWorkRevertCommand() intentCommand {
	return intentCommand{object: "work", action: "revert", laidOut: true, audience: "person", summary: "restore a process act's declaration delta as a new branch commit", usage: []string{"metasystem work revert G --act ID --reason TEXT [--patch FILE]"}, examples: []string{"metasystem work revert improve-checks --act ID --reason \"Remove excess checks\""}, maxArgs: 1, accepts: []string{refGoal}, flags: []intentFlag{{name: "act", value: "ID", usage: "the original declaration act"}, {name: "reason", value: "TEXT", usage: "why to remove this intervention"}, {name: "patch", value: "FILE", usage: "explicit person repair when inverse evidence is unavailable"}}, run: runIntentWorkCommit}
}
func (inv *intentInvocation) prepareDeclarationInverse(id, worktree string) (act processchange.ProcessAct, code int) {
	if inv.input.text("act") == "" || inv.input.text("reason") == "" {
		return act, inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "revert needs an original act and a reason"})
	}
	act = processchange.ProcessAct{Goal: id, Checkout: inv.stateRoot, Reason: inv.input.text("reason")}
	by, _, refusal := inv.settingsPerson(inv.layout, "reverse committed declarations", &act.Proof)
	if refusal != nil {
		return act, inv.render(*refusal)
	}
	refused := func(err error) (processchange.ProcessAct, int) {
		return act, inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the declaration inverse remains pending: " + err.Error(), Data: act, next: append(inv.typedArgvLess("patch", "json"), "--patch", "FILE"), nextReason: "review an explicit delta at your enrolled terminal"})
	}
	currentBranch, err := inv.work().git(worktree, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(string(currentBranch)) != "goal/"+id {
		return refused(fmt.Errorf("the worktree is not this goal's branch"))
	}
	path, patch := filepath.ToSlash(filepath.Join(inv.layout.InstallationRel, "metasystem.conf")), []byte(nil)
	if inv.input.has("patch") {
		patch, err = os.ReadFile(inv.inputPath(inv.input.text("patch")))
		if err != nil {
			return refused(err)
		}
	}
	content, err := inv.work().git(worktree, "show", "HEAD:"+path)
	if err != nil && patch == nil {
		return refused(err)
	}
	act, err = processchange.PrepareInverse(inv.stateRoot, inv.input.text("act"), strings.TrimSpace(string(currentBranch)), path, string(content), patch, act, inv.unitRunner().Manager.Now())
	if err != nil {
		return refused(err)
	}
	if err := inv.recordUnitStopOverride(id, "work-revert", act.Reason, "Impact: restore only these declarations. Earlier rounds and unrelated holds stay as they are.", by); err != nil {
		return refused(err)
	}
	return act, 0
}
