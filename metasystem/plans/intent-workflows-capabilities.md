# Public capability preservation

Goal: verbs-match-intent. This implements the capability inventory taken at
c8ecd4bb62706c02331f12a4de4ae0025b634568 for
[Complete tasks through intent](designs/intent-workflows.md): all48 original commands
and13 additional outcomes, plus the planning/manual-work additions. Source inventory
updated26September2026. The table names actual public routes and concrete evidence
owners; an existing alias alone never counts as a public completion path.

The two inventory gaps, abandonment succession and complete settings discovery,
have public routes and passing owner fixtures. Three independent Fable implementation
reviews are complete; all material findings are addressed. Root's final acceptance,
including the last literal-only help correction, is recorded in
`intent-workflows-verification.md` and the three designs' completed obligation tables.
The inventory below distinguishes each test's boundary; the final runtime join
provides observed proof for every named critical/high obligation.

Every command below omits the initial `metasystem`. Uppercase tokens are arguments to replace, not literal values; quote text/file arguments as needed. For goal names that collide with subject words, use **`show --goal G`, `status goal G`, `wait goal G`, `review goal G`**. These are actual handler forms, not suggested aliases. `build G`, `revise G`, `land G` and the goal-only verbs take even reserved words as their single goal argument. Do not turn them into unsupported two-word `goal G` forms. `--check` ends build option parsing. Process job references come from `status work`: preserve the complete printed reference, such as `j1:ID` or `j2:ID`, when using `status job`, `wait job`, `stop job` and `review job`.

Evidence notation: **P** public-adapter fixture; **R** physical Git/real owner process or connected runtime fixture (models/proof/authority effects remain stubbed where the test declares them); **C** preserved compatibility-route fixture exercising the same owner; **O** existing owner test, not a new public-route journey. All names below were found in source. Tests under `cmd/metasystem` unless another package is named.

## Original 48 commands

| # / Original capability | Actual public route and owner | Strongest located test evidence (boundary notation below) |
| --- | --- | --- |
| 1 goals | `goals`; `goals --ready`; `goals --all --history`; `runIntentGoalViews` → accepted projection/frontier | P `TestIntentGoalsArchivedHistory`, `TestIntentGoalsReadFlags`, `TestIntentPlanningReadyTiersAndNotes` |
| 2 show | `show --goal G`; `runIntentShow` → accepted goal record/history/budget | P `TestIntentGoalAuthorityAndState/reads`, `TestIntentClaimContinuesReservedGoalByItsShow` |
| 3 approve | `approve G --budget norm`; same goal approval owner, multi-goal act retained | P `TestIntentGoalAuthorityAndState/queued approval under each tier's box in one act` |
| 4 budget | `budget G`; `budget G norm`; budget/approval/resume state semantics preserved | P `TestIntentGoalAuthorityAndState`, `TestIntentAttorneyBudgetKeepsAuthorityInputs` |
| 5 pause | `pause G --reason TEXT`; goal park | P `TestIntentHumanPauseNeedsNoTypedName`, `TestIntentGoalAuthorityAndState/pause keeps the literal reason` |
| 6 resume | `resume G`; `resume mission M`; goal unpark/standing-box resume, or mission owner | P `TestIntentParkedResumeActors`, `TestIntentResumeAttorney`; O missionrunner `TestInternalRunResumeVerdicts` |
| 7 done | `done G --reason TEXT`; obligations, conclusion, sweep remain owned | P `TestIntentGoalAuthorityAndState/done checks its obligations then concludes`, `TestIntentDoneNeedsTheHumansProof`, `TestIntentLandedActsReportPartialFollowUp` |
| 8 start | `start checkout`; `start session`; `start ui`; `start mission M`; `start machine NAME`; distinct existing owners | P `TestIntentProcessAdoptedRoots`, `TestIntentProcessTargets`; O mission/seat tests below |
| 9 stop | `stop checkout`; `stop session --by NAME`; `stop job J`; `stop ui`; stop-transition, session-stop, selected job owner, lifecycle | P `TestIntentProcessAndAnswerTargets`, `TestIntentProcessCorrections`, `TestIntentProcessTargets` |
| 10 restart | `restart checkout`; `restart ui`; stop succeeds before start | P `TestIntentProcessAndAnswerTargets/restart names the stopped state when its start refuses`, `TestIntentProcessCorrections/the interface reports its state and restart through one result` |
| 11 status | `status checkout`; `status goal G --work NAME`; `status job J`; actual selected owner (raw `status unit U` remains compatible but is absent from ordinary help) | P `TestIntentReservedGoalNames`, `TestIntentProcessAndAnswerTargets`; R `TestIntentConnectedJourneyRealClose` |
| 12 enroll | `enroll --name NAME`; terminal enrollment/publication owner | P `TestIntentProcessAndAnswerTargets/enrollment reports a local enrollment whose publication failed` |
| 13 ask | `ask G --question TEXT --option 'yes: proceed' --option 'no: wait'`; channel ask owner | P `TestIntentProcessCorrections/an ordinary question is recorded and its delivery reported`, `TestIntentAskContinuationsAreFollowed` |
| 14 answer | `answer M/Q TEXT`; `answer channel:Q` supplies authenticated channel instructions, not invented local authority | P `TestIntentQuestionJourney`, `TestIntentProcessAndAnswerTargets/a mission answer advances or rolls back through its transition` |
| 15 fleet | `status --machines`; optional `--refresh`; fleet reader | P `TestIntentProcessAndAnswerTargets/readings keep unknown and quoted paths`, `TestIntentProcessTargets` |
| 16 doctor | `check`; steward health with public remedies | P `TestIntentCheckPublicRemedies`, `TestIntentProcessAndAnswerTargets/stop, status and doctor read the same installation` |
| 17 ui | `start ui`; `stop ui`; `status ui`; `restart ui`; lifecycle owner | P `TestIntentProcessTargets`, `TestIntentProcessCorrections`; O lifecycle `TestRestartStartsOnlyAfterStoppedOutcomes` |
| 18 open | `open G --intent TEXT --next TEXT --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis TEXT`; intake owner; an agent also names its held blocker with `--blocks` | P `TestIntentPlanningIntakeAndEdit` |
| 19 edit | `edit G --next TEXT`; intent/risk/labels/obligation flags retained; exact supplied fields only | P `TestIntentPlanningIntakeAndEdit`, `TestIntentTextFilesReachOwners`; O goal `TestGovernedObligationRoundTripsTypedAssumptionsAndTriggers` |
| 20 claim | `claim`; `claim G`; `claim G --take-over --reason TEXT`; ready-frontier/claim/steal authority | P `TestIntentPlanningClaimAndRelease`, `TestIntentClaimContinuesReservedGoalByItsShow`, `TestIntentBuildAutoClaimPositive` |
| 21 release | `release G --reason TEXT`; holder release (G optional only when unambiguous) | P `TestIntentPlanningClaimAndRelease` |
| 22 ready | `land G --queue-only`; goal land-ready only | P `TestIntentQueueOnly` |
| 23 decide | `accept-risk G --finding F --review R --reason TEXT`; existing accepted-risk/register/proof owners | P `TestIntentReviewAcceptedRiskException`; C `TestIntentPlanningDecidePartial` |
| 24 pin | `pin G MACHINE`; `pin G --clear`; set-pin | P `TestIntentPlanningHumanOnlyActs`; O goal `TestMachinePinning` |
| 25 prioritize | `prioritize G 1 --sequence N`; set-priority | P `TestIntentPlanningHumanOnlyActs` |
| 26 reopen | `reopen G --next TEXT`; reopen plus next-step update | P `TestIntentResumeArchivedGoalContinuesToReopen`; O goal `TestReopenFromAbandonedIsProvenHumanUnrankedUnapprovedAndKeepsTheEvent` |
| 27 abandon | `abandon G --reason TEXT --successor G2`; atomic goal abandon or retained succession recovery | P `TestIntentAbandonWithSuccessor` executes real Abandon/CarryAbandoned, including dependent repoint, invalid successor, authority and replay |
| 28 block | `block G --on G2`; goal block | O goal `TestBlockParksALiveGoalAndRecordsTheDisplacement`; public intake edge exercised by `TestIntentPlanningIntakeAndEdit` |
| 29 unblock | `unblock G --on G2`; goal unblock; early release remains human-reserved | O goal `TestUnblockOfTheLastUnsatisfiedEdgeReturnsTheGoal`, `TestEarlyUnblockNeedsAHumanAndRecordsItsSessionProvenance` |
| 30 unapprove | `unapprove G --reason TEXT`; withdraw approval and park standing claim | P `TestIntentPlanningHumanOnlyActs`, `TestIntentPlanningPartialAfterPublication` |
| 31 grant | `grant --tiers 1 --acts approve,budget,resume-parked --until DATE`; attorney owner | P `TestIntentPlanningAttorneyGrant` |
| 32 revoke | `revoke GRANT`; attorney revoke | P `TestIntentPlanningAttorneyGrant`, `TestIntentPlanningPartialAfterPublication` |
| 33 split | `split G --plan FILE`; existing closed draft grammar and atomic split | P grammar `TestIntentSplitHelpExampleParses`; O goal `TestSplitIsAtomicPermanentAndRewritesDependencies` |
| 34 group | `group G GROUP`; goal set-arc | O goal `TestQueuedJoinsClaimedArcUnderTheClaimantOnly`, `TestArcClaimsAsOneUnit` |
| 35 ungroup | `ungroup G`; goal detach | O goal `TestDetachReleasesWithoutSplittingTheQuota` |
| 36 resolve | `review goal G --finding F --review R --test NAME`; review-obligation discharge. Qualified fixture proof flags also retained | C `TestIntentPlanningResolveByOwningSession`; O goal `TestFixtureObligationLifecycle` |
| 37 notes | `notes G`; `notes G --read LABEL --add TEXT`; `notes G --close ITEM --accepted TEXT` (also fixed/moved choices) | P `TestIntentPlanningNotesAuthority`, `TestIntentPlanningReadyTiersAndNotes` |
| 38 recover | `repair goals`; `repair waits`; installation journal and own durable continuations kept separate | P `TestIntentRepairAuthority` (clean journal); O goal `TestTheOneRecoveryRule`; `recoverWaits` reuses the existing `waitContinuations` owner |
| 39 red | `incidents`; `incidents claim I --goal G`; `incidents close I --reason TEXT`; trunk-red owner | O goal `TestTrunkRedRecordOwnClearAndCloseTransactions`; C authority coverage `TestIntentPlanningHumanOnlyActs` |
| 40 brief | `brief G --out FILE`; accepted goal/design scaffold, caller-relative file | P `TestIntentBriefCarriesAcceptedDesign`; R `TestIntentBriefBeforeFirstWorkspaceBuilds` |
| 41 build | `build G --work NAME --brief FILE --check COMMAND ARG`; claim/worktree/unit runner/proof/first read | R `TestIntentBuilderChildInGeneratedWorktree`, `TestIntentConnectedJourneyRealClose`; P `TestIntentBuildAutoClaimPositive`, `TestIntentBuildRetainedRequest` |
| 42 wait | `wait goal G --work NAME`; `wait goal G --for landing`; `wait goal G --for human-act --verb approve --since TIP`; `wait job J`; `wait question Q`; `wait unit U`; `wait resume ID`; durable/launch/unit/channel owners | R `TestIntentWaitGoalEventGitAdapterObservesAPersonsAct`, `TestIntentWaitGoalLandingGitAdapterObservesTheRealLanding`, `TestIntentWaitQuestionGitAdapterContinuesToTheRealAnswer`; P `TestIntentWaitJobKeepsTheSelectedOwner` |
| 43 test | `test --goal G --mode auto`; existing risk-selected test owner, proof continuation | P `TestIntentWaitTestAndSettingsAdapters`, `TestIntentProofReferenceFeedsWaitProof`; these assert routing/parser and attempt reference, not a full admitted proof run |
| 44 settings | `settings`; `settings KEY`; selected-installation launch reader and general config get | P `TestIntentSettingsSelectedInstallation`; key discovery and validation below |
| 45 review | `review goal G --work NAME`; `review design FILE --goal G`; `review job J`; `review commit SHA --goal G`; real examination/close/collect owners | R `TestIntentConnectedJourneyRealClose`, `TestIntentDesignReviewRealOwnerJourney`; P `TestIntentReviewCommitClosesThenPublishes` |
| 46 fold | `revise G --work NAME --after N --brief FILE --dispositions FILE`; `revise job R --dispositions FILE --brief FILE`; retained revision or review-chain continuation | R `TestIntentConnectedJourneyRealClose`; O launch `TestRevisionRequestReplay`, `TestRevisionRetainsReviewedFindingsAfterFailure`; C `TestIntentFoldComposesTheReviewedDecision` |
| 47 close | `review goal G --work NAME --dispositions FILE`; also `review job J --dispositions FILE`, design/commit review completion; whole close plus collected/published read | R `TestIntentConnectedJourneyRealClose`; P `TestIntentGoalReviewEmptyJoinRealClose`, `TestIntentCloseWholeOwner`, `TestIntentCloseRecordWriterPreflight` |
| 48 land | `land G`; `land G --through COMMIT`; `land job J`; hand landing or configured batch admission | R `TestIntentLandWholeOwnerGitAdapter`, `TestIntentLandBatchAdmissionGitAdapter`, `TestIntentManualWorkLandsOnEndpoint`; batch admission legitimately returns in-progress, not sealed/pushed |

## Additional 13 outcomes

| # / Outcome | Public route → actual owner | Strongest located test evidence / boundary |
| --- | --- | --- |
| A1 autonomous start/resume/status | `start mission M`; `resume mission M`; `status mission M` → `runIntentMission` → existing mission subprocess | O missionrunner `TestInternalRunResumeVerdicts`, `TestInternalRunAnswerAndResumeChain`; no successful new public mission-start journey located |
| A2 disputed workspace resolution | `repair mission M --problem N --confirm-restored TREE --by NAME --reason TEXT`; alternatively `--accept-workspace --waive CLAIM` → mission resolve-taint | P `TestIntentRepairAuthority` reaches real owner and refusal; O missionrunner `TestResolveTaintRestore`, `TestResolveTaintAdoptDisputedTree`, `TestResolveTaintThroughWrapper`. Restore form verifies already-restored files; it does not restore them |
| A3 add a machine | `start machine NAME --destination PATH`; `--resume ID` → seat launch | P `TestIntentProcessTargets`; O seat/launch `TestAClonedMachineIsAFleetSeatBeforeItIsEnrolled` |
| A4 coordinator designate/withdraw/read | `settings coordinator`; `settings coordinator --declare --by NAME`; `settings coordinator --withdraw --by NAME` → brain | R `TestIntentCoordinatorGitAdapterDeclaresAndWithdrawsThroughTheBrainOwner` |
| A5 inspect project memory | `show designs --goal G`; `show decisions`; `show record ID`; `show design --goal G` → project readers/design attempts | R `TestIntentDesignAuthorRealOwnerJourney`, `TestIntentDesignReviewRealOwnerJourney` for design workflow; O project `TestListNarrowsByGoalAndStatus`, `TestReferencedByReadsTheOtherHalf` |
| A6 retry/await/withdraw question | `ask --retry Q`; `wait question channel:Q`; `ask --withdraw Q --reason TEXT` → targeted channel lock/read/wait | P `TestIntentQuestionJourney`, `TestIntentAskContinuationsAreFollowed`; R `TestIntentWaitQuestionGitAdapterContinuesToTheRealAnswer` |
| A7 precise proof exception | `land G --exception CODE --reason TEXT --by NAME`; `land G --using-exception ID`; replacement/transfer/expiry remain explicit | R `TestIntentCarriedGoalDeliveryGitAdapter`; P `TestIntentCarriedReplay`, `TestIntentCarriedReplacement`, `TestIntentCarriedChannelWordAdoption`. Real battery refusal remains effective before continuation |
| A8 partly recorded abandonment succession | Repeat `abandon G --reason TEXT --successor G2` → existing CarryAbandoned; fresh call uses atomic AbandonSpec.Carried | P `TestIntentAbandonWithSuccessor` proves original reason retained, enrolled authority, actual successor and dependent changes, invalid successor leaves live state, completed replay adds no history |
| A9 recover journal/session waits | `repair goals`; `repair waits --session S`; journal recovery plus session-checked continuation read | P `TestIntentRepairAuthority` (journal scope); O goal `TestTheOneRecoveryRule`; existing `waitContinuations` is called, not a new scheduler |
| A10 accept remote rewrite/reviewed manual edits | `repair goals --accept-remote-history --by NAME`; `check goals`; `repair goals --accept-edits --by NAME`; `repair goals --refresh` → repair/reconcile | R `TestIntentRepairGoalsGitAdapterAcceptsRewoundRemoteHistory`, `TestIntentRepairGoalsGitAdapterAcceptsRemoteHistoryInADeclaredCoordinator`, `TestIntentRepairGoalsGitAdapterAcceptsHandEditsThroughReconcile` |
| A11 legacy upgrade/engine floor | `repair goals --upgrade --by NAME` first reveals digest; repeat with `--source-digest SHA256`; `settings compatibility --minimum-engine SHA --by NAME` → migrate/engine-floor | R `TestIntentRepairGoalsGitAdapterUpgradesOnlyTheReviewedDigest`, `TestIntentCompatibilityGitAdapterRecordsTheFloorThroughTheEngineFloorOwner` |
| A12 settings beyond launch | `settings KEY`; `settings --keys --matching PREFIX`; `check settings` → existing config get/keys/validate owners in selected installation | P `TestIntentSettingsSelectedInstallation`, `TestIntentSettingsKeysAndCheck`: actual engine enumerates synthetic non-launch keys, accepts valid config and rejects incomplete/missing contracts from outside selected repository, leaving config unchanged |
| A13 discover work before stopping | `status work`; `status work --all`; follow each qualified `stop job J` / `wait job J` → exact launch/dispatch owner | P `TestIntentProcessAndAnswerTargets/a job id names exactly one retained record`, `TestIntentWaitJobKeepsTheSelectedOwner` |

Mechanical protocols (process identity, locks, custody, low-level ledger writes,
worker execution, canonical hashing, accounting) remain callable by their existing
machine consumers. They are executed by owners, not presented as extra tasks a
person or agent must manually perform. Their preservation is checked through
consumer regression tests, not by displaying a technical catalogue in public help.

Goal-event waiting is also public: `wait G --for landing|human-act [--verb V] [--since TIP]`; existing event/after aliases stay callable. This is required by IW-C8, alongside work and question waits.
