package main

// Owner invocation context (plans/designs/verbs-object-action.md 6.2,
// VOA-02). Where a caller used to run an owner verb as a child of the engine,
// it now calls the owner function in its own process. A child classified its
// parent, proved human ancestry from it, and inherited its lineage through
// METASYSTEM_OWNER_LINEAGE; an owner function called in-process would read
// its own caller's parent and a process-global variable instead. So the owner
// function takes the invocation context explicitly: the process identity that
// classification and human proof start from, and the lineage the request
// carries. Nothing on these paths reads os.Getppid() or the lineage variable,
// and nothing sets process-global environment to imitate a child.
//
// The supplied identity is fixed per call edge (VOA-02-R2): an edge that
// replaced a child supplies the current process, because that is the parent
// the child observed; a process entry supplies its own caller, as it always
// did; the landing owner supplies itself.

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// processIdentity is one process as a classification starting point: its pid
// and, when known, its kernel start time (Unix seconds), which guards the pid
// against reuse between the moment it is supplied and the moment it is read.
type processIdentity struct {
	pid       int64
	startedAt int64
}

// currentProcessIdentity is the identity an edge that replaced a child
// supplies: this process, the parent the child used to observe.
func currentProcessIdentity() processIdentity {
	pid := int64(os.Getpid())
	supplied := processIdentity{pid: pid}
	if exact, state, err := (identity.KernelProber{}).Probe(pid); err == nil && state == identity.Alive {
		supplied.startedAt = exact.StartedAt.Unix()
	}
	return supplied
}

// entryCallerIdentity is the identity a process entry supplies: the process
// that started this one, read once at the entry's boundary.
func entryCallerIdentity() processIdentity {
	return processIdentity{pid: int64(os.Getppid())}
}

// classifiablePid returns the supplied pid, refusing when a recorded start
// time no longer matches the live process (the pid was reused).
func (p processIdentity) classifiablePid(prober identity.Prober) (int64, error) {
	if p.startedAt == 0 || prober == nil {
		return p.pid, nil
	}
	exact, state, err := prober.Probe(p.pid)
	if err != nil || state != identity.Alive || exact.StartedAt.Unix() != p.startedAt {
		return 0, fmt.Errorf("the supplied caller identity pid %d started at %d is no longer that process (state=%s): %v", p.pid, p.startedAt, state, err)
	}
	return p.pid, nil
}

// ownerInvocation is the explicit context of one owner call.
type ownerInvocation struct {
	// caller is the process identity classification and human proof start
	// from.
	caller processIdentity
	// lineage is the owner lineage the request carries; it replaces the
	// child's inherited METASYSTEM_OWNER_LINEAGE.
	lineage string
}

// ownerCallFromThisProcess is the context of an edge that replaced a child
// run with lineage: the current process is the supplied identity.
func ownerCallFromThisProcess(lineage string) ownerInvocation {
	return ownerInvocation{caller: currentProcessIdentity(), lineage: lineage}
}

// syncDependencies are the synced-ledger request dependencies of this
// invocation: the production readers, with the supplied identity and lineage
// in place of the process's parent and environment.
func (invocation ownerInvocation) syncDependencies() syncRequestDependencies {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts.caller = invocation.caller
	lineage := invocation.lineage
	dependencies.ownerLineage = func() string { return lineage }
	return dependencies
}

// syncRequest builds one synced-ledger mutation request that names no human:
// the caller is classified from the supplied identity, and the request
// carries the supplied lineage. stopping selects the stopping-act builder
// (release), which proves nothing further without a --by.
func (invocation ownerInvocation) syncRequest(verb, root string, stopping bool) (goal.VerbRequest, error) {
	dependencies := invocation.syncDependencies()
	if stopping {
		return syncStoppingReqWithProofWithDependencies(verb, root, "", invocation.lineage, nil, goalCommandNow, dependencies)
	}
	return syncReqWithProofAtWithDependencies(verb, root, "", invocation.lineage, nil, goalCommandNow, dependencies)
}

// ownerPublishError is a published result as the error a former child's
// nonzero exit became: the refusal, or the unconfirmed outcome it printed.
func ownerPublishError(verb string, res goal.PublishResult, err error) error {
	if err != nil {
		return fmt.Errorf("goal %s: %w", verb, err)
	}
	if res.Outcome != goal.OutcomeConfirmed {
		return fmt.Errorf("goal %s: outcome=%s tip=%s detail=%s", verb, res.Outcome, res.Tip, res.Detail)
	}
	return nil
}

// goalHandoverOwner transfers a claim under the invocation's context.
func goalHandoverOwner(invocation ownerInvocation, request goalHandoverRequest) error {
	request.Root = cleanOwnerRoot(request.Root)
	if problem := request.usage(); problem != "" {
		return fmt.Errorf("%s", problem)
	}
	res, err := goalHandoverEffect(invocation, request)
	return ownerPublishError("handover", res, err)
}

// goalEditNextOwner rewrites one goal's next step under the invocation's
// context, as `goal edit --id ID --next TEXT --lineage L` did.
func goalEditNextOwner(invocation ownerInvocation, root, goalID, next string) error {
	root = cleanOwnerRoot(root)
	if goalID == "" {
		return fmt.Errorf("goal edit needs --id")
	}
	if !converted(root) {
		return fmt.Errorf("goal edit: %w", errLegacyLedger)
	}
	req, err := invocation.syncRequest("edit", root, false)
	if err != nil {
		return fmt.Errorf("goal edit: %w", err)
	}
	res, err := goalEditEffect(req, &syncFlags{root: root, id: goalID, next: next, lineage: invocation.lineage}, goalCommandNow)
	return ownerPublishError("edit", res, err)
}

// goalReleaseOwner releases this machine's claim on one goal under the
// invocation's context, as `goal release --id ID --lineage L` did.
func goalReleaseOwner(invocation ownerInvocation, root, goalID string) error {
	root = cleanOwnerRoot(root)
	if goalID == "" {
		return fmt.Errorf("goal release needs --id")
	}
	if !converted(root) {
		return fmt.Errorf("goal release: %w", errLegacyLedger)
	}
	req, err := invocation.syncRequest("release", root, true)
	if err != nil {
		return fmt.Errorf("goal release: %w", err)
	}
	res, err := goal.Release(req, goalID)
	return ownerPublishError("release", res, err)
}

// cleanOwnerRoot resolves a root as the former child's path flag did.
func cleanOwnerRoot(root string) string {
	if resolved, err := resolvePathFlag(root); err == nil {
		return resolved
	}
	return root
}
