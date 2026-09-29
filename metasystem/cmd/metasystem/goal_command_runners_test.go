package main

import (
	"io"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The goal command runners the tests drive the live owners through: each
// composes the production request builders and effects exactly as the retired
// goal command lines did, so the tests keep asserting on the owners' words and
// ledger effects while production reaches the same owners through the intent
// verbs.

func runGoalReadItemsCloseWithInputs(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, resolveCodeCommit func(root, ref string) (string, error)) int {
	return runGoalReadItemsCloseWithProof(args, nil, commandNow, dependencies, resolveCodeCommit)
}

func runGoalDischargeReviewObligationWithOwners(args []string, requestBuilder func(verb, root, by, lineage string) (goal.VerbRequest, error), discharge func(goal.VerbRequest, string, string, string, string, string, ...goal.DischargeEvidence) (goal.PublishResult, error), stdout, stderr io.Writer) int {
	dependencies := defaultSyncRequestDependencies()
	dependencies.stdout, dependencies.stderr = stdout, stderr
	return runGoalDischargeReviewObligationWithDependencies(args, requestBuilder, discharge, dependencies)
}

func runGoalResumeWithAuthorityFacts(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), facts goalAuthorityReadFacts, stdout, stderr io.Writer) int {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts = facts
	dependencies.stdout, dependencies.stderr = stdout, stderr
	return runGoalResumeWithInputs(args, prove, commandNow, dependencies, dispatchcore.ResolveGoalBinding)
}

func runGoalSetObligationWithAuthorityFacts(args []string, prove goalAuthorityProver, facts goalAuthorityReadFacts, stdout, stderr io.Writer) int {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts = facts
	dependencies.stdout, dependencies.stderr = stdout, stderr
	return runGoalSetObligationWithAuthorityFactsAtWithDependencies(args, prove, goalCommandNow, dependencies)
}

func runGoalEditWithDependencies(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	requestBuilder := func(verb, root, by, lineage string) (goal.VerbRequest, error) {
		return syncReqWithProofAtWithDependencies(verb, root, by, lineage, nil, commandNow, dependencies)
	}
	run := runSyncOnlyWithDependencies("edit", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goalEditEffect(req, f, commandNow)
	}, requestBuilder, dependencies, "id")
	return run(args)
}

// claimGoalOwner claims a goal, or its whole arc with --arc, for this machine.
func claimGoalOwner(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
	budget, err := f.budgetTuple(false)
	if err != nil {
		return goal.PublishResult{}, err
	}
	if f.arc != "" {
		if budget != nil {
			return goal.ClaimArc(req, f.id, *budget)
		}
		return goal.ClaimArc(req, f.id)
	}
	if budget != nil {
		return goal.Claim(req, f.id, *budget)
	}
	return goal.Claim(req, f.id)
}
