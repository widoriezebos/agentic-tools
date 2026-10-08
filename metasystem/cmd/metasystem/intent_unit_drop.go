package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// applyUnitDrop reverses a committed optional unit under its source reservation.
func (inv *intentInvocation) applyUnitDrop(targets []intentTarget, work *reviewWorkContext) *intentResult {
	if work.review == nil || work.subject == nil || work.subject.Commit == "" {
		return inv.refuseReviewDrop(targets, work)
	}
	review, source := *work.review, work.subject
	fail := func(err error) *intentResult {
		summary := "the drop remains pending"
		if source.Drop != nil && (source.Drop.Phase == "recorded" || source.Drop.Phase == "closed") {
			summary = "the unit is dropped; its question repair remains pending"
		}
		return &intentResult{Targets: targets, Outcome: intentInProgress, code: 1, Summary: summary, Details: []string{err.Error()}, Data: source.Drop, next: inv.sameCommand()}
	}
	actor, proof, problem := inv.actingAs("work drop", work.goal, actorEither)
	if problem != nil {
		return problem
	}
	if len(work.dropScope) > 0 && (proof == nil || !proof.ValidFor(inv.stateRoot)) {
		return requiredDropRemedy(inv, work)
	}
	current, err := inv.unitRunner().Status(work.run)
	if err != nil {
		return fail(err)
	}
	if _, problem := inv.reviseDecisions(work.goal, launch.NamedWork{Unit: work.work, Record: &current}, 0); problem != nil && len(work.dropScope) == 0 {
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
		if err := work.retain(*source); err != nil {
			return fail(err)
		}
	}
	drop := source.Drop
	if drop.Decisions != digest || drop.Requirements != scope {
		if drop.Subject.Published != "" {
			result := fail(fmt.Errorf("the bound decisions or accepted requirements changed"))
			result.Summary = "the inverse is published; reconcile the changed decisions or requirements before retrying"
			return result
		}
		return fail(fmt.Errorf("the bound decisions or accepted requirements changed; code is retained"))
	}
	if drop.At == "" {
		drop.Actor, drop.Reason = unitStopActor(actor), inv.input.text("reason")
		if drop.Actor == "" {
			projection, _, problem := inv.projection()
			if problem != nil {
				return problem
			}
			file, _ := goalRecord(projection, work.goal)
			if file == nil || file.Claimed == nil {
				return fail(fmt.Errorf("the drop's claim holder cannot be read"))
			}
			drop.Actor = file.Claimed.Machine + "+" + file.Claimed.Lineage
		}
		if drop.Reason == "" {
			decisions, violations := validate.Dispositions(inv.flagPath("dispositions"))
			if len(violations) != 0 {
				return fail(fmt.Errorf("the bound dispositions cannot be read: %s", strings.Join(violations, "; ")))
			}
			for _, decision := range decisions {
				if strings.HasPrefix(decision, "dropped:") {
					drop.Reason = strings.TrimSpace(strings.TrimPrefix(decision, "dropped:"))
					break
				}
			}
		}
		drop.At = inv.unitStopNow().UTC().Format(time.RFC3339Nano)
		if who := unitStopActor(actor); who != "" {
			drop.Impact = "Impact: remove this optional unit's changes after checks pass.\nOther work remains. Restore the saved changes to undo the drop.\nThe prior read stays available."
			if len(work.dropScope) > 0 {
				drop.Impact = "Impact: remove this unit's code and required scope after proof and publication. Findings close as dropped, without a clean read. Dependent work remains required. Absence risk: the required behavior will be missing; unreadable advisory facts remain unknown. Undo: restore the retained changes, then metasystem goal scope restore " + work.goal + " " + work.work + " --by " + who + "."
			}
			if err := inv.recordUnitStopOverride(work.goal, "work-drop", drop.Reason, drop.Impact, who); err != nil {
				return fail(err)
			}
		}
		if err := work.retain(*source); err != nil {
			return fail(err)
		}
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
	if drop.Subject.Published == "" {
		pushed, err := conn.push(branch.PushRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base, GoalID: work.goal, OpID: drop.Subject.Operation + "-push", CheckClaim: check, Transport: conn.transport})
		if err != nil {
			return fail(err)
		}
		if pushed.Tip != drop.Subject.Commit {
			return fail(fmt.Errorf("publication returned a different branch tip"))
		}
		drop.Subject.Published = pushed.Tip
		if err := work.retain(*source); err != nil {
			return fail(err)
		}
	}
	stop := current.Rounds[len(current.Rounds)-1].Stop
	if stop == nil {
		return fail(fmt.Errorf("the exact stopped read is unavailable"))
	}
	var findings []string
	for _, read := range current.Rounds[len(current.Rounds)-1].Reads {
		for _, finding := range read.Findings {
			findings = append(findings, finding.ID)
		}
	}
	outcome := goal.UnitDrop{Unit: work.work, Operation: drop.Subject.Operation, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Covered: drop.Covered, Findings: findings, Commit: drop.Subject.Commit, Tree: drop.Subject.StagedTree, Proof: drop.Subject.GateRunID, Decisions: drop.Decisions, Requirements: drop.Requirements, Revision: work.dropRevision, Actor: drop.Actor, Reason: drop.Reason, Impact: drop.Impact, At: drop.At}
	result := inv.goalAct(work.goal, "drop unit", inv.syncOwner("work-drop", append([]string{"--root", inv.stateRoot, "--id", work.goal}, actor...), proof, false, func(req goal.VerbRequest, _ *syncFlags) (goal.PublishResult, error) {
		return goal.RecordUnitDrop(req, work.goal, outcome, work.dropScope...)
	}, "id"))
	if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		if _, blocked, err := goal.PushedBlocking(inv.stateRoot); err == nil && blocked {
			result.next = inv.publicArgv("goal", "sync", "--recover")
		} else {
			result.next = inv.sameCommand()
		}
		return &result
	}
	drop.Phase = "recorded"
	if err := work.retain(*source); err != nil {
		return fail(err)
	}
	at, err := time.Parse(time.RFC3339Nano, drop.At)
	if err != nil {
		return fail(err)
	}
	if err := channel.RecordUnitStopAct(inv.layout.InstallationRoot.Path(), channel.UnitStopAct{ID: drop.Subject.Operation, Goal: work.goal, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Findings: findings, Kind: "work-drop", Reason: drop.Reason, At: at, UnitClosed: true}); err != nil {
		return fail(err)
	}
	drop.Phase = "closed"
	if err := work.retain(*source); err != nil {
		return fail(err)
	}
	return &intentResult{Targets: targets, Outcome: intentConfirmed, Summary: "the unit is dropped; its matching questions are closed", Data: drop, next: inv.publicArgv("work", "status", work.goal, "--work", work.work)}
}

func requiredDropRemedy(inv *intentInvocation, work *reviewWorkContext) *intentResult {
	return &intentResult{Outcome: intentRefused, code: 1, Summary: "Required work needs a person's explicit scope exclusion; code and scope remain.", next: inv.publicArgv("work", "review", work.goal, "--work", work.work, "--dispositions", "FILE", "--reason", "TEXT", "--by", "NAME"), Details: []string{fmt.Sprintf("Only a proven person can exclude unit %s.", work.work)}}
}
