package main

import (
	"context"
	"os/exec"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
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

// forecastTestingDependencies is testrun.Forecasting as the batch-cost tests
// spell it; the worker policy is the proof-run limits', as in production.
type forecastTestingDependencies struct {
	workspace     gittree.Workspace
	candidateIO   candidateEngineIO
	openCandidate func(string, string) (proofrun.CandidateWorkspace, error)
	now           func() time.Time
	scratch       *proofrun.ScratchRun
}

func forecastTestingSelectionPrepared(selection testrun.CostSelection, proofCapMinutes uint64, request testrun.SelectionRequest,
	prepared testrun.Preparation, dependencies forecastTestingDependencies) (testrun.CostEvidence, error) {
	return testrun.ForecastPrepared(selection, proofCapMinutes, request, prepared, testrun.Forecasting{Workspace: dependencies.workspace,
		CandidateIO: dependencies.candidateIO.engine(), OpenCandidate: dependencies.openCandidate, Now: dependencies.now,
		Scratch: dependencies.scratch, WorkerPolicy: testingWorkerPolicy})
}

// candidateEngineIO is how the goal-landing and batch-cost tests still
// spell the candidate engine's Git, worktree and build seams, with
// unexported fields; production passes candidateengine.Native(). C8a part 2
// ports those tests to candidateengine.IO and deletes this type.
type candidateEngineIO struct {
	runGit    func(*exec.Cmd) error
	open      func(gittree.Workspace, string) (candidateengine.DetachedWorkspace, error)
	buildArgv func(output string) []string
}

// engine is the seams as the candidate engine owner takes them.
func (seams candidateEngineIO) engine() candidateengine.IO {
	return candidateengine.IO{RunGit: seams.runGit, Open: seams.open, BuildArgv: seams.buildArgv}
}

func candidateEngineBuildIdentityUsing(ctx context.Context, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, seams candidateEngineIO) (string, error) {
	return candidateengine.BuildIdentityUsing(ctx, workspace, installationPrefix, candidateTree, environment, seams.engine())
}
