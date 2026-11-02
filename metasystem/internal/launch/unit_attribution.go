package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

type unitComparison struct {
	Base         string              `json:"base"`
	Tree         string              `json:"tree"`
	Command      PlainBrief          `json:"command"`
	LaunchID     string              `json:"launchId"`
	Directory    string              `json:"directory"`
	Before       *repositorySnapshot `json:"before,omitempty"`
	Verified     bool                `json:"verified,omitempty"`
	MainRecorded bool                `json:"mainRecorded,omitempty"`
	StartedAt    string              `json:"startedAt,omitempty"`
	FinishedAt   string              `json:"finishedAt,omitempty"`
}

// Attribution reads retained executions; it never rebuilds a completed step.
func (runner *UnitRunner) attributeProof(record *UnitRunRecord, round *UnitRound, index int, plan UnitPlan, deadline time.Time) (bool, error) {
	step := &round.Steps[index]
	if step.State == StepFailed && step.Cause == "unclassified" && checkCause(step, round.Directory) {
		step.Reason = "the frozen check stopped at proof.deadline"
		// The collecting save books this launch as environment before a
		// retry replaces the step's launch id.
		driver := runner.driver(record, round)
		if err := driver.save(); err != nil {
			return false, err
		}
		launches := len(step.LaunchIDs)
		capped, err := driver.retryFailedStep(index, deadline)
		if err != nil || capped || len(step.LaunchIDs) == launches {
			return capped, err
		}
		return runner.attributeProof(record, round, index, plan, deadline)
	}
	if step.State != StepFailed || step.Cause != "unclassified" || step.Retained == nil {
		return false, nil
	}
	capped, err := runner.compareProof(record, round, index, plan, deadline)
	if err != nil {
		if IsCode(err, "BUDGET_REFUSED") || IsCode(err, "BUDGET_UNKNOWN") {
			return false, err
		}
		step.Cause, step.Reason = "unclassified", "the failed command cannot be attributed: "+err.Error()
		return false, runner.save(*record)
	}
	return capped, nil
}

func (runner *UnitRunner) compareProof(record *UnitRunRecord, round *UnitRound, index int, plan UnitPlan, deadline time.Time) (capped bool, err error) {
	base := plan.Base
	step := &round.Steps[index]
	launch, err := runner.Manager.Store.Read(step.LaunchID)
	if err != nil {
		return false, err
	}
	evidence := loopstop.RedEvidence{Goal: record.Goal, Replays: []loopstop.RedReplay{{Role: "base", Named: loopstop.RedRun{Wrapper: true, Whole: true}}}}
	complete := false
	if runner.RedUnits != nil {
		evidence.Failed, complete, err = runner.RedUnits(launch)
		if err != nil {
			return false, err
		}
	}
	if plan.Check != nil {
		latest, unknown := newestCheckExecution(round.Directory, step.LaunchID)
		if len(unknown) > 0 {
			evidence.CannotRun = "the frozen check's execution evidence cannot be read: " + strings.Join(unknown, "; ")
		} else if latest == nil {
			evidence.CannotRun = "no frozen-check execution of launch " + step.LaunchID
		} else {
			failed := evidence.Failed
			evidence.Failed = nil
			for i, section := range latest.Steps {
				if section.Outcome == "passed" {
					continue
				}
				if i == 0 && complete && len(failed) > 0 {
					evidence.Failed = append(evidence.Failed, failed...)
				} else {
					name := section.ID[strings.LastIndex(section.ID, ":")+1:]
					evidence.Failed = append(evidence.Failed, loopstop.RedUnit{Unit: "section/" + name})
				}
			}
		}
	}
	if step.Alone != nil {
		var capped bool
		evidence.Alone, capped, err = runner.rerunAlone(record, round, index, plan, deadline)
		if err != nil || capped {
			return capped, err
		}
	} else if step.FlakeRepeat {
		evidence.Alone = loopstop.RedRun{Attempted: true, Exited: launch.ExitCode != nil && launch.Cause == "" && (launch.State == Completed || launch.State == Failed), Wrapper: true, Launch: launch.ID, Failed: evidence.Failed}
	}
	verdict := loopstop.ClassifyRed(evidence)
	if verdict.Next == "alone" && !step.FlakeRepeat {
		step.FlakeRepeat = true
		step.Alone = &UnitAlone{LaunchID: step.LaunchID + "-alone", Selectors: verdict.Selectors}
		if err := runner.save(*record); err != nil {
			return false, err
		}
		var capped bool
		evidence.Alone, capped, err = runner.rerunAlone(record, round, index, plan, deadline)
		if err != nil || capped {
			return capped, err
		}
		verdict = loopstop.ClassifyRed(evidence)
	}
	if verdict.Cause != nil {
		step.Cause, step.Reason = verdict.Cause.Kind, verdict.Words
		step.Flakes = verdict.Flaked
		if step.Cause == "flake" {
			step.State, step.Cause = StepPassed, ""
		}
		if step.Reason == "" {
			step.Reason = "the red could not be classified: " + evidence.CannotRun
			if step.Cause == "environment" {
				step.Reason = "the rerun alone could not finish or has no complete report: " + evidence.Alone.Launch
			}
		}
		return false, runner.driver(record, round).save()
	}

	var command PlainBrief
	data, err := os.ReadFile(step.Retained.Brief)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, &command); err != nil {
		return false, err
	}
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	tree, err := git.Run(record.Worktree, nil, "rev-parse", "--verify", base+"^{tree}")
	if err != nil || strings.TrimSpace(string(tree)) == "" {
		return false, fmt.Errorf("the retained base tree cannot be read: %v", err)
	}
	root, err := git.Run(record.Worktree, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(strings.TrimSpace(string(root)), command.Dir)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("the failed command's directory is outside the base tree")
	}
	directory := filepath.Join(round.Directory, fmt.Sprintf("base-%d", index))
	if step.Comparison == nil {
		step.Comparison = &unitComparison{Base: base, Tree: strings.TrimSpace(string(tree)), Command: command, LaunchID: step.LaunchID + "-base", Directory: filepath.Join(directory, relative)}
		if err := runner.save(*record); err != nil {
			return false, err
		}
	}
	comparison := step.Comparison
	if comparison.StartedAt == "" {
		comparison.StartedAt = runner.Manager.Now().UTC().Format(time.RFC3339Nano)
		if err := runner.save(*record); err != nil {
			return false, err
		}
	}
	if comparison.Base != base || comparison.Tree != strings.TrimSpace(string(tree)) || !reflect.DeepEqual(comparison.Command, command) || comparison.Directory != filepath.Join(directory, relative) {
		return false, fmt.Errorf("the retained comparison names a different base tree or command; request a reasoned revision")
	}
	cleanup := false
	defer func() {
		if cleanup {
			err = errors.Join(err, os.RemoveAll(directory))
		}
	}()
	baseline, readErr := runner.Manager.Store.Read(comparison.LaunchID)
	if readErr != nil && !os.IsNotExist(readErr) {
		return false, readErr
	}
	if os.IsNotExist(readErr) {
		cleanup = true
		// A separate index and refs export the base without registering a
		// worktree or allowing the comparison to change the source repository.
		if _, err := git.Run(record.Worktree, nil, "init", "--quiet", directory); err != nil {
			return false, err
		}
		objects, err := git.Run(record.Worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
		if err != nil {
			return false, err
		}
		if err := os.WriteFile(filepath.Join(directory, ".git", "objects", "info", "alternates"), []byte(strings.TrimSpace(string(objects))+"\n"), 0600); err != nil {
			return false, err
		}
		commit, err := git.Run(record.Worktree, nil, "rev-parse", "--verify", base+"^{commit}")
		if err != nil {
			return false, err
		}
		if _, err := git.Run(directory, nil, "update-ref", "--no-deref", "HEAD", strings.TrimSpace(string(commit))); err != nil {
			return false, err
		}
		if _, err := git.Run(directory, nil, "read-tree", "--reset", "-u", comparison.Tree); err != nil {
			return false, err
		}
		actual, err := git.Run(directory, nil, "rev-parse", "--verify", "HEAD^{tree}")
		if err != nil || strings.TrimSpace(string(actual)) != comparison.Tree {
			return false, fmt.Errorf("the isolated comparison tree does not match the retained base")
		}
		before, err := runner.snapshotRepository(comparison.Directory)
		if err != nil {
			return false, err
		}
		comparison.Before = &before
		if err := runner.save(*record); err != nil {
			return false, err
		}
		command.Dir = comparison.Directory
		data, _ := json.Marshal(command)
		brief := filepath.Join(round.Directory, fmt.Sprintf("base-%d.json", index))
		if _, err := atomicfile.WriteFile(brief, data, 0600, runner.root()); err != nil {
			return false, err
		}
		spec := StartSpec{ID: comparison.LaunchID, Kind: "proof", Goal: record.Goal, Tag: record.Unit, WorkingDirectory: command.Dir, Brief: brief, Round: round.Number, MaxRounds: record.MaxRounds}
		if runner.BeforeModelLaunch != nil {
			if err := runner.BeforeModelLaunch(*record, spec); err != nil {
				return false, err
			}
		}
		startedAt := runner.Manager.Now().UTC().Format(time.RFC3339Nano)
		spec.wait = runner.CommandWait
		started, startErr := runner.Manager.Start(spec)
		if started.ID != "" {
			cleanup = false
		}
		if startErr != nil && started.ID == "" {
			comparison.StartedAt, comparison.FinishedAt = startedAt, runner.Manager.Now().UTC().Format(time.RFC3339Nano)
			if err := runner.save(*record); err != nil {
				return false, err
			}
			if runner.CollectLaunch != nil {
				if err := runner.CollectLaunch(*record, Record{ID: comparison.LaunchID, State: Failed, StartedAt: comparison.StartedAt, FinishedAt: comparison.FinishedAt}, "unclassified"); err != nil {
					return false, err
				}
			}
			return false, startErr
		}
		baseline = started
	}
	baseline, terminal, err := runner.waitLaunch(baseline.ID, max(time.Duration(0), deadline.Sub(runner.Manager.Now())))
	if err != nil {
		cleanup = false
		return false, err
	}
	if !terminal {
		cleanup = false
		return true, nil
	}
	cleanup = true
	if runner.CollectLaunch != nil {
		if err := runner.CollectLaunch(*record, baseline, failedStepCause(baseline)); err != nil {
			return false, err
		}
	}
	if comparison.Before == nil {
		return false, fmt.Errorf("the comparison's original tree observation is unavailable")
	}
	if !comparison.Verified {
		after, err := runner.snapshotRepository(comparison.Directory)
		if err != nil || len(comparison.Before.changed(after)) > 0 {
			return false, fmt.Errorf("the comparison tree moved or cannot be read")
		}
		comparison.Verified = true
		if err := runner.save(*record); err != nil {
			return false, err
		}
	}
	if baseline.ExitCode == nil || baseline.Cause != "" || baseline.State != Completed && baseline.State != Failed {
		return false, fmt.Errorf("the base command has no completed exit evidence")
	}
	cause := "own"
	if baseline.State == Failed {
		cause = "other"
		main, err := git.Run(record.Worktree, nil, "rev-parse", "--verify", "origin/main")
		if err == nil {
			mainTree, err := git.Run(record.Worktree, nil, "rev-parse", "--verify", strings.TrimSpace(string(main))+"^{tree}")
			if err != nil {
				return false, err
			}
			if strings.TrimSpace(string(mainTree)) == comparison.Tree {
				cause = "main"
				if runner.RecordMain != nil && !comparison.MainRecorded {
					comparison.MainRecorded = true
					if err := runner.save(*record); err != nil {
						return false, err
					}
					if err := runner.RecordMain(baseline, strings.TrimSpace(string(main)), comparison.Tree); err != nil {
						return false, err
					}
				}
			}
		}
	}
	step.Cause = cause
	return false, runner.save(*record)
}

// rerunAlone retains a separate execution without replacing the failed check.
func (runner *UnitRunner) rerunAlone(record *UnitRunRecord, round *UnitRound, index int, plan UnitPlan, deadline time.Time) (loopstop.RedRun, bool, error) {
	step := &round.Steps[index]
	alone := step.Alone
	run := loopstop.RedRun{Attempted: true, Launch: alone.LaunchID, Selectors: alone.Selectors}
	if alone.Outcome == "environment" {
		return run, false, nil
	}
	launch, err := runner.Manager.Store.Read(alone.LaunchID)
	if os.IsNotExist(err) && alone.Outcome == "" {
		var command PlainBrief
		data, err := os.ReadFile(step.Retained.Brief)
		if err != nil {
			return run, false, err
		}
		if err := json.Unmarshal(data, &command); err != nil {
			return run, false, err
		}
		if plan.Check != nil {
			command.Argv = []string{"/bin/sh", "-c", plan.Check.Cheap}
			command.Dir, command.Env = plan.Check.Directory, plan.Check.Environment
		}
		if len(plan.FullArgv) > 0 {
			command.Argv = plan.FullArgv
		}
		command.Env = childEnvironment(command.Env, alone.Selectors)
		brief := filepath.Join(round.Directory, fmt.Sprintf("alone-%d.json", index))
		if err := writeUnitJSON(brief, command, runner.root()); err != nil {
			return run, false, err
		}
		spec := StartSpec{ID: alone.LaunchID, Actor: runner.Actor, Kind: "proof", Goal: record.Goal, Tag: record.Unit, WorkingDirectory: command.Dir, Brief: brief, Round: round.Number, MaxRounds: record.MaxRounds}
		if runner.BeforeModelLaunch != nil {
			if err := runner.BeforeModelLaunch(*record, spec); err != nil {
				return run, false, err
			}
		}
		alone.StartedAt = runner.Manager.Now().UTC().Format(time.RFC3339Nano)
		spec.wait = runner.CommandWait
		launch, err = runner.Manager.Start(spec)
		if err != nil && launch.ID == "" {
			if IsCode(err, "BUDGET_REFUSED") || IsCode(err, "BUDGET_UNKNOWN") {
				return run, false, err
			}
			alone.Outcome, alone.FinishedAt = "environment", runner.Manager.Now().UTC().Format(time.RFC3339Nano)
			if err := runner.save(*record); err != nil {
				return run, false, err
			}
			if runner.CollectLaunch != nil {
				err = runner.CollectLaunch(*record, Record{ID: alone.LaunchID, State: Failed, StartedAt: alone.StartedAt, FinishedAt: alone.FinishedAt}, "environment")
			}
			return run, false, err
		}
	}
	if err != nil && launch.ID == "" {
		return run, false, err
	}
	if !slices.Contains(step.LaunchIDs, alone.LaunchID) {
		step.LaunchIDs = append(step.LaunchIDs, alone.LaunchID)
		if err := runner.driver(record, round).save(); err != nil {
			return run, false, err
		}
	}
	launch, terminal, err := runner.waitLaunch(alone.LaunchID, max(time.Duration(0), deadline.Sub(runner.Manager.Now())))
	if err != nil || !terminal {
		return run, !terminal, err
	}
	alone.Outcome, alone.StartedAt, alone.FinishedAt = string(launch.State), launch.StartedAt, launch.FinishedAt
	run.Exited = launch.ExitCode != nil && launch.Cause == "" && (launch.State == Completed || launch.State == Failed)
	if runner.RedUnits != nil && run.Exited {
		run.Failed, run.Complete, err = runner.RedUnits(launch)
		if err != nil {
			return run, false, err
		}
	}
	run.Passed = run.Exited && *launch.ExitCode == 0 && run.Complete && len(run.Failed) == 0
	return run, false, runner.driver(record, round).save()
}

func (result UnitResult) AttributionSummary(now time.Time) string {
	if !result.Capped || len(result.Record.Rounds) == 0 {
		return ""
	}
	for _, step := range result.Record.Rounds[len(result.Record.Rounds)-1].Steps {
		if step.State == StepFailed && step.Cause == "unclassified" && step.Comparison != nil {
			started, _ := time.Parse(time.RFC3339Nano, step.Comparison.StartedAt)
			minutes := 0
			if !started.IsZero() {
				minutes = max(0, int(now.Sub(started).Minutes()))
			}
			return fmt.Sprintf("%s: comparing the red with the base (%d min)", result.Record.Unit, minutes)
		}
	}
	return ""
}
