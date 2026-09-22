package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var engineBindingStandardTests = []string{
	"TestAmbientTrustedPolicyDecisionCannotBypassRetainedEngine",
	"TestBaseMovedToAnEngineChangeReArmsOnce",
	"TestBaseMovedUnderTheRunRestartsPreparationOnce",
	"TestCandidateBuiltCommitPassesDispatchSkewPreflight",
	"TestCandidateEngineArtifactReuseValidatesBytesAndBuildInputs",
	"TestCandidateEngineBuildEnvironmentIsPinnedWithoutDroppingProofCustody",
	"TestCandidateEngineBuildFailureCannotFallBackToPolicyEngine",
	"TestCandidateEngineIsBuiltFromCandidateTreeAndBindsExecutionIdentity",
	"TestCandidateEngineTrimpathIsReproducibleAcrossMaterializationDirectories",
	"TestCauseOutcomesRenderDeclaredCommandCount",
	"TestDecisionMismatchNamesTheField",
	"TestEngineBindingWitnessInventory",
	"TestEngineRefusalsNameTheirCause",
	"TestFastForwardBlockerRemediesPreserveCheckout",
	"TestFailedCandidateEngineBuildCannotFillArtifactCache",
	"TestFrozenNegativeProbeResponseIsLegacyOnlyForActualInvalidResult",
	"TestFrozenPolicyProbeRefusalNamesResultPathPredicate",
	"TestFrozenPublicVersionOneCorpusNormalizesSupportedSourceLayouts",
	"TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker",
	"TestFrozenPublicVersionOneProtectionCorpusIsComplete",
	"TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable",
	"TestFrozenWorkerProbesTraverseActiveEmptyLegacyAndForgedReuse",
	"TestFrozenWorkerProbePathsCanonicalizeRootOnce",
	"TestFrozenWorkerProbeReaderAcceptsCandidateGroupFields",
	"TestLandedRearmDecidesFromTheThreeFacts",
	"TestLandedRearmFactsFailOnARepositoryFailureInsteadOfJudgingHeadUnlanded",
	"TestLandedRearmFastForwardsRebuildsAndReArms",
	"TestLandedRearmJudgmentIsProgressBounded",
	"TestLandedRearmRebuildWaitsForHostSlotAndClearsCustody",
	"TestLandedRearmReadsTheCheckoutAgainstItsRemote",
	"TestLandedRearmRefusesARepositoryFailureReadingTheLandingRefWithGitDetail",
	"TestLandedRearmRefusesARepositoryFailureResolvingCheckoutHeadWithGitDetail",
	"TestLandedRearmRefusesAFastForwardBlockedByDirtyLedgers",
	"TestLandedRearmRefusesAncestorPathCollisionsBeforeFastForward",
	"TestLandedRearmRefusesAStalledTipCompareByName",
	"TestNonDescendantPolicyBaseRefusesDecisionMismatch",
	"TestPolicyChildNeverFetchesOrReArms",
	"TestProtectedCoverageFloorCannotFallOrDisappear",
	"TestPublishTestingResultSpillsOverTheBound",
	"TestPublishTestingResultUnderTheBoundIsUnchanged",
	"TestRefusalQuotesPathFacts",
	"TestRunOwnerSurvivesTestingWorkerFilters",
	"TestSecondBaseMoveRefusesBaseMoved",
	"TestTestListCheckPlanAndVerifyWithoutLaunching",
	"TestTestPlanReArmsOnALandedEngine",
	"TestTestRunRearmsOnALandedEngine",
	"TestTestingPlanAdoptsCandidateFallbackOnlyWhenBaseHasNone",
	"TestTestingCommandAdmissionSamplesAfterPreparationAndAtForcedFallback",
	"TestTestingSelectionCarriesDeliveryAllGroupsOnlyForExecution",
	"TestTestWorkerBuildIdentityCompatibilityDoorIsPolicyProbeOnly",
	"TestTrustedPolicyEngineIsRequiredWithoutBuildingDuringReadOnlySelection",
	"TestUnenrolledLinkedWorktreeNamesItsMainCheckout",
	"TestUnexplainedPolicyFieldRefusesDecisionMismatch",
	"TestUnprintableBlockerPathsAreCountedNotRendered",
	"TestVerifyDoesNotKeyLegacyCandidateDigestByWholeTreeReceipt",
	"TestVerifyRecoversCandidateDigestFromNewestSufficientAttempt",
	"TestVerifySamplesFreshnessAfterRetainedProofRevalidation",
	"TestDiagnosticsReadersFollowTheCandidatePair",
	"TestBatchPrefixProofControlRootAcceptsALinkedLandingWorktree",
	"TestBatchPrefixProofControlRootMustOwnLinkedExecution",
	"TestBatchPrefixTestingControlRootRetainsAttemptOutsideExecution",
	"TestLinkedWorktreeMainInstallationPreservesInstallationSubdirectory",
	"TestTestRunKeepsProofRecordsUnderTheControlRoot",
}

func TestEngineBindingWitnessInventory(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not find metasystem module root")
		}
		root = parent
	}
	files := []string{
		"cmd/metasystem/test_test.go",
		"cmd/metasystem/rearm_on_landed_test.go",
		"cmd/metasystem/engine_refusal_test.go",
		"cmd/metasystem/engine_inventory_test.go",
		"internal/enginecause/cause_test.go",
	}
	actual := map[string]bool{}
	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(name)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name != "TestMain" && strings.HasPrefix(function.Name.Name, "Test") {
				actual[function.Name.Name] = true
			}
		}
	}
	declared := map[string]bool{}
	for _, name := range engineBindingStandardTests {
		if declared[name] {
			t.Errorf("engine-binding-standard declares %s twice", name)
		}
		declared[name] = true
	}
	for name := range actual {
		if !declared[name] {
			t.Errorf("engine-binding-standard does not declare %s", name)
		}
	}
	for name := range declared {
		if !actual[name] {
			t.Errorf("engine-binding-standard declares missing test %s", name)
		}
	}
}
