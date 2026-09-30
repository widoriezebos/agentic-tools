package main

// The engine's owner functions called in-process under an explicit
// invocation context (internal/goal/ownercall): the handover, the next-step
// edit and the release, each building its synced-ledger request from the
// supplied identity and lineage instead of the process's parent and
// environment.

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
)

// ownerSyncDependencies are the synced-ledger request dependencies of one
// invocation: the production readers, with the supplied identity and
// lineage in place of the process's parent and environment.
func ownerSyncDependencies(invocation ownercall.Invocation) syncRequestDependencies {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts.caller = invocation.Caller
	lineage := invocation.Lineage
	dependencies.ownerLineage = func() string { return lineage }
	return dependencies
}

// ownerSyncRequest builds one synced-ledger mutation request that names no
// human: the caller is classified from the supplied identity, and the
// request carries the supplied lineage. stopping selects the stopping-act
// builder (release), which proves nothing further without a --by.
func ownerSyncRequest(invocation ownercall.Invocation, verb, root string, stopping bool) (goal.VerbRequest, error) {
	dependencies := ownerSyncDependencies(invocation)
	var req goal.VerbRequest
	var err error
	if stopping {
		req, err = syncStoppingReqWithProofWithDependencies(verb, root, "", invocation.Lineage, nil, goalCommandNow, dependencies)
	} else {
		req, err = syncReqWithProofAtWithDependencies(verb, root, "", invocation.Lineage, nil, goalCommandNow, dependencies)
	}
	if err == nil && invocation.LaneEpoch > 0 {
		// The lane's stable claim identity acts at the custody epoch its
		// kernel read from the host record, never a session's lease epoch.
		req.ClaimEpoch, req.EpochAuthority = invocation.LaneEpoch, goal.EpochAuthorityLane
	}
	return req, err
}

// goalHandoverOwner transfers a claim under the invocation's context.
func goalHandoverOwner(invocation ownercall.Invocation, request ownercall.HandoverRequest) error {
	request.Root = cleanOwnerRoot(request.Root)
	if problem := request.Usage(); problem != "" {
		return fmt.Errorf("%s", problem)
	}
	res, err := goalHandoverEffect(invocation, request)
	return ownercall.PublishError("handover", res, err)
}

// goalEditNextOwner rewrites one goal's next step under the invocation's
// context, as `goal edit --id ID --next TEXT --lineage L` did.
func goalEditNextOwner(invocation ownercall.Invocation, root, goalID, next string) error {
	root = cleanOwnerRoot(root)
	if goalID == "" {
		return fmt.Errorf("goal edit needs --id")
	}
	if !converted(root) {
		return fmt.Errorf("goal edit: %w", errLegacyLedger)
	}
	req, err := ownerSyncRequest(invocation, "edit", root, false)
	if err != nil {
		return fmt.Errorf("goal edit: %w", err)
	}
	res, err := goalEditEffect(req, &syncFlags{root: root, id: goalID, next: next, lineage: invocation.Lineage}, goalCommandNow)
	return ownercall.PublishError("edit", res, err)
}

// goalReleaseOwner releases this machine's claim on one goal under the
// invocation's context, as `goal release --id ID --lineage L` did.
func goalReleaseOwner(invocation ownercall.Invocation, root, goalID string) error {
	root = cleanOwnerRoot(root)
	if goalID == "" {
		return fmt.Errorf("goal release needs --id")
	}
	if !converted(root) {
		return fmt.Errorf("goal release: %w", errLegacyLedger)
	}
	req, err := ownerSyncRequest(invocation, "release", root, true)
	if err != nil {
		return fmt.Errorf("goal release: %w", err)
	}
	res, err := goal.Release(req, goalID)
	return ownercall.PublishError("release", res, err)
}

// cleanOwnerRoot resolves a root as the former child's path flag did.
func cleanOwnerRoot(root string) string {
	if resolved, err := resolvePathFlag(root); err == nil {
		return resolved
	}
	return root
}
