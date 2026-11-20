package launch

import (
	"fmt"
	"time"
)

// FinishedBefore proves that the recorded execution has ended, including its
// entire process group. A terminal label alone does not release custody.
func (m *Manager) FinishedBefore(record Record, before time.Time) error {
	if !record.State.Terminal() {
		return fmt.Errorf("launch %s has not reached a terminal state", record.ID)
	}
	start, startErr := time.Parse(time.RFC3339Nano, record.StartedAt)
	end, endErr := time.Parse(time.RFC3339Nano, record.FinishedAt)
	if record.ID == "" || record.Kind == "" || record.Goal == "" {
		return fmt.Errorf("launch %s has incomplete execution identity", record.ID)
	}
	if startErr != nil || endErr != nil || end.Before(start) {
		return fmt.Errorf("launch %s has unreadable or regressed execution times", record.ID)
	}
	if record.State == Completed && (record.ExitCode == nil || *record.ExitCode != 0) {
		return fmt.Errorf("launch %s has no successful exit evidence", record.ID)
	}
	if !end.Before(before) {
		return fmt.Errorf("launch %s ended within the last steward interval", record.ID)
	}
	if record.ProcessGroup != nil && m.Processes == nil || !m.provenDead(record) {
		return fmt.Errorf("launch %s still has live or uncertain process custody", record.ID)
	}
	return nil
}

// BookEndedSteps writes only the execution facts. It neither reads the plan
// nor classifies the result, changes the worktree, retries or starts a step.
// Ownership is checked again under the same locks used by session collection.
func (runner *UnitRunner) BookEndedSteps(id string, before time.Time, owns func(UnitRunRecord, Record) bool) ([]EndBooking, error) {
	record, err := runner.read(id)
	if err != nil {
		return nil, err
	}
	if runner.tree == nil {
		return treeCall(runner, record.Worktree, func(bound *UnitRunner) ([]EndBooking, error) { return bound.BookEndedSteps(id, before, owns) })
	}
	held, err := runner.lock(id)
	if err != nil {
		return nil, err
	}
	defer releaseUnitLock(held)
	record, err = runner.read(id)
	if err != nil {
		return nil, err
	}
	if record.ID != id || record.State != "running" && record.State != UnitStepEnded || len(record.Rounds) == 0 {
		return nil, nil
	}
	round := &record.Rounds[len(record.Rounds)-1]
	booked := []EndBooking{}
	active := false
	for index := range round.Steps {
		step := &round.Steps[index]
		if step.State != StepRunning && step.State != StepStarting {
			continue
		}
		execution, err := runner.Manager.Store.Read(step.LaunchID)
		if err != nil {
			return nil, err
		}
		if owns == nil || !owns(record, execution) {
			return nil, fmt.Errorf("launch %s has no matching checkout ownership", step.LaunchID)
		}
		if execution.State == Starting || execution.State == Running {
			active = true
			continue
		}
		if err := runner.Manager.FinishedBefore(execution, before); err != nil {
			return nil, err
		}
		booking := EndBooking{By: "steward", At: runner.Manager.Now().UTC().Format(time.RFC3339Nano), Launch: execution.ID, State: execution.State}
		runner.driver(&record, round).endStep(index, execution)
		step.FinishedAt = execution.FinishedAt
		step.EndBooking = &booking
		booked = append(booked, booking)
	}
	if len(booked) == 0 {
		return nil, nil
	}
	for _, step := range round.Steps {
		active = active || step.State == StepRunning || step.State == StepStarting
	}
	if !active {
		record.State = UnitStepEnded
	}
	if err := runner.save(record); err != nil {
		return nil, err
	}
	return booked, nil
}
