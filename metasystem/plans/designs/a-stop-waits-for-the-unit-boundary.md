# A stop waits for the unit boundary, and a seat pauses at its daily spend line

- Kind: design
- Status: accepted
- Goals: a-stop-waits-for-the-unit-boundary, a-seat-pauses-at-its-spend-line
- Amends: metasystem/plans/token-spend-fence-design.md (Id 01M3A2YHDN94AP79J2RT1875W1, goal token-spend-fence, abandoned): this page is its step 2, kept for Wido's word (R-60-m1) and given on 2026-10-19.

Written 2026-10-19 by the opus-designer for the steward (m1e, Wido's name) from Wido's four decisions of 2026-10-19 (10:10, 12:50, 13:40). Critique: three Astra rounds by hand (gpt-6-astra; every machine was stopped before the reboot, so `design review` could not run): 11 -> 6 -> 5 material, each round folded from the code the designer read; revision 4 is the final text. Accepted at the stop by the steward in Wido's name on 2026-10-19 at 15:00 CEST; the remaining points are brief obligations BO-1..BO-5 below. The formal `metasystem design review` runs at the first engine start; a material finding there reopens this page. Goal split (Wido's rule: a goal has at most five units): a-stop-waits-for-the-unit-boundary = U1a, U1b (no automatic stop cuts a unit; the breached unit finishes); a-seat-pauses-at-its-spend-line = U2, U3, U4 (or U4a, U4b), blocked by the first. Logs: agentic-tools-evidence/lane-test-20261009/analysis/spend-fence-amendment-critique-{1,2,3}.log.

## Amendment: a seat pauses at the unit boundary when it passes its daily spend line (2026-10-19, revision 4, final text after three Astra rounds)

Target: `metasystem/plans/token-spend-fence-design.md` (Id 01M3A2YHDN94AP79J2RT1875W1, Goals: token-spend-fence, accepted). This is the step 2 that R-60-m1 kept for Wido's word. Anchors were read on main b572f47640 in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem. Rulings: R-60-m1 (`memory/rulings.md:107`), R-124-m1u (`:183`, the smallest thing that works), R-104-m1e (`:163`, artificial clocks), R-142-m1e and R-143-m1e (`:201-202`, a person is never overruled and is told the impact). The 10-06 no-cap rule is quoted at `plans/designs/fleet-survives-its-providers.md:12`: no context cap, no session-hour limit, and a headless session ends only at a completed unit boundary with its handoff.

### Wido's binding words

- 2026-10-19 about 10:10 CEST: "Pause at the unit boundary." A seat that passes its daily spend line finishes its unit, writes its handoff, pauses and tells him. Nothing is cut mid-work. It resumes on the next day, or when a person says so. The line starts at 250 M tokens per seat per day and is raised on evidence.
- 2026-10-19 about 12:50 CEST, three decisions (as relayed by the steward):
  1. Resume: "a person's early resume holds for the REST OF THE DAY window; the seat pauses again only at the next day's crossing; the resume is recorded as the person's override with its effect ("runs unpaused until <local time>")."
  2. Day window: "the ledger keeps the UTC day, but a person never sees or sets UTC: every time shown to a person (status, machine list, alert, resume remedy) is in the host's local time zone, and every time a person sets is read as local time and converted; the status says e.g. "resumes 02:00" in local time, never "00:00Z"."
  3. Breach stop: "the elapsed-budget breach stop ALSO waits for the unit boundary: a breached goal finishes its unit, writes its handoff, then stops and asks."
- 2026-10-19 about 13:40 CEST, word 4 (as relayed by the steward): "YES, only the current unit's own jobs. A breached goal's current unit still obtains its own remaining jobs (its read and at most the allowed corrections) past the elapsed limit; no new unit is admitted; the overrun is recorded on the goal as a budget exception with its minutes."

### Problem

- The fence only alerts. `spend.mode` accepts only `alert` (`internal/config/spend.go:121-127`, default `defaults.go:139`, commented in `metasystem.conf:95`).
- The alert fired and changed nothing. This checkout holds 77 spend-owned episodes (`artifacts/agents/steward/alerts/`). Day tokens reached 5x the line on 10-01 and 10-02 and 4x on 10-03, and every episode reads TRANSPORT_SUBMITTED. The 10-19 assessment counts the ui seat at 1.0 B on 10-03, m1j at 751 M, and one session at 730 M over 22 h (`analysis/behaviour-assessment-2026-10-19.md:29`). "Four seats crossed 250 M" was not recounted (hypothesis).
- The crossing is known in one place. `checkSpendFenceWithMeasureAndMachine` builds `day-<UTC day>` crossings (`internal/steward/health.go:622-686`). The tick passes them only to the alert store (`tick.go:721`, `alert_episode.go:582`).
- The meter covers less than "every seat, every day". The seat reader knows Claude only (`internal/spend/reader.go:97`); a Codex seat's own turns are unmeasured (`reader_claude.go:69`). A delegate job counts only once it is terminal, and then on its start date (`measure.go:215-232`), so a job that runs across midnight never lands in a day the fence can still act on.
- A headless session already ends at every completed unit. The steward's `CompletedBoundary` (`cmd/metasystem/steward_seat.go:197-244`, run each pass at `runner.go:386-389`) records the boundary only for a reviewed unit whose read is published and whose steps are terminal and finished after the session began (`unit_boundary.go:63-158`, stage test `:88-100`). It then signals the session after its handoff is durable (`unit_handoff.go:81-108`). The brief tells a headless seat to stop there (`seat_start.go:603-606`, unless `seat.driver=person`, `:778`).
- **Two paths start the next session, and nothing on either asks about spend** (the first draft named only one):
  - the seat ladder `seatDecision` (`seat_start.go:401-490`, shared with health at `health.go:1329`);
  - the handoff continuation: the runner resumes any live intent (`runner.go:420-428`, `revive.go:422-429`), and `CompleteRevival` decides through `decideForHandoffWithReader`, which returns `ActRevive` for a dead predecessor (`handoff.go:65-110`). Its last look before launch is the provider recheck (`revive.go:174-184`).
- `SeatAtUnitBoundary` (`rearm_checkout.go:59-71`) means only "nothing live". A dead step is logged as an orphan, not counted as busy (`seat_idle.go:101-128`). So it is not proof that the unit finished.
- **The elapsed-budget breach stop can already cut a unit mid-work.** It runs each tick before health (`tick.go:318-321`); an elapsed breach becomes a stop (`internal/dispatch/stop.go:118-120`), and the stop cancels the goal's jobs (`internal/delegation/phases.go:283-290`). This is older than the fence and conflicts with Wido's no-cut word (D9).
- Past its elapsed limit, admission refuses every new job of the goal (`internal/dispatch/admission.go:503-516`: unit launches `unit_launch.go:62`, critic reads `admission.go:328-336`). So a unit whose build finishes after the limit cannot get its review and never completes.
- The breach custodian runs in the tick (`tick.go:321`) BEFORE the runner records the boundary and binds the handoff (`runner.go:386-389`). A completed-work check alone could stop the goal before its handoff exists.
- The provider-hold helper queues its notice on every pass (`revive.go:274`). Delivery removes the pending file (`notify.go:301`) and the transport does not deduplicate (`notify.go:140`), so a held handoff would be announced again on every runner pass.
- The completed-boundary path probes the kernel directly: `SeatAtUnitBoundary` calls `SeatBusyAt` with no probe (`rearm_checkout.go:70`, fallback `seat_idle.go:67`), and the production `CompletedBoundary` builds its work with `identity.KernelProber{}` (`cmd/metasystem/steward_seat.go:229`).
- Two automatic producers start an elapsed stop, and both find it through `dispatch.FindBreachStops` (`internal/dispatch/stop.go:386`):
  - the steward custodian (`tick.go:321`);
  - a refused dispatch, which runs `runBreachStopRoutes` (`internal/delegation/admission.go:138-175`).

  Fresh dispatch and follow-ups each pass two budget checks: revision admission and `requireGoalAdmission` (`delegation/dispatch_phase.go:625-632`, `followup.go:608-612` → `dispatch/admission.go:140-160`, `:319-336`).
- The runner's own authorization for a correction is `allowCorrection`, `MaxRounds` and the counted cap (`internal/launch/unit_run.go:291-300`, `unit_revise.go:178-185`). `allowCorrection` requires the read's decision to be "continue" (`unit_stop.go:284-293`). The correction budget is frozen from `review.stop` (`unit_stop.go:19-30`).
- Public `CompleteRevival` builds its own kernel prober (`revive.go:102`), and the production caller passes an empty `TickConfig` (`cmd/metasystem/steward_verbs.go:250`).
- Alert delivery is at-least-once: the attempt is saved, transport runs, then the result is saved (`alert_episode.go:540-576`).

### Decisions

D1. **Modes.** `spend.mode` takes `alert` or `pause`; `enforce` stays refused. `pause` is refused with "pause lands with U4" until the whole of U4 is on main. If U4 is split, U4a keeps refusing `pause` and U4b lifts the refusal. The compiled default stays `alert`. The root conf line `spend.mode=pause` lands after that, with U1a-U3 on main, so the breach wait, the dry run, the exemptions and every surface exist before anything holds (R-60-m1). Only `day-<d>.tokens` holds a seat. Money and the goal lines stay alert-only: money is a floor while rows are unpriced (design §4), and a goal scope spans the goal's life with no window to resume in.

D2. **The guarantee is narrower than the words, and says so.** It covers measured runtimes within the accounting window, which is the ledger's UTC day. The spend-fence line already prints `inflight`, `unmeasured` and `seat codex: unmeasured`. No meter is added. A Codex seat pauses on its delegates' spend only. A job that runs across midnight counts on its start day, after that day is over.

D3. **One record, one writer per field.** `internal/spendpause` holds `artifacts/agents/steward/spend/pause.json`:
- the crossing state (day, multiple, tokens, limit, crossedAt);
- the state: `pending`, `held`, `resumed` or `exempt`;
- the activation number;
- the transitions {at, from, to, owner};
- the closed hold spans, kept across day changes for 90 days.

The tick writes the crossing state after `tick.go:721`, and only from a valid observation (`SpendObservation` gains `Mode`). The guards (D4) write `held`. D7 writes `resumed` and the span end. A malformed record is an error naming its path: health reads unknown and no guard holds.

D4. **The hold sits on both successor paths, and only after a completed unit.** The test is `heldWorkCompleted(root, home, work, prober)`: no live work, and every held run reviewed, its read published, the last round's steps terminal. It is extracted from `unit_boundary.go:88-150`, so the boundary record and the hold share one rule. A dead open step, a failed step without its review, or a missing review counts as unfinished and recovers first. The hold applies in three places:
- `seatDecision`, after the provider check (`seat_start.go:470-472`), as `Decision{verdict, ActNone, words}`;
- `decideForHandoffWithReader`, as a case beside `providerOutage` (`handoff.go:113-114`), with `ActHold`;
- the final recheck beside the provider recheck (`revive.go:174-184`).

A held seat handoff keeps its intent and binding live through a quiet hold, which returns `ReviveOutcome{Held: true}` and queues no notice (R2-02). A seat-idle intent is cancelled, and the ladder mints a new one after the window. Only the tick and `CompleteRevival` write transitions, both under arbitration (`tick.go:256`, `revive.go:107`). The health call of `seatDecision` reads only. Nothing signals, kills or shortens a live session. A `seat.driver=person` seat skips the guard (D8).

D5. **One sentence, the same everywhere, in local time.**
- "over its daily spend line; pauses after its current unit";
- "paused: daily spend line (251 M of 250 M tokens) since 18:40, resumes 02:00 or `metasystem machine resume m1j`";
- "resumed by Wido at 20:02: runs unpaused until 02:00";
- "exempt: person-driven" or "exempt: at the helm".

Every person-facing time is converted from the UTC instant with the host zone (`time.Local`, not `spend.zone` and not a fixed offset); a time of another day carries its day. Readers:
- the session-main role (`health.go:1353-1354`);
- the spend-fence `pause=` segment (`health.go:670-686`), which reads `would-hold` in alert mode;
- presence, a new optional `seat.Record` field (`internal/seat/record.go:62-81`, set at `seatpresence.go:191`), shown by `machineActivity` (`cmd/metasystem/intent_machine.go:913`);
- the `status` board `SeatView` (`internal/board/view.go:97-102`, `:236-239`), filled in `hostBoardView` (`cmd/metasystem/intent_table.go:610`, as `Reserved` is at `:636-641`);
- the crossing and pause notices and the resume verb.

D6. **One delivery owner for the pause: the `spend-pause` episode.** The crossing episode stays as it is: digest-keyed, never re-sent once submitted or suppressed (`alert_episode.go:639-660`). The `spend-pause` episode opens at the first `pending→held` of an activation, with digest sha256("spend-pause\n<day>\n<activation>"). Each tick, under the alerts lock, reconciles the persisted activation with its episode: it opens a missing one, retries a failed one, and resolves it when the hold ends. The guarantee:
- exactly one episode per activation;
- no further delivery once a submitted result is durable;
- across a crash between transport and result persistence, one redelivery is possible (the store's at-least-once recovery, `alert_episode.go:540-576`). No new delivery machinery is added for that.

The quiet hold (D4) means no handoff hold notice competes. Text: "Seat m1j finished its unit and paused: daily spend line (251 M of 250 M tokens). Resumes 02:00. Resume now: metasystem machine resume m1j (it then runs unpaused until 02:00)."

D7. **Resume.**
- The window ends at the ledger's UTC day end (`measure.go:147,156,232`). The first tick after it closes the span at the window's end, never at the tick's time.
- `metasystem machine resume SEAT` is a person's act at an enrolled terminal, proved as `machine revive` is (`intent_machine.go:87`). It holds for the rest of the window whatever the multiple; the next day's crossing pauses again (binding word 1). It records the override with its effect and prints "m1j runs unpaused until 02:00" before it confirms (R-143). It takes no time argument, so no local-time parser is needed.
- `machine revive` keeps its meaning (`intent_process.go:303-309`). It closes the open span at its start, so its working session is never excluded. It cannot stand in for the resume, because it refuses a cleanly completed launch (`seat_start.go:682-683`).
- `alert clear` proves no person (`intent_alert.go:268-292`).

D8. **Exemptions are shown, not hidden.** A person-driven seat gets no automatic handoff (`unit_handoff.go:24-26`) and is never paused (R-142); it reads `exempt: person-driven`, and its crossing still notifies. A helm tick decides nothing and runs no health (`tick.go:276-283`, `helmTick` `:347-364`). It gains one report-only pass beside its pattern pass (`:360`): measure, write the crossing state as `exempt: at the helm`, update the crossing episodes.

D9. **Every automatic elapsed stop waits for the unit's durable handoff (binding word 3).** Eligibility is one function, `dispatch.BreachStopEligible(root, file, now, prober)`, applied inside `FindBreachStops` (`stop.go:417-463`). That covers both automatic producers: the custodian and `runBreachStopRoutes`. An ineligible breach becomes a route with condition `DEFERRED` ("goal G finishes its unit first"): the custodian reports it, and `runBreachStopRoutes` refuses the dispatch without stopping. A breach is eligible when:
- (a) the goal's current unit has a boundary event carrying a durable handoff (`unit-boundaries.json`, `Handoff` set by `bindUnitHandoff`, `unit_handoff.go:54-66`). A person-driven seat needs the boundary event alone. The read and type of that file move to a small `internal/unitboundary` package that steward and dispatch share;
- (b) the current unit cannot advance (D10's terminal disposition) and no live step or job of the goal remains, so remaining live work is preserved, never cancelled;
- (c) the goal has no unit run in progress.

The custodian additionally stops when the seat cannot advance at all: the stop fence is closed, or the abnormal-restart bound gives a reason (`seat_start.go:484-487`). A person's explicit stop and the stop-fence refusal (`unit_launch.go:66`) do not go through eligibility and stay immediate. The handoff decision gains one rule: a goal with a stop fence counts as no longer owned (`handoff.go:65-70`), so no successor starts. The notice asks once per revision (`tick.go:203`; `signal_channel.go:104-106`, which gains "after finishing its unit", the overrun and, for (b), the reason).

D10. **The current unit finishes past the elapsed limit (binding word 4).** The grace is request-specific. A request qualifies when it names an existing unit run of the goal whose first round was admitted before the limit instant, and either of these holds:
- it is that run's read;
- it is a correction or revision the runner itself has already authorized (`allowCorrection`, `MaxRounds`, counted cap, divergence: `unit_run.go:291-300`, `unit_revise.go:178-185`; the frozen correction budget `unit_stop.go:19-30`).

For a qualifying request, every elapsed check waives the elapsed refusal (closed or breach) and the elapsed live-stop reason. The checks are:
- unit-launch admission (`unit_launch.go:43-70`);
- revision admission (`admission.go:319-336`, including `liveStopReason` at `:328`);
- goal admission (`admission.go:140-160`).

The delegation session carries the request's unit-run evidence into both checks (`dispatch_phase.go:625-632`, `followup.go:608-612`). The waiver never covers a new run, attempt or minute limits, or a stop-fenced goal. `elapsedGrace: true` on a launch record is admission evidence only.

Terminal disposition: when the runner refuses the next correction (decision not "continue", rounds or counted cap exhausted, divergence), or a non-waived attempt or minute limit refuses the read or correction, the goal records the reason. It admits no new unit, keeps its live work, and becomes eligible under D9(b) once that work ends. Then it stops and asks.

Overrun: `elapsedOverrun` is the effective goal clock (`budget.go:375`, minus every exclusion of D11) past the elapsed limit, frozen at the unit's boundary or at the stop. Waived job minutes are reported separately as `graceJobMinutes`. Both are shown in goal show, goal budget and the stop notice.

D11. **The accepted invariant is amended.** "No admission code imports or reads spend" (design §7, `:244`) becomes: admission and dispatch never read spend measurement or ceilings and never refuse on them. The budget projection may read `spendpause` hold spans for clock accounting only. Those spans join the existing union (`dispatch/budget.go:938-957`) for goals claimed by the held machine within a span, and the steward's age sample (`tick.go:590-597`). A malformed span history makes the budget unknown with its path, as provider history does (`budget.go:939-941`). `TestAdmissionNeverConsultsSpend` (`internal/spend/measure_test.go:859`) is replaced by U4's behavioural test.

D12. **Every probe on the tested path is injected.** U1a threads one `identity.Prober` through:
- `RunTick` (`tick.go:262`) and the health completion (`:685`);
- `SeatAtUnitBoundary` and `SeatBusyOptions.Probe` (`rearm_checkout.go:59-70`, `seat_idle.go:67`);
- `ObserveUnitBoundary` and `heldWorkCompleted`;
- the production `CompletedBoundary` callback (`steward_seat.go:197-244`, `:229`), from the steward config;
- public `CompleteRevival` (`revive.go:102`, `HandoffProber`) through a new `TickConfig.Prober`, which the production caller fills (`steward_verbs.go:250`);
- `FinishUnitHandoff` (`unit_handoff.go:95`), with an injected signaller.

Production passes `identity.KernelProber{}`, as today.

### Units

Production lines only; seams count, tests do not. The inventory comes from the sites above, and no line is counted twice. Order: U1a, U1b, U2, U3, U4 (or U4a then U4b), then the root conf switch.

- **U1a, every automatic elapsed stop waits for the unit (D9, D12; usable alone: no automatic stop cuts a unit).** Seams, including `CompleteRevival` and its caller (49). `heldWorkCompleted` (35). The `internal/unitboundary` move (30). `BreachStopEligible` (a)/(b)/(c) with its live-work probe (55). `FindBreachStops` wiring and the `DEFERRED` condition (15). The custodian branch (20). Words and notice (12). Fenced-goal handoff (6). The terminal-reason record (15). About 237; bound 250.

  Before U1b lands, an elapsed refusal is itself a terminal disposition: the unit stops when its live work ends, and is never cut.
- **U1b, the current unit finishes past the limit (D10; usable: a breached unit completes its read and corrections).** The grace predicate on runner authorization (30). Three admission sites (45). Delegation evidence at the two dispatch paths (20). `elapsedOverrun` from the clock and `graceJobMinutes` (35). About 130; bound 150.
- **U2, dry run (pause refused; calibration; helm and exempt reporting).** The record with activation, transitions, spans and malformed handling (110). Local-time words (30). The tick writer and `Mode` (12). The `would-hold` segment (18). The helm report pass (25). The exempt words (8). The refusal text (4). About 207; bound 225.
- **U3, the surfaces (they show the dry run until U4).** The presence field, raw read, compose and `machineActivity` (24). Board `SeatView`, `hostBoardView`, `seatText` (18). The crossing text in both modes (10). About 52; bound 70.
- **U4, the hold (lifts the `pause` refusal).** Config (6). The `seatDecision` guard with the person skip (17). The handoff case and recheck (22). The quiet hold (8). Transitions, including the revive span end (18). The `spend-pause` episode and its reconcile (55). `machine resume`: table, run, `spendpause.Resume` (77). Clock spans, the union and the age sample (42). About 245; bound 250.

  The split line is fixed now: if the brief's measured inventory passes 250, U4 becomes U4a (config refusal kept, guards, quiet hold, transitions, episode; about 126) and U4b (resume, clock, lifting the refusal; about 125).

Five units, about 870 lines.

### Tests

Each test drives one runner pass (`runner.go:360-440`: tick, `CompletedBoundary`, ladder start, `Resumable` revival) through the production callbacks. It injects the clock, the measure func (`health.go:609`), the D12 prober with synthetic process identities, a fallback probe that fails the test if invoked, the signaller and a fake launcher. It counts seat launches, continuation launches and every transport: `Deliver`, the channel and the notify queue (R-104).

- **U1a.** These run through real delegate dispatch and a follow-up (`dispatch_phase.go:625`, `followup.go:608`) as well as the runner pass.
  - `TestBreachStopWaitsForTheDurableHandoff` (ordered events): it starts with a published review and NO handoff. Neither the custodian nor a refused dispatch stops on that tick. The runner binds the handoff, and the following tick stops. Handoff persistence precedes the stop record.
  - `TestElapsedBreachWaitsForTheUnitPastTheSpendLine`: zero `CancelUnitLaunch` and proof-cancel calls while the step is live, through both producers.
  - `TestPersonStopAndFenceRefusalStayImmediate`.
  - `TestBreachStopRunsWhenTheSeatCannotAdvance`: the restart bound has a reason.
  - `TestRevivalUsesTheInjectedProber`: public `CompleteRevival` and the production `CompletedBoundary`, with a synthetic predecessor and the real-kernel fallback forbidden.
  - Mutations: make eligibility `heldWorkCompleted` alone (the ordered test stops before the handoff); bypass eligibility in `runBreachStopRoutes` (the dispatch-path cancel count becomes 1); leave the kernel prober in `CompleteRevival` (the forbidden fallback fails).
- **U1b.**
  - `TestBreachedGoalFinishesItsCurrentUnit` (Astra's production-admission test), through real delegate dispatch and a follow-up: the limit passes while the build runs; the review is requested after the limit and passes revision and goal admission; it completes, the read is published, and the handoff becomes durable; then the stop runs and notifies once; a new unit is refused.
  - `TestCorrectionAllowanceExhaustedStopsAndAsks`: the read decides "stop", and no correction is admitted. The live build finishes, the reason is recorded, then the stop runs.
  - `TestElapsedPlusAttemptOrMinuteExhaustion`: the attempt limit and the minute limit are each reached with elapsed waived. The read is refused, live work is kept, and the stop runs with that reason.
  - `TestOverrunFromTheGoalClock`: a build admitted at limit −10 min that ends at limit +10 min, then a 5 min read, gives `elapsedOverrun` 15 min and `graceJobMinutes` 5; two overlapping 5 min grace launches add 5 min of overrun and 10 waived job minutes; a 2 h hold span inside the overrun is excluded.
  - Mutations: drop the grace from goal admission (the review is refused at `admission.go:156`); sum grace launches as the overrun (15 becomes 5).
- **U2.**
  - `TestDryRunReportsWouldHoldAfterTheCompletedUnit`.
  - `TestHeldWorkCompletedNeedsPublishedReviewAndTerminalSteps`: a dead open step, a failed step without review and a missing read are each false.
  - `TestMalformedPauseRecordHoldsNothing`.
  - `TestCodexSeatPausesOnDelegatesOnly` and `TestJobAcrossMidnightCountsOnItsStartDay`, on the real bed (`internal/spend/testdata/`).
  - `TestHelmTickReportsTheCrossing`.
  - `TestPauseWordsAreLocalAcrossSeasons`, under `TZ=Europe/Amsterdam`. Each fixture checks the exact local resume time and that the release instant equals the ledger's 00:00 UTC boundary: 2026-07-15 → "02:00"; 2026-12-15 → "01:00"; 2026-10-24 (the window ends 2026-10-25 00:00 UTC, before the 01:00 UTC change) → "02:00"; 2026-10-25 → "01:00"; 2026-03-28 → "01:00"; 2026-03-29 → "02:00".
  - Mutations: use `SeatAtUnitBoundary` alone (the dead-step case fails); replace the zone conversion with a fixed +2 h (the winter and transition fixtures fail).
- **U3.**
  - `TestDryRunReadsTheSameEverywhere`: `machine list`, the `status` board and the session-main line carry identical words and local times for the summer and winter fixtures.
  - Mutation: the presence field is omitted, and the machine-list case fails.
- **U4.**
  - `TestSpendPauseHoldsTheHandoffContinuation`: a genuine durable handoff. Over five runner passes and a steward restart there are zero continuation launches, the intent stays live, and the transport total is exactly 1, the `spend-pause` notice. At 02:00:01 local the same nonce launches, and the span is 18:40 to 02:00.
  - `TestSpendPauseHoldsTheLadder`.
  - `TestSpendPauseRecoversADeadOpenStep`.
  - `TestPersonDrivenSeatIsNeverHeld`.
  - `TestPauseNoticeOncePerActivation`: an alert→pause switch mid-day, a suppressed crossing, and a revive then re-hold; the transport total is one per activation.
  - `TestPauseNoticeAcrossRestarts`. A restart before episode creation gives one transport. A clean restart after a submitted result gives no further transport. A crash injected after transport and before result persistence allows exactly one redelivery, and none after that.
  - `TestMachineResumeLastsTheRestOfTheWindow`, through the public verb. An agent is refused. A person's resume prints and records "runs unpaused until 02:00" (and "01:00" in the winter fixture). The seat still runs at 501 M and at 751 M, and holds again at the next day's crossing.
  - `TestGoalClockExcludesOnlyHeldTime`, through `ProjectBudget`. A crossing at 14:00, work to 20:00, held 20:00 to 22:00, revived 22:00 to 00:00, held 00:00 to 02:00, and a provider wait 01:00 to 01:30 exclude 4 h. Yesterday's span survives today's record.
  - `TestAdmissionIgnoresSpendOutsideHeldSpans`: alert mode with a crossed ledger leaves admission unchanged; a malformed span history gives an unknown budget.
  - Mutations: the hold queues its ordinary notice (transport total 6); drop the handoff case (continuation count 1); end the resume at the next multiple (the 501 M check fails); replace the union with `outage.Paused(providerSpans) + outage.Paused(spendSpans)` (4.5 h, not 4 h).

### Threat model

- **A seat that never reaches a boundary** (the 730 M session). It is never cut. It reads pending, and each multiple is a new crossing episode. There is no finite spend bound.
- **A breached unit that keeps needing corrections.** It ends at the runner's own authorization (D10 terminal disposition) or the restart bound, with live work preserved, and then asks.
- **A crossing during a lane proof.** The lane starts no seat (`seat_start.go:407-417`), so a proof never pauses. A returned goal waits for the window. Lane clocks stay with lane-recovers-and-pauses D1.
- **A breach and a crossing together.** The stop runs after the durable handoff (D9a), and the fenced goal gets no successor, so the hold has nothing to start.
- **Two seats on one account.** Each seat has its own nominal line; the account limit is fleet-survives-its-providers P2.
- **The time zone.** The ledger counts the UTC day. People see local time across both seasons and the transition days.
- **A resume before the window.** It is a recorded override that lasts to the window's end.
- **A held handoff overnight.** Handoff expiry has no default limit (`handoff.go:48-50`).
- **A stale or unreadable record.** It holds nothing and reads unknown.

### Not in scope

Pausing on money or goal lines; a fleet or account sum; a Codex seat meter; cutting, signalling or capping any session, context or hour; dispatcher refusal (`enforce`); price rows; lane waiting.

### OPEN (Wido)

None. Wido settled all four decisions (binding words 1 to 4).

### Brief obligations

The steward cannot settle these from this page's reads. Each brief verifies them and records the result before the build.
- **BO-1 (U1b).** How the unit-run identity and the runner's authorization reach `requireGoalAdmission` and `requireGoalRevisionAdmission`. Delegation reads `unitRun` only from job records today (`phases.go:292`); the brief names the field that carries it into the claim, inside U1b's bound.
- **BO-2 (U1a).** Whether the runner's refusals (`allowCorrection`, `roundLimit`, counted cap, divergence) persist on the unit run record. If they do not, U1a records them at `unit_run.go:291-300` and `unit_revise.go:178-185`, inside its terminal-reason lines.
- **BO-3 (U1a).** The person-stop verbs and the stop-fence refusal do not reach `FindBreachStops`. The brief lists their call sites, and `TestPersonStopAndFenceRefusalStayImmediate` covers them.
- **BO-4 (U4).** The measured inventory decides the U4a/U4b split at the fixed line. U4a keeps refusing `pause`.
- **BO-5 (all).** Line counts are inventory estimates. A unit whose measured diff passes 250 is split at a named, independently usable seam before its build, not after.

### Critique record

- Round 1 (Astra, gpt-6-astra, 2026-10-19, `analysis/spend-fence-amendment-critique-1.log`; 11 material, 2 minor). Revision 2 folded it as follows; Wido answered SF-03 and SF-11 at about 12:50 CEST.

| Id | Disposition |
|---|---|
| SF-01 | folded: D4, the handoff case and final recheck keep the intent live; Problem corrected (two successor paths) |
| SF-02 | folded: D4 `heldWorkCompleted`; unfinished work recovers first |
| SF-03 | folded on Wido's words 3 and 4: D9 (stop after the durable handoff) and D10 (the current unit finishes past the limit), in U1a and U1b, with Astra's tests |
| SF-04 | folded: D3 transitions with owners, retained spans; D7 revive ends the span; D10 excludes held time only |
| SF-05 | folded: U1 named a dry run; U3 carries the resume and the clock; units re-inventoried with the seams |
| SF-06 | folded: D2 narrows the guarantee; real-meter fixtures in U1 |
| SF-07 | folded: tests drive the runner pass; seams counted in U1; mutations fixed (dead open step, exact additive substitution) |
| SF-08 | folded: D10 amends the invariant; behavioural test with a malformed record |
| SF-09 | folded: D8 exempt states and a helm report-only pass; U4 test |
| SF-10 | folded: D6 `spend-pause` episode per activation |
| SF-11 | folded on Wido's words 1 and 2: rest-of-window resume recorded as an override with its effect; UTC ledger day, local time for every person |
| m-01 | noted: episode figures confirmed; the four-seat count stays a hypothesis |
| m-02 | folded: the line is nominal, with no finite spend bound |

- Round 2 (Astra, gpt-6-astra, 2026-10-19, `analysis/spend-fence-amendment-critique-2.log`; 6 material, 2 minor, down from 11). Revision 3 folds all of it; this is the last fold round under the stop rule.

| Id | Disposition |
|---|---|
| R2-01 | folded on Wido's word 4 (13:40 CEST): D10 waives the elapsed breach for the current unit's own jobs only (its read and at most the allowed corrections), never a new unit, and records the overrun minutes on the goal; Astra's production-admission test `TestBreachedGoalFinishesItsCurrentUnit` is in U1 |
| R2-02 | folded: D4 quiet hold, no ordinary handoff notice; D6 makes the `spend-pause` episode the only delivery owner, reconciled every tick and after restart; the U4 test counts every transport over five passes and a restart |
| R2-03 | folded: D12 threads the prober through `SeatBusyAt`, `SeatAtUnitBoundary`, `ObserveUnitBoundary`, `heldWorkCompleted` and the production `CompletedBoundary` callback; failing-fallback test; U1 recounted (41 seam lines) |
| R2-04 | folded: D9 stop eligibility needs the current unit's durable handoff (or a seat that cannot advance), and a fenced goal gets no successor; the ordered-event test starts with a published review and no handoff |
| R2-05 | folded: recut so the breach wait (U1), the dry run with helm and exempt reporting (U2) and the surfaces (U3) land before the enforcing U4; the person-driven skip and its test sit in U4; the conf switch comes after U4; no line counted twice |
| R2-06 | folded: dated summer, winter and both transition fixtures with exact local times, the release instant equal to the ledger boundary, and a fixed +2 h mutation that fails |
| R2-m01 | folded: D11's behavioural test is in U4, the clock unit, and the text says so |
| R2-m02 | folded: the provider case is cited at `handoff.go:113-114` |

- Round 3 (Astra, gpt-6-astra, 2026-10-19, `analysis/spend-fence-amendment-critique-3.log`; 5 material, 1 minor; 11 → 6 → 5). This is the last round under the stop rule; revision 4 is the final text. Status stays draft until the steward accepts it.

| Id | Disposition |
|---|---|
| R3-01 | folded: D9 puts eligibility in `FindBreachStops`, which covers both producers, the custodian and `runBreachStopRoutes`, while person stops and fence refusal stay immediate; D10 carries the grace through unit-launch, revision and goal admission with delegation evidence; U1 recut into U1a (237) and U1b (130), each usable; tests run through real dispatch and a follow-up; BO-1, BO-3 |
| R3-02 | folded: D12 threads the prober through public `CompleteRevival` via `TickConfig.Prober` and its production caller; `TestRevivalUsesTheInjectedProber` forbids the kernel fallback |
| R3-03 | folded: D10 defines grace on the runner's own authorization (`unit_run.go:291-300`, `unit_revise.go:178-185`, `unit_stop.go:19-30`, `:284-293`), not `board.Round.Max`; the terminal disposition keeps live work, records the reason, admits no new unit, then stops and asks; tests for allowance exhaustion and elapsed-plus-attempt or minute exhaustion; BO-2 |
| R3-04 | folded: `elapsedOverrun` comes from the effective goal clock with exclusions, frozen at the boundary or the stop; `elapsedGrace` is admission evidence only; `graceJobMinutes` is named separately; exact fixtures for the straddling build and overlapping launches |
| R3-05 | folded: D6 states one episode per activation, no delivery after a durable submitted result, and a possible redelivery across the crash gap; `TestPauseNoticeAcrossRestarts`; exactly one transport for ordinary passes and a clean restart |
| R3-m01 | folded: D1 says U4a keeps refusing `pause` until U4b lands |
