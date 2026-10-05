# Design for health-is-green-when-the-seat-is-healthy

- Kind: design
- Id: 01M447PESPJ5SNXMJ2K77M359E
- Status: accepted
- Goals: health-is-green-when-the-seat-is-healthy
- Critique: closed at round 2 on one residual folded (design-critic-33b212e1cd19505db69c369f: 6 material in round 1, all folded in revision 2; 1 residual in round 2 folded by hand and accepted under the critique stop rule); accepted 2026-10-05 07:30 CEST by Wido via m1e

Step 3, the standing reds; 2026-10-04; lines at main 75933ab51 under `metasystem/`; evidence of 21:39-22:10 CEST. Revision 2 (2026-10-05) folds critique round 1; the lines it adds are read at 074efb978.

## Scope

- Changes: six checks (rows 3, 4, 6-10) and the Stop hook's health phase.
- Changes: dead verdicts retired in rows 1, 2, 5.
- Changes: the standing rule and the rule for which roles apply to a checkout.
- Leaves alone: the role names, the health episode's lifecycle, the watcher repair, steps 1 and 2.
- Leaves alone: the appetite role's cost, the coordinator's identity.

## The rows

| Red | Cause | Decision | Owner | Test red without it |
| --- | --- | --- | --- | --- |
| retro-debt (10) | Committed register `memory/retro-debt.json` (`goal/verbs.go:2327`), one obligation read by all ten; not a supervisor fact. | Retire from health; `metasystem receipt status` reports it. | Wido | `TestOpenRetroDebtLeavesTheSeatHealthy` |
| trunk-red (10) | Reads the ledger's cadence (`trunkred.go:46-56`); its writer (`gaterun/validate.go:84`) nothing triggers; deep mode fails on this Mac. | Retire the cadence verdicts, keep open trunk-red entries; the lane's per-push proof replaces it. | lane | `trunkred_test.go` `TestTrunkRedIgnoresAnOverdueCadence` |
| capability-snapshots (8) | Walks `metasystem.runtimes` (`defaults.go:63`); only delegate admission writes snapshots, by its probe on a snapshot miss (`delegation/admission.go:492`); this seat admitted codex only. | Automatic remedy: the tick runs admission's probe, `ownerAdapter.Probe` (`delegation/owners.go:257`): the adapter's `probe` verb, whose `probeCommon` (`adapter/supervisor/claude.go:91`) reads the configuration identity, checks the CLI's login and writes the snapshot (`adapter/snapshot.go:32`); no model, no agent job. For a listed runtime on PATH with a missing or stale snapshot, at most once per runtime per tick, ended by the five-observation breaker; off PATH reads alive. | steward | `tick_test.go` `TestTickProbesAStaleSnapshotOncePerRuntime` (one probe, no job record) |
| ledger-attention (6) | Dead when the tip moved 30 min ago and no journaling goal verb here fetched it (`ledgerattention.go:336-358`), though the steward's pass diffed and staged it (`:553-568`), telling nobody. | Automatic remedy `examineLedgerMove` (new): one narrator digest line naming the moved goals, and the tip marked examined. It runs when a staged diff is promoted (`:276`), once at the steward's start (`RunLoop`, `runner.go:138`) and on the tick where the role first reads dead (beside `requestWatcherRepair`, `tick.go:557`); the last two cover a move promoted earlier, its diff rebuilt from the examined tip (`:221`). | steward | `ledgerattention_test.go` `TestAttentionPassNarratesAndExaminesTheMove`; `TestAttentionRemedyCoversAMovePromotedBeforeInstall` (an idle seat installed after the move) |
| context-budget (5) | Dead over `ProofMaxTokens` 200k (`context.go:356-363`), a constant only the weekly report uses (`contextreport.go:600`); Wido removed the caps 2026-09-20. | Retire the dead verdict; the line keeps count and handoff advice. | Wido | `context_test.go` `TestContextOverTheProofMaximumReadsAlive` |
| repo-watcher (4) | m1f: the watcher's pass fails every minute (`repo-watcher.json`: PASS_FAILED, attempt 348, success 243 at 18:22; m1e alike). A stale success triggers the owner's repair; the replacement fails too, and `componentFreshness` reports the old success's pid (`health.go:1079-1083`), not the failure: wrong reason, no second repair. The failing step's text is digested away (`supervise_component.go:245-247`). | Fix: the record keeps the last failure's text; the role reads it first. Measurement: run `WatcherPass` once on m1f; the step itself is deferred. | steward | `component_evidence_additional_test.go` `TestFailedPassKeepsItsText`; `TestRepoWatcherNamesTheFailingPass` |
| session-main (lane, m1i) | Dead when `mains/` is absent (`health.go:1104`); the lane never announces one; m1i was idle with nothing to seat. | Fix: off the lane by the applies rule below; on a seat, alive when `PlanSeat` (`seat_ladder.go:156`) starts nothing, dead with a step due (ladder the lawful remedy). | steward | `TestSessionMainReadsAliveWhenNoStepIsDue` |
| narrator-freshness (m1e, m1l) | Dead the instant the installation generation moves (`health.go:1066`), before the narrator's first pass under it (m1l: restart 21:48:51, read 21:48:59). | Fix: a mismatch with a fresh success reads alive; dead when stale. | steward | `TestNarratorWaitsForTheFirstPassOfANewGeneration` |
| hook-freshness (m1e, m1l) | An attempt spans the Stop (`runtime_hook_stop.go:500`); the tick read mid-Stop (m1l: 21:48:49-21:49:07, read 21:48:59) and calls any open attempt dead (`health.go:651-653`). | Fix: an open attempt younger than the Stop budget (60 s) is pending; dead past it. | steward | `TestHookFreshnessReadsAnOpenAttemptAsPending` |
| stop-hook-duration (m1e) | The Stop section. | Fix: the Stop's health phase reads the tick's `health.json` under two ticks old, evaluating only when none is fresh; a Stop that re-armed the engine is excused once. | steward | `internal/hooks/runtime_hook_harness_test.go` `TestStopHealthPhaseReadsTheTickVerdict`; `TestStopDurationExcusesOneRearm` |

## Standing rule and role sets

**Standing (D2).** One function decides it, `standingRoles(state, roles)` (new, `health.go`): a dead role without lawful remedy is `Standing` when the stored count for its role and cause is at `healthFailureLimit` (5); its line is kept, out of the aggregate, `ShouldAlert` and `healthFindingDigest` (`health.go:1912`). The tick calls it in `applyHealthObservation` (`health.go:721`) after advancing the count. The read-only projections, `metasystem health` (`PreviewInstalledHealth`) and the Stop's own evaluation (`PreviewHealthAt`), call it from `previewHealthAtWithMeasure` (`health.go:398`, which today computes its own aggregate) with the record the tick last saved, advancing no counter and saving nothing; all three flip on the tick's fifth observation. The count keys on the role and `healthCause(role)`: the `Cause` a check sets, else its reason with every token that holds a digit dropped (ages, counts, timestamps, pids, tips); the state keeps it in `FailureCauses`. A changed cause restarts a no-remedy role's count at one and clears standing; changed diagnostic text does not; alive clears it. Rows 3 and 4 join `hasLawfulAutomaticRemedy` (`health.go:814`) and follow today's breaker, which counts as before. Tests: `TestStandingFlipsOnTheSameObservationForEveryReader` (the tick, both projections, `Sequence` unmoved); `TestStandingCountSurvivesAnAgingReason` ("43 minutes ago", then "44": standing at five; a new cause: one).

**Defect.** The same tick files it, after `updateAlertEpisodesWith` (`tick.go:574`): `fileStandingDefects` (new) runs one `UpdatePatterns` cycle (`pattern_episode.go:238`; pattern `health-standing-red`, so owner `pattern:health-standing-red`; work the role; no pattern state kept), a finding per standing role carrying reason, remedy and `health.json`, a clear for every other role. On the fifth observation the role has left the digest, so `updateAlertEpisodesWith` (`alert_episode.go:372`), the one owner of health episodes, first resolves the health notice that carried it, or clears it when the seat now reads healthy; then the defect opens. Close before open, inside one tick: a failure never has two open notices, and a tick that dies between the two leaves none until the next. Test: `tick_test.go` `TestStandingHandoffLeavesOneOpenNotice`. Routing onward is blocked-seats' work.

**Applies (D3).** One closed rule replaces the kinds: `roleApplies(role, checkout)` (new, filtering at `health.go:474`) keeps a role only where the checkout runs the producer or duty the role checks; a role that does not apply is absent from the verdict.

| Roles | Producer or duty | The code can tell by |
| --- | --- | --- |
| steward-runner, supervision-owner, repo-watcher, census-freshness, narrator-freshness, seat-presence, ledger-attention, capability-snapshots | This checkout's steward tick, the components it supervises, its attention pass (`tick.go:389`) and its probe (row 3). | A running component: the tick that evaluates health is that steward. Always. |
| trunk-red, spend-fence, governed-obligations, nonterminal-jobs, proof-attempts, proof-admission, disk, the three claim roles | The ledger, registers and run state every checkout with a tick keeps; empty reads alive. | The same tick. Always. |
| session-main, context-budget | The session the seat ladder seats (`PlanSeat`) and its announcement in `mains/`. | A registration: the host's lane record (`lane.Read(board.Home())`, `landing/lane/lane.go:132`) does not name this checkout, so the ladder seats here. |
| hook-freshness, stop-hook-duration | The Stop hook of that seated session (`runtime_hook_stop.go:500`). | The same registration, and a hook installed: `hookswitch.RegisteredRuntimes` (`hookswitch/switch.go:198`) is not empty. |

The lane's record names its checkout, so the ladder seats nothing there and the four session roles do not apply; the coordinator's checkout seats a session, so every role applies. Evidence against the fold brief's premise that the lane runs no session and no hooks: the keeper's landing launches (`cmd/metasystem/landing_agent.go:114`) announce no main but ran the Stop hook there (lane `health.json`, 2026-10-04: hook turn generation 29, last Stop 3 s); question 5. Fixture: a new lane checkout, named by the lane record, one tick run, hook settings files present, no `mains/`, no hook or Stop record. Tests: `TestNewLaneCheckoutReadsHealthy` (healthy; none of the four lines); `TestSeatWithoutHookEvidenceReadsHookFreshnessDead` (the rule hides nothing on a seat).

## The 29-second Stop

Yes, the health call is still the largest share: m1e's Stops of 14:04-14:18 UTC (`hooks.log`) spent 21.3, 23.1 and 15.1 s in health; the 29 s Stop, 23.1 s. The phase runs `steward.PreviewHealthAt` (`steward_verbs.go:48`), every role afresh; behind it, claimed-goal-appetite: 15.2 s on m1e, 0.4-1.9 s elsewhere, `goal.CountCarriesAtEndpoint` over code history (`health.go:1294`). The other share, up 23.4 and 27.4 s, is the engine re-arm after a landed rebuild (`arming.log`). The 2026-09-16 regression fits this walk; not proven.

## Items AY and AO

- AY: the tick's verdict ladder (`verdict.go:17`), not health; stays with machinery-blocks-of-2026-10-04.
- AO: no row touches outage marks; stays there.

## Retired reds of 2026-10-03

- stop-capability-epoch on m1f, m1j: gone; both read "epochs match the live lease".
- ui's repo-watcher ownership reason: row 6's chain; covered there.

## Moved effects

| Effect | From | To | Code |
| --- | --- | --- | --- |
| The Stop's health line | the hook's own evaluation | the tick's last verdict | `metasystem/internal/hooks/runtime_hook_stop.go`, `metasystem/cmd/metasystem/steward_verbs.go` |
| Marking a ledger move examined | a journaling goal verb of the seat | the steward: its attention pass, its start and its tick on the first dead reading | `metasystem/internal/steward/ledgerattention.go`, `metasystem/internal/steward/runner.go`, `metasystem/internal/steward/tick.go` |
| Probing a missing or stale snapshot (ownerAdapter.Probe, writer probeCommon) | delegate admission only, on a snapshot miss | admission and the steward's tick, beside the watcher repair | `metasystem/internal/delegation/admission.go`, `metasystem/internal/delegation/owners.go`, `metasystem/internal/adapter/supervisor/claude.go`, `metasystem/internal/steward/tick.go` |
| The open notice for a dead role without remedy, from its fifth observation | the seat's health alert episode | the role's standing-defect episode, opened by fileStandingDefects; in the same tick the standing role leaves the health episode's key set, so updateAlertEpisodesWith closes that episode when no other role is dead and otherwise keeps it open for the other roles only (one notice per failure, never two for the standing one) | `metasystem/internal/steward/alert_episode.go`, `metasystem/internal/steward/pattern_episode.go`, `metasystem/internal/steward/tick.go`, `metasystem/internal/steward/health.go` |

## Deferred

- The carry walk cached by code tip, and the watcher's failing step; build on `RoleVerdict.DurationMillis` and row 6's failure text.
- An applies fact for the coordinator, the retro worker's automatic launch, routing episodes onward; build on `roleApplies`, the seat ladder, the episode's `Owner`.

## Open questions for Wido

1. trunk-red: retire the cadence verdicts (recommended), or the lane runs deep validation in its VM when due?
2. context-budget: retire the dead verdict (recommended), or one at a real handoff ceiling?
3. retro-debt: retire from seat health (recommended); later, should the steward launch the retro worker?
4. Give the coordinator's checkout its own applies fact now? Recommended no; it seats a session like any seat.
5. The lane's landing sessions do run the Stop hook (evidence under Applies). Keep hook-freshness and stop-hook-duration off the lane as the coordinator decided (recommended; the lane-silent episode and the per-push proof watch the lane), or apply them while a landing launch is live?

## Units

| Unit | Rows | Production | Test |
| --- | --- | --- | --- |
| U3a checks | 1, 2, 5-9 | ~220 | ~300 |
| U3b standing rule | the decision, the cause, the three readers, the handoff | ~240 | ~340 |
| U3c Stop | 10 | ~90 | ~120 |
| U3d remedies | 3, 4 | ~260 | ~310 |
| U3e applies rule | the applies table | ~90 | ~150 |

Re-estimated after the fold: ~900 production and ~1,220 test lines, the largest unit 580. Step 1 keeps its reach: the ten rows, the standing rule and the applies rule; the fold adds no row. Tests stub Git (D6); unpathed tests: `internal/steward/health_test.go`.

## Critique round 1

Astra, design-critic-33b212e1cd19505db69c369f: six material findings, all accepted by the coordinator (m1e, in Wido's word, 2026-10-05 07:00 CEST).

- MOVED-EFFECTS-CAPABILITY-PROBE (high), accepted: row 3 runs admission's probe (`ownerAdapter.Probe`, writer `probeCommon`), never the self-test; no agent job, at most once per runtime per tick; the third moved-effects row names the operation and its owners.
- CLOSED-RULE-ROLE-APPLICABILITY (high), accepted: the applies rule replaces the kinds, with producer and tell per role; a new lane checkout reads green. Its premise that the lane runs no hooks is corrected by evidence and left to question 5.
- HEALTH-STANDING-READERS (medium), accepted: `standingRoles` is the one decision; the tick, `metasystem health` and the Stop's evaluation call it; the projections advance nothing; one test holds the three to the same transition.
- HEALTH-STANDING-REASON-IDENTITY (medium), accepted: the count keys on the role and `healthCause`; an aging reason reaches five; a changed cause restarts it.
- HEALTH-LEDGER-EXISTING-MOVE (medium), accepted: `examineLedgerMove` also runs at the steward's start and on the first dead reading, with the idle-seat test.
- MOVED-EFFECTS-STANDING-NOTICE (medium), accepted: the fourth moved-effects row; the standing role leaves the health episode's key set in the tick that opens its defect, so the health notice closes when no other role is dead and otherwise stays open for the other roles only, with the handoff test (round 2 residual: an unhealthy sibling role must not keep a second notice open for the standing failure; folded here).
