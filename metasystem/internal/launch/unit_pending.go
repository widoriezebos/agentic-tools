package launch

import (
	"errors"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// UnitOperation retains request identity and actor provenance, never authority.
type UnitOperation struct {
	CallerDirectory string   `json:"callerDirectory"`
	Argv            []string `json:"argv"`
	InputDigest     string   `json:"inputDigest"`
	Actor           string   `json:"actor"`
	ClaimSubject    string   `json:"claimSubject"`
}

type LaunchWait struct {
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt,omitempty"`
	EndReason string `json:"endReason,omitempty"`
}

type UnitPendingAct struct {
	Operation    string         `json:"operation"`
	Request      *UnitOperation `json:"request,omitempty"`
	Goal         string         `json:"goal"`
	Unit         string         `json:"unit"`
	Worktree     string         `json:"worktree"`
	Base         string         `json:"base"`
	Registration lane.Record    `json:"registration"`
	Code         string         `json:"code"`
	Reason       string         `json:"reason"`
	Waits        []LaunchWait   `json:"waits"`
}

// CapacityRefusal carries the admission owner's observation, not a permit.
type CapacityRefusal struct {
	*CodedError
	Registration lane.Record
}

func (e *CapacityRefusal) Unwrap() error { return e.CodedError }

func capacityWaiting(err error) bool {
	return IsCode(err, "LAUNCH_BUILD_CAPACITY") || IsCode(err, "LAUNCH_BUILD_PERSON")
}

func (driver stepDriver) retainCapacityWait(index int, err error) error {
	step := &driver.round.Steps[index]
	if step.PendingAct == nil {
		act := &UnitPendingAct{Operation: step.LaunchID, Goal: step.Retained.Goal, Unit: step.Retained.Tag,
			Worktree: step.Retained.WorkingDirectory}
		if driver.record != nil {
			act.Request, act.Base = driver.record.Operation, driver.record.Base
		}
		step.PendingAct = act
	}
	act := step.PendingAct
	act.Code, act.Reason = ErrorCode(err), err.Error()
	var refusal *CapacityRefusal
	if errors.As(err, &refusal) {
		act.Registration = refusal.Registration
	}
	if len(act.Waits) == 0 || act.Waits[len(act.Waits)-1].EndedAt != "" {
		act.Waits = append(act.Waits, LaunchWait{StartedAt: driver.manager.Now().UTC().Format(time.RFC3339Nano)})
	}
	return driver.save()
}

func (driver stepDriver) closeCapacityWait(index int, reason string, at string) {
	act := driver.round.Steps[index].PendingAct
	if act != nil && len(act.Waits) > 0 && act.Waits[len(act.Waits)-1].EndedAt == "" {
		wait := &act.Waits[len(act.Waits)-1]
		wait.EndedAt, wait.EndReason = at, reason
	}
}

// PendingWork reports only the current round's unfinished capacity operation.
func PendingWork(record UnitRunRecord) *UnitPendingAct {
	if record.State != "running" || len(record.Rounds) == 0 {
		return nil
	}
	for _, step := range record.Rounds[len(record.Rounds)-1].Steps {
		if step.State == StepStarting && step.PendingAct != nil && len(step.PendingAct.Waits) > 0 && step.PendingAct.Waits[len(step.PendingAct.Waits)-1].EndedAt == "" {
			return step.PendingAct
		}
	}
	return nil
}

// QueueDuration uses retained intervals once; absent or unreadable timing is unknown.
func (act *UnitPendingAct) QueueDuration(now time.Time) (time.Duration, bool) {
	if act == nil || len(act.Waits) == 0 {
		return 0, false
	}
	var duration time.Duration
	for _, wait := range act.Waits {
		start, err := time.Parse(time.RFC3339Nano, wait.StartedAt)
		if err != nil {
			return 0, false
		}
		end := now
		if wait.EndedAt != "" {
			end, err = time.Parse(time.RFC3339Nano, wait.EndedAt)
		}
		if err != nil || end.Before(start) {
			return 0, false
		}
		duration += end.Sub(start)
	}
	return duration, true
}
