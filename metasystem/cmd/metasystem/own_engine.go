package main

import (
	"fmt"
	"os"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

type ownEngineOwners struct {
	executable func() (string, error)
	serving    func(string) (string, string)
	lookupEnv  func(string) (string, bool)
}

// checkOwnEngine keeps an agent's public verbs on the serving engine unless
// the caller deliberately selects this build. A person may run either build.
func (inv *intentInvocation) checkOwnEngine() *intentResult {
	owners := inv.owners.ownEngine
	if owners.executable == nil {
		owners.executable = os.Executable
	}
	if owners.serving == nil {
		owners.serving = delegationToolInstallation
	}
	if owners.lookupEnv == nil {
		owners.lookupEnv = os.LookupEnv
	}
	executable, err := owners.executable()
	if err != nil {
		return nil
	}
	installation := launchSeatInstallation(executable)
	serving, checkout, engine := dispatchcore.ResolveTool(installation, owners.serving, owners.lookupEnv)
	if serving == installation {
		return nil
	}
	if override, set := owners.lookupEnv("METASYSTEM_BIN"); set && override != "" && realpath.Resolve(override) == realpath.Resolve(executable) {
		return nil
	}
	// A worktree's agent session is announced at its serving checkout.
	class := inv.owners.agent.withDefaults().caller(inv, checkout)
	if class == lease.ClassMain || class == lease.ClassDelegate {
		next := inv.typedArgv()
		next[0] = engine
		return &intentResult{Outcome: intentRefused, code: 1,
			Summary: "this engine is the goal worktree's own build, so nothing was done",
			next:    next, nextReason: "run the same command with the serving engine",
			Details: []string{"setting METASYSTEM_BIN to " + shellCommand([]string{executable}) + " runs this build deliberately"}}
	}
	fmt.Fprintf(inv.stderr, "running build %s; dispatches use %s\n", shellCommand([]string{executable}), shellCommand([]string{engine}))
	return nil
}
