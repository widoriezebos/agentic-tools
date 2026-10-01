package main

// The engine's owner functions called in-process under an explicit
// invocation context (internal/goal/ownercall): the handover, the next-step
// edit and the release, each building its synced-ledger request from the
// supplied identity and lineage instead of the process's parent and
// environment.

import (
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
	if ledger := invocation.Ledger; ledger != nil {
		// The landing lane's own goal writes go through its publication
		// boundary (lane runtime design r10, K3); a seat's never do.
		resolve := dependencies.endpoint
		dependencies.endpoint = func(root string) (goal.Endpoint, error) {
			endpoint, err := resolve(root)
			if err != nil {
				return endpoint, err
			}
			return ledger(endpoint), nil
		}
	}
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

// cleanOwnerRoot resolves a root as the former child's path flag did.
func cleanOwnerRoot(root string) string {
	if resolved, err := resolvePathFlag(root); err == nil {
		return resolved
	}
	return root
}
