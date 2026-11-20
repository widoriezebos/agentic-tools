package launch

import (
	"os"
	"slices"
)

// Collection joins every terminal execution by its physical launch id. The
// step's tree observation may classify an otherwise successful exit as environment.
func (runner *UnitRunner) collectLaunches(record UnitRunRecord, round UnitRound) error {
	if runner.CollectLaunch == nil {
		return nil
	}
	for _, step := range round.Steps {
		ids := append([]string(nil), step.LaunchIDs...)
		if record.State == "cancelled" && step.State == StepStarting && step.LaunchID != "" && !slices.Contains(ids, step.LaunchID) {
			ids = append(ids, step.LaunchID)
		}
		if step.Unadmitted != "" && step.State == StepFailed {
			ids = append(ids, step.Unadmitted)
		}
		if step.Comparison != nil {
			ids = append(ids, step.Comparison.LaunchID)
		}
		for _, id := range ids {
			execution, err := runner.Manager.Store.Read(id)
			if os.IsNotExist(err) && record.State == "cancelled" && (id == step.LaunchID && step.State == StepStarting || step.Comparison != nil && id == step.Comparison.LaunchID && step.Comparison.FinishedAt == "") {
				execution, err = runner.neverCreated(id), nil
			}
			if os.IsNotExist(err) && step.Comparison != nil && id == step.Comparison.LaunchID {
				if step.Comparison.FinishedAt == "" {
					continue
				}
				execution = Record{ID: id, State: Failed, StartedAt: step.Comparison.StartedAt, FinishedAt: step.Comparison.FinishedAt}
				err = nil
			}
			if os.IsNotExist(err) && id == step.LaunchID && step.State == StepStarting {
				continue
			}
			if os.IsNotExist(err) && (id == step.LaunchID || id == step.Unadmitted) && step.State == StepFailed && step.FinishedAt != "" {
				execution = Record{ID: id, State: Failed, StartedAt: step.StartedAt, FinishedAt: step.FinishedAt}
				err = nil
			}
			if err != nil {
				return err
			}
			if !execution.State.Terminal() {
				continue
			}
			cause := failedStepCause(execution)
			if (id == step.LaunchID || id == step.Unadmitted) && step.Cause != "" {
				cause = step.Cause
			}
			if err := runner.CollectLaunch(record, execution, cause); err != nil {
				return err
			}
		}
	}
	for _, subject := range record.Subjects {
		if subject.Round != round.Number {
			continue
		}
		for _, id := range subject.GateLaunches {
			execution, err := runner.Manager.Store.Read(id)
			// Publication retains its id before Start; a failed start may never
			// create a launch. There is no execution to collect in that case.
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			// Unstamped publication history has no proven booking owner.
			if readString(execution.AdapterData, "unitRun") != record.ID || readString(execution.AdapterData, "unitInstallation") != runner.Installation || runner.Installation == "" {
				continue
			}
			if execution.State.Terminal() {
				if err := runner.CollectLaunch(record, execution, failedStepCause(execution)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
