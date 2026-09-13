package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// JobWatch is the delegate-job waiter:
// it blocks until the job record is
// terminal and exits with the pinned code — completed=0, failed=1,
// missing or unparsable=4, with the waiter layer's operational codes for
// registration failures. The waiter record it holds is what the turn
// verdict's unwatched rule reads; it is removed on every exit path.
func JobWatch(root, jobId string, caller run.Caller, poll time.Duration) int {
	record, ok := readJobStatus(root, jobId)
	if !ok {
		return run.ExitNoRecord
	}
	store := &run.Store{Root: root}
	target := run.WaiterTarget{StartedAt: record.StartedAt}
	if err := store.RegisterWaiter("job", jobId, caller, target); err != nil {
		return run.WaiterExitCode(err)
	}
	defer store.RemoveWaiter("job", jobId, caller)
	if poll <= 0 {
		poll = 2 * time.Second
	}
	for {
		record, ok := readJobStatus(root, jobId)
		if !ok {
			return run.ExitNoRecord
		}
		switch record.Status {
		case "completed":
			return 0
		case "failed":
			return 1
		case "timeout":
			return 2
		case "cancelled":
			return 3
		case "pending-setup", "pending", "running":
			// still in flight
		default:
			return run.ExitNoRecord // an unknown status is a malformed record
		}
		time.Sleep(poll)
	}
}

type jobStatus struct {
	JobID       string `json:"jobId"`
	OperationID string `json:"operationId"`
	Round       int64  `json:"round"`
	Status      string `json:"status"`
	StartedAt   string `json:"startedAt"`
	EndedAt     string `json:"endedAt"`
}

func readJobStatus(root, jobId string) (jobStatus, bool) {
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", jobId+".json"))
	if err != nil {
		return jobStatus{}, false
	}
	var record jobStatus
	if json.Unmarshal(data, &record) != nil {
		return jobStatus{}, false
	}
	return record, true
}

// ObserveJob reads the job through its record owner and translates only a
// durable job status. The reservation operation identifier exists during
// setup; the waiter adopts round and start only after running makes them an
// immutable incarnation, so identifier reuse cannot satisfy an older wait.
func ObserveJob(ctx context.Context, root, jobID string, pinned run.WaiterTarget, _ string) (run.SourceObservation, error) {
	select {
	case <-ctx.Done():
		return run.SourceObservation{}, ctx.Err()
	default:
	}
	record, ok := readJobStatus(root, jobID)
	if !ok {
		return run.SourceObservation{ExitCode: run.ExitNoRecord, Reason: "the job record is missing or invalid", Outcome: "invalid-source"}, nil
	}
	if record.JobID != "" && record.JobID != jobID {
		return run.SourceObservation{ExitCode: run.ExitNoRecord, Reason: "the job record identifies another job", Outcome: "invalid-source"}, nil
	}
	if record.OperationID == "" {
		return run.SourceObservation{ExitCode: run.ExitNoRecord, Reason: "the job record lacks its reservation operation identifier", Outcome: "invalid-source"}, nil
	}
	if record.Status == "pending-setup" {
		incarnation := run.WaiterTarget{OperationID: record.OperationID}
		return run.SourceObservation{Pending: true, Incarnation: incarnation, Outcome: record.Status, Evidence: "job:" + jobID + ":" + record.OperationID + ":pending-setup"}, nil
	}
	preRunning := pinned.OperationID == record.OperationID && pinned.Round == 0 && pinned.StartedAt == "" && record.Status != "running"
	if !preRunning && (record.Round < 1 || record.StartedAt == "") {
		return run.SourceObservation{ExitCode: run.ExitNoRecord, Reason: "the job record lacks its round or start stamp", Outcome: "invalid-source"}, nil
	}
	incarnation := run.WaiterTarget{OperationID: record.OperationID, Round: record.Round, StartedAt: record.StartedAt}
	if preRunning {
		incarnation = pinned
	}
	evidence := fmt.Sprintf("job:%s:%s:r%d:%s", jobID, record.OperationID, record.Round, record.StartedAt)
	observation := run.SourceObservation{Pending: true, Incarnation: incarnation, Outcome: record.Status, Evidence: evidence}
	switch record.Status {
	case "pending", "running":
		return observation, nil
	case "completed":
		observation.Pending, observation.ExitCode, observation.Reason = false, run.ExitGreen, "job completed"
	case "failed":
		observation.Pending, observation.ExitCode, observation.Reason = false, run.ExitRed, "job failed"
	case "timeout":
		observation.Pending, observation.ExitCode, observation.Reason = false, run.ExitEndedUnknown, "job reached its target timeout"
	case "cancelled":
		observation.Pending, observation.ExitCode, observation.Reason = false, run.ExitLaunchFailed, "job was cancelled"
	default:
		observation.Pending, observation.ExitCode, observation.Reason, observation.Outcome = false, run.ExitNoRecord, "the job record has an unknown status", "invalid-source"
	}
	observation.TerminalStamp = record.EndedAt
	return observation, nil
}
