# Covenant harvest survey: the paper as the metasystem's own intent

Status: survey, read-only. Companion draft: `plans/covenant-draft.json` (inert; not
`covenant.json`). Nothing under `metasystem/` was changed. Date: 2026-09-21.

The question was whether `metasystem/docs/paper/` can be reverse engineered into the
covenant this repository does not have. The short answer: yes for about half of what
the paper commits to, and the other half is the interesting part, because it is where
the paper promises what the software does not yet demonstrate.

Two facts the human needs before anything is birthed:

1. **A live `covenant.json` without `docs/covenant-evidence.md` turns validation red.**
   `scripts/validate-metasystem.sh:719-732` runs `bin/metasystem covenant evidence`
   whenever `covenant.json` exists, and the evidence walk refuses a missing table
   (`internal/evidencetable/walk.go:93`, "does not exist in the tree"). The covenant and
   the evidence table are born together or not at all.
2. **The nested app root does not fit the wall's path model.** The metasystem's app
   root is `metasystem/` (that is where `metasystem.conf` lives and where validation
   looks for `covenant.json`), but the git top level is the parent. The wall compares
   guardrails against `git diff --name-only` paths (`internal/gittree/gittree.go:374`,
   no `--relative`), which carry the `metasystem/` prefix, while the covenant's
   self-custody line matches the bare name `covenant.json`
   (`internal/mission/guardrails.go:78`). Every adopted app so far sits at its git top
   level, so this never bit. For this repository it decides how the guardrail entries
   are spelled, and it may be a defect. Section 5 says what the draft assumes.

## 1. Method

**What was read.** `internal/covenant/covenant.go` in full first, so the shape of a
requirement was fixed before any chapter was opened. Then all twenty chapters of the
paper (`index.md` and `01` to `19`), in order, without sampling. Then the inception skill
for the harvest discipline (not run). Then the proof surface: `metasystem/testing.json`
(every group, its packages, its named tests, its obligations), `scripts/validate-metasystem.sh`
section by section, `metasystem.conf`, the mission covenant gate
(`internal/missionrunner/covenantgate.go`), the contract grammar
(`internal/contract/contract.go`, `measure.go`), the wall (`internal/mission/guardrails.go`,
`internal/missionrunner/wall.go`, `internal/validate/authorization.go`), the master
design's Project section (`plans/user-interface-design.md:473-490`), and the record the
paper's chapter 18 cites (`records/misc/delivery-process-reset-2026-09-17.md`).

**How a commitment was told from an argument.** A passage counts as a commitment when
it names a condition the delivery machinery must hold at a boundary, in terms an
observer could check: a refusal that happens, a record that exists, an actor that may
not act. The paper marks these itself with "must", "refuses", "never", "enforced", and
with the chapter taglines. A passage counts as argument when it explains why (the
ladder in chapter 1, the mimicry trap, the economics), describes what people do
(chapters 13 to 15), or states the design's direction (chapter 18, the appendix's
diagrams). The narrated session-expiry example was treated as illustration, never as a
claim about this software.

**Where the line was drawn.** A commitment made it into the draft only when (a) the
paper states it as binding, and (b) a named test or validation section in this
repository demonstrates it today. Every cited test was verified by name with
`grep '^func TestName('` and its group membership by reading `testing.json`; the file
and line of each is in the table. Where the paper's claim is broader than the proof, the
draft row quotes the narrower sentence the proof actually covers, and the survey says
so. Where no proof exists, the claim is in section 3, not the draft. That is the type's
own rule: a requirement whose proof "is missing or never runs is intent that floats"
(`covenant.go:54-57`).

**One deliberate reading.** The appendix's closing section ("Where the software stands
today", `19-appendix-functional-design.md:211-215`) is the paper's own list of gaps. It
was taken as authoritative unless the code contradicted it; two places where it seems
stale are in section 3 and in the final report.

**What "proof" means below.** A `testing.json` group id plus the named Go tests inside
it, or a named section of `scripts/validate-metasystem.sh`. Groups with `tests=all`
(for example `goal-full-coverage`, `runtime-owner-standard`, `governed-standard`,
`refusal-register-standard`) run every test in their packages, so a test cited under
them is exercised without being listed. `go-affected` runs every test of a changed
package and its consumers; a test cited under it alone runs when its package moves,
which is exactly when the proof matters, but it has no standing cadence, and the survey
marks those.

## 2. Candidate requirements

Chapter references are `NN:line` into `metasystem/docs/paper/`. Test references are
`file:line` under `metasystem/`. Groups are `testing.json` group ids.

| id | claim, in the paper's words | source | proof |
| --- | --- | --- | --- |
| `budget-stops-work` | "A budget is enforced: when it is exhausted, the machinery stops, records what it learned and then escalates or closes the attempt according to its authority." "Resuming needs a fresh complete budget from a human." | 05:63; 19:213 | `goal-decision-standard`: `TestBudgetTupleIsCompletePositiveAndCanonical` (internal/goal/budget_test.go:9), `TestElapsedBreachDurationAppliesGraceAtTheStopBoundary` (budget_test.go:30), `TestSetBudgetLiftsCompletedFenceInOneTransaction` (internal/goal/stop_test.go:271), `TestHolderSetBudgetRebindsEpochForProofAdmission` (cmd/metasystem/goalsync_mutations_test.go:217); `dispatch-proof-standard`: `TestProofOnlyGoalStopBatch` (internal/dispatch/proof_attempt_test.go:72), `TestOrdinaryProofSharesGoalBudget` (:35); `mission-decision-standard`: `TestProjectFences` (internal/missionrunner/cycle_test.go:46); `section/goal-cli-fixtures`. Obligation `budget-stop-authority`. |
| `approval-is-a-human-act` | "a claim refuses a goal that no human approved, and approval needs proof that a human gave it, either from the terminal they enrolled or by a verified word over the channel"; "an edit to an approved item invalidates the approval". | 19:137; 19:193; 13:79 | `goal-full-coverage`: `TestUnapprovedExecutionPathsRefuseApprovalRequired` (internal/goal/approval_test.go:404), `TestApprovedGoalClaimsAndPayloadEditsInvalidateBinding` (:317); `authority-standard`: `TestProofRequiresExactAgentFreeEnrolledAncestry` (internal/humanauthority/authority_test.go:121), `TestProofFailsClosedOnTerminalAndAncestryUncertainty` (:692), `TestStatusThreadTokenWithValidCodeApprovesMarkedGoal` (internal/channel/channel_test.go:1049), `TestActiveObligationRefusesIncompleteHumanAuthorization` (internal/governance/types_test.go:24), `TestDecideFailsClosedWithoutALawfulState` (:90); `section/authority-regression-fixtures`. Obligation `human-authority-integrity`. |
| `channel-word-is-verified` | "the node that committed the reply checks two things: that the sender is the authority's account, and that the one-time code the authority typed is current and unused." | 19:165 | `authority-standard`: `TestPollRejectsWrongUserNoCodeBadCodeReplay` (internal/channel/channel_test.go:927), `TestTOTPWindowAndReplay` (:74), `TestDelayedTOTPReplayRemainsConsumed` (:93), `TestPollAtomicallyConsumesTOTP` (:129), `TestInboundCheckpointSurvivesCrashAndDeduplicates` (:1014), `TestPollCrashRecoveryExactlyOnce` (:1316), `TestSecretsScrubbedFromErrors` (internal/channel/slack/slack_test.go:99). |
| `evidence-binds-to-the-exact-candidate` | "The evidence belongs to the exact candidate that was examined. When the builder revises that candidate, the old results cannot authorize the new one." | 01:51; 06:25 | `proof-standard`: `TestRetryDecisionIgnoresFailuresFromOtherCandidateTrees` (internal/proofrun/attempt_test.go:799), `TestAttemptRejectsCandidateTreeDisagreement` (:926), `TestCandidateTreeFieldsMustAgree` (:604), `TestExactReuseFollowsTheCandidatePair` (internal/proofrun/test_reuse_test.go:304), `TestForceAttemptReusesOnlyExactNewestGreenIdentity` (:205); `engine-binding-standard`: `TestCandidateEngineIsBuiltFromCandidateTreeAndBindsExecutionIdentity` (cmd/metasystem/test_test.go:462). Obligation `test-execution-integrity`. Also the local invariant at `docs/project-rules.md:48`. |
| `a-rule-change-never-judges-its-own-case` | "A rule change must never judge its own case." "The self-change needs an older basis that the proposal cannot alter." | 17:3; 17:21 | `engine-binding-standard`: `TestAmbientTrustedPolicyDecisionCannotBypassRetainedEngine` (cmd/metasystem/test_test.go:1684), `TestCandidateEngineBuildFailureCannotFallBackToPolicyEngine` (:969), `TestTrustedPolicyEngineIsRequiredWithoutBuildingDuringReadOnlySelection` (:452), `TestProtectedCoverageFloorCannotFallOrDisappear` (:1297); `policy-protection`: `TestBasePolicyAndBlackBoxGuardsSurviveCandidateRemovalAndInternalAPIChange` (internal/testpolicy/protection_test.go:9); `policy-canary`: `TestModesDoNotLowerRequiredRisk` (internal/testpolicy/select_test.go:127); `landing-command-standard`: `TestBatchJoinRefusesDroppedProtectedTest` (internal/landing/batch/gate_test.go:41), `TestLandingBatchJoinRefusesDroppedListedTest` (cmd/metasystem/landing_batch_join_test.go:114). Obligations `testing-policy-protected`, `test-execution-integrity`. |
| `the-examiner-is-fresh` | "Each new independent examiner receives the materials needed to examine the claim, but not the builder's private reasoning trace or path to the work." "Every build gets one independent read of the whole build in a fresh context." | 06:41; 18:80 | `launch-standard`: `TestEachRoundReadsFreshWithThePreviousReadAsInput` (internal/launch/unit_test.go:422), `TestProofRunsBeforeTheRead` (:244), `TestUnitRunNeverWritesToTheRepository` (:530); `goal-decision-standard`: `TestCommitReadRefusesUnrecordedFastGateRun` (cmd/metasystem/goal_branch_test.go:707), `TestGoalBranchReadRunsGateDispatchesAndCollectsClosedCritic` (internal/goal/branch/read_test.go:201); `go-affected` only: `TestHazardCloseRejectsStaleCriticClosure` (internal/dispatch/hazard_closure_test.go:721), `TestHazardEvidenceAdmissionAcceptsFreshContextFollowUp` (:515). Standing rule at `docs/project-rules.md:64`. |
| `examination-rounds-are-bounded` | "Repeated rounds stop for one of three explicit reasons: a bounded search completes without a new material issue, the judging budget is exhausted and forces escalation, or an open question requires a human ruling." | 06:45 | `cap-contract`: `TestBoundaryAtThreeRounds` (internal/dispatch/critique_chain_test.go:107), `TestCritiqueBeforeRoundLimitAllowsContinuation` (internal/dispatch/decisions_test.go:979), `TestCompletedAndFailedRoundsCountButCancelledDoesNot`, `TestCritiqueSevereTerminalBoundary`, `TestCritiqueExhaustionCodeCriticChain` (:1109); `metasystem.conf:20` `metasystem.budget.review-round-max=3`. Obligation `critique-cap-contract`. See contradiction C1 in section 3. |
| `refusals-name-the-fact` | "The refusal must identify the exact candidate, the condition that does not hold and the route by which work may continue." | 05:55; 06:23 | `refusal-register-standard` (internal/refusal, all): `TestHCL03EveryCodeRowed`, `TestHCL03EngineCausesEachCarryARemedy`, `TestHCL03EveryRowReal`; `engine-binding-standard`: `TestEngineRefusalsNameTheirCause` (cmd/metasystem/engine_refusal_test.go:14), `TestDecisionMismatchNamesTheField` (:154), `TestRefusalQuotesPathFacts` (internal/enginecause/cause_test.go:69). Obligation `gate-integrity`. |
| `silence-is-detected-and-deadlined` | "active work needs visible progress, a deadline and a recorded end state." "Every wait also has a deadline and an owner." | 03:35; 09:19 | `proof-standard`: `TestSupervisorLegitimateWaitDeadDumpAndReaderFailures` (internal/proofrun/supervisor_test.go:58), `TestRunawayAndDeadAreFailedDeliveryStatuses` (:399), `TestLauncherReapsSurvivorsIntoTheVerdict` (internal/proofrun/fixture_reap_test.go:60); `launch-standard`: `TestCancelProvesDeathBeforeItReportsCancelled` (internal/launch/launch_test.go:194), `TestStatusReconcilesALostSupervisor` (:265), `TestWaitHonoursTheCapAndReportsTheTerminalState`; `wait-stop-standard`: `TestWaitRegisterHumanNeedsAQuestionAndADeadline` (cmd/metasystem/wait_register_test.go:126), `TestLocalWaitOfADeadProcessDoesNotAllowTheStop` (internal/goal/turnverdict_localwait_test.go:59), `TestHumanWaitAllowsTheStopUntilItsDeadline` (:111); `runtime-owner-standard`: `TestCensusLockTakesOverDeadOwner` (internal/supervise/censuslock_test.go:111), `TestWatchdogDeadAcknowledgedStillShouts` (internal/supervise/acknowledged_test.go:265); `governed-standard`: `TestWitnessResolverStallsOnlyWhenAStepIsSilent` (internal/steward/rearm_clock_test.go:63); `section/supervision-and-census-fixtures`. Obligations `runtime-custody`, `governed-cadence`. |
| `identity-is-checked-at-use` | "An identifier that was accurate once is not authority now." | 09:27 | `batch-buildb-standard`: `TestSupervisorTakeoverRefusesStaleEpochProof` (cmd/metasystem/proof_run_test.go:1370), `TestLandingOwnerComponentStopsActingAfterLeaseLoss` (cmd/metasystem/supervise_component_run_test.go:336), `TestWorkerAuthorizedRefusesAForeignRoot` (proof_run_test.go:1044); `landing-command-standard`: `TestBatchTickStopsBeforePassAfterLeaseLoss` (cmd/metasystem/landing_batch_owner_seam_test.go:394); `runtime-owner-standard`: `TestDeadOwnerLockReleaseIsFencedByTheRecordedIdentity` (internal/supervise/arming_test.go:1736); `goal-decision-standard`: `TestCommitRefusesNonHolder` (internal/goal/branch/commit_test.go:260), `TestAmendRechecksClaimBeforeBranchUpdate` (:316), `TestLandPushRechecksClaimBeforePush` (internal/goal/branch/publish_test.go:121). Obligation `runtime-custody`. |
| `failure-leaves-a-record` | "A worker may disappear, but the fact that it failed and what it learned must not disappear with it." "Earlier states remain visible to authorized readers rather than being rewritten." | 05:75; 08:17 | `launch-standard`: `TestSuperviseRecordsTheTerminalStateFromTheExitStatus` (internal/launch/launch_test.go:175), `TestTerminalStateIsFinal` (:235), `TestRefusedLaunchLeavesNoRecordAndOneRefusalRow` (internal/launch/admit_test.go:103); `proof-standard`: `TestFailThenPassRerunIsAFindingNeverAGreen` (internal/proofrun/test_rerun_test.go:18), `TestFailThenFailStaysRedAndNamesBothRuns` (:101); `landing-command-standard`: `TestBatchHistoryIsAppendOnly` (internal/landing/batch/store_test.go:178), `TestFastForwardRefusesNonAppendRegisters` (internal/landing/advance_test.go:510). |
| `handoff-by-record` | "A coordinator keeps a bounded context and hands off by record at a fixed point well before the bound. The handoff note, not the context, carries the state." | 18:82; 08:41 | `context-standard`: `TestHandoffWritesAVerifiedStateFile` (internal/steward/handoff_capture_test.go:154), `TestHandoffPublicationIsExclusiveAndReverified`, `TestHandoffAcceptsOnlyTheActiveContinuation`, `TestTurnVerdictAllowsTheStopUnderARecordedHandoff`, `TestHandoffStateVerifierChecksLessonsNoteOrder`; `governed-standard`: `TestRevivalReapsAProvablyDeadContinuationBeforeComputingTheGuard` (internal/steward/revive_test.go:158); `goal-decision-standard`: `TestGoalHandoverAppliesCompleteFieldTable` (internal/goal/handover_test.go:148), `TestHandedOverRecordRequiresEveryCoordinate` (:44); `metasystem.conf:38-39` (`context.ceiling.tokens`, `context.handoff.margin.tokens`). |
| `isolation-by-construction` | "Isolation is a property of the work's surroundings rather than a request that workers avoid colliding." | 09:37 | `proof-standard`: `TestArchiveAndFrozenContextIdentity` (internal/proofrun/execution_context_test.go:14), `TestDetachedWorktreeHeadArchivesCandidate` (internal/gittree/detached_test.go:18); `batch-buildcd-standard`: `TestProofAttemptBinaryLaunchesUseSharedIsolation` (cmd/metasystem/proof_fixture_test.go:165); `launch-standard`: `TestUnitRunNeverWritesToTheRepository` (internal/launch/unit_test.go:530), `TestLaunchOutputsStayUnderTheStateDirectory`. |
| `a-question-parks-work-at-a-safe-stop` | "The work that raised the question stops, keeps its records and stays ready to resume; other work continues only within recorded permission and while reversal remains possible." | 13:95; 19:165 | `wait-stop-standard`: `TestNextStepNamesAPendingHumanWord` (internal/goal/humanword_test.go:5), `TestIdleBacklogContinuationSkipsAGoalWaitingOnAHumanWord` (internal/goal/turnverdict_idle_test.go:600), `TestIdleBacklogContinuationLeavesAHeldGoalThatWaitsOnAHumanWord`, `TestHumanWaitAllowsTheStopUntilItsDeadline`; `authority-standard`: `TestAskWritesRecordBeforePosting` (internal/channel/channel_test.go:171), `TestAskDedupsOpenQuestion` (:190). |
| `landings-are-batched-under-one-proof` | "Landings are batched. A lane opens when two builds are ready or ninety minutes have passed, and one proof covers the batch." | 18:84 | `batch-buildcd-standard`: `TestBatchLandingTransportRunsWholeSeriesOnce` (internal/landing/batch/land_test.go:57), `TestBatchMemberLandsEveryBuildInOrder` (:423), `TestBatchLandingLifecycleEndToEnd` (cmd/metasystem/landing_batch_e2e_test.go:64); `landing-command-standard`: `TestBatchSealFreezesMembership` (internal/landing/batch/seal_test.go:149), `TestBatchSealRegates`. The "two builds or ninety minutes" opening rule is not in a test; the one-proof-per-batch part is. Standing rule at `docs/project-rules.md:59-63`. |
| `a-run-required-to-fail` | "an examination can include runs that are required to fail, and a tool that reports success on everything is exposed by the case that had to fail." | 09:61; 06:3 | `section/gate-fail-open-tripwire` (`scripts/validate-metasystem.sh:912-928`): a broken `gofmt` shim must make `go-gate.sh --fast` refuse and name it. Obligation `gate-integrity`. One instance of the discipline, for one gate; see gap G5 for the general claim. |
| `examination-duty-follows-hazard-class` | "Dispatch fixes the minimum examination duty from a declared hazard class." "The chain's closure gate refuses to close work whose required examination is missing, stale or performed by a session that saw the builder's path." | 19:213 | `go-affected` only: `TestHazardConfigurationRefusesAWeakenedMinimum` (internal/dispatch/composition_test.go:1296), `TestHazardDutiesGateChainCompletion` (:1810), `TestHazardEvidenceMustCoverFinalWorkState` (:2039), `TestMixedHazardChainRetainsBuilderDuties` (:1779), `TestHazardCloseRejectsStaleCriticClosure` (internal/dispatch/hazard_closure_test.go:721); `cap-contract`: `TestCritiqueExhaustionCodeCriticChain`. No named group lists the hazard tests; they run only when `internal/dispatch` or a consumer changes. |

Seventeen rows made the draft. The claims below did not.

## 3. The gaps

Three kinds: **missing proof** (the software does it, nothing demonstrates it),
**not mechanically checkable** (the claim is real but no test could hold it), and
**not built** (the software does not do it yet, usually admitted by the paper itself).

| id | claim | source | kind | note |
| --- | --- | --- | --- | --- |
| G1 `spend-fence-refuses` | "when it is exhausted, the machinery stops" applied to money and tokens. | 05:63; 19:215 | not built | The appendix says it: "The spend fence measures tokens and alerts at a ceiling but does not yet refuse." `metasystem.conf:31-33`: "Spend accounting is alert-only." Elapsed, attempts, machine minutes and concurrency do stop (row 1); spend does not. |
| G2 `bounded-release-and-care` | The releaser exposes to a small part of traffic, compares against recorded bounds, contains or rolls back; the standing watch after release. | 10:7-19; 19:215 | not built | Admitted: "Bounded release, production observation against intent conditions and the care machinery are the design's direction; the software does not hold them yet." Chapter 10 has no software behind it. |
| G3 `separation-holds-at-the-action` | "the separations that matter must hold at the actions rather than in the actor's conduct: acceptance refuses work that no independent examination covers, and release refuses any candidate that has not passed acceptance, whoever asks." | 07:95; 19:215 | not built (partly) | The appendix admits the coordinator seat "combines dispatch, custody of landing and reporting in one actor, exactly the configuration Chapter 7 warns about, so until the queued guards land, that separation holds by conduct". The chain closure gate (row 17) is the part that exists. See also C2. |
| G4 `custodian-accepts-only-the-complete-chain` | The custodian "verifies that the candidate presented for acceptance is the one the independent examiner challenged, that every required result belongs to that candidate and that every enforced rule is satisfied. Then it performs only the authorized acceptance action." | 07:71; 19:213 | not built | Admitted: closure "binds that evidence to the final round of work, not yet to an exact candidate tree. Closure is custody of evidence: it is not the custodian's acceptance and not a release." |
| G5 `discriminating-checks` | "A discriminating test must pass on the supported version and fail on a relevant broken one." "A check that has never failed on a broken version has proved nothing yet." | 06:33; 06:3 | missing proof | Row 16 proves it for one gate. No general mechanism demonstrates that this repository's tests fail on a known-bad. The inception skill defers "full mutation adequacy" to the counselor's later work. This is the gap the covenant type's comment is about. |
| G6 `governance-record-per-rule` | Every enforced rule carries "a governance record names the owner (who may maintain or withdraw the rule), the review date and the appeal route"; the gate "checks three facts in the rule's governance record". | 12:35-37; 07:107 | not built | `testing.json` obligations have ids but no owner, review date, known-bad fixture or appeal route. `memory/rulings.md` carries rulings, not per-rule governance with dates. Nothing flags an overdue review. |
| G7 `four-risk-questions` | "Four questions set verification depth": severity, novelty, exposure, accumulated change. | 06:51; 11:21 | not checkable as stated | The software's risk vocabulary is severity, exposure, reversibility, detection, recovery (`testing.json:3`, per-surface `risk`), and `TestModesDoNotLowerRequiredRisk` proves depth cannot be lowered by mode. Novelty and accumulated change have no representation. Two of four are modelled, under different names. |
| G8 `narrator-changes-no-state` | "It changes no state and makes no decision." "Every material claim points back to the source record." | 07:45; 07:83 | missing proof | Reports and digests exist (`internal/counselor`, `TestRenderCarriesNamedLimitationsInNarrative`). No test asserts that report generation is read-only, and no test walks a report's claims back to records. |
| G9 `human-learning` | "The system must develop the human judgment it depends on"; practice cases with diagnosis withheld; assessment on unfamiliar cases. | 14:3; 19:215 | not built | Admitted: "The practice cases and assessments for human learning are also proposed work." |
| G10 `fleet-shared-inbox-and-pull` | First-commit-wins shared inbox, idle node pulls approved work, the answer archive, the brain. | 19:165; 19:193; 19:215 | not built | Admitted in the appendix's last paragraph. Row 3 covers the verification of one reply on one machine; the fleet half is design. |
| G11 `process-is-measured` | "The measure of the process is hours from a goal's opening to its landing and weighted tokens per landed changed line. Both are recorded". | 18:86 | not built | The cited record says the durable form "is goal spend-fence-reports-tokens-per-model-and-cause (priority 1). The scan scripts live in seat m1e's scratchpad until then" (`records/misc/delivery-process-reset-2026-09-17.md:121-122`). `metasystem launch report` records per-launch usage (`TestReportListsDesignAndReadUsageAgainstTheBaseline`), not the two named measures. "Both are recorded" is not true today. |
| G12 `no-model-context-waits` | "No model context waits. A delegate that launches a job, a proof or a landing returns at once". | 18:81 | missing proof (half) | The return-at-once half is proven: `TestLaunchStartReturnsAfterTheChildIsRecorded` (internal/launch/launch_test.go:156), `TestGoalBranchReadDelegateUsesBinarySeamAndReturnsWithoutWaiting` (cmd/metasystem/goal_branch_test.go:22). The no-wait half is observed, not refused: `metasystem.conf:42` `context.toolgate.mode=observe`, and `TestToolGateObserveModeAllowsAndRecordsTheDenyDecision` (internal/adapter/toolgate_run_test.go:326) proves that observe mode allows the call. |
| G13 `intent-is-versioned-and-revisable` | "intent is versioned rather than declared once... The change must be explicit, authorized and traceable". | 02:49; 05:29 | not checkable as stated | Goal revisions exist (`internal/goalrevision`; `TestBudgetEpisodeRevisionTransitions`, `TestApprovalEpisodeRevisionGrammar`) and an edit invalidates approval (row 2). But the paper's intent record (outcome, constraints, freedoms, observations, rulings) is not a software structure; a goal's intent is one line. The Project thread design is where this is going. |
| G14 `least-authority-per-role` | "each task receives only the information, resources and actions needed for that task." | 09:41; 08:29 | missing proof | `metasystem.conf:132-136` declares `dispatch.permissions.<role>` (critic, none, workspace); `TestExpandPermissions` (internal/dispatch/decisions_test.go:291) exercises expansion. No test demonstrates that a critic-permissioned job cannot write. |
| G15 `appeal-reaches-another-person` | "an affected person must be able to reach another responsible authority, not the mechanism that made the disputed classification." | 13:67; 18:49 | not checkable | Organizational, not software. Belongs in the Project's Constraints as prose. |
| G16 `intent-record-precedes-checks` | "Only then does the builder turn it into checks"; "The sitting proposes; only records rule." | 01:45; 15:19 | not checkable as stated | The sittings machinery is design (`plans/user-interface/g1-s21-project-thread-design.md`). Nothing today refuses a check whose expected result has no recorded ruling behind it. |
| G17 `battery-measures` | The battery "must actually measure: it emits `metric=<id>=<value>`". | inception SKILL.md:162; `internal/contract/measure.go:188` | missing proof | No script in this repository emits the metric grammar except fixtures (`scripts/adopt-fixtures.sh:962`, `scripts/agents/mission-fixtures.sh:113`). `scripts/validate-metasystem.sh` prints a verdict line, not a metric. The draft names it as the battery anyway (section 5) and this wrapper is the steel thread. |

Contradictions between the paper and the software today:

- **C1. Design critique rounds.** Chapter 18 (18:79) rules "Design critique runs one
  round... A second round follows only a fold that changed a rule, and there is never a
  third." The software's ceiling is three: `metasystem.conf:19-20` "Decision 06, R-42-m0:
  three review rounds is the ceiling", proven by `TestBoundaryAtThreeRounds`. Row 7's
  ref therefore quotes chapter 6's three stop reasons, which the software does satisfy,
  not chapter 18's one-round rule, which it does not enforce. The human should say which
  is the rule.
- **C2. Landing and the closed chain.** The appendix (19:213) says "the ordinary landing
  of a change does not yet consume a closed chain". The landing package does read chain
  closure for a chain-declared landing and refuses a stale one:
  `internal/landing/observe.go` bar `a` and `TestLandingRejectsStaleCriticClosure`
  (internal/landing/observe_closure_test.go:244). Two qualifications keep this from
  being a clean contradiction: "The caller enforces refusing outcomes only for agent
  commits; human commits stay sovereign" (observe.go:104-105), and a temporary
  would-refuse exception is open (`records/misc/a5-would-refuse-review-2026-09-19.md`,
  cited at observe.go:38-42). The appendix sentence looks stale rather than wrong; a
  human read of `observe.go` settles it.
- **C3. Approval by proxy.** Chapter 13 (13:45) says "a value ruling stays outside"
  any delegation. The appendix (19:137) allows "Under a recorded power of attorney...
  the brain may take decisions of that class." Not a software contradiction (the brain
  is not built), but the two chapters draw the line in different places and the
  covenant cannot carry both.

## 4. What a covenant cannot carry

A covenant is rows, a battery, bounds, guards and a net. Most of the paper is not
that shape, and forcing it in would produce exactly the floating intent the type warns
about. Where the rest belongs, using the master design's Project subsections
(`plans/user-interface-design.md:479-484`):

- **Intent.** The thesis (01:9, 01:23), the five activities (02:21), the principles as a
  set (chapter 5), the four protections that "must not be lost" (18:49), and the
  falsifiability statement (17:61). This is the paragraph `docs/project-rules.md:8`
  still holds as `<one paragraph>`. A candidate, in the paper's words: "machinery takes
  on more of the building and delivering, and engineers own the design and governance
  of the machinery doing it" (01:23), for "software released repeatedly, changed by many
  hands or trusted with money, identity, safety or essential work" (01:73).
- **Architecture.** The seven roles and their prohibited combinations (chapter 7), the
  one authoritative record with role-scoped views (chapter 8), the living-system
  protections (chapter 9), and the appendix's seven diagrams. `docs/architecture.md`
  and `docs/concepts.md` already exist; the appendix's "Where the software stands
  today" (19:213-215) is the honest bridge between the diagrams and the code and
  belongs beside them.
- **Designs.** The fleet, the brain, the channel's shared inbox, bounded release and
  care, the learning practice: everything in G2, G9, G10, and the ch18 rules that are
  process rules rather than checks (18:77-86).
- **Constraints and assurance.** This is where the covenant lives, alongside the local
  invariants in `docs/project-rules.md:43-70`, `testing.json`'s obligations, and the
  chapter 12 governance-record design that G6 says is not built. The ch18 rules that
  are enforceable "in the brief template, the orchestration loop and the design
  principles" (18:88) are constraints, and the two targets on 18:88 (under eight hours,
  under two thousand weighted tokens per landed line) are the paper's own budgets in
  waiting; they become covenant budgets only when G11's measurement exists.
- **Open questions.** Chapter 18's open problems (18:55-69): federation and portable
  evidence, liability and audit, runaway spend across domains, representation before
  harm. Chapter 9's security disclaimer (09:61): "I offer no security design here".
  Chapter 17's limit: self-application "cannot show by itself that the approach works
  beyond the system that ran it". And C1 to C3 above.
- **Sittings.** Chapter 15 entirely. The interview this survey precedes is one.

## 5. Identity, battery, budgets, guards, guardrails

Read from `covenant.go:47-87` and from what the repository already runs.

**Identity.** `name`: `metasystem`. `entryPoint`: `bin/metasystem`, the binary
`scripts/agents/go-build.sh` builds from `cmd/metasystem` (go-build.sh:68 uses
`go run ./cmd/metasystem` as the source form). `sourcePaths`: `cmd/**`, `internal/**`,
`scripts/**`, `go.mod`, `go.sum`, and also `AGENTS.md`, `wow.md`, `docs/**`, `skills/**`.
The last four are unusual for a source list and deliberate: for this application the
instructions are executed, by agents, and `testing.json` already treats them as a
surface (`instructions`, testing.json:27) with the obligation `instruction-integrity`.
Paths are relative to the app root `metasystem/`, matching `internal/covenant/testdata/taskrun-covenant.json`.

**Battery.** `bash scripts/validate-metasystem.sh`, metric `validation-green`,
direction `max`, threshold `>=1`. Reasoning: the paper's own definition of green for
this repository is the full validation ("a platform moves to the verified tier only when
a full `scripts/validate-metasystem.sh` run passes", `docs/project-rules.md:121-122`),
and it is the command that runs every section the rows cite. Threshold defined before
looking at any score, as the inception skill demands. The honest defect: it emits no
`metric=validation-green=<value>` line (G17), so the first mission contract that
declares `covenant.path` would fail at `gate.command` measurement. The alternative,
`bin/metasystem test run --root .`, is the landing-time selection and is cheaper but is
not what "green" has meant here. The wrapper that turns the verdict line into the
metric line is a few lines of shell and is the steel thread.

**Budgets.** Empty. The repository has three real ratchets that are budgets in the
type's sense (`Budget` is "the ratchet's subject", covenant.go:71): the per-package
coverage floors in `scripts/agents/coverage-ratchet.json` (with a Linux variant), the
parallel-test ratchet (`internal/parallelratchet`, `TestCheckParallelRatchetUsesPackageCeilingsAndReasonedExemptions`),
and the dependency ratchet (`TestGoGateFastModeRunsParallelRatchetBesideDependencyRatchet`).
All three are enforced by `go-gate.sh`, none emits the `metric=` grammar, and none has a
single scalar bound the covenant row could carry (the coverage ratchet is per package).
The inception skill's rule applies: "A budget or guard earns its row only whole...
Anything less is OMITTED and becomes a goal." Listed here so nobody thinks they were
missed. The two ch18 targets (18:88) are the first real budgets once G11 measures them.

**Guards.** Empty, for the same reason. The natural first guard is the fail-open
tripwire (row 16) at cadence 1, floor 1, but `measure.go:169-190` requires a guard
command that exits 0 and prints `metric=<name>=<value>`; the tripwire is a section
inside the validation script, not a standalone command. A second candidate is
`scripts/agents/preflight-commands.sh` (the command inventory, `docs/project-rules.md:136`),
which is closer to standalone. Both need the wrapper.

**Guardrails.** Relative to the app root, comma-free, none under the wall's protected
table (`internal/mission/guardrails.go:45-52`: `scripts/agents/`, `plans/goals/`,
`plans/goals.md`, `plans/goals-accepted.json`, `memory/instruction-ledger.md`,
`memory/known-issues.md` are already custodied and must not be declared, so
`go-gate.sh` and the ratchet files are covered without listing):

| entry | reason |
| --- | --- |
| `testing.json` | Names every proof the rows cite. The net must close over its own proofs. |
| `scripts/validate-metasystem.sh` | The battery. |
| `metasystem.conf` | Carries the round ceiling, the budget tiers, the context bound, and the toolgate mode that rows 1, 7, 12 and G12 depend on. |
| `docs/project-rules.md` | The local invariants three rows cite as standing rules. |
| `memory/rulings.md` | The recorded rulings; the paper's precedents (08:63). |
| `docs/paper/` | The refs point into it by chapter and line. A rewrite moves the ground under every row; a mission that edits the paper should take the warden's lane. This is a choice the human may reverse: it makes the paper harder to edit from inside a mission, which may be exactly right or exactly wrong. |

Considered and omitted: `docs/covenant-evidence.md` (does not exist yet; add it at
birth, it is "gate-defining input" per the inception skill), `docs/design/` (design, not
net), `AGENTS.md` and `wow.md` (instructions; guardrailing them would put every
instruction edit on the warden's lane, which the `instruction-integrity` obligation
already covers differently), `bin/` (built output), `plans/` (the goal machinery
"custodies itself").

The spelling caveat from the top of this document applies to every entry: if the wall
sees git paths, these need the `metasystem/` prefix, and `covenant.json`'s own
self-custody does not work for a nested root at all. That has to be settled in code or
by moving the root before the covenant is live.

## 6. Recommendation

**Birth it, small.** The seventeen rows are real: each is a sentence the paper states as
binding and a test this repository runs. Writing them down changes one thing for anyone
working here: the sentence "the metasystem has no stated intent" stops being true, and a
mission that weakens one of these seventeen has to say so to a human, because the
covenant gate refuses a contract whose gate, threshold or net disagrees
(`covenantgate.go:47-80`). That is what a covenant is for, and it costs nothing to run
that the repository does not already pay.

**What it would change for people working here.** Three things. The evidence table
becomes a file agents read before they claim adequacy. Editing `testing.json`,
`metasystem.conf`, `validate-metasystem.sh`, `project-rules.md`, `rulings.md` or the
paper inside a mission goes down the warden's lane. And `bin/metasystem covenant
validate` prints "adequacy not established" on every run, which is the truth and is
worth being reminded of.

**What to leave out of a first version.** Budgets and guards (empty is lawful and
honest). The `docs/paper/` guardrail if the human wants to keep editing the paper freely
from missions. Rows 6 and 17, whose strongest tests run only under `go-affected`; keep
them if the human is content that the proof runs when the code moves, drop them if
"never runs" must mean "has a standing cadence". Everything in section 3.

**What has to happen before it is live**, in order: settle the nested-root path
question (top of this document); write `docs/covenant-evidence.md` in the exact shape
the evidence gate parses (inception SKILL.md:86-98), one row per draft requirement with
status `referenced-not-run` (nothing was executed for this survey) and the G-rows as
`planned-floating`; add the thin wrapper so the battery emits its metric (G17); then
`bin/metasystem covenant validate` and a full validation. That is the inception
interview's step 4 to step 8, with the human present, and it is not this survey's to do.

**The single most valuable row** is `a-rule-change-never-judges-its-own-case`. It is the
paper's stress test (chapter 17), the one claim the paper says the metasystem "must
pass", and it is the best-proven row in the draft: the trusted policy engine is built
from the base, not the candidate; a candidate cannot drop a protected test or lower a
coverage floor; a build failure cannot fall back to the candidate's own policy. If the
covenant carried only that row and the battery, it would still say the most important
thing the paper says.

## 7. The draft, verified

Written 2026-09-22, against `ui-development` at `666cd6adf`, which is the survey's tree
plus the merge of `origin/main`. Section 6 said the draft's status was
`referenced-not-run` because nothing had been executed for the survey. That is no longer
true: everything below was run, and the results are recorded whether or not they flatter
the draft.

**Shape.** `metasystem covenant validate` on a root holding the draft as `covenant.json`:

> covenant shape valid: metasystem (17 requirement(s), battery "bash scripts/validate-metasystem.sh" on validation-green >=1, 0 budget(s), 0 guard(s), net [docs/paper/ docs/project-rules.md memory/rulings.md metasystem.conf scripts/validate-metasystem.sh testing.json]); adequacy not established — shape says the rows parse, never that the proofs guard the intent

The verb's own last clause is the right caveat and the reason for the rest of this
section.

**The references resolve.** The seventeen rows name 106 distinct Go tests and 24
`testing.json` groups. Every one of the 106 exists as a `func Test…` in a tracked
`_test.go` file, and every one of the 24 group names is present in `testing.json`. This
was worth checking rather than assuming: `origin/main` rewrote `testing.json` and much of
`internal/proofrun` between the survey and this verification, and a covenant whose proofs
have been renamed underneath it is worse than no covenant, because it still validates.
Nothing had drifted.

**The net exists.** All six guardrail paths are present at the app root.

**G17 confirmed by running it, not by reading it.** `scripts/validate-metasystem.sh`
contains no `metric=validation-green` emission. The battery would therefore fail at
`gate.command` measurement exactly as section 5 predicted, and the wrapper remains the
steel thread.

**The proofs pass.** The 106 tests were run across the whole module. Ninety-six packages
passed with no individual test failing. `cmd/metasystem` was rerun separately because the
first pass hit Go's ten-minute default while two builds were loading the machine; with
`-timeout 40m` it passes in 675 s. So all 106 named proofs pass on this tree.

What this does not establish, and section 3 still governs: that the proofs are
*discriminating* (G5 — a passing test that would also pass on a broken version has proved
nothing), that the seventeen rows cover the paper (they do not; the seventeen gaps and
three contradictions stand), or that any row is adequate to the sentence it quotes. Shape
and reference integrity are the cheap half. The recommendation of section 6 is unchanged:
birth it small, with the human present.
