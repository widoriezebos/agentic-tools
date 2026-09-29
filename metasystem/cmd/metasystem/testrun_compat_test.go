package main

import (
	"context"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// The names below are how the goal-landing and batch tests C8a part 2
// moves still reach the testing selection in internal/testrun, with the
// unexported fields they were written against. Part 2 ports those tests to
// the package and deletes this file.

// retainedTestingVerification is testrun.Verification as those tests spell
// it; the worker policy is the proof-run limits', as in production.
type retainedTestingVerification struct {
	clock         func() time.Time
	revalidate    func(context.Context, proofrun.TestRunRequest, []proofrun.Attempt) (map[string]string, error)
	workspace     gittree.Workspace
	candidateIO   candidateEngineIO
	openCandidate func(string, string) (proofrun.CandidateWorkspace, error)
	scratch       *proofrun.ScratchRun
}

func verifyRetainedTestingPrepared(request testrun.SelectionRequest, prepared testrun.Preparation, dependencies retainedTestingVerification) (proofrun.TestResult, error) {
	return testrun.VerifyPrepared(request, prepared, testrun.Verification{Clock: dependencies.clock, Revalidate: dependencies.revalidate,
		Workspace: dependencies.workspace, CandidateIO: dependencies.candidateIO.engine(), OpenCandidate: dependencies.openCandidate,
		Scratch: dependencies.scratch, WorkerPolicy: testingWorkerPolicy})
}

var (
	bindTestingFreshnessProjectionWithWorkspace = testrun.BindFreshnessProjectionWithWorkspace
	testingFreshnessBinding                     = testrun.FreshnessBinding
	planWithTrustedPolicyEngine                 = testrun.PlanWithTrustedPolicyEngine
	trustedPolicyFloorRequest                   = testrun.TrustedPolicyFloorRequest
)
