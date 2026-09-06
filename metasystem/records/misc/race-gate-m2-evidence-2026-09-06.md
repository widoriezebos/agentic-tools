# Race-gate evidence from m2, 2026-09-06

Gathered on m2 for seat m1c, goal race-gate-red-on-main. Nothing was fixed,
claimed, or run beyond the two greps and one Go test run below.

## Machine

- Host: Widos-MacBook-Pro.local (m2)
- Cores: 16 (sysctl -n hw.ncpu)
- Memory: 34359738368 bytes (sysctl -n hw.memsize), 32 GiB
- Go: go1.26.6 darwin/amd64

## Commit and uptime before the fresh run

- Commit: 4aa9f08e (plain main, pulled with --ff-only at 2026-09-06T08:28Z)
- Uptime line, taken at 2026-09-06T08:28:13Z before the run started:

    10:28  up 11:03, 4 users, load averages: 2.06 2.24 2.12

## 1. The old evidence, 2026-09-04 21:13Z

The log of that run no longer exists. artifacts/agents/gate-failures/ holds
five logs, the newest from 2026-08-31 (20260831T123305Z-13638.log); none
mentions TestArmConfirmsTheGuardAndDisarmEndsIt. The run was the adopt
fixture's Go gate, launched by the m2 seat session f124c7ef-788a-4c69-9218-b2f45f3ee52e
from its scratchpad worktree wt-main4, with the log tee'd to that
session's scratchpad at scratchpad/main-adopt.log. That scratchpad directory
has been removed; the gate's own coverage log was a mktemp file and is gone
too. No copy exists under artifacts/agents/suite-failures for 2026-09-04.

What survives is the session transcript's grep output over that log. The
seat never printed the failure block itself, so the FAIL block cannot be
reproduced verbatim. The surviving lines, verbatim from the transcript's
tool results (line numbers are line numbers in the lost main-adopt.log):

    $ grep -n "^FAIL\|^--- FAIL\|panic: test timed out" main-adopt.log        # 2026-09-04T21:14:00Z
    32:panic: test timed out after 30m0s
    87:FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/goal	1802.236s
    173:--- FAIL: TestNestedCheckoutMissionBirth (27.19s)
    329:panic: test timed out after 30m0s
    526:FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner	1800.787s
    535:--- FAIL: TestHCL03EveryCodeRowed (1.39s)
    538:FAIL
    540:FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/refusal	2.267s
    550:--- FAIL: TestArmConfirmsTheGuardAndDisarmEndsIt (8.86s)
    552:FAIL
    554:FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/steward	1042.060s
    562:FAIL

    $ grep -n -B2 -A6 "^FAIL.*internal/goal" main-adopt.log | head -14          # 2026-09-04T21:13:45Z
    85-created by testing.(*T).Run in goroutine 1
    86-	/usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2101 +0xb13
    87:FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/goal	1802.236s
    88-ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget	2.467s	coverage: 96.1% of statements
    89-ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision	5.784s	coverage: 68.1% of statements
    90-ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/governance	2.290s	coverage: 100.0% of statements
    91-ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/hooks	3.348s	coverage: 88.9% of statements
    92-ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/host	16.171s	coverage: 81.4% of statements
    93-ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority	35.231s	coverage: 75.7% of statements

    $ tail -3 main-adopt.log                                                    # 2026-09-04T21:09:49Z, mid-run
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth	4.643s	coverage: 83.0% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun	66.704s	coverage: 80.7% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/gittree	221.564s	coverage: 68.5% of statements

Readable facts from those lines: the steward test itself took 8.86s and the
package reported FAIL after 1042.060s, so the package's time was spent in
other tests, not in the failing one; that run was not verbose (-v absent),
so line 550 to 562 held only the failure message and the package trailer,
twelve lines in all; the run was at f40fcf50 with -timeout 30m and without
-count=1, as the adopt fixture's gate step, on what the seat described at
the time as a quiet Mac.

## 2. Fresh full race gate on plain main, 2026-09-06

Command, run from the metasystem directory at 08:28:13Z, finished 08:56:53Z
(28m40s wall clock), exit status 1:

    go test -race -cover -count=1 -v -timeout 60m ./internal/... > /tmp/race-gate-m2.log 2>&1

The sixty-minute ceiling was never reached: no package printed
"panic: test timed out". Uptime line after the run:

    10:57  up 11:33, 4 users, load averages: 3.61 4.30 4.24

The steward test did NOT fail again. Its whole verbose block:

    === RUN   TestArmConfirmsTheGuardAndDisarmEndsIt
    --- PASS: TestArmConfirmsTheGuardAndDisarmEndsIt (6.83s)

Red packages: internal/goal (two data races, TestFreshLedgerFailureAndFetchTimeoutBlockTheStop
and TestWatchdogProtocol), internal/missionrunner (TestTerminateGroup, a
real assertion failure, not a timeout), internal/refusal
(TestHCL03EveryCodeRowed). Everything else is green.

### Package lines (grep -E '^(ok|FAIL|panic)')

    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/acp	18.082s	coverage: 90.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/adapter	91.780s	coverage: 86.4% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/atif	5.405s	coverage: 77.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile	6.162s	coverage: 77.5% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/audit	4.647s	coverage: 93.1% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/authority	4.194s	coverage: 97.7% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface	35.111s	coverage: 81.5% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec	5.661s	coverage: 92.3% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/capability	4.399s	coverage: 79.4% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/census	29.870s	coverage: 76.4% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/channel	174.148s	coverage: 82.6% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake	13.139s	coverage: 90.7% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase	5.799s	coverage: 52.7% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/channel/slack	4.599s	coverage: 86.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/channel/telegram	7.926s	coverage: 85.6% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/config	11.486s	coverage: 85.9% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/contract	203.006s	coverage: 65.3% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/counselor	8.929s	coverage: 84.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/covenant	6.242s	coverage: 82.4% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/critique	5.847s	coverage: 96.2% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/delegate	5.748s	coverage: 83.1% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch	219.412s	coverage: 77.2% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/events	5.982s	coverage: 82.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/evidence	9.720s	coverage: 80.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/evidencetable	6.572s	coverage: 87.6% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth	4.219s	coverage: 83.0% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun	36.233s	coverage: 80.5% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/gittree	139.345s	coverage: 68.5% of statements
    FAIL
    FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/goal	1678.306s
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget	3.784s	coverage: 96.1% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision	6.828s	coverage: 68.1% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/governance	4.665s	coverage: 100.0% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/hooks	4.717s	coverage: 88.9% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/host	10.002s	coverage: 81.4% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority	47.110s	coverage: 75.7% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/identity	4.058s	coverage: 88.3% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/janitor	4.498s	coverage: 94.0% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit	4.286s	coverage: 96.9% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/landing	275.552s	coverage: 77.2% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/lease	62.691s	coverage: 79.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/lock	8.354s	coverage: 88.0% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/metrics	152.384s	coverage: 88.6% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/mission	150.855s	coverage: 73.9% of statements
    FAIL
    FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner	1476.350s
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/missionstate	2.900s	coverage: 78.9% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest	5.717s	coverage: 77.3% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate	5.841s	coverage: 75.6% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/outage	4.172s	coverage: 85.7% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass	2.297s	coverage: 74.2% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/progress	3.667s	coverage: 75.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun	7.098s	coverage: 72.3% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/receipt	4.354s	coverage: 94.6% of statements
    FAIL
    FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/refusal	1.499s
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/registry	4.060s	coverage: 91.5% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/report	17.672s	coverage: 85.6% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt	4.403s	coverage: 79.7% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema	3.468s	coverage: 82.1% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/run	13.694s	coverage: 75.5% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes	2.959s	coverage: 90.5% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/spend	24.168s	coverage: 83.9% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot	7.034s	coverage: 89.0% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/steward	683.224s	coverage: 78.3% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/supervise	61.671s	coverage: 89.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/turn	1.614s	coverage: 100.0% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/up	5.352s	coverage: 56.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/usage	7.763s	coverage: 86.8% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/validate	125.058s	coverage: 81.2% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/watch	5.702s	coverage: 70.1% of statements
    ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc	1.882s	coverage: 87.9% of statements
    FAIL

### Slowest tests per package of interest (from the -v output)

    internal/goal: package 1678.306s, 353 top-level tests, sum of test times 1675.6s
           58.39s  PASS  TestUnapproveWithdrawsApprovedAndClaimedWork
           48.09s  PASS  TestMixedArcCascadesMoveOnlyEligibleMembers
           47.51s  PASS  TestMixedArcJoinUsesOwnPairOrNewestAllParkedRecord
           38.08s  PASS  TestSetArcComposesMovesUnderTheMatrix
           36.25s  PASS  TestAuthenticatedChannelApprovalRequiresTheTokenOnce
           35.07s  PASS  TestReopenAdoptsTheArcState
           34.41s  PASS  TestClaimedArcToForeignClaimedArcLandsQueuedOnBothSurfaces
           27.94s  PASS  TestSplitPreconditionsRefuseByNameAndHumanOriginInherits
    internal/missionrunner: package 1476.350s, 295 top-level tests, sum of test times 1474.3s
           78.81s  PASS  TestInternalRunFullCycle
           57.18s  PASS  TestInternalRunCleanExitOverloadDocumentStaysOffTheBreaker
           55.81s  PASS  TestInternalRunDispatchTerminalCycle
           52.86s  PASS  TestResolveTaintRestore
           52.81s  PASS  TestInternalRunOverloadedHostStaysOffTheBreaker
           52.77s  PASS  TestStillbornInitCleansItsArtifacts
           44.92s  PASS  TestSealedBaselineBirthsAndRuns
           42.82s  PASS  TestLostStateFreezesTheBornMission
    internal/steward: package 683.224s, 204 top-level tests, sum of test times 681.0s
          272.76s  PASS  TestNarrationCapsItsHistory
           43.79s  PASS  TestClaimedGoalDeliveryVerdicts
           17.03s  PASS  TestLedgerAttentionInitializesFrontierAfterLocalOrPreBootstrapState
           16.69s  PASS  TestLedgerAttentionPinsAndQueueSequenceChanges
           16.45s  PASS  TestNotifyVerdictsReachTheQueue
           16.40s  PASS  TestProviderOutagePausesTheAging
           15.03s  PASS  TestLedgerAttentionPreBootstrapSaveRetiresOldFrontier
           14.58s  PASS  TestLedgerAttentionFetchTimeoutLeavesAcceptedAndTransportDead
        -> TestArmConfirmsTheGuardAndDisarmEndsIt: PASS in 6.83s

### Every FAIL block (grep -n -B3 -A40 -- '--- FAIL: ')

Line numbers are line numbers in /tmp/race-gate-m2.log on m2.

    3491-==================
    3492-=== NAME  TestFreshLedgerFailureAndFetchTimeoutBlockTheStop
    3493-    testing.go:1712: race detected during execution of test
    3494:--- FAIL: TestFreshLedgerFailureAndFetchTimeoutBlockTheStop (1.67s)
    3495-    --- PASS: TestFreshLedgerFailureAndFetchTimeoutBlockTheStop/fetch_failure (0.66s)
    3496-    --- PASS: TestFreshLedgerFailureAndFetchTimeoutBlockTheStop/fetch_timeout (0.69s)
    3497-=== RUN   TestMissingAcceptedGoalTreeReferenceBlocksAsUncertainty
    3498---- PASS: TestMissingAcceptedGoalTreeReferenceBlocksAsUncertainty (0.41s)
    3499-=== RUN   TestUnreadableTurnVerdictStateBlocksAsUncertainty
    3500---- PASS: TestUnreadableTurnVerdictStateBlocksAsUncertainty (0.76s)
    3501-=== RUN   TestMissingTemplateStateRootBlocksAsUncertainty
    3502---- PASS: TestMissingTemplateStateRootBlocksAsUncertainty (0.00s)
    3503-=== RUN   TestTemplateCheckoutTurnVerdictUsesTheMetasystemStateRoot
    3504---- PASS: TestTemplateCheckoutTurnVerdictUsesTheMetasystemStateRoot (1.33s)
    3505-=== RUN   TestValidSessionStopBypassesHangingFetchAndSpendsExactlyOnce
    3506---- PASS: TestValidSessionStopBypassesHangingFetchAndSpendsExactlyOnce (2.10s)
    3507-=== RUN   TestFetchDeadlineLeavesNewlyValidSessionStopUnspent
    3508---- PASS: TestFetchDeadlineLeavesNewlyValidSessionStopUnspent (2.14s)
    3509-=== RUN   TestBlockedHumanVerdictLeavesValidSessionStopUnspent
    3510---- PASS: TestBlockedHumanVerdictLeavesValidSessionStopUnspent (0.79s)
    3511-=== RUN   TestSessionStopLibraryAndConsumerRequireHumanClassificationProof
    3512---- PASS: TestSessionStopLibraryAndConsumerRequireHumanClassificationProof (0.40s)
    3513-=== RUN   TestSessionStopIsHolderBoundSingleUseAndConsumedAcrossQuietTurns
    3514---- PASS: TestSessionStopIsHolderBoundSingleUseAndConsumedAcrossQuietTurns (5.34s)
    3515-=== RUN   TestSessionStopConsumeOnUseSurvivesAbsentOrFailedSessionEnd
    3516-=== RUN   TestSessionStopConsumeOnUseSurvivesAbsentOrFailedSessionEnd/SessionEnd_did_not_run
    3517-=== RUN   TestSessionStopConsumeOnUseSurvivesAbsentOrFailedSessionEnd/SessionEnd_failed_before_it_could_inspect_the_marker
    3518---- PASS: TestSessionStopConsumeOnUseSurvivesAbsentOrFailedSessionEnd (4.85s)
    3519-    --- PASS: TestSessionStopConsumeOnUseSurvivesAbsentOrFailedSessionEnd/SessionEnd_did_not_run (2.43s)
    3520-    --- PASS: TestSessionStopConsumeOnUseSurvivesAbsentOrFailedSessionEnd/SessionEnd_failed_before_it_could_inspect_the_marker (2.42s)
    3521-=== RUN   TestSessionStopConsumeErrorBlocksBeforeAuthorization
    3522---- PASS: TestSessionStopConsumeErrorBlocksBeforeAuthorization (0.87s)
    3523-=== RUN   TestSessionStopCannotCrossASessionEndWithoutAStopHook
    3524---- PASS: TestSessionStopCannotCrossASessionEndWithoutAStopHook (1.85s)
    3525-=== RUN   TestSessionEndDurablySpendsUnusedStopAuthorizationBeforeAnnouncementRetirement
    3526---- PASS: TestSessionEndDurablySpendsUnusedStopAuthorizationBeforeAnnouncementRetirement (1.88s)
    3527-=== RUN   TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat
    3528---- PASS: TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat (4.68s)
    3529-=== RUN   TestVerdictDualSlotSequence
    3530---- PASS: TestVerdictDualSlotSequence (0.48s)
    3531-=== RUN   TestPrecedenceLadder
    3532---- PASS: TestPrecedenceLadder (0.27s)
    3533-=== RUN   TestAbsenceAdvisoryVsDeletionDegraded
    3534---- PASS: TestAbsenceAdvisoryVsDeletionDegraded (0.12s)
    --
    3674-      /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2101 +0x38
    3675-==================
    3676-    testing.go:1712: race detected during execution of test
    3677:--- FAIL: TestWatchdogProtocol (1.18s)
    3678-=== RUN   TestInventoryFailureVetoes
    3679---- PASS: TestInventoryFailureVetoes (0.06s)
    3680-=== RUN   TestUnwatchedAndWarnings
    3681---- PASS: TestUnwatchedAndWarnings (0.47s)
    3682-=== RUN   TestGreenPrefixConsistency
    3683---- PASS: TestGreenPrefixConsistency (0.35s)
    3684-=== RUN   TestHumanOwnsHumanRuns
    3685---- PASS: TestHumanOwnsHumanRuns (0.18s)
    3686-=== RUN   TestGreenCursorRereadsDisk
    3687---- PASS: TestGreenCursorRereadsDisk (0.36s)
    3688-=== RUN   TestTurnVerdictConvertedClaimHasTheFloor
    3689---- PASS: TestTurnVerdictConvertedClaimHasTheFloor (2.27s)
    3690-=== RUN   TestTurnVerdictConvertedBudgetlessQueueIsQuiet
    3691---- PASS: TestTurnVerdictConvertedBudgetlessQueueIsQuiet (1.32s)
    3692-=== RUN   TestTurnVerdictConvertedFreshFreeIsAllClear
    3693---- PASS: TestTurnVerdictConvertedFreshFreeIsAllClear (1.37s)
    3694-=== RUN   TestPublishLandsWithParentAndTrailer
    3695---- PASS: TestPublishLandsWithParentAndTrailer (1.69s)
    3696-=== RUN   TestSameTargetRaceExactlyOneWins
    3697---- PASS: TestSameTargetRaceExactlyOneWins (1.79s)
    3698-=== RUN   TestLeaseRefusalOnMidflightCompetitor
    3699---- PASS: TestLeaseRefusalOnMidflightCompetitor (2.35s)
    3700-=== RUN   TestBenignAdvancementRetriesWithinTheDeadline
    3701---- PASS: TestBenignAdvancementRetriesWithinTheDeadline (3.17s)
    3702-=== RUN   TestPushedEntryBlocksTheWholeClone
    3703---- PASS: TestPushedEntryBlocksTheWholeClone (0.46s)
    3704-=== RUN   TestValidationRefusalIsRejectedByName
    3705---- PASS: TestValidationRefusalIsRejectedByName (1.07s)
    3706-=== RUN   TestSingleMachineModeCASNeverMovesHead
    3707---- PASS: TestSingleMachineModeCASNeverMovesHead (1.03s)
    3708-=== RUN   TestPushFailureClassifierSeparatesLeaseFromTransport
    3709---- PASS: TestPushFailureClassifierSeparatesLeaseFromTransport (0.00s)
    3710-=== RUN   TestTransactionsAddressTheTreeUnderASubdirectoryRoot
    3711---- PASS: TestTransactionsAddressTheTreeUnderASubdirectoryRoot (2.86s)
    3712-=== RUN   TestABrokenAcceptedRefRefusesMutations
    3713---- PASS: TestABrokenAcceptedRefRefusesMutations (1.68s)
    3714-=== RUN   TestAMalformedAcceptedRefFileRefusesMutations
    3715---- PASS: TestAMalformedAcceptedRefFileRefusesMutations (1.49s)
    3716-=== RUN   TestADanglingAcceptedRefSymlinkRefusesMutations
    3717---- PASS: TestADanglingAcceptedRefSymlinkRefusesMutations (1.49s)
    --
    5341-host process group 30607 is not provably ours; leaving it to the census rather than signaling an unowned group
    5342-host process group 30609 is not provably ours; leaving it to the census rather than signaling an unowned group
    5343-    host_process_test.go:207: the owned group survived its wind-down
    5344:--- FAIL: TestTerminateGroup (5.02s)
    5345-=== RUN   TestSmallPureHelpers
    5346---- PASS: TestSmallPureHelpers (0.00s)
    5347-=== RUN   TestDeniedCapabilitiesActNowhere
    5348---- PASS: TestDeniedCapabilitiesActNowhere (0.00s)
    5349-=== RUN   TestEngineFixtureConstructionRefusal
    5350---- PASS: TestEngineFixtureConstructionRefusal (0.00s)
    5351-=== RUN   TestInternalRunFailRamp
    5352---- PASS: TestInternalRunFailRamp (0.54s)
    5353-=== RUN   TestActiveJobs
    5354---- PASS: TestActiveJobs (0.00s)
    5355-=== RUN   TestCloseableChains
    5356---- PASS: TestCloseableChains (0.00s)
    5357-=== RUN   TestCloseableChainsRootAlone
    5358---- PASS: TestCloseableChainsRootAlone (0.00s)
    5359-=== RUN   TestActiveJobsSeesUnstampedReservationHusks
    5360---- PASS: TestActiveJobsSeesUnstampedReservationHusks (0.00s)
    5361-=== RUN   TestCloseableChainsSkipsDispatchRefusedHusks
    5362---- PASS: TestCloseableChainsSkipsDispatchRefusedHusks (0.00s)
    5363-=== RUN   TestCloseTerminalChainsFinishesTheSweepPastAFailure
    5364---- PASS: TestCloseTerminalChainsFinishesTheSweepPastAFailure (0.42s)
    5365-=== RUN   TestDrainReapCadenceIsDecoupled
    5366---- PASS: TestDrainReapCadenceIsDecoupled (0.66s)
    5367-=== RUN   TestLaunchGuardLadder
    5368---- PASS: TestLaunchGuardLadder (0.00s)
    5369-=== RUN   TestArmAndPreflightRefusals
    5370---- PASS: TestArmAndPreflightRefusals (0.80s)
    5371-=== RUN   TestAcquireLeaseLifecycle
    5372---- PASS: TestAcquireLeaseLifecycle (0.31s)
    5373-=== RUN   TestVerifyStateShapeRefusals
    5374---- PASS: TestVerifyStateShapeRefusals (0.00s)
    5375-=== RUN   TestMissionLeaseAcquireAndCleanupShareOneLock
    5376---- PASS: TestMissionLeaseAcquireAndCleanupShareOneLock (0.65s)
    5377-=== RUN   TestPreWallStateRefusesResumeByName
    5378-=== RUN   TestPreWallStateRefusesResumeByName/version-1
    5379-=== RUN   TestPreWallStateRefusesResumeByName/version-2
    5380-=== RUN   TestPreWallStateRefusesResumeByName/version-3
    5381-=== RUN   TestPreWallStateRefusesResumeByName/missing-baseline
    5382---- PASS: TestPreWallStateRefusesResumeByName (0.01s)
    5383-    --- PASS: TestPreWallStateRefusesResumeByName/version-1 (0.00s)
    5384-    --- PASS: TestPreWallStateRefusesResumeByName/version-2 (0.00s)
    --
    6338-=== RUN   TestHCL03EveryCodeRowed
    6339-    register_test.go:40: collected refusal token "VERIFIED_CHANNEL_ANSWER" has no row or exclusion
    6340-    register_test.go:48: collected 171 refusal-shaped tokens
    6341:--- FAIL: TestHCL03EveryCodeRowed (0.91s)
    6342-=== RUN   TestHCL03EveryRowReal
    6343---- PASS: TestHCL03EveryRowReal (0.01s)
    6344-=== RUN   TestHCL03PendingRowsNamed
    6345---- PASS: TestHCL03PendingRowsNamed (0.00s)
    6346-FAIL
    6347-coverage: [no statements]
    6348-FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/refusal	1.499s
    6349-=== RUN   TestBindingAndClosureSurviveCompaction
    6350---- PASS: TestBindingAndClosureSurviveCompaction (0.00s)
    6351-=== RUN   TestWatermarkGovernsGenerationRetention
    6352---- PASS: TestWatermarkGovernsGenerationRetention (0.00s)
    6353-=== RUN   TestCleanClosedClaimsAreDropped
    6354---- PASS: TestCleanClosedClaimsAreDropped (0.00s)
    6355-=== RUN   TestSweepableClaimSurvivesUntilSwept
    6356---- PASS: TestSweepableClaimSurvivesUntilSwept (0.00s)
    6357-=== RUN   TestUnboundCustodyGraceWindow
    6358---- PASS: TestUnboundCustodyGraceWindow (0.00s)
    6359-=== RUN   TestTornMarkersAndFragmentsCompactAway
    6360---- PASS: TestTornMarkersAndFragmentsCompactAway (0.00s)
    6361-=== RUN   TestCompactionFailsClosedOnCorruption
    6362---- PASS: TestCompactionFailsClosedOnCorruption (0.00s)
    6363-=== RUN   TestHealthyOperationCompactsToOneGeneration
    6364---- PASS: TestHealthyOperationCompactsToOneGeneration (0.00s)
    6365-=== RUN   TestWriteCompactedRewritesAtomically
    6366---- PASS: TestWriteCompactedRewritesAtomically (0.22s)
    6367-=== RUN   TestCorruptionErrorNamesBothLines
    6368---- PASS: TestCorruptionErrorNamesBothLines (0.00s)
    6369-=== RUN   TestAppendFrameSurfacesOpenFailure
    6370---- PASS: TestAppendFrameSurfacesOpenFailure (0.10s)
    6371-=== RUN   TestReadFramesSurfacesReadFailure
    6372---- PASS: TestReadFramesSurfacesReadFailure (0.10s)
    6373-=== RUN   TestReadFramesSkipsBlankAndWhitespaceLines
    6374---- PASS: TestReadFramesSkipsBlankAndWhitespaceLines (0.06s)
    6375-=== RUN   TestWriteCompactedFailsIntoMissingDirectory
    6376---- PASS: TestWriteCompactedFailsIntoMissingDirectory (0.04s)
    6377-=== RUN   TestWriteCompactedEmptyKeepsEmptyFile
    6378---- PASS: TestWriteCompactedEmptyKeepsEmptyFile (0.35s)
    6379-=== RUN   TestCustodyRecordTimeUnparseableMeansZero
    6380---- PASS: TestCustodyRecordTimeUnparseableMeansZero (0.00s)
    6381-=== RUN   TestNumberAcceptsIntegerKinds

### Context the grep above cuts off

The goal package's two failures are race-detector reports that precede
the FAIL line. TestFreshLedgerFailureAndFetchTimeoutBlockTheStop, log
lines 3435 to 3497:

    === RUN   TestFreshLedgerFailureAndFetchTimeoutBlockTheStop
    === RUN   TestFreshLedgerFailureAndFetchTimeoutBlockTheStop/fetch_failure
    === RUN   TestFreshLedgerFailureAndFetchTimeoutBlockTheStop/fetch_timeout
    ==================
    WARNING: DATA RACE
    Write at 0x0000058dbf40 by goroutine 54391:
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.TestFreshLedgerFailureAndFetchTimeoutBlockTheStop.func1()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict_idle_test.go:85 +0x39
      testing.(*common).Cleanup.func1()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:1317 +0x168
      testing.(*common).runCleanup()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:1667 +0x225
      testing.tRunner.func2()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2030 +0x4c
      runtime.deferreturn()
          /usr/local/Cellar/go/1.26.6/libexec/src/runtime/panic.go:668 +0x5d
      testing.(*T).Run.gowrap1()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2101 +0x38

    Previous read at 0x0000058dbf40 by goroutine 54460:
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.fetchProjectionWithinDeadline.func1()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/project.go:103 +0x7e

    Goroutine 54391 (running) created at:
      testing.(*T).Run()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2101 +0xb12
      testing.runTests.func1()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2585 +0x84
      testing.tRunner()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2036 +0x21c
      testing.runTests()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2583 +0x9e9
      testing.(*M).Run()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2443 +0xf4b
      main.main()
          _testmain.go:760 +0x164

    Goroutine 54460 (finished) created at:
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.fetchProjectionWithinDeadline()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/project.go:102 +0x1bc
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.Project()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/project.go:52 +0x15c
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.readClaimableBudgetedWork()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/project.go:298 +0x30b
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.(*Store).TurnVerdict.func1()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict.go:181 +0x3c4
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.(*Store).withLock()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:117 +0x927
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.(*Store).TurnVerdict()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict.go:173 +0x34d
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.TestFreshLedgerFailureAndFetchTimeoutBlockTheStop.func3()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict_idle_test.go:109 +0x196
      testing.tRunner()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2036 +0x21c
      testing.(*T).Run.gowrap1()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2101 +0x38
    ==================
    === NAME  TestFreshLedgerFailureAndFetchTimeoutBlockTheStop
        testing.go:1712: race detected during execution of test
    --- FAIL: TestFreshLedgerFailureAndFetchTimeoutBlockTheStop (1.67s)
        --- PASS: TestFreshLedgerFailureAndFetchTimeoutBlockTheStop/fetch_failure (0.66s)
        --- PASS: TestFreshLedgerFailureAndFetchTimeoutBlockTheStop/fetch_timeout (0.69s)
    === RUN   TestMissingAcceptedGoalTreeReferenceBlocksAsUncertainty

TestWatchdogProtocol, log lines 3547 to 3577 (the first of its race
reports; the rest repeat the same pair of frames):

    === RUN   TestWatchdogProtocol
    ==================
    WARNING: DATA RACE
    Write at 0x00c000384330 by goroutine 58416:
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.(*Store).TurnVerdict()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict.go:171 +0x1d6
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.TestWatchdogProtocol.func1()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict_test.go:397 +0x14c

    Previous read at 0x00c000384330 by goroutine 58418:
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.(*Store).TurnVerdict()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict.go:167 +0x184
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.TestWatchdogProtocol.func1()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict_test.go:397 +0x14c

    Goroutine 58416 (running) created at:
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.TestWatchdogProtocol()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict_test.go:395 +0x812
      testing.tRunner()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2036 +0x21c
      testing.(*T).Run.gowrap1()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2101 +0x38

    Goroutine 58418 (running) created at:
      github.com/widoriezebos/agentic-tools/metasystem/internal/goal.TestWatchdogProtocol()
          /Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/turnverdict_test.go:395 +0x812
      testing.tRunner()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2036 +0x21c
      testing.(*T).Run.gowrap1()
          /usr/local/Cellar/go/1.26.6/libexec/src/testing/testing.go:2101 +0x38
    ==================

TestTerminateGroup in internal/missionrunner, the whole block, log lines
5340 to 5344:

    === RUN   TestTerminateGroup
    host process group 30607 is not provably ours; leaving it to the census rather than signaling an unowned group
    host process group 30609 is not provably ours; leaving it to the census rather than signaling an unowned group
        host_process_test.go:207: the owned group survived its wind-down
    --- FAIL: TestTerminateGroup (5.02s)
