package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// The names below are how the files C8a part 2 moves (landing_batch_*.go,
// landing_path.go, intent_delivery.go, goal_branch.go) still reach the
// testing selection that moved to internal/testrun. Part 2 ports those
// files to the package and deletes this file. readStrictJSON is also read
// by test.go and landing_verbs.go, which part 2 points at strictjson.Read.

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
	loadPhysicalTestingContract = testrun.LoadContract
	linkedWorktreeMainCheckout  = testrun.LinkedWorktreeMainCheckout
	readStrictJSON              = strictjson.Read
)
