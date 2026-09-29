package main

import (
	"errors"
	"fmt"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// forecastTestingSelection uses the same protected selection and retained
// evaluator as test verify. It creates no proof attempt or native build/test.
// Existing toolchain metadata reads may run bounded version/environment tools.
func forecastTestingSelection(root string, selection testrun.CostSelection, proofCapMinutes uint64) (_ testrun.CostEvidence, err error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(selection.Tree)
	if err != nil {
		return testrun.CostEvidence{}, err
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	executionRoot := batch.ModuleRoot(detached.Workspace().Dir)
	request := testrun.SelectionRequest{Root: executionRoot, ControlRoot: batch.ModuleRoot(root), GoalID: selection.GoalID,
		Tree: selection.Tree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		BatchPrefixReceipt: !selection.Admission, BatchAdmission: selection.Admission,
		BatchRequirements: slices.Clone(selection.Requirements), FreshEpisode: selection.FreshEpisode,
		FreshExpiresAt: selection.FreshExpiresAt}
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		return testrun.CostEvidence{}, err
	}
	commandClock, _, err := goalCommandClock(prepared.ProofControlRoot())
	if err != nil {
		return testrun.CostEvidence{}, err
	}
	// Revalidation materializes the candidate and a managed environment like
	// test verify does, so it owns a fresh scratch run for exactly that long.
	scratch, err := proofrun.CreateScratchRun(prepared.ProofControlRoot())
	if err != nil {
		return testrun.CostEvidence{}, fmt.Errorf("scratch root: %w", err)
	}
	evidence, forecastErr := testrun.ForecastPrepared(selection, proofCapMinutes, request, prepared, testrun.Forecasting{
		Workspace: gittree.Workspace{Dir: prepared.ProjectRoot}, CandidateIO: candidateengine.Native(), Now: commandClock,
		Scratch: scratch, WorkerPolicy: testingWorkerPolicy,
	})
	if cleanupErr := scratch.Cleanup(nil); cleanupErr != nil {
		return testrun.CostEvidence{}, errors.Join(forecastErr, cleanupErr)
	}
	return evidence, forecastErr
}
