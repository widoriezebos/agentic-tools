package steward

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ReadRunnerExecution observes the recorded runner without substituting the
// current enrollment for the executable that process actually runs.
func ReadRunnerExecution(root string, prober identity.Prober) enginepin.Execution {
	var record RunnerRecord
	if err := readJSON(runnerRecordPath(root), &record); err != nil {
		return enginepin.Execution{Cause: err.Error()}
	}
	return enginepin.Read(root, identity.Ref{Pid: record.Pid, StartedAtSec: record.PidStartedAt, StartTicks: record.StartTicks, BootID: record.BootID}, prober)
}
