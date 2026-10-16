package supervise

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ReadExecutions observes the owner and both production components through
// their owning records. Missing records remain explicit UNKNOWN entries.
func ReadExecutions(root string, prober identity.Prober) map[string]enginepin.Execution {
	result := map[string]enginepin.Execution{}
	owner, err := ReadArmingOwner(root)
	if err != nil {
		result["owner"] = enginepin.Execution{Cause: err.Error()}
	} else {
		result["owner"] = enginepin.Read(root, identity.Ref{Pid: owner.Pid, StartedAtSec: owner.PidStartedAt, StartTicks: owner.PidStartTicks, BootID: owner.BootID}, prober)
	}
	held, err := recordedHeld(root)
	for _, component := range productionComponentSet {
		observation := enginepin.Execution{Cause: "no recorded " + string(component)}
		if err != nil {
			observation.Cause = err.Error()
		} else {
			for _, process := range held {
				if process.Component == component {
					observation = enginepin.Read(root, process.Identity, prober)
				}
			}
		}
		result[string(component)] = observation
	}
	return result
}
