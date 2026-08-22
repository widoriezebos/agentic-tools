package lease

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The direct-invoker check: the classifier's
// ancestry walk checks the exact caller node for announcements but
// starts SIGNATURE checks at its parent — deliberate for ordinary
// verbs (the caller is a shell), but it means an agent binary spawning
// a verb DIRECTLY is judged by its ancestors, not by what it is. Verbs
// whose refusal must cover "an agent invoked me itself" (the
// acknowledge verb) call these for the caller node, and every unprovable
// state is an ERROR the caller refuses on — silence suppression is
// optional, so uncertainty must refuse rather than manufacture
// trustworthy silence.

// AgentArgv reports which INSTALLED adapter's delegate signature the
// argv matches — every adapter on disk, not only the configured
// runtimes: adoption retains all adapters while configuring one, and
// an unconfigured runtime's binary is still an agent (a
// configured-only universe would let an unconfigured invoker
// classify HUMAN and evade).
func AgentArgv(root, argv string) (string, bool, error) {
	signatures, err := allAdapterSignatures(root)
	if err != nil {
		return "", false, fmt.Errorf("adapter signatures unreadable: %w", err)
	}
	if len(signatures) == 0 {
		// A metasystem root always ships adapters; an empty universe
		// means the root is wrong, and a refusal gate must not read
		// "no signatures found" as "provably not an agent".
		return "", false, fmt.Errorf("no adapter signatures under %s; cannot judge the invoker", root)
	}
	runtime := census.Runtime(argv, signatures)
	return runtime, runtime != "", nil
}

// DirectAgentInvoker probes pid's live argv with the kernel prober and
// answers AgentArgv for it.
func DirectAgentInvoker(root string, pid int64) (string, bool, error) {
	return directAgentInvoker(root, pid, identity.KernelProber{})
}

// directAgentInvoker is the prober-injected body: an unreadable
// process, an unproven liveness, or an UNKNOWN ARGV all error — the
// identity contract says ArgvKnown=false is absence of evidence, and
// joining an unknown argv into "" would read as provably-not-an-agent
// (a fail-open).
func directAgentInvoker(root string, pid int64, prober identity.Prober) (string, bool, error) {
	exact, state, err := prober.Probe(pid)
	if err != nil || state != identity.Alive {
		return "", false, fmt.Errorf("direct invoker %d is not provably readable", pid)
	}
	if !exact.ArgvKnown {
		return "", false, fmt.Errorf("direct invoker %d's argv is unreadable; refusing rather than assuming a non-agent", pid)
	}
	return AgentArgv(root, strings.Join(exact.Argv, " "))
}
