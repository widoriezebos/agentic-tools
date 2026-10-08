package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// applyUnitDrop removes an optional unit under its source reservation.
func (inv *intentInvocation) applyUnitDrop(targets []intentTarget, work *reviewWorkContext) *intentResult {
	if work.review == nil || work.subject == nil {
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
	if err != nil {
		return fail(err)
	}
	pendingPatch := source.Drop != nil && source.Drop.PatchDigest != ""
	if source.Drop == nil {
		pendingPatch, err = inv.roundPatchPending(work, head)
		if err != nil {
			return fail(err)
		}
	}
	if dirty != "" && !pendingPatch {
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
		if len(source.Drop.Covered) == 0 && source.Commit != "" && dirty == "" {
			return fail(fmt.Errorf("no effective commits of this unit remain"))
		}
		if pendingPatch {
			if review.Round.Result == nil {
				return fail(fmt.Errorf("the exact pending patch was not retained"))
			}
			patch, err := os.ReadFile(filepath.Join(review.Round.Directory, "result.patch"))
			if err != nil || launch.UnitResultDigest(string(patch)) != review.Round.Result.PatchDigest || len(patch) == 0 {
				return fail(fmt.Errorf("the retained pending patch cannot be verified: %v", err))
			}
			source.Drop.PatchDigest, source.Drop.StartingResult = review.Round.Result.PatchDigest, dirty
			source.Drop.PendingPatch, err = runner.WorktreeDiff(review.Record.Worktree, head)
			if err != nil {
				return fail(err)
			}
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
			if err := inv.recordUnitStopOverride(work.goal, "work-drop", drop.Reason, drop.Impact, who); err != nil {
				return fail(err)
			}
		}
		if err := work.retain(*source); err != nil {
			return fail(err)
		}
	}
	retain := func(subject launch.UnitSubject) error { drop.Subject = subject; return work.retain(*source) }
	if drop.PatchDigest != "" && drop.Subject.Conflict != "" {
		drop.ConflictTrees = append(drop.ConflictTrees, drop.Subject.GateWorktree)
		drop.PendingPatch, err = runner.WorktreeDiff(review.Record.Worktree, head)
		if err != nil {
			return fail(err)
		}
		drop.StartingResult, drop.Subject = dirty, launch.UnitSubject{Operation: drop.Subject.Operation, ExpectedParent: head}
		if err := retain(drop.Subject); err != nil {
			return fail(err)
		}
	}
	if drop.Subject.Commit == "" && drop.Phase != "tree-applied" && drop.Phase != "recorded" && drop.Phase != "closed" {
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
				candidate, err := conn.commit(branch.CommitRequest{PrepareOnly: len(drop.Covered) == 0, Repo: install, Remote: endpoint.Remote, EndpointTip: base, GoalID: work.goal, Unit: work.work, OpID: drop.Subject.Operation, Kind: branch.Drop, FrozenPatch: []byte{}, CheckClaim: check, Transport: conn.transport, ResumeWorktree: drop.Subject.GateWorktree, KeepWorktree: func() bool { return true }, BeforeInstall: func() (func() error, error) {
					if drop.PatchDigest == "" {
						return nil, nil
					}
					return inv.installPendingRemoval(work, check)
				}, BeforeCommit: func(dir, parent, tree string) error {
					if parent != head {
						return fmt.Errorf("the branch moved before inverse preparation")
					}
					if drop.Subject.GateWorktree == "" {
						drop.Subject.GateWorktree = dir
						if err := retain(drop.Subject); err != nil {
							return err
						}
						if drop.PatchDigest != "" {
							if err := inv.preparePendingRemoval(work, dir); err != nil {
								return err
							}
						}
						if len(drop.Covered) > 0 {
							if _, err := git(dir, append([]string{"revert", "--no-commit"}, drop.Covered...)...); err != nil {
								drop.Subject.Conflict = err.Error()
								_ = retain(drop.Subject)
								return err
							}
						}
					}
					if drop.Subject.Conflict != "" {
						return fmt.Errorf("inverse conflict retained in %s: %s; scratch recovery remains pending", dir, drop.Subject.Conflict)
					}
					if drop.ResultTree != "" {
						staged, err := git(dir, "write-tree")
						if err != nil || strings.TrimSpace(string(staged)) != drop.Subject.StagedTree {
							return fmt.Errorf("the retained candidate changed: %v", err)
						}
						_, err = git(dir, "diff", "--exit-code")
						return err
					}
					proof, err := runner.ProveRoundResult(review, dir, &drop.Subject, retain)
					if err != nil {
						return err
					}
					drop.Subject.GateRunID = proof
					current, dirty, err := runner.WorktreeResult(review.Record.Worktree)
					if err != nil || current != head || dirty != drop.StartingResult {
						return fmt.Errorf("the owned tree moved after proof: %v", err)
					}
					result, err := git(dir, "write-tree")
					if err != nil {
						return err
					}
					drop.Subject.StagedTree = strings.TrimSpace(string(result))
					if drop.PatchDigest != "" && len(drop.Covered) > 0 {
						resultTree := drop.Subject.StagedTree
						remaining := filepath.Join(review.Round.Directory, drop.Subject.Operation+"-remaining.patch")
						_, err := git(dir, "apply", "--reverse", "--index", "--binary", "--allow-empty", remaining)
						if err != nil {
							drop.Subject.Conflict = err.Error()
							_ = retain(drop.Subject)
							return err
						}
						result, err = git(dir, "write-tree")
						if err != nil {
							return err
						}
						drop.ResultTree = resultTree
						drop.CommitTree, drop.Subject.StagedTree = strings.TrimSpace(string(result)), strings.TrimSpace(string(result))
					}
					if len(drop.Covered) == 0 {
						drop.ResultTree = drop.Subject.StagedTree
					}
					drop.Phase, drop.Subject.Conflict = "proved", ""
					return retain(drop.Subject)
				}})
				if err == nil && len(drop.Covered) > 0 {
					drop.Subject.Commit = candidate
				}
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
	if len(drop.Covered) > 0 && drop.Subject.Published == "" {
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
	if drop.ResultTree != "" {
		outcome.Tree, outcome.PatchDigest, outcome.CommitTree = drop.ResultTree, drop.PatchDigest, drop.CommitTree
	}
	result := inv.goalAct(work.goal, "drop unit", inv.syncOwner("work-drop", []string{"--root", inv.stateRoot, "--id", work.goal}, nil, false, func(req goal.VerbRequest, _ *syncFlags) (goal.PublishResult, error) {
		return goal.RecordUnitDrop(req, work.goal, outcome)
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
	return &intentResult{Targets: targets, Outcome: intentConfirmed, Summary: "the optional unit is dropped; its matching questions are closed", Data: drop, next: inv.publicArgv("work", "status", work.goal, "--work", work.work)}
}
