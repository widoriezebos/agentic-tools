package main

// The owner functions public commands call in their own process
// (plans/designs/verbs-object-action.md 6.2). Each replaced a child of the
// engine; the edge supplies the current process as the caller identity,
// because that process is the parent the child classified, and the owner's
// output comes back on the caller's buffers exactly as the child's pipes
// carried it. Production uses defaultIntentOwnerCalls; tests give an
// invocation its own.

import (
	"bytes"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type intentOwnerCalls struct {
	// brain declares or withdraws the coordinator; its human gate
	// classifies caller.
	brain func(choice string, caller processIdentity, stdout, stderr io.Writer, root, by string) int
	// goalFetch is the read-side advance of the goal ledger.
	goalFetch func(stdout, stderr io.Writer, root string) int
	// goalRepair accepts fetched remote history; the brain's human-word
	// gate classifies caller.
	goalRepair func(caller processIdentity, stdout, stderr io.Writer, root, by string) int
	// configKeys enumerates configured keys.
	configKeys func(stdout io.Writer, conf, matching string) int
	// configValidate validates the configuration domain.
	configValidate func(stdout, stderr io.Writer, conf, repo string) int
	// delegate is the delegate boundary: dispatch, review, follow-up.
	delegate func(request delegateRequest, stdout, stderr io.Writer) int
}

func defaultIntentOwnerCalls() *intentOwnerCalls {
	return &intentOwnerCalls{
		brain: func(choice string, caller processIdentity, stdout, stderr io.Writer, root, by string) int {
			if choice == "withdraw" {
				return brainWithdraw(caller, stdout, stderr, cleanOwnerRoot(root), by, false)
			}
			return brainDeclare(caller, stdout, stderr, cleanOwnerRoot(root), by, false)
		},
		goalFetch: func(stdout, stderr io.Writer, root string) int {
			return goalFetchTo(stdout, stderr, cleanOwnerRoot(root), goal.ResolveEndpoint)
		},
		goalRepair: func(caller processIdentity, stdout, stderr io.Writer, root, by string) int {
			facts := defaultGoalAuthorityReadFacts()
			facts.caller = caller
			return goalRepairAcceptRemoteTo(stdout, stderr, cleanOwnerRoot(root), by, facts, goal.ResolveEndpoint)
		},
		configKeys: func(stdout io.Writer, conf, matching string) int {
			return configKeysTo(stdout, conf, matching, os.Environ())
		},
		configValidate: func(stdout, stderr io.Writer, conf, repo string) int {
			return configValidateTo(stdout, stderr, conf, cleanOwnerRoot(repo))
		},
		delegate: runDelegateWith,
	}
}

// ownerCalls returns the invocation's owner functions.
func (inv *intentInvocation) ownerCalls() *intentOwnerCalls {
	delivery := inv.delivery()
	if delivery.calls == nil {
		delivery.calls = defaultIntentOwnerCalls()
	}
	return delivery.calls
}

// ownerCall runs one owner function in this process on fresh buffers and
// returns what it wrote and its status, as a child's pipes and exit code
// were returned.
func ownerCall(run func(stdout, stderr io.Writer) int) intentProcessResult {
	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr)
	return intentProcessResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: code}
}
