package hostturn

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/host"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

// hostPreflight is the optional operation a runtime answers before the
// host turn's start gate; done ends the turn with code.
type hostPreflight interface {
	HostPreflight(t *supervisor.Turn) (code int, done bool)
}

// runHostOps is one host turn of a runtime through its operations (role
// host): the runtime prepares the command, the turn runs its CLI in the
// checkout, and the runtime's finalize names what host.FinishTurn judges.
func runHostOps(t *Turn, ops supervisor.Operations) int {
	d := t.d
	description, err := ops.Describe(d)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	schema, err := t.protocolFile("orchestrator.schema.json", protocol.RoleSchema, "orchestrator")
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	requested, err := t.protocolFile("workspace-permissions.json", protocol.Permissions, "workspace")
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	turn := supervisor.NewHostTurn(d, supervisor.HostTurnFacts{
		Runtime: t.runtime, TurnDir: t.TurnDir, Mission: t.Mission, TurnID: t.TurnID,
		Prompt: t.Prompt, Schema: schema, ResumeSession: t.ResumeSession, Tag: t.InstanceTag,
		Requested: requested, Result: t.Result,
	})
	// A runtime may end the turn before the start gate (the fake host's
	// turn-record check and unverified start).
	if preflight, ok := ops.(hostPreflight); ok {
		if code, done := preflight.HostPreflight(turn); done {
			return code
		}
	}
	if !t.requireCLI(description.CLI) {
		return 3
	}
	launch, err := ops.Prepare(turn)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	for _, refusal := range []*supervisor.Refusal{launch.EarlyRefusal, launch.Refusal} {
		if refusal != nil {
			fmt.Fprintln(d.Stderr, refusal.Error)
			return 3
		}
	}
	var status int
	if launch.Simulated != nil {
		// An in-process CLI stand-in (the fake host) runs to its exit.
		status = launch.Simulated(make(chan struct{}))
	} else if launch.Protocol != nil {
		var exit *int
		if status, exit = supervisor.RunHostProtocol(d, turn, launch); exit != nil {
			return *exit
		}
	} else {
		status = supervisor.RunHostCLI(d, turn, launch, t.Path("host.log"))
	}
	final, err := ops.Finalize(turn, supervisor.FinalInput{Launch: launch, Status: status, Usage: t.Path("usage.json")})
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if final.HostExit != nil {
		return *final.HostExit
	}
	usage := final.Usage
	if usage == "" {
		usage = t.Path("usage.json")
	}
	code, err := host.FinishTurnTransport(t.Result, final.HostSession, usage, final.HostRaw, final.HostReturn,
		final.HostAccepted, int64(status), final.HostRequireReply, final.HostTransport)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
	}
	if final.AfterFinish != nil {
		final.AfterFinish(code)
	}
	return code
}
