package steward

import (
	"errors"
	"io/fs"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// ReadRunnerExecution observes the recorded runner without substituting the
// current enrollment for the executable that process actually runs.
func ReadRunnerExecution(root string, prober identity.Prober) enginepin.Execution {
	var record RunnerRecord
	if err := readJSON(runnerRecordPath(root), &record); err != nil {
		return enginepin.Execution{Absent: errors.Is(err, fs.ErrNotExist), Cause: err.Error()}
	}
	return enginepin.Read(root, identity.Ref{Pid: record.Pid, StartedAtSec: record.PidStartedAt, StartTicks: record.StartTicks, BootID: record.BootID}, prober)
}

// ReadExecutions observes the four engine roles through their owning records.
func ReadExecutions(root string, prober identity.Prober) map[string]enginepin.Execution {
	roles := supervise.ReadExecutions(root, prober)
	roles["runner"] = ReadRunnerExecution(root, prober)
	return roles
}
