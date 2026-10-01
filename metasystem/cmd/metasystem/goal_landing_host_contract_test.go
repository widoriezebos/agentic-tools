package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGoalLandingHostContractRetainsPriorGroupsAndGoGate(t *testing.T) {
	t.Parallel()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if contract.SchemaVersion != testpolicy.ExecutionContractSchemaVersion {
		t.Fatalf("schema = %d", contract.SchemaVersion)
	}
	ids := map[string]testpolicy.Group{}
	for _, group := range contract.Groups {
		ids[group.ID] = group
		if group.Phase == "" || group.EnvironmentMode != "inherit" {
			t.Fatalf("group %s lost phase or inherited environment", group.ID)
		}
		if group.ID != "refusal-register-standard" && group.ID != "fast-static-build" && group.ID != "policy-canary" && group.ID != "adapter-canary" && group.ID != "command-interface-smoke" && group.ID != "verb-ratchet" && group.ID != "delegation-layering" && group.Phase != "acceptance" {
			t.Fatalf("group %s entered admission outside the reviewed static reproof and canary floor", group.ID)
		}
	}
	for _, id := range []string{"refusal-register-standard", "fast-static-build", "policy-canary", "adapter-canary", "command-interface-smoke", "verb-ratchet", "delegation-layering"} {
		if ids[id].Phase != "admission" {
			t.Fatalf("%s must be admission", id)
		}
	}
	if ids["go-affected"].PackageSelection != "changed-and-consumers" || string(ids["go-affected"].Tests) != `"all"` {
		t.Fatal("host Go selector must discover every affected package test")
	}
	if !slices.Contains(contract.Always.Standard, "go-batchtest") ||
		!slices.Equal(ids["go-batchtest"].BuildTags, []string{"batchtest"}) ||
		!slices.Equal(ids["go-batchtest"].Packages, []string{"cmd/metasystem"}) {
		t.Fatal("host batch-tag tests disappeared from standard acceptance")
	}
	weakened := contract
	weakened.Groups = append([]testpolicy.Group(nil), contract.Groups...)
	for index := range weakened.Groups {
		if weakened.Groups[index].ID == "go-affected" {
			weakened.Groups[index].PackageSelection = ""
			weakened.Groups[index].Packages = []string{"internal/testpolicy"}
			weakened.Groups[index].Tests = []byte(`["TestContractRejectsMissingNoopAndUnknownFields"]`)
		}
	}
	protected := testpolicy.ProtectedContract(contract, weakened)
	for _, group := range protected.Groups {
		if group.ID == "go-affected" && (group.PackageSelection != "changed-and-consumers" || len(group.Packages) != 0 || string(group.Tests) != `"all"`) {
			t.Fatal("candidate policy narrowed protected package selector")
		}
	}
	assertLegacyHostContractCoverage(t, contract)
	// A test retires with its verb only when the verb is gone.
	registered := families()
	for test, verb := range retiredWithDeletedVerb {
		if words := strings.Fields(verb); len(words) == 2 && familyHasVerb(registered, words[0], words[1]) {
			t.Errorf("%s retired with %s, which still routes", test, verb)
		}
	}
}

// The frozen schema-1 fixture is independent of the detached candidate HEAD.
// It captures every old native group and mandatory selection reference.
// retiredWithDeletedVerb names legacy mandatory tests (group/test) whose only
// subject was an internal verb deleted for having no caller
// (plans/designs/verbs-object-action.md 6.1). Each maps to that verb; nothing
// else may leave the legacy floor this way.
var retiredWithDeletedVerb = map[string]string{
	// U9b (B3): test merge left the public surface; git's merge driver,
	// which system setup registers, is the one merge (its witness is
	// TestTestingMergeDriverPreservesOursOnRefusal).
	"command-interface-smoke/TestTestingMergeVerbRoutesAndPreservesOutputOnRefusal": "test merge",
	"context-standard/TestRuntimeContextSampleVerb":                                 "runtime context-sample",
	"launch-standard/TestPackCheckVerbPrintsOneLine":                                "launch pack-check",
	"launch-standard/TestUnitRunPrintsOneLine":                                      "unit run",
	"launch-standard/TestUnitFamilyIsRegistered":                                    "unit run",
	// U7c: the only caller was dependency-tree-guard.sh.
	"batch-buildcd-standard/TestAuditParallelRatchetVerbRefusesAndLowers": "audit parallel-ratchet",
	// U7c step B: proc fixture-key lost its last caller with the fixture
	// libraries; identity.EncodeKey, its owner, stays.
	"command-interface-smoke/TestProcFixtureKeyPrintsAnEncodedKey": "proc fixture-key",
	// U9a: verbs only tests, or only text, ran; each test's subject was the
	// verb's own argument handling or output, and the verb went.
	"command-interface-smoke/TestHealthNamesTheCertainFixtureSurvivor":     "health",
	"command-interface-smoke/TestProcDefaultSignalsHelper":                 "proc default-signals",
	"command-interface-smoke/TestProcDefaultSignalsRestoresIgnoredSignals": "proc default-signals",
	"landing-command-standard/TestProveRoundFindsTheAttemptByCandidate":    "job prove-round",
	"launch-standard/TestRoundTaskRefusals":                                "launch round-task",
	"launch-standard/TestLaunchStartRejectsRemovedPoll":                    "launch start",
	"launch-standard/TestLaunchSettingsPrintsEachValueAndSource":           "launch settings",
	"launch-standard/TestLaunchWaitNamesTheCapAndThePendingState":          "launch wait",
	"goal-decision-standard/TestRestampVerbRefusesTheByFlag":               "goal restamp",
	"context-standard/TestContextPruneVerb":                                "context prune",
	// U9b: internal verbs no launcher starts; the census scan and cadence
	// owners they wrapped keep their own tests.
	"command-interface-smoke/TestProcFixtureSurvivorsVerb":                                           "proc fixture-survivors",
	"command-interface-smoke/TestProcFixtureSurvivorsReapsALiveSurvivor":                             "proc fixture-survivors",
	"landing-command-standard/TestGateCadenceTickRefusesEveryNonOwnerAndPrintsOneOwnerResult":        "gate cadence-tick",
	"landing-command-standard/TestGateCadenceTickClassifiesOnlyTickRefusalsNonZero":                  "gate cadence-tick",
	"landing-command-standard/TestCadenceResultNamesTriggerForJoinedAndOccupiedClaims":               "gate cadence-tick",
	"batch-buildcd-standard/TestGateUnitGreenOutputAndAggregatedSteps":                               "gate unit",
	"batch-buildcd-standard/TestGateUnitRedOutputAndExitCode":                                        "gate unit",
	"command-interface-smoke/TestAuditStopDecisionSurfaceVerb":                                       "audit stop-decision-surface",
	"command-interface-smoke/TestAuditStopDecisionSurfaceRefusesRecordTheGoalParserRejects":          "audit stop-decision-surface",
	"command-interface-smoke/TestChannelFakeServePublicCommandListensUnderSyntheticFixtureAuthority": "channel fake",
	"command-interface-smoke/TestChannelFakeServeRequiresBoundedLifetime":                            "channel fake",
	"command-interface-smoke/TestChannelFakeServeStopsAtInjectedExpiry":                              "channel fake",
	"goal-decision-standard/TestGoalBranchCheckPrintsKinds":                                          "goal branch",
	"goal-decision-standard/TestGoalBranchCheckFetchesCurrentOriginTip":                              "goal branch",
	"goal-decision-standard/TestGoalBranchHelpNamesPush":                                             "goal branch",
	"goal-decision-standard/TestGoalBranchStatusReportsAbsentOrigin":                                 "goal branch",
	"goal-decision-standard/TestTransportMirrorAndLeasedDelete":                                      "goal branch",
	"goal-decision-standard/TestGoalBranchSweepListsParkedDoneAbandonedAndOrphan":                    "goal branch",
	"goal-decision-standard/TestGoalListJSONKeepsItsShapeAndRequiresHistoryFlag":                     "goal list",
	"goal-decision-standard/TestGoalBranchCommitRefusesToMoveAnArmedCheckout":                        "goal branch",
	"goal-decision-standard/TestCommitReadRefusesUnrecordedFastGateRun":                              "goal branch",
	"supervision-bed-standard/TestRunLaunchReadsClosedFenceBeforeCallerGate":                         "run launch",
	"supervision-bed-standard/TestSupervisionBedAWaitMeasureJoinsLiveWaits":                          "wait measure",
	"supervision-bed-standard/TestWaitMeasureVerb":                                                   "wait measure",
}

// retiredWithDeletedBed names legacy mandatory tests whose only subject was a
// deleted shell fixture bed; the bed's scenarios moved to named Go tests
// (verbs-object-action 6.7, U5).
// retiredWithDeletedLibrary names legacy mandatory tests whose only subject
// was library code deleted for having no caller (C5, repo hygiene): the
// parked fleet-channel-gateway receive library, recoverable from 444eda07c
// and 57c310a1e. Each maps to what was deleted.
var retiredWithDeletedLibrary = map[string]string{
	"authority-standard/TestVerifyOrderAndStep":               "channel/inbox.go Verify",
	"authority-standard/TestVerifyMasksTextForEveryOutcome":   "channel/inbox.go Verify",
	"authority-standard/TestVerifyUsesNowForZeroSentAt":       "channel/inbox.go Verify",
	"authority-standard/TestInboundRecordMapsProviderFields":  "channel/inbox.go InboundRecord",
	"authority-standard/TestStripCode":                        "channel/totp.go StripCode",
	"authority-standard/TestMaskCodesPreservesEveryOtherByte": "channel/totp.go MaskCodes",
	// Its record half went with InboundRecord; its git-window half is
	// TestGitWindowRemainsUTC in the same group.
	"authority-standard/TestChannelRecordTimesAndGitWindowRemainUTC": "channel/inbox.go InboundRecord",
	// Commands print on their invocation's writers; the process-stream swap
	// it tested is gone, held by TestNoTestSwapsTheProcessStreams.
	"batch-buildcd-standard/TestCaptureCommandOutputAllowsNestedAndConcurrentDisjointStreams": "cmd/metasystem captureCommandOutput",
	// C8a part 2: production-unreachable helpers left production with the
	// tests of only them; the live paths keep their own tests.
	"batch-buildb-standard/TestBatchDiagnosticRefusalHoldsUnclassified":                       "landing/batch HoldUnclassified",
	"batch-buildcd-standard/TestReverseDependentsIncludeDirectTransitiveTestAndTaggedImports": "gopackages ReverseDependents",
	"batch-buildcd-standard/TestBatchJoinGateStepsChangedThenSortedDependentsAndBatchTests":   "landing/batch/goadapter JoinGatePackageSteps",
	"goal-decision-standard/TestGoalReadItemsListJSONShape":                                   "cmd/metasystem runGoalReadItemsListWithInputs",
	// Lane runtime design r10 K-c: the moved-base republish was deleted (a
	// moved base is returned, never republished); its rule is held by
	// TestBatchLandingMovedBaseReopensAndNeverRepublishes and
	// TestBatchLandProductionSeamsReopenAMovedBase.
	"batch-buildcd-standard/TestBatchLandingPersistsRecoveryBranchOnSecondRefusal":        "landing/batch LandSeams.RecoverPush",
	"batch-buildcd-standard/TestBatchLandingOpensAfterThreeRecoveryPushRounds":            "landing/batch LandSeams.RecoverPush",
	"batch-buildcd-standard/TestBatchLandingResumesPendingMovedOriginRecovery":            "landing/batch LandSeams.RecoverPush",
	"batch-buildcd-standard/TestBatchLandingMovedInputReturnsOpenOnNewBase":               "landing/batch LandSeams.RecoverPush",
	"batch-buildcd-standard/TestBatchLandingCountsRecoveryFailureBeforeBranchPublication": "landing/batch LandSeams.RecoverPush",
	"batch-buildcd-standard/TestBatchLandTrunkMovedRebasesOrReopens":                      "landing/batchowner RecoverMovedBatchPushWith",
	"batch-buildcd-standard/TestBatchMovedPushRecoveryDoesNotRetryUnchangedOrigin":        "landing/batchowner RecoverMovedBatchPushWith",
	"batch-buildcd-standard/TestBatchLandProductionSeamsBoundRecoveryAndAbandon":          "landing/batchowner BatchLandRecoverPush",
	"batch-buildcd-standard/TestRecoveryFailureDoesNotAbortAmbientRebase":                 "landing/batchowner ReopenMovedBatchAfterRecoveryFailure",
	// Unit D: its tagged-witness half checked the old owner's batchtest
	// witness, deleted with the owner; its input pins stay as
	// TestMetaSystemBatchBuildCDPinsInputs.
	"batch-buildcd-standard/TestMetaSystemBatchBuildCDPinsInputsAndTaggedWitnessOwner": "cmd/metasystem TestBatchTaggedCapabilityWitnessExecutesInProof",
	// Lane runtime design r10 §5, unit D: the old batch owner, its state
	// machine, diagnosis, verifier retries, re-arm, cadence tick driver,
	// replay reconstruction and supervised component are deleted (hard
	// cutover); its tests went with it. The lane's kernel verbs carry the
	// kept behavior, held by their own tests.
	"batch-buildb-standard/TestBatchOwnerReportsResumeEnumerationFailure":                           "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentAnnounceErrorReleasesOnStop":                    "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentForeignHolderDoesNotReannounce":                 "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentKeepsHeartbeatLoopOnSetupErrors":                "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentRecoversFromEnvironmentFailure":                 "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentRecoversFromHolderProofFailure":                 "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentReleaseJoinsCadenceTick":                        "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentReleaseLeavesNoCadenceChild":                    "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentRetriesAfterEnrollmentAppears":                  "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentRetriesConstructionWithFreshInputs":             "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentSetupFailureDoesNotReannounce":                  "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerComponentStopsActingAfterLeaseLoss":                      "landing/batchowner and landing/batch old batch owner",
	"batch-buildb-standard/TestLandingOwnerIsProductionRegistryComponent":                           "supervise landing-owner component (its rule is now TestAnOlderEnginesLandingOwnerRecordsStayReadable)",
	"batch-buildb-standard/TestProductionArmingTakeoverStopsAndRelaunchesLandingOwner":              "supervise landing-owner component (its rule is now TestProductionArmingTakeoverStopsAnOlderEnginesLandingOwnerAndNeverRelaunchesIt)",
	"batch-buildb-standard/TestProductionSupervisorTakeoverRelaunchesLandingOwner":                  "supervise landing-owner component (its rule is now TestProductionSupervisorTakeoverRelaunchesTheProductionSet)",
	"batch-buildcd-standard/TestAbandonLandingBranchIsIdempotent":                                   "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestApplyCertifiedPatchStagesTheTransportedBytes":                       "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchBranchCommitRequiresPassingProvenance":                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchChainCommitKeepsWouldRefuseVerdictBehavior":                    "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchCommitRefusalIsAtomicAndKeepsJoinOrder":                        "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchConflictReopenPublishesOneConsistentUpdate":                    "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchDelegationRules":                                               "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchDiagnosisForwardsStoredPrefixEvidence":                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchDiagnosticRefusalHoldsWithoutEjection":                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchEjectAndReassemble":                                            "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchGoalEjectionRebuildsLeasedLandingBranch":                       "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingCrashAfterPushRecognizesPublishedSeries":                "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingDoesNotRepeatRefusedPushOnUnchangedOrigin":              "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingHeldRefusalNamesCause":                                  "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingHoldsNonLeaseRejectionUntilOriginMoves":                 "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingLifecycleEndToEnd":                                      "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingLostAcknowledgementTakesP6Recovery":                     "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingRequiresCommitLeaseBase":                                "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingResumeRebuildsCompleteSeries":                           "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchLandingTransportRunsWholeSeriesOnce":                           "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchMemberLandsEveryBuildInOrder":                                  "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchMemberRecordReceiptAndNextCarryIdentity":                       "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchMixedBranchAndChainMembersLandInBothOrders":                    "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchOwnerSearch":                                                   "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchProofInputsMovedIgnoresSiblingEnginePaths":                     "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchProofInputsMovedIncludesEnginePaths":                           "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchPushRejectionAppearsInStatus":                                  "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchRecoveryRecordsPostPushRearm":                                  "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchRecoveryRetriesPostPushRearmWithoutRepush":                     "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchReopenNeedsNewTree":                                            "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchSealExcludesReturnedUnits":                                     "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchSingleOwnerRedEjectsAndSurvivorsLand":                          "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchStatusExposesReturnRevisionHeadroomOwnerLockSampleAndDeadline": "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchTaggedCapabilityWitnessExecutesInProof":                        "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchTaggedCapabilityWitnessRejectsMissingTest":                     "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchWaitReturnsOnEveryTerminalOrHeldState":                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchWaitUsesInjectedBound":                                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchWithdrawBeforeSealAndRefusesAfterSeal":                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchWithdrawCommandUsesRecordedJoinerIdentity":                     "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestBatchWithdrawRefusesUnspecifiedCases":                               "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestCommitWithRealWrapperWritesBatchTrailersAndExplicitIdentity":        "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestCommitWithWrapperRequiresExactlyOneNewCommitAndStrictPassVerdict":   "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestCommitWithWrapperUsesTempRepoTokenAndExplicitIdentity":              "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestDiagnosticInputMatchingUsesManifestPathsNotSuffixes":                "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestDiagnosticNoReuseForcesFreshRunsAndDeliveryRefuses":                 "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestFinishBatchLandingLeavesUnchangedPushRejectionQuiet":                "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestLandingBranchLeasesEndpointAndDeletesCandidateAtomically":           "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestLedgerTrunkRedOwnerClearAcceptsUnchangedProjection":                 "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestLedgerTrunkRedOwnerClearRefusesChangedBranch":                       "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestLedgerTrunkRedOwnerClearRefusesChangedOwner":                        "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestLedgerTrunkRedOwnerClearRefusesChangedProjection":                   "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixAdmissionRefusalDoesNotWithdrawCapacity":                      "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixBudgetRefusalWithdrawsMember":                                 "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptAcceptsSufficientReusableExit":                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptAllowsIdentityReuseAndBindsRevisions":                  "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptClassifiesBudgetForWithdrawal":                         "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptClassifiesCapacityWithoutBudgetWithdrawal":             "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptClassifiesRevisionMove":                                "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptPersistsRedResultFromExitOne":                          "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptRetriesInfrastructureExitWithNonTerminalStatus":        "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixReceiptsReuseByIdentity":                                      "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixRedEjectsItsUnit":                                             "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixRedPersistenceFailureIsReturned":                              "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestPrefixRevisionMoveReturnsForRevision":                               "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestReassembleSurvivorsAdoptsPublishedEquivalentTreeAfterStoreLoss":     "landing/batchowner and landing/batch old batch owner",
	"batch-buildcd-standard/TestTrunkRedHookAndNarrowHold":                                          "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchCommandsReleaseAfterConstructionFailure":                     "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchCommandsResolveInputsBeforeAnnouncement":                     "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchDiagnosticDiscardsStaleResultAfterFailedRun":                 "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchLedgerOwnerDefaultRefusesUnbound":                            "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchLedgerOwnerUsesLandingIdentity":                              "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchLockIsFifoAndStaleSafe":                                      "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerCadenceWiringBound":                                     "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerHoldsLandingOnRegisteredTrunkRedButNotStatusAlone":      "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerHoldsPersistentUnknownCensus":                           "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerLaunchesAtMaximumWait":                                  "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerLoopStopJoinsCadenceTick":                               "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerLoopStopsBeforePassAfterLeaseLoss":                      "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerManualAcquireCleansFailedAnnouncement":                  "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerManualProofAndEnvironmentFailuresReleaseAnnouncement":   "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerNextProcessRecoversReleasedAndKilledHolder":             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerReconcilesAtFetchedTree":                                "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerRecordsHeldTrunkRed":                                    "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerReleaseDoesNotRecreateRemovedRoot":                      "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerResumesEveryLiveBatch":                                  "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerResumesLandingAfterTrunkRedClearError":                  "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerRetirementErrorsReachEveryCaller":                       "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerRetriesGreenTipClearAfterLandingFinishes":               "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerReturnsAfterReconcile":                                  "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerSavesGreenTreeThatCannotClear":                          "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerSkipsRecordedHoldAndReleasesLockBeforeLedger":           "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerWakesOnSignal":                                          "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchOwnerWiringBound":                                            "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchProductionRegistryComplete":                                  "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchRolloutRequiresSealReceiptDiagnosisAndRecovery":              "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchRuntimeInputsCannotRegisterCapability":                       "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchSealDryRunsTheBoundary":                                      "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchSealExecutesChangedInputs":                                   "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchSealFreezesMembership":                                       "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchSealRegates":                                                 "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchSealReleasesLockDuringGateAndRefusesChangedCandidate":        "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchStartRule":                                                   "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchStatusReportsCadenceNoneOverdueNonGreenAndGreen":             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchTickReconcilesAKilledJoinerOnce":                             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchTickStopsBeforePassAfterLeaseLoss":                           "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestBatchVerbsUnavailableWithoutFilesystemWrites":                     "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestFirstTrunkRedHoldUsesLedgerOwnerOpid":                             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestGreenDiagnosticDiscoveryErrorWithholdsReopenUntilRetry":           "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestGreenTipProofClearsHeldlessEntryOnlyWhenExecuted":                 "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestLandingOwnerComponentCadenceWiringBound":                          "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestLedgerTrunkRedOwnerClearClassifiesFixCommit":                      "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestLedgerTrunkRedOwnerRecordsIdempotently":                           "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestMixedDiagnosticClearsPassedEntryBeforeRehold":                     "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestRedDiagnosticRecordsDespiteUnclearablePassedEntry":                "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestReopenClearsOnGreenDescendantOnly":                                "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestReopenDiscoversEveryLedgerEntryHeldByBatch":                       "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestReopenRetriesPartialClearAndSkipsClosedEntries":                   "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedFailedOutcomeMintsOneNewOpid":                             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedFailedOutcomeRefusesMissingMint":                          "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedHoldIsDurableAndCallsNoOwner":                             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedHoldIsDurableBeforeAnyLedgerCall":                         "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedHoldRetryAndRehold":                                       "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedHoldTakesJoinersAndRefusesOtherStates":                    "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedLeavesABatchThatMovedOnAlone":                             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedRecordOutcomeDistinguishesAlreadyFromMovedOn":             "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedRefusalLeavesHeldWithEmptyEntries":                        "landing/batchowner and landing/batch old batch owner",
	"landing-command-standard/TestTrunkRedUnboundOwnerRefusesByName":                                "landing/batchowner and landing/batch old batch owner",
	// The simple lane deleted landing validate and with it internal/cadence,
	// the validation cadence these tests checked.
	"landing-command-standard/TestCadencePreparationDoesNotRequireClaimedGoal":                "internal/cadence (landing validate's cadence)",
	"landing-command-standard/TestCadenceRunStoreRereadsLandingOwnerEpoch":                    "internal/cadence (landing validate's cadence)",
	"landing-command-standard/TestCadenceRevalidationDefersMissingEngineBuildUntilClaimedRun": "internal/cadence (landing validate's cadence)",
	"landing-command-standard/TestCadenceRevalidationDoesNotBuildBeforeClaim":                 "internal/cadence (landing validate's cadence)",
}

var retiredWithDeletedBed = map[string]string{
	"batch-buildcd-standard/TestLandFixtureConfigurationsPinProofAdmission": "land-fixtures.sh",
	"batch-buildcd-standard/TestLandFixtureScenarioRegistryMatchesCount":    "land-fixtures.sh",
	// U7c: the landing lane worker had no caller; the fixture-bed group map
	// was derived from sections that no longer run fixture scripts.
	"launch-standard/TestLandingLaneFixtureScenariosHaveGoWitness":   "landing-lane-worker.sh",
	"landing-command-standard/TestFixtureGroupsForChangedBeds":       "fixture-bed-groups.tsv",
	"landing-command-standard/TestFixtureBedGroupMapMatchesSections": "fixture-bed-groups.tsv",
}

// retiredWithDeletedScript names legacy mandatory tests (group/test) whose only
// subject was a script deleted by its provider transition (6.4) and maps each to
// the test that now carries the behavior, which must be in the contract.
var retiredWithDeletedScript = map[string]string{
	"proof-standard/TestGoGateCopiedRootStopsAfterOneUnauthorizedRelaunch":                "TestGateRefusesAnUnauthorizedRelaunchedChild",
	"proof-standard/TestGoGateRelaunchUsesBuiltEngineForWorkerAuthorization":              "TestGateRelaunchesAStandaloneRunUnderItsRetainedProofOwner",
	"batch-buildcd-standard/TestGoGateFastModeRunsParallelRatchetBesideDependencyRatchet": "TestStaticRatchetsRefuseBeforeAnyToolRuns",
	// validate-metasystem.sh and its section selector (U7b-1): script beds
	// declare their argv and are judged by exit status alone.
	"proof-standard/TestSupervisorSectionResultGrowthCountsAsOutput": "TestSectionArgvBedRunsWithCandidateEngineAndJudgesExit",
	"proof-standard/TestFailedScriptGroupKeepsPresetReason":          "TestSectionBedFailureKeepsNativeStatusAndLaterIndependentResult",
	// The proof scripts' relaunch custody moved into devgate with go-gate.sh (U7a).
	"proof-standard/TestProofScriptsRefuseUnauthorizedRelaunchedChildren": "TestGateRefusesAnUnauthorizedRelaunchedChild",
	"proof-standard/TestProofScriptsExportDepthAndAuthenticationEngine":   "TestGateRelaunchesAStandaloneRunUnderItsRetainedProofOwner",
	// fixture-stop-report.sh and report stop-response (U7c): the tests drive
	// the stopreport owner under their new names.
	"wait-stop-standard/TestReportStopResponseResolvesUnderChangedWording": "TestStopResponseResolvesUnderChangedWording",
	"wait-stop-standard/TestReportStopResponseRefusesAnUnreadableResponse": "TestStopResponseRefusesAnUnreadableResponse",
	// C4: metasystem.conf holds overrides only, so no tracked file carries
	// the launch keys; the launch defaults are rows of the compiled table.
	"launch-standard/TestTrackedConfCarriesEveryLaunchKey": "TestLaunchDefaultsAreTheCompiledDefaults",
}

// providerTransitions names legacy groups whose command moved to a new
// provider under the two-step transition (plans/designs/verbs-object-action.md
// 6.4): the group keeps its id so a candidate's proof runs the base's command,
// and holds exactly the new provider's definition. Nothing else may leave the
// legacy definition this way.
var providerTransitions = map[string]func(testpolicy.Group) bool{
	// go-gate.sh --fast became the Go bootstrap's static leaf.
	"fast-static-build": func(group testpolicy.Group) bool {
		return group.Adapter == "command" && slices.Equal(group.Argv, []string{"go", "run", "./cmd/devgate", "static"}) &&
			slices.Contains(group.Obligations, "gate-integrity")
	},
	// witness-gate-fixtures.sh became the devgate witness and arming tests.
	"section/witness-gate-fixtures": func(group testpolicy.Group) bool {
		return group.Adapter == "go" && slices.Equal(group.Packages, []string{"cmd/devgate"}) && len(group.Tests) > 2 &&
			slices.Contains(group.Obligations, "test-execution-integrity")
	},
}

func assertLegacyHostContractCoverage(t *testing.T, current testpolicy.Contract) {
	t.Helper()
	installation, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Dir(installation)
	data, err := os.ReadFile(filepath.Join("testdata", "goal_landing_legacy_testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "0951d3c46e488ca28c13cb25c517bfef54de5836ca0beb8de3cad4859664717a" {
		t.Fatalf("frozen legacy host contract changed: %s", got)
	}
	// The frozen contract predates the declared script beds: its section
	// groups name a selector section and no argv, which today's validation
	// refuses. It is read as the record it is, never validated as a candidate.
	var previous testpolicy.Contract
	if err := json.Unmarshal(data, &previous); err != nil {
		t.Fatal(err)
	}
	containsAll := func(owner string, actual, required []string) {
		t.Helper()
		for _, name := range required {
			if !slices.Contains(actual, name) {
				t.Errorf("%s dropped mandatory %s", owner, name)
			}
		}
	}
	currentGroups := map[string]testpolicy.Group{}
	for _, group := range current.Groups {
		currentGroups[group.ID] = group
	}
	// A retired fixture section may leave the contract only when every one of
	// its scenarios was ported to named Go tests that a replacement group,
	// itself on the cadence, discovers, or when the behavior it checked was
	// deleted with its script (no replacement, a named reason). Only these
	// exact sections qualify.
	retiredSections := map[string]struct {
		replacement string
		tests       []string
		reason      string
	}{
		// second-session.sh moved into internal/seat/launch (verbs-object-action U3).
		"section/second-session-fixtures": {replacement: "launch-machine-standard", tests: []string{
			"TestTheManifestIsTheAdaptersDeclaredContract",
			"TestSecondSessionCreatesAnIsolatedArmedWorktree",
			"TestSecondSessionMintsANameAndRefusesUnlawfulOnes",
			"TestSecondSessionStopsWhenArmingFails",
		}},
		// land-fixtures.sh moved into internal/landing/landpath (U5; scenario
		// map in the unit's evidence).
		"section/land-fixtures": {replacement: "landing-path-standard", tests: []string{
			"TestDriverPushRetryRecoversOneMovingOriginRejection",
			"TestCarriedFreshLandsOneCommitPastOneRefusal",
			"TestCarriedCrashAfterPushCompletesTheRecord",
			"TestAbandonRouteNormalRefusesAtTheFetchedParent",
			"TestAbandonRouteRecertifiedHeldBeforePushAndParks",
		}},
		// pre-commit-guard-fixtures.sh moved into landpath.Guard's tests (U5).
		"section/pre-commit-guard-fixtures": {replacement: "landing-path-standard", tests: []string{
			"TestGuardLedgerFenceOutranksBothExceptions",
			"TestGuardNewPlanNeedsAcknowledgment",
			"TestGuardRefusesPatchBackups",
		}},
		// U7b-3: the ten fixture beds became Go bed groups.
		"section/suite-progress-fixtures":         {replacement: "suite-progress-bed-standard"},
		"section/supervision-and-census-fixtures": {replacement: "supervision-bed-standard"},
		"section/mission-fixtures":                {replacement: "mission-bed-standard"},
		"section/conformance-fixtures":            {replacement: "conformance-bed-standard"},
		"section/brain-fixtures":                  {replacement: "brain-bed-standard"},
		"section/return-schema-fixtures":          {replacement: "return-schema-bed-standard"},
		"section/static-reproof-fixtures":         {replacement: "static-reproof-bed-standard"},
		// U7c: the Bash fixture libraries it tested are deleted with their last
		// user, and their Go witnesses with them.
		"section/fixture-bed-scenarios-fixtures": {reason: "fixture-budget.sh and fixture-bed-scenarios.sh are deleted (verbs-object-action U7c)"},
		// U7b-1: validate-metasystem.sh's inline sections became Go groups.
		"section/engine-delivery-contract":       {replacement: "shipped-installation-standard"},
		"section/static-placeholder-scan":        {replacement: "shipped-installation-standard", tests: []string{"TestAuditMetasystemRefusals"}},
		"section/metasystem-audit":               {replacement: "shipped-installation-standard"},
		"section/static-contract-audits":         {replacement: "shipped-installation-standard"},
		"section/runtime-contract-audits":        {replacement: "runtime-contract-standard"},
		"section/agent-protocol-fixtures":        {replacement: "agent-protocol-standard"},
		"section/workflow-tooling-fixtures":      {replacement: "workflow-tooling-standard"},
		"section/watch-background-jobs-fixtures": {reason: "the job watcher it tested lost its last caller and was deleted with it (verbs-object-action U9b)"},
		"section/covenant-evidence-pre-rebuild":  {reason: "the covenant evidence verb it drove is deleted; the gate runs inside delivery (verbs-object-action U9b)"},
		"section/covenant-evidence-post-rebuild": {reason: "the covenant evidence verb it drove is deleted; the gate runs inside delivery (verbs-object-action U9b)"},
		"section/supervisor-fingerprint-heal-harness": {replacement: "runtime-owner-standard", tests: []string{
			"TestWatcherRestartRequestReplacesOnlyTheEnrolledGenerationWithinOneCycle",
			"TestCompletedWatcherRequestIsReplacedByANewGeneration",
		}},
		"section/suite-host-prerequisites":  {reason: "the host prerequisites it checked belonged to the retired validator and dispatcher bed"},
		"section/enumeration-mode-fixtures": {reason: "enumeration mode was deleted with validate-metasystem.sh"},
		// goal-cli-fixtures.sh moved into the goal CLI Go tests (verbs-object-action U7b part 2).
		"section/goal-cli-fixtures": {replacement: "goal-cli-standard"},
		// C6 (repo hygiene): tombstones that only printed their retirement.
		// Their scenarios run as Go tests in the owners' packages, which the
		// package-selected go-affected group discovers.
		"section/supervision-go-fixtures":                        {reason: "retired to Go tests in internal/supervise (verbs-object-action U7c)"},
		"section/gate-fence-fixtures":                            {reason: "retired to Go tests in internal/gaterun"},
		"section/telemetry-census-fixtures":                      {reason: "retired to Go tests in internal/census and internal/adapter/supervisor"},
		"section/config-identity-fixtures":                       {reason: "retired to Go tests in internal/config (verbs-object-action U7c)"},
		"section/authority-regression-fixtures":                  {reason: "retired to Go tests in internal/delegation and the owners' packages (verbs-object-action U6b)"},
		"section/record-protocol-fixtures":                       {reason: "retired to Go tests in internal/delegation and the owners' packages (verbs-object-action U6b)"},
		"section/evidence-segment-fixtures":                      {reason: "retired to Go tests in internal/evidence and internal/delegation (verbs-object-action U7c)"},
		"section/lease-succession-fixtures":                      {reason: "retired to Go tests in internal/lease and internal/missionrunner (verbs-object-action U7c)"},
		"section/flight-recorder-fixtures":                       {reason: "retired to Go tests in internal/lease and internal/events (verbs-object-action U7c)"},
		"section/acp-fixtures":                                   {reason: "retired to Go tests in internal/adapter/supervisor, internal/missionrunner/hostturn and internal/acp"},
		"section/delegate-caps-fixtures":                         {reason: "retired to Go tests in internal/delegation and the owners' packages (verbs-object-action U6b)"},
		"section/adapter-deadline-fixtures":                      {reason: "retired to Go tests in internal/adapter/supervisor"},
		"section/dispatcher-adapter-and-mission-runner-fixtures": {reason: "retired to Go tests in internal/delegation and the owners' packages"},
		"section/project-extra-suites":                           {reason: "its one suite went with the benchmark kit's drivers (verbs-object-action U8b)"},
		"section/shell-and-dependency-audits":                    {reason: "it parsed metasystem/scripts/, which is deleted; the static gate's shell parse judges every .sh file"},
	}
	currentGroupIDs := map[string]bool{}
	for _, group := range current.Groups {
		currentGroupIDs[group.ID] = true
	}
	// The mandatory references of a retired section move to its replacement;
	// a section retired with its behavior leaves them.
	retiredReferences := func(values []string) []string {
		var out []string
		for _, name := range values {
			if retired, ok := retiredSections[name]; ok && !currentGroupIDs[name] {
				if retired.replacement == "" {
					continue
				}
				name = retired.replacement
			}
			out = append(out, name)
		}
		return out
	}
	for index := range previous.Surfaces {
		previous.Surfaces[index].Standard = retiredReferences(previous.Surfaces[index].Standard)
		previous.Surfaces[index].Deep = retiredReferences(previous.Surfaces[index].Deep)
		previous.Surfaces[index].Critical = retiredReferences(previous.Surfaces[index].Critical)
	}
	previous.Always.Canary = retiredReferences(previous.Always.Canary)
	previous.Always.Standard = retiredReferences(previous.Always.Standard)
	previous.Unknown = retiredReferences(previous.Unknown)
	currentSurfaces := map[string]testpolicy.Surface{}
	for _, surface := range current.Surfaces {
		currentSurfaces[surface.ID] = surface
	}
	// The landing path's shell beds retired into the landing-path-standard
	// Go group (U5), and the beds' own surfaces went with them.
	// U7c: the supervision-go fixture and the fixture-bed libraries were
	// deleted; their surfaces' only paths went with them.
	retiredSurfaces := map[string]bool{"land-fixture": true, "pre-commit-guard-fixture": true,
		"supervision-go-fixture": true, "fixture-bed-scenarios-fixture": true}
	for _, old := range previous.Surfaces {
		now, ok := currentSurfaces[old.ID]
		if !ok && retiredSurfaces[old.ID] {
			continue
		}
		if !ok {
			t.Errorf("legacy surface %s disappeared", old.ID)
			continue
		}
		containsAll(old.ID+" standard", now.Standard, old.Standard)
		containsAll(old.ID+" deep", now.Deep, old.Deep)
		containsAll(old.ID+" critical", now.Critical, old.Critical)
		containsAll(old.ID+" cross-cutting", now.CrossCutting, old.CrossCutting)
	}
	containsAll("always canary", current.Always.Canary, previous.Always.Canary)
	containsAll("always standard", current.Always.Standard, previous.Always.Standard)
	containsAll("unknown", current.Unknown, previous.Unknown)
	containsAll("cadence", current.Cadence, retiredReferences(previous.Cadence))
	for _, old := range previous.Groups {
		now, ok := currentGroups[old.ID]
		if retired, isRetired := retiredSections[old.ID]; !ok && isRetired {
			if retired.replacement == "" {
				if retired.reason == "" {
					t.Errorf("retired section %s names neither a replacement nor a reason", old.ID)
				}
				continue
			}
			replacement, present := currentGroups[retired.replacement]
			if !present {
				t.Errorf("retired section %s has no replacement group %s", old.ID, retired.replacement)
				continue
			}
			probe := replacement
			if len(retired.tests) != 0 {
				if probe.Tests, err = json.Marshal(retired.tests); err != nil {
					t.Fatal(err)
				}
			} else if string(probe.Tests) == `"all"` || len(probe.Tests) == 0 {
				t.Errorf("retired section %s: replacement %s must name the ported tests", old.ID, retired.replacement)
				continue
			}
			probeContract := testpolicy.Contract{SchemaVersion: current.SchemaVersion, Groups: []testpolicy.Group{probe}}
			if err := proofrun.CheckNativeDiscovery(t.Context(), projectRoot, installation, probeContract, os.Environ()); err != nil {
				t.Errorf("retired section %s: replacement %s does not discover its ported tests: %v", old.ID, retired.replacement, err)
			}
			continue
		}
		if !ok {
			t.Errorf("legacy host group %s disappeared", old.ID)
			continue
		}
		if transitioned, moved := providerTransitions[old.ID]; moved {
			if !transitioned(now) {
				t.Errorf("legacy group %s left its definition without holding its new provider's", old.ID)
			}
			continue
		}
		requiredInputs := old.Inputs
		if old.ID == "hook-start-audit-standard" {
			// U4 retired the shell hook bed and the sourced degraded-form
			// renderer; both moved into internal/hooks, which the group names.
			// Only these exact retired inputs may be replaced, and only by it.
			retired := map[string]bool{
				"metasystem/scripts/agents/supervision-hook-fixtures.sh": true,
				"metasystem/scripts/agents/stop-degraded-forms.sh":       true,
			}
			requiredInputs = nil
			for _, input := range old.Inputs {
				if !retired[input] {
					requiredInputs = append(requiredInputs, input)
				}
			}
			requiredInputs = append(requiredInputs, "metasystem/internal/hooks/**")
		}
		if old.ID == "batch-buildcd-standard" {
			// U5 retired the landing scripts; the landing path is the Go
			// package the group names instead. Only these exact inputs.
			retired := map[string]bool{
				"metasystem/scripts/agents/commit.sh":        true,
				"metasystem/scripts/agents/land-fixtures.sh": true,
			}
			requiredInputs = nil
			for _, input := range old.Inputs {
				if !retired[input] {
					requiredInputs = append(requiredInputs, input)
				}
			}
			requiredInputs = append(requiredInputs, "metasystem/internal/landing/landpath/**")
		}
		// A single-file input leaves only with its deleted file (a script, or
		// a Go source such as the retired proofrun test_section.go), and a
		// whole-directory input (dir/**) only with its deleted directory.
		var liveInputs []string
		for _, input := range requiredInputs {
			path := input
			if directory, whole := strings.CutSuffix(input, "/**"); whole {
				path = directory
			}
			if !strings.ContainsAny(path, "*?[") {
				if _, statErr := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(path))); os.IsNotExist(statErr) {
					continue
				}
			}
			liveInputs = append(liveInputs, input)
		}
		containsAll(old.ID+" inputs", now.Inputs, liveInputs)
		containsAll(old.ID+" packages", now.Packages, old.Packages)
		containsAll(old.ID+" obligations", now.Obligations, old.Obligations)
		var oldTests []string
		if len(old.Tests) != 0 && string(old.Tests) != `"all"` {
			if err := json.Unmarshal(old.Tests, &oldTests); err != nil {
				t.Fatal(err)
			}
		}
		nowSelectsAll := string(now.Tests) == `"all"`
		var nowTests []string
		if len(now.Tests) != 0 && !nowSelectsAll {
			if err := json.Unmarshal(now.Tests, &nowTests); err != nil {
				t.Fatal(err)
			}
		}
		var automaticallyCovered []string
		for _, name := range oldTests {
			if verb, retired := retiredWithDeletedVerb[old.ID+"/"+name]; retired {
				t.Logf("%s %s retired with the deleted verb %s", old.ID, name, verb)
				continue
			}
			if bed, retired := retiredWithDeletedBed[old.ID+"/"+name]; retired {
				t.Logf("%s %s retired with the deleted bed %s", old.ID, name, bed)
				continue
			}
			if library, retired := retiredWithDeletedLibrary[old.ID+"/"+name]; retired {
				t.Logf("%s %s retired with the deleted library %s", old.ID, name, library)
				continue
			}
			if replacement, retired := retiredWithDeletedScript[old.ID+"/"+name]; retired {
				carried := false
				for _, group := range current.Groups {
					var names []string
					if json.Unmarshal(group.Tests, &names) == nil && slices.Contains(names, replacement) {
						carried = true
					}
				}
				if !carried {
					t.Errorf("%s %s retired with its deleted script, but %s is not in the contract", old.ID, name, replacement)
				}
				continue
			}
			requiredNames := []string{name}
			if old.ID == "authority-standard" && name == "TestTemporaryGoalProofUsesTheRealWallClock" {
				// Temporary goal authority keeps both guarantees from the retired
				// wall-clock test: wrapper validation and deterministic time policy.
				// Only this exact legacy name in this group may use replacements.
				requiredNames = []string{
					"TestTemporaryGoalProofWrapperRejectsIncompleteWordPair",
					"TestTemporaryGoalProofEnforcesReviewExpiryAndHorizon",
				}
			}
			if old.ID == "landing-command-standard" && (name == "TestProductionJoinGateSelectsPackagesAgainstTheLandingRoot" || name == "TestDeletedGoPackagesSelectNearestExistingDirectory") {
				// The replacement tests retain deleted-package selection and the
				// configured contract and project-root guarantees. Deleted-package
				// selection moved with the Go selection into the Go adapter
				// (batch lane U0), so its test is carried by a group that covers
				// that package, not by this one.
				requiredNames = nil
				if name == "TestProductionJoinGateSelectsPackagesAgainstTheLandingRoot" {
					requiredNames = []string{"TestLandingBatchProtectedTestsUseConfiguredContractAndProjectCWD"}
				}
				carriedByAdapter := false
				for _, group := range current.Groups {
					var names []string
					if slices.Contains(group.Packages, "internal/landing/batch/goadapter") &&
						json.Unmarshal(group.Tests, &names) == nil && slices.Contains(names, "TestDeletedGoPackagesSelectNearestExistingDirectory") {
						carriedByAdapter = true
					}
				}
				if !carriedByAdapter {
					t.Errorf("%s dropped mandatory TestDeletedGoPackagesSelectNearestExistingDirectory: no group covering internal/landing/batch/goadapter lists it", old.ID)
				}
			}
			if old.ID == "test-environment-standard" && name == "TestAgedSweepRemovesFixtureRecordFiles" {
				// Age alone no longer authorizes deletion: a sweep may remove
				// sidecars only after an authenticated dead home and a settled
				// inherited lease and custodian, and must retain failed settlements.
				requiredNames = []string{
					"TestSweepKeepsSidecarsWithoutAuthenticatedDeadHome",
					"TestRegistrySidecarCleanupWaitsForInheritedLeaseAndCustodian",
					"TestRegistrySidecarCleanupRetainsFailedSettlement",
				}
			}
			if old.ID == "goal-decision-standard" && name == "TestReadItemsCloseRefusesClosedItem" {
				// U-idem: closing a closed item the same way is now an
				// unchanged success, and another disposition is still
				// refused; the renamed test carries both guarantees.
				requiredNames = []string{"TestReadItemsCloseRepeatsAndRefusesAnotherDisposition"}
			}
			if nowSelectsAll {
				automaticallyCovered = append(automaticallyCovered, requiredNames...)
			} else {
				containsAll(old.ID+" tests", nowTests, requiredNames)
			}
		}
		if len(automaticallyCovered) != 0 {
			probe := now
			probe.Tests, err = json.Marshal(automaticallyCovered)
			if err != nil {
				t.Fatal(err)
			}
			probeContract := testpolicy.Contract{SchemaVersion: current.SchemaVersion, Groups: []testpolicy.Group{probe}}
			if err := proofrun.CheckNativeDiscovery(t.Context(), projectRoot, installation, probeContract, os.Environ()); err != nil {
				t.Errorf("%s automatic test selection dropped mandatory legacy names: %v", old.ID, err)
			}
		}
		old.Inputs, now.Inputs = nil, nil
		old.Packages, now.Packages = nil, nil
		old.Tests, now.Tests = nil, nil
		old.Requires, now.Requires = nil, nil
		old.Phase, now.Phase = "", ""
		old.EnvironmentMode, now.EnvironmentMode = "", ""
		old.Resources, now.Resources = testpolicy.GroupResources{}, testpolicy.GroupResources{}
		old.Freshness, now.Freshness = "", ""
		old.FreshnessMaxAgeMS, now.FreshnessMaxAgeMS = nil, nil
		// A selector section became a declared script bed (U7b-1): the argv is
		// the one reviewed addition.
		// Its bed may declare the further tools that argv needs.
		if old.Adapter == "section" && now.Adapter == "section" && len(old.Argv) == 0 && len(now.Argv) != 0 {
			now.Argv = nil
			var kept []testpolicy.Tool
			for _, tool := range now.Tools {
				if slices.ContainsFunc(old.Tools, func(prior testpolicy.Tool) bool { return reflect.DeepEqual(prior, tool) }) {
					kept = append(kept, tool)
				}
			}
			if len(kept) == len(old.Tools) {
				now.Tools = old.Tools
			}
		}
		if !reflect.DeepEqual(old, now) {
			t.Errorf("legacy group %s changed its native definition outside the reviewed migration fields", old.ID)
		}
	}
}

func TestGitAdapterGoManifestInventoryMatchesNativeGoList(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace := gittree.Workspace{Dir: root}
	base, err := workspace.TreeOf("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	detached, err := workspace.NewDetachedWorktree(base)
	if err != nil {
		t.Fatal(err)
	}
	defer detached.Close()
	module := filepath.Join(detached.Workspace().Dir, "metasystem")
	modPath := filepath.Join(module, "go.mod")
	modBytes, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modPath, append(modBytes, []byte("\n// host inventory probe\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	testingFixtureGit(t, detached.Workspace().Dir, "add", "metasystem/go.mod")
	candidate := strings.TrimSpace(testingFixtureGit(t, detached.Workspace().Dir, "write-tree"))
	selection, err := gopackages.Select(filepath.Join(root, "metasystem"), base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "list", "./...")
	command.Dir = module
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list current Go packages: %v: %s", err, output)
	}
	var want []string
	for _, imported := range strings.Fields(string(output)) {
		pkg := "."
		if imported != selection.ModulePath {
			pkg = "./" + strings.TrimPrefix(imported, selection.ModulePath+"/")
		}
		want = append(want, pkg)
	}
	slices.Sort(want)
	if !slices.Equal(selection.Packages, want) {
		t.Fatalf("current package inventory differs from native go list: selected=%v native=%v", selection.Packages, want)
	}
}
