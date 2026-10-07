package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
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
}

// Attribution reads retained executions; it never rebuilds a completed step.
func (runner *UnitRunner) attributeProof(record *UnitRunRecord, round *UnitRound, index int, base string, deadline time.Time) (bool, error) {
	step := &round.Steps[index]
	if step.State != StepFailed || step.Cause != "unclassified" || step.Retained == nil {
		return false, nil
	}
	capped, err := runner.compareProof(record, round, index, base, deadline)
	if err != nil {
		step.Cause, step.Reason = "unclassified", "the failed command cannot be attributed: "+err.Error()
		return false, runner.save(*record)
	}
	return capped, nil
}

func (runner *UnitRunner) compareProof(record *UnitRunRecord, round *UnitRound, index int, base string, deadline time.Time) (capped bool, err error) {
	step := &round.Steps[index]
	launch, err := runner.Manager.Store.Read(step.LaunchID)
	if err != nil {
		return false, err
	}
	if runner.KnownFlake != nil && !step.FlakeRepeat {
		known, err := runner.KnownFlake(launch)
		if err != nil {
			return false, err
		}
		if known {
			step.FlakeRepeat, step.State, step.Cause, step.Reason = true, StepPending, "", ""
			if err := runner.save(*record); err != nil {
				return false, err
			}
			capped, err := runner.advanceStep(record, round, index, *step.Retained, deadline)
			if err != nil || capped {
				return capped, err
			}
			return runner.attributeProof(record, round, index, base, deadline)
		}
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
		started, startErr := runner.Manager.Start(StartSpec{ID: comparison.LaunchID, Kind: "proof", Goal: record.Goal, Tag: record.Unit, WorkingDirectory: command.Dir, Brief: brief, Round: round.Number, MaxRounds: record.MaxRounds})
		if started.ID != "" {
			cleanup = false
		}
		if startErr != nil && started.ID == "" {
			return false, startErr
		}
		baseline = started
	}
	baseline, terminal, err := runner.Manager.Wait(baseline.ID, max(time.Duration(0), deadline.Sub(runner.Manager.Now())))
	if err != nil {
		cleanup = false
		return false, err
	}
	if !terminal {
		cleanup = false
		return true, nil
	}
	cleanup = true
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
