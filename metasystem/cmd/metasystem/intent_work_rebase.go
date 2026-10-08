package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
	if result.Behind > 0 {
		summary = fmt.Sprintf("goal %s is %d commits behind current remote main; its branch was %s", id, result.Behind, result.State)
	}
	lines := append(inv.rebaseReviewLines(id, result), warning...)
	for _, unit := range result.NeedsReview {
		if why := result.ReviewReasons[unit]; why != "" {
			lines = append(lines, "work "+unit+" needs a read: "+why,
				"person repair: metasystem work build "+id+" --work declaration-repair --brief FILE --reason TEXT --by NAME --check COMMAND")
		}
	}
	return inv.render(intentResult{Targets: targets, Outcome: outcome, Summary: summary, Data: result, text: lines})
}

// rebaseGoal is the shared branch operation for rebase and land.
func (inv *intentInvocation) rebaseGoal(id string) (out branch.RebaseResult, warnings []string, problem *intentResult) {
	targets := []intentTarget{{Kind: "goal", ID: id}}
	refused := func(err error) (branch.RebaseResult, []string, *intentResult) {
		var conflict *branch.RebaseConflict
		if errors.As(err, &conflict) {
			result := inv.rebaseJudgementQuestions(id, conflict)
			return branch.RebaseResult{}, nil, &result
		}
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
	runner := inv.unitRunner()
	releaseTree, err := runner.ReserveMutation(root, id, "rebase")
	if err != nil {
		result := inv.treeFailure(err)
		return branch.RebaseResult{}, nil, &result
	}

	defer func() {
		if err := releaseTree(); err != nil {
			if problem != nil {
				problem.Details = append(problem.Details, err.Error())
			} else {
				_, _, problem = refused(err)
			}
		}
	}()

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
			GoalID: id, CheckClaim: check, Gate: conn.rebaseGate, SubjectCheck: conn.subjectCheck, Transport: conn.transport, Resolve: inv.resolveRebaseRound(id, runner),
			RecordDrop: func(before, after goal.UnitDrop) error {
				args := append([]string{"--root", inv.stateRoot, "--id", id}, inv.forward("lineage")...)
				result := inv.goalAct(id, "drop unit", inv.syncOwner("work-drop", args, nil, false, func(req goal.VerbRequest, _ *syncFlags) (goal.PublishResult, error) {
					projection, _, problem := inv.projection()
					if problem != nil {
						return goal.PublishResult{}, errors.New(problem.Summary)
					}
					file, _ := goalRecord(projection, id)
					if file == nil {
						return goal.PublishResult{}, fmt.Errorf("goal %s is no longer live", id)
					}
					after.Revision = file.Revision
					return goal.RecordUnitDrop(req, id, after, before)
				}, "id"))
				if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
					remedy := inv.publicArgv("work", "rebase", id)
					if _, blocked, err := goal.PushedBlocking(inv.layout.InstallationRoot.Path()); err == nil && blocked {
						remedy = inv.publicArgv("goal", "sync", "--recover")
					}
					return fmt.Errorf("%s\nrun: %s", result.Summary, shellCommand(remedy))
				}
				return nil
			},
			Answer: func(question string) (string, error) {
				q, err := channel.ReadQuestion(inv.layout.InstallationRoot.Path(), question)
				if err != nil {
					return "", err
				}
				if q.Answer != nil {
					return q.Answer.Text, nil
				}
				return "", nil
			}})
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

func (inv *intentInvocation) resolveRebaseRound(id string, runner *launch.UnitRunner) func(branch.RebaseResolution) (string, error) {
	return func(stop branch.RebaseResolution) (string, error) {
		work, problem := inv.goalWork(id)
		if problem != nil {
			return "", errors.New(problem.Summary)
		}
		for _, unit := range work {
			if unit.Unit != stop.Unit || unit.Record == nil {
				continue
			}
			owners := inv.delivery()
			laneRoot, configured, err := owners.laneRoot(inv.layout.InstallationRoot.Path(), owners.now())
			if err != nil {
				return unit.Record.Plan, err
			}
			if configured {
				install, err := inv.laneInstallOf(laneRoot)
				if err != nil {
					return unit.Record.Plan, err
				}
				entry, ok, err := inv.latestLaneEntry(install, id, stop.MainTip)
				if err != nil {
					return unit.Record.Plan, err
				}
				if ok && entry.Conflict != nil && entry.Conflict.Main == stop.MainTip {
					for _, path := range entry.Conflict.Paths {
						if slices.Contains(stop.Paths, path.Path) && path.Resolution != "" {
							stop.Conflicts += fmt.Sprintf("\nWritten resolution from the lane for %s: %s\n", path.Path, path.Resolution)
						}
					}
				}
			}
			request := launch.UnitRevisionRequest{Run: unit.Run, Brief: []byte(stop.Conflicts + fmt.Sprintf("\nResolve-round base: %s. Build in %s.\n", stop.Base, stop.Worktree)),
				Rebase: &launch.UnitRebasePlan{Worktree: stop.Worktree, Base: stop.Base, Commit: stop.Commit}}
			for {
				result, err := runner.Revise(request)
				record := filepath.Join(filepath.Dir(unit.Record.Plan), "run.json")
				if err != nil {
					return record, err
				}
				if result.Capped {
					continue
				}
				if result.Record.Rounds[result.Round-1].Outcome != "green" {
					return record, fmt.Errorf("resolve round ended %s", result.Record.Rounds[result.Round-1].Outcome)
				}
				return record, nil
			}
		}
		return "", fmt.Errorf("build %s has no retained unit brief and check; run: metasystem work status %s", stop.Unit, id)
	}
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
	if len(rebase.Regenerated) != 0 {
		reason += "; regenerated: " + strings.Join(rebase.Regenerated, ", ")
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

func (inv *intentInvocation) rebaseJudgementQuestions(id string, conflict *branch.RebaseConflict) intentResult {
	file := filepath.Join(inv.goalBranchInstallation(id), "artifacts", "agents", "goals", id, "conflict.json")
	var previous struct{ Paths []map[string]string }
	if data, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(data, &previous)
	}
	recorded := previous.Paths
	result := intentResult{Targets: []intentTarget{{Kind: "goal", ID: id}}, Outcome: intentRefused, code: 1,
		Summary: fmt.Sprintf("rebase needs your choice; main is at %s; nothing was changed", shortCommit(conflict.MainTip)),
		Data:    map[string]any{"code": conflict.Code}, next: inv.publicArgv("work", "status", id),
		nextReason: "shows the goal's branch"}
	for _, path := range conflict.Paths {
		question := fmt.Sprintf("Goal %s and main both changed lines %d to %d of %s.\nKeep main's, keep the goal's, or write a third.", id, path.FirstLine, path.LastLine, path.Path)
		if path.FirstLine == 0 {
			question = fmt.Sprintf("Goal %s and main both changed %s where no common version exists.\nKeep main's, keep the goal's, or write a third.", id, path.Path)
		}
		mainChange := path.MainGoal
		if mainChange == "" {
			mainChange = path.MainCommit
		}
		mainImpact := fmt.Sprintf("main's drops what %s did there and unit %s loses its read", id, conflict.Unit)
		goalImpact := fmt.Sprintf("the goal's undoes main's change there (%s)", mainChange)
		thirdImpact := "a third is read again"
		in := channelAskInput{Goal: id, Kind: "other", Facts: []string{question,
			"Impact: " + mainImpact + ";\n" + goalImpact + "; a third is read again.\nNothing lands until you answer.",
			fmt.Sprintf("Blobs: original %s, main %s, goal %s.", path.Original, path.Main, path.Goal)},
			Options: []string{"keep main's: " + mainImpact, "keep the goal's: " + goalImpact, "write a third: " + thirdImpact}}
		asked, warnings, code, err := inv.connection().askRebase(inv.layout.InstallationRoot.Path(), in)
		recorded = slices.DeleteFunc(recorded, func(item map[string]string) bool { return item["path"] == path.Path })
		recorded = append(recorded, map[string]string{"path": path.Path, "original": path.Original, "main": path.Main, "goal": path.Goal,
			"class": "judgement", "resolution": "", "question": asked.ID, "impact": in.Facts[1]})
		result.text = append(result.text, warnings...)
		if asked.ID != "" {
			result.text = append(result.text, "question "+asked.ID+": "+path.Path,
				"run: "+shellCommand(inv.publicArgv("question", "wait", asked.ID)))
			if len(result.next) > 2 && result.next[1] == "work" {
				result.next = inv.publicArgv("question", "wait", asked.ID)
				result.nextReason = "waits for your choice"
			}
		}
		if err != nil || code != 0 || asked.ID == "" {
			result.text = append(result.text, "the question could not be sent:", question)
		}
	}
	data, err := json.Marshal(map[string]any{"base": conflict.Base, "main": conflict.MainTip, "paths": recorded})
	if err == nil {
		_, err = atomicfile.WriteText(file, string(data)+"\n", "")
	}
	if err != nil {
		result.text = append(result.text, "the conflict question's blob binding could not be kept: "+err.Error())
	}
	return result
}
