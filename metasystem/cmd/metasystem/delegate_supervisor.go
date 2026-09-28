package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner/hostturn"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// runDelegateSupervisor is the delegate-supervisor process entrypoint
// (design verbs-object-action 3.2): `delegate-supervisor RUNTIME VERB --root
// ROOT [flags]`. The dispatch driver launches it as the persistent owner of
// one delegate round (dispatch, follow-up) through completion and result
// publication; the mission runner launches it for one host turn
// (start-turn). Its other verbs are the runtime adapter's small reads
// (identity, probe, contract, output-stream, cancel, selftest) for the
// dispatcher that remains a script until its port.
//
// A fixture-owned launch may lead its arguments with the fixture owner and
// attempt words (METASYSTEM_FIXTURE_OWNER=..., METASYSTEM_FIXTURE_ATTEMPT=...)
// so a survivor scan finds the process by its argv; they must equal the
// environment the process inherited.
func runDelegateSupervisor(args []string) int {
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return refuseUnknownOption(nil, runtimes.SupervisorEntry, args[0], "it takes RUNTIME VERB --root ROOT [flags]")
	}
	for len(args) > 0 {
		name, value, found := strings.Cut(args[0], "=")
		if !found || (name != identity.FixtureOwnerEnv && name != identity.FixtureAttemptEnv) {
			break
		}
		if os.Getenv(name) != value {
			fmt.Fprintf(os.Stderr, "metasystem %s: the %s argv carrier does not match the environment\n", runtimes.SupervisorEntry, name)
			return 2
		}
		args = args[1:]
	}
	if len(args) >= 2 && args[1] == runtimes.SupervisorHostTurn {
		return hostturn.Main(args, supervisor.ProcessDeps)
	}
	return supervisor.Main(args, supervisor.ProcessDeps)
}
