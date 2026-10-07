package launch

import "os"

// Collection joins every terminal execution by its physical launch id. The
// step's tree observation may classify an otherwise successful exit as environment.
func (runner *UnitRunner) collectLaunches(record UnitRunRecord, round UnitRound) error {
	if runner.CollectLaunch == nil {
		return nil
	}
	for _, step := range round.Steps {
		ids := append([]string(nil), step.LaunchIDs...)
		if step.Comparison != nil {
			ids = append(ids, step.Comparison.LaunchID)
		}
		for _, id := range ids {
			execution, err := runner.Manager.Store.Read(id)
			if os.IsNotExist(err) && id == step.LaunchID && step.State == StepStarting {
				continue
			}
			if os.IsNotExist(err) && id == step.LaunchID && step.State == StepFailed && step.FinishedAt != "" {
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
			if id == step.LaunchID && step.Cause != "" {
				cause = step.Cause
			}
			if err := runner.CollectLaunch(record, execution, cause); err != nil {
				return err
			}
		}
	}
	return nil
}
