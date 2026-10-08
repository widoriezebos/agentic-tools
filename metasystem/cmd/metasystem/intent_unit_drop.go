package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// applyUnitDrop reverses a committed optional unit under its source reservation.
func (inv *intentInvocation) applyUnitDrop(targets []intentTarget, work *reviewWorkContext) *intentResult {
	if work.review == nil || work.subject == nil || work.subject.Commit == "" {
		return inv.refuseReviewDrop(targets, work)
	}
	review, source := *work.review, work.subject
	fail := func(err error) *intentResult {
		return &intentResult{Targets: targets, Outcome: intentInProgress, code: 1, Summary: "the drop remains pending: " + err.Error(), Data: source.Drop, next: inv.publicArgv("work", "status", work.goal, "--work", work.work)}
	}
	actor, _, problem := inv.actingAs("work drop", work.goal, actorEither)
	if problem != nil {
		return problem
	}
	current, err := inv.unitRunner().Status(work.run)
	if err != nil {
		return fail(err)
	}
	if _, problem := inv.reviseDecisions(work.goal, launch.NamedWork{Unit: work.work, Record: &current}, 0); problem != nil {
		return problem
	}
	if work.dropRequirements == "" {
		return inv.refuseReviewDrop(targets, work)
	}
	runner, git, conn := inv.unitRunner(), inv.work().git, inv.connection()
	install := inv.goalWorktreeInstallation(review.Record.Worktree)
	endpoint, err := conn.endpoint(inv.layout.InstallationRoot.Path())
	if err != nil {
		return fail(err)
	}
	check := conn.claimCheck(inv.layout.InstallationRoot.Path(), work.goal, endpoint)
	base, err := conn.endpointTip(inv.layout.InstallationRoot.Path(), endpoint)
	if err != nil {
		return fail(err)
	}
	head, dirty, err := runner.WorktreeResult(review.Record.Worktree)
	if err != nil || dirty != "" {
		return fail(fmt.Errorf("the owned tree must be clean before a committed drop: %v", err))
	}
	body, err := os.ReadFile(inv.flagPath("dispositions"))
	if err != nil {
		return fail(err)
	}
	digest, scope := launch.UnitResultDigest(string(body)), work.dropRequirements
	if source.Drop == nil {
		op := source.Operation + "-drop-" + digest[:12]
		source.Drop = &launch.UnitDrop{Decisions: digest, Requirements: scope, Revision: work.dropRevision, Phase: "prepared", Subject: launch.UnitSubject{Operation: op, ExpectedParent: head}}
		ids, err := git(install, "rev-list", "--first-parent", base+".."+head)
		if err != nil {
			return fail(err)
		}
		for _, id := range strings.Fields(string(ids)) {
			kind, err := branch.KindOfWithRaw(install, id, work.goal, git)
			if err != nil {
				return fail(err)
			}
			if kind.Kind == branch.Unit && slices.Contains(kind.Units, work.work) {
				if len(kind.Units) != 1 {
					return fail(fmt.Errorf("shared commit %s lacks exact unit coverage; code is retained", id))
				}
				source.Drop.Covered = append(source.Drop.Covered, id)
			}
		}
		if len(source.Drop.Covered) == 0 {
			return fail(fmt.Errorf("no effective commits of this unit remain"))
		}
		if who := unitStopActor(actor); who != "" {
			if err := inv.recordUnitStopOverride(work.goal, "work-drop", inv.input.text("reason"), "Impact: remove this optional unit's changes after checks pass.\nOther work remains. Restore the saved changes to undo the drop.\nThe prior read stays available.", who); err != nil {
				return fail(err)
			}
		}
		if err := work.retain(*source); err != nil {
			return fail(err)
		}
	}
	drop := source.Drop
	if drop.Decisions != digest || drop.Requirements != scope {
		return fail(fmt.Errorf("the bound decisions or accepted requirements changed; code is retained"))
	}
	retain := func(subject launch.UnitSubject) error { drop.Subject = subject; return work.retain(*source) }
	if drop.Subject.Commit == "" {
		ids, err := git(install, "log", "--format=%H %P %T", "--fixed-strings", "--grep=Goal-Drop: "+work.goal+"/"+work.work+" "+drop.Subject.Operation, base+".."+head)
		if err != nil {
			return fail(err)
		}
		for _, line := range strings.Split(string(ids), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 3 || fields[1] != drop.Subject.ExpectedParent || fields[2] != drop.Subject.StagedTree || drop.Subject.GateRunID == "" {
				continue
			}
			kind, err := branch.KindOfWithRaw(install, fields[0], work.goal, git)
			if err == nil && kind.Kind == branch.Drop && kind.Operation == drop.Subject.Operation && kind.Unit == work.work {
				drop.Subject.Commit = fields[0]
			}
		}
		if drop.Subject.Commit == "" {
			if head != drop.Subject.ExpectedParent {
				return fail(fmt.Errorf("the branch moved; retained inverse and proof need reconciliation"))
			}
			err = conn.commitToken(install, func() error {
				var err error
				drop.Subject.Commit, err = conn.commit(branch.CommitRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base, GoalID: work.goal, Unit: work.work, OpID: drop.Subject.Operation, Kind: branch.Drop, FrozenPatch: []byte{}, CheckClaim: check, Transport: conn.transport, ResumeWorktree: drop.Subject.GateWorktree, KeepWorktree: func() bool { return true }, BeforeCommit: func(dir, parent, tree string) error {
					if parent != head {
						return fmt.Errorf("the branch moved before inverse preparation")
					}
					if drop.Subject.GateWorktree == "" {
						drop.Subject.GateWorktree = dir
						if err := retain(drop.Subject); err != nil {
							return err
						}
						if _, err := git(dir, append([]string{"revert", "--no-commit"}, drop.Covered...)...); err != nil {
							drop.Subject.Conflict = err.Error()
							_ = retain(drop.Subject)
							return err
						}
					}
					if drop.Subject.Conflict != "" {
						return fmt.Errorf("inverse conflict retained in %s: %s; scratch recovery remains pending", dir, drop.Subject.Conflict)
					}
					proof, err := runner.ProveRoundResult(review, dir, &drop.Subject, retain)
					if err != nil {
						return err
					}
					drop.Subject.GateRunID = proof
					current, dirty, err := runner.WorktreeResult(review.Record.Worktree)
					if err != nil || current != head || dirty != "" {
						return fmt.Errorf("the owned tree moved after proof: %v", err)
					}
					result, err := git(dir, "write-tree")
					if err != nil {
						return err
					}
					drop.Subject.StagedTree = strings.TrimSpace(string(result))
					drop.Phase, drop.Subject.Conflict = "proved", ""
					return retain(drop.Subject)
				}})
				return err
			})
			if err != nil {
				return fail(err)
			}
		}
		drop.Phase = "tree-applied"
		if err := work.retain(*source); err != nil {
			return fail(err)
		}
	}
	return &intentResult{Targets: targets, Outcome: intentPartial, code: 1, Summary: "the committed inverse is applied and proved; durable goal outcome, read exemption, landing and question closure remain pending", Data: drop, next: inv.publicArgv("work", "status", work.goal, "--work", work.work)}
}
