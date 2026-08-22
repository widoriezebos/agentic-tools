package dispatch

import (
	"encoding/json"
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
	Status    string `json:"status"`
	StartedAt string `json:"startedAt"`
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
