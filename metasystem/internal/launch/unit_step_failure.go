package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

// Baseline comparison and flake attribution can replace unclassified only
// with demonstrated evidence. A command exit by itself proves no own defect.
func failedStepCause(record Record) string {
	if (loopstop.Cause{Kind: record.Cause}).Valid() {
		return record.Cause
	}
	if record.Cause != "" || record.State == Failed && record.ExitCode == nil {
		return "environment"
	}
	return "unclassified"
}

// Retained command and input bytes survive collection and a step retry.
func (driver stepDriver) retainStep(index int, spec StartSpec) (StartSpec, error) {
	step := &driver.round.Steps[index]
	if step.Retained != nil {
		return *step.Retained, nil
	}
	directory := filepath.Join(driver.round.Directory, fmt.Sprintf("step-%d-inputs", index+1))
	if err := os.MkdirAll(directory, 0700); err != nil {
		return spec, err
	}
	spec.Inputs = append([]string(nil), spec.Inputs...)
	paths := []*string{&spec.Brief, &spec.Page, &spec.UnitsPage, &spec.DiffFile}
	for i := range spec.Inputs {
		paths = append(paths, &spec.Inputs[i])
	}
	for i, path := range paths {
		if *path == "" {
			continue
		}
		data, err := os.ReadFile(*path)
		if err != nil {
			return spec, err
		}
		target := filepath.Join(directory, fmt.Sprintf("%d-%s", i, filepath.Base(*path)))
		if _, err := atomicfile.WriteFile(target, data, 0600, ""); err != nil {
			return spec, err
		}
		*path = target
	}
	step.Retained = &spec
	return spec, driver.save()
}

func (driver stepDriver) retryFailedStep(index int, deadline time.Time) (bool, error) {
	step := &driver.round.Steps[index]
	if step.Before != nil {
		after, err := driver.snapshot(step.Retained.WorkingDirectory)
		if err != nil {
			step.State, step.Cause, step.Reason = StepFailed, "unclassified", err.Error()
		} else if step.Moved = step.Before.changed(after); len(step.Moved) > 0 {
			step.State, step.Cause, step.Reason = StepFailed, "environment", "the tree moved during the step"
		}
		step.Before = nil
		if err := driver.save(); err != nil {
			return false, err
		}
	}
	if step.State != StepFailed || step.Cause != "environment" || len(step.LaunchIDs) >= 2 || step.Deadline {
		return false, nil
	}
	if driver.mayStart != nil {
		if err := driver.mayStart(index); err != nil {
			return false, err
		}
	}
	step.Moved = nil
	step.State, step.Reason, step.Cause, step.FinishedAt = StepPending, "", "", ""
	if err := driver.save(); err != nil {
		return false, err
	}
	return driver.advanceStep(index, *step.Retained, deadline)
}

func (runner *UnitRunner) holdFailedStep(record *UnitRunRecord, round *UnitRound) error {
	round.Material = -1
	round.Stop = &loopstop.Stop{Loop: "unit-build", Subject: record.Goal + "/" + record.Unit + "/" + record.ID,
		Attempt: round.Number, Decision: "stop", Handoff: "stopped " + round.Cause,
		Class: "the failed step needs a person's decision", Cause: &loopstop.Cause{Kind: round.Cause, Goal: record.Goal},
		Evidence: filepath.Join(round.Directory, "read-decision.json"), At: runner.Manager.Now().UTC().Format(time.RFC3339Nano)}
	for _, step := range round.Steps {
		if step.State == StepFailed {
			round.Stop.Class = step.Name + ": " + step.Reason
			round.Stop.Cause.Evidence = step.LaunchID
			break
		}
	}
	return runner.saveDecision(record, round)
}

// RetryFailedStep records a person's admission and resumes the retained step.
// The completed steps and the autonomous correction allowance are preserved.
func (runner *UnitRunner) RetryFailedStep(id, person, reason string, recordImpact func() error) (UnitResult, error) {
	record, err := runner.read(id)
	if err != nil {
		return UnitResult{}, err
	}
	if runner.tree == nil {
		return treeCall(runner, record.Worktree, func(r *UnitRunner) (UnitResult, error) { return r.RetryFailedStep(id, person, reason, recordImpact) })
	}
	held, err := runner.lock(id)
	if err != nil {
		return UnitResult{}, err
	}
	defer releaseUnitLock(held)
	record, err = runner.read(id)
	if err != nil {
		return UnitResult{}, err
	}
	result := UnitResult{Record: record, Round: len(record.Rounds)}
	if person == "" || reason == "" || recordImpact == nil || len(record.Rounds) == 0 {
		return result, fmt.Errorf("retrying a failed step needs a proven person and reason")
	}
	round := &record.Rounds[len(record.Rounds)-1]
	for index := range round.Steps {
		step := &round.Steps[index]
		// A replay answers the same hold: the step is no longer failed, or the
		// retry was recorded for its current launch. A new hold is retried again.
		if step.RetryBy == person && step.RetryReason == reason && (step.State != StepFailed || step.RetryLaunch == step.LaunchID) {
			return result, nil
		}
		if step.State != StepFailed {
			continue
		}
		if round.Stop == nil || round.Stop.Loop != "unit-build" || step.Retained == nil || (round.Cause != "environment" && round.Cause != "deadline") {
			return result, fmt.Errorf("this step needs a correction rather than an environment retry")
		}
		if err := recordImpact(); err != nil {
			return result, err
		}
		entry, _ := json.Marshal(map[string]any{"kind": "stop", "status": "cleared", "stop": round.Stop})
		if _, err := atomicfile.WriteFile(filepath.Join(round.Directory, "stop-register.json"), entry, 0600, runner.root()); err != nil {
			return result, err
		}
		// Publication consumes the result of the resumed execution; keep the
		// earlier result beside its physical launch for diagnosis.
		for _, name := range []string{"proof-before", "proof-after"} {
			if err := os.Rename(filepath.Join(round.Directory, name+".json"), filepath.Join(round.Directory, name+"-"+step.LaunchID+".json")); err != nil && !os.IsNotExist(err) {
				return result, err
			}
		}
		step.RetryBy, step.RetryReason, step.RetryLaunch = person, reason, step.LaunchID
		step.State, step.Reason, step.Cause, step.FinishedAt = StepPending, "", "", ""
		step.Before, step.Moved, step.Deadline = nil, nil, false
		for later := index + 1; later < len(round.Steps); later++ {
			if round.Steps[later].State == StepSkipped {
				round.Steps[later].State, round.Steps[later].Reason = StepPending, ""
			}
		}
		round.Stop, round.Outcome, round.Cause, record.State = nil, "", "", "running"
		err := runner.save(record)
		return UnitResult{Record: record, Round: round.Number}, err
	}
	return result, fmt.Errorf("this round has no failed step to retry")
}
