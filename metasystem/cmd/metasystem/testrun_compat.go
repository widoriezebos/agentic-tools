package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// The names below are how the files C8a part 2 moves (landing_batch_*.go,
// landing_path.go, intent_delivery.go, goal_branch.go) still reach the
// testing selection that moved to internal/testrun. Part 2 ports those
// files to the package and deletes this file; nothing else uses these names.

type (
	testingPreparation      = testrun.Preparation
	testingPreparationState = testrun.PreparationState
	testingSelectionRequest = testrun.SelectionRequest
	testingPlanOutput       = testrun.PlanOutput
	upOutcome               = testrun.UpOutcome
	costSelection           = testrun.CostSelection
	costSelectionEvidence   = testrun.CostEvidence
)

var (
	batchRequirementsArgument   = testrun.BatchRequirementsArgument
	newTestingFreshEpisode      = testrun.NewFreshEpisode
	loadPhysicalTestingContract = testrun.LoadContract
	linkedWorktreeMainCheckout  = testrun.LinkedWorktreeMainCheckout
	landedRearmRebuild          = testrun.RebuildLandedEngine
	landedRearmUp               = testrun.UpLandedEngine
	readStrictJSON              = strictjson.Read
	costSelectionForPrefix      = testrun.PrefixCostSelection
	proofCostCap                = testrun.ProofCostCap
)
