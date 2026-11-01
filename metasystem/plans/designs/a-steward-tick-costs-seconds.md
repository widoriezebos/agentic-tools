# Design for a-steward-tick-costs-seconds

- Kind: design
- Id: a-steward-tick-costs-seconds
- Status: accepted
- Goals: a-steward-tick-costs-seconds
- Critique: closed at round 1 (Astra, direct read designs/tick-cost-critique-1.findings.md; 2 material and 5 minor accepted, folded in revision 2)

Accepted 2026-11-01 by a1 in Wido's name (decision 127) after Astra's round 1 was folded. Revision 2, written 2026-11-01 by opus-designer (Claude Opus 5.5) from the steward's request in Wido's name (decision 127). It folds Astra's round 1 (`agentic-tools-evidence/lane-test-20261009/designs/tick-cost-critique-1.findings.md`: 2 material, 5 minor, "accept after the folds") as the steward decided them (`agentic-tools-evidence/lane-test-20261009/designs/tick-cost-page-r1-dispositions.md`, every finding accepted; no round 2). The folds: finding 1 into D6, T7 and section 1's read inventory; finding 2 into D1, D2, T1 and T2; the minors into D3, T5, D8, D5 and section 5; the diagnosis note into section 1. Revision 1 is kept beside this page as `a-steward-tick-costs-seconds.md.before-astra-r1`. Revision 1 was written the same day from finding 470 (`agentic-tools-evidence/lane-test-20261009/findings.md:1170`). Code: the m1e checkout at `1627fdbf03bf777ba356e83368aaf422a48d28af` for both revisions; every `file:line` below was read there, paths under `metasystem/`. The lane's running engine is build `cbcf8161810b` (`agentic-tools-landing/metasystem/artifacts/agents/steward/identity.json`), and `git diff cbcf816181 1627fdbf03` is empty for `internal/steward`, `internal/landing`, `internal/goal` and `cmd/metasystem/landing_agent.go`, so the code read is the code running. Rulings relied on: R-104-m1e (time-bound tests run on an injected clock). Wido's intent: "A steward tick costs seconds, not a minute: the tick line times each duty, the tick reads the ledger once, and the claim-quota role sends one note per blocking occurrence; the landing steward no longer burns a core." Measure first.

## 1. Problem

The landing lane's steward runner burns most of a core. The tick line cannot say where. Two premises of the finding are wrong, and the page corrects them first.

Measured (ps, 2026-11-01, own CPU only; ps `TIME` leaves out reaped children):

- Lane steward, pid 93496: 66:55 at 2:56:58 elapsed (finding 470); 81:30 at 3:32:50 (15:48 local). That is 14.6 CPU minutes in 35.9 minutes, 41 percent of a core in the Go process alone. The m1e steward (pid 16868): 16:28 in 2:24:49, 11 percent. The ui steward (pid 90907): 18:37 in 3:33:15, 9 percent.

Premise corrected: the lane steward does not tick every 120 s.

- The cadence is `TickSeconds`: git config `metasystem.steward.tick-seconds`, default 600 (`internal/steward/runner.go:107-122`, used at `cmd/metasystem/steward_verbs.go:439`). It is unset in both checkouts (`git config --get` exits 1), so both run at 600 s.
- The lane steward's tick attempts: 20 between 11:18:44Z and 14:41:48Z (`components/steward-tick.json`, `attemptSeq`), one per 641 s. The m1e steward: 15 between 12:26:46Z and 14:46:47Z, one per 600 s.
- The word "slept" counts only the 200 ms sleeps (`runner.go:332`, `:570`). While the lane waits, held included, the runner steps the lane's keeper every 15 s (`laneRecheck = lane.AgentTick`, `runner.go:477`; `internal/landing/lane/agent.go:48`; the step at `runner.go:535-538`). That step's time is in no word of the line.
- So a lane cycle is about 641 s. Ticks 14 to 16 (`agentic-tools-landing/.../runner.log:8652-8655`) hold 77.3, 74.9 and 92.2 s of work and 129.8, 120.9 and 120.8 s of sleep; 434, 445 and 428 s are timed by nobody (Astra's arithmetic, checked).

The likeliest sink, an inference U1 measures: the lane keeper's step. Each recheck runs `keeper()` and the plain landing steward pass, composed in `landingStewardTick` (`cmd/metasystem/landing_agent.go:511-527`). The keeper refreshes the lane's shadow before any early return (`landing_agent.go:292-296`, `plain.RefreshShadow`). The steward pass calls `ObserveStatus` (`internal/landing/plain/steward.go:88`), whose observation reads the lane's raw files (`internal/landing/plain/observation.go:66-70`), runs git for HEAD, branch, merge head, main and origin main (`observation.go:144-148`), and lists and reads launch records (`internal/launch/record.go:224`). The keeper's line in `runner.log` changes about once a minute, which fits steps of 55 to 60 s each. On every other steward the keeper returns `AgentNotKept` (`landing_agent.go:514-515`) and the wait does not step it again.

Premise corrected: the claim-quota role does not send 36 notes per tick at this SHA.

- `askOnce` returns before publishing when the subject's receipt exists (`internal/steward/ask_once.go:23-28`). The m1e steward holds 313 receipts; three were written after noon today (15:17, and two new pairs at 15:47). So the ask is once per (ready, held) pair.
- The per-tick cost is real, and it sits in "health", not in "other". For each ready priority-1 goal the role runs `ClaimQuotaBlocker` (`internal/steward/claim_quota.go:55-59`), which reads the landing lane for the held goal (`internal/goal/verbs.go:1480-1481`, through `cmd/metasystem/landing_plain.go:41-63`). It then runs `claimActivity` (`claim_quota.go:65`, `:78-93`), which reads every job record (`internal/goal/project.go:590-604`; 150 files on m1e) and every open question. With 36 ready goals and one holder, that is 36 lane reads and 5 400 job-file reads for one answer. m1e's health phase is 7 to 30 s, the lane's 1.5 to 6 s (the lane checkout holds no claim).

What "other" is:

- `other = work - health - census - situation` (`runner.go:322-326`). `work` runs from the tick's start to the wait (`runner.go:330-334`). The census is not a phase of its own: the runner hands one `timedCensus` accumulator (`internal/steward/runner_reads.go:98-108`) to the tick (`runner.go:346`) and to seat recovery (`runner.go:449`; the census read at `internal/steward/recovery_request.go:121`), so its time sits inside those two. So "other" is the tick body outside health and census, and every duty after the tick: disk sweep (`:359-361`), question replies (`:369-373`), unit boundary (`:392-394`), rearm (`:395-404`), the first keeper step (`:406`), resume and revive (`:424-447`), seat recovery (`:449`), notifications (`:453-454`), channel (`:455-461`), cache trim (`:465-467`).
- `recordDuty` stamps each duty's end in `duties/duties.json` (`internal/steward/duties.go:61-80`). Two cycles read from those stamps:

| Phase | lane, cycle from 14:41:48Z | m1e, cycle from 14:46:47Z |
|---|---|---|
| tick (`RunTick`, health and census included) | 16.0 s (`lastDurationMillis` 15977) | 49.1 s (49136) |
| disk sweep | 13.7 s | 22.9 s |
| question replies | 0.8 s | 0.7 s |
| situation page | 10.0 s | 22.0 s |
| unit boundary | 0.03 s | 0.8 s |
| rearm, keeper step, resume, seat recovery | 60.9 s | 24.8 s |
| notifications | 0.1 s | 0.02 s |
| channel | 1.5 s | 0.8 s |
| cache trim | 10.6 s | 10.3 s (previous cycle) |

- The situation page rewrites `situation.json` every tick: 23.7 MB on the lane, 25.1 MB on m1e (ls). Whether the write or the read costs the 10 to 22 s is not known (hypothesis).

The full ledger reads U1 counts. A full read lists and reads the goal blobs with git (`internal/goal/validate.go:687-691`; `internal/goal/repository.go:52-67`) and parses every record (`ParseTreeFiles`, `validate.go:99`); 660 goal records today. Validation then runs every at-rest rule (`ValidateTree`, `validate.go:254`). Readers in one cycle (Astra's inventory, each anchor checked):

| Owner | Reads and conditions | Anchor |
|---|---|---|
| health | one shared offline projection | `internal/steward/health_ledger.go:39-48` |
| open-work decision | one fetch-first projection; a fetch that moves the tip also validates the fetched commit | `internal/steward/openwork.go:64`; `internal/goal/project.go:453`, `:268-277` |
| ledger attention | a baseline or a before projection, one per transition (not bounded), and a `ValidateCommit` on every successful remote pass | `internal/steward/ledgerattention.go:230`, `:253`, `:601`, `:661`; `internal/goal/validate.go:744-760` |
| presence | a lazy `SeatBox` projection when the seat's work names a goal | `internal/steward/seatpresence.go:186`; `internal/steward/seatworking.go:87` |
| seat operations | separate conditional projections for reaping, selection and launch facts | `internal/steward/seat_start.go:324`, `:430`, `:1046` |
| validation failure | an offline projection, only on a cadence failure | `internal/steward/validation_window.go:193` |
| disk sweep | a lazy projection, again when the accepted tip moved | `internal/steward/disk_attempts.go:53-83` |
| situation | its own shared projection | `internal/steward/situation.go:172-175` |
| rearm, recovery, revival | conditional fresh reads | `internal/steward/rearm_checkout.go:66`; `internal/steward/recovery_request.go:128`; `internal/steward/revive.go:385` |
| channel | no routine projection; a pending split approval triggers a fresh one | `internal/channel/phase/phase.go:233`; `internal/channel/split.go:60` |

So the count per cycle has no fixed bound: ledger attention reads once per transition. Revision 1's "four to eight" is withdrawn. Revision 1 also priced a read with the comment at `health_ledger.go:11-12` ("about half a second apiece"); that is a historical note, not a measurement. U1 measures count and cost (D6).

A main-thread sample (finding 470) shows wait4, fork and open. The process's own CPU (41 percent) says Go work, not only children. U1 tells both apart.

## 2. Decisions

- **D1 The tick line names every phase.** One line per cycle, written where it is today (`runner.go:322-327`). The top-level phases are exclusive and run in order: the tick, every duty after it, the first keeper step. They sum to `work`. Included subtotals stand in brackets after the phase that holds them and are never added again: the tick's inner phases (D2), health inside the tick, census inside the tick and inside seat recovery. The wait follows: slept, the keeper steps as a count and a total. Example: `tick 20: work 113.6 s (tick 16.0 s [governed 0.2, validation 0.0, rulings 0.1, counselor 0.0, reap 0.1, ledger-attention 1.2, presence 2.4, decide 4.1, narrate 0.6, health 3.6, reports 3.7; census 0.1], disk-sweep 13.7 s, question-replies 0.8 s, situation 10.0 s, unit-boundary 0.0 s, rearm 0.4 s, lane-keeper 58.2 s (keeper 3.1, lane-steward 55.1), revive 0.0 s, seat-recovery-requests 2.3 s [census 0.1], notifications-pending 0.1 s, channel-pending 1.5 s, cache-trim 10.6 s); wait 527.4 s (slept 121.0 s, lane-keeper 7 steps 406.4 s); cpu self 71.2 s, children 9.8 s; ledger 9 reads 4.1 s`. The figures are illustration, not measurement. The words health, census, situation and slept keep their meaning; "other" goes.
- **D2 One clock, marks between phases.** A `tickPhases` recorder (new `internal/steward/tick_phases.go`) starts at the tick and marks each top-level phase end with `deps.Now()`; a phase's time is the difference to the previous mark. At each mark it also takes the census accumulator's growth since the previous mark (`timedCensus.elapsed`, `runner_reads.go:101`), so the census is shown inside the phase it ran in. The runner marks beside each `recordDuty` and each step (`runner.go:360`, `:372`, `:378`, `:393`, `:396`, `:406`, `:430-447`, `:449`, `:454`, `:460`, `:466`). Inside `RunTick` the inner marks use the clock the health phase already uses (`tickHealthNow`, `tick.go:126`, `:535`, `:785`) and ride back on `TickResult` beside `healthElapsed` (`tick.go:195`). Inner marks after: governed (`:355`), validation (`:359`), rulings (`:362`), counselor (`:365`), reap (`:553`), ledger attention (`:562`), presence (`:576`), decide (`:600`), narrate (`:608-633`), health (`:634`), reports (the deferred `runTickReports`, `:318-324`). The inner health mark is the line's health figure; `healthElapsed` is not printed beside it.
- **D3 The wait is timed, with its limits named.** `runnerWait` adds each keeper step's time and count to the recorder (`runner.go:536-538`). `slept` keeps meaning the sleeps alone. Outside every measured figure stay: the wait's setup, which resolves settings and the provider outage mark (`runner.go:522-529`); the polling between sleeps (the stop-file stat and helm read, `:531-539`); and, rare, the provider probe while a limit stands (`:549-568`). The line prints them as one remainder, `wait other`, so `work + slept + keeper steps + wait other` equals the cycle on the clock.
- **D4 The keeper step says its two halves.** `landingStewardTick` (`landing_agent.go:511-527`) times `keeper()` and `step()` with the engine's clock and returns both on `lane.AgentRun` (`internal/landing/lane/agent.go:153-160`, two `time.Duration` fields). The line shows `lane-keeper 58.2 s (keeper 3.1, lane-steward 55.1)`, and the wait's steps the same way in total.
- **D5 CPU of the process and of its reaped children.** Each phase records the delta of `getrusage(RUSAGE_SELF)` and `getrusage(RUSAGE_CHILDREN)` user plus system time, behind an injected reader. `RUSAGE_CHILDREN` holds only children already reaped by wait4; `exec.Cmd` reaps each before it returns, so a phase's children are counted in that phase. The line prints the cycle's totals and, per phase, `cpu` only when self or children exceed one second. Today's helper reads SELF only (`internal/steward/cpu_time_unix_test.go:11-18`, a test file); production gets a reader for both in `tick_cpu_unix.go`, with a zero reader for other systems. The page declines a subprocess count: the kernel gives no per-process fork count on darwin, and counting at the 17 `exec.Command` sites in `internal/steward` plus those in `internal/goal`, `internal/landing` and `cmd/metasystem` means a wrapper in every package; children's CPU answers which phase pays for forks.
- **D6 Every full ledger read is counted and timed.** The count is one per `ParseTreeFiles` call (`internal/goal/validate.go:99`), the parser every full read passes: projections through `loadTreeFor` (`verbs.go:472`), `ValidateCommit` (`validate.go:749`), the fresh fetch's `validateCommitFor` (`validate.go:770`), and the attention and validated reads (`internal/goal/attention.go:471`, `:524`; `internal/goal/validatedread.go:46`). The cost is the time spent in the three steps of a full read: the blob read (`readCommitGoalBlobsWithRun`, `validate.go:691`), the parse (`ParseTreeFiles`) and the at-rest rules (`ValidateTree`, `validate.go:254`), each timed with a package clock the tests replace. An exported `goal.LedgerReads()` returns count and total; the line prints the cycle's delta (`ledger 9 reads 4.1 s`). Verbs in the same process count too; the steward runs none during a cycle, so the delta is the cycle's. The blob time includes the channel tree read at `internal/goal/channel.go:163`; the page says so rather than split it.
- **D7 U2 fixes what U1 names; the steward fills it in.** PLACEHOLDER, filled by the steward from the first six tick lines after U1 runs on the lane and on m1e: phase `<P>`, cost `<C s per cycle>`, remedy `<R>`, mechanism `<file:line>`. The page is amended with the numbers and gets one Astra round on D7 alone. The candidates, only so the reader sees what U1 will decide between; none is decided here:
  - the lane's keeper step leads: the plain landing steward pass (`landing_agent.go:517`, `plain.StewardStep`) runs once per cycle, and the 15 s recheck steps only the keeper's start check;
  - ledger reads lead: one read per cycle through the existing seam `TickConfig.GoalProjection` (`tick.go:40-41`, `:164-171`, `:543-545`, `:595-597`), fetched once, handed to the readers that may take a snapshot. A shared read must keep what each owner needs: ledger attention's historical tips, the disk sweep's changed-tip check before a deletion (`disk_attempts.go:53-66`), and fresh reads before revival, recovery, rearm and approval delivery;
  - the claim-quota role leads (m1e health): `claimActivity` and the lane read once per held goal per evaluation, not once per ready goal (`claim_quota.go:55-66`);
  - disk sweep or cache trim leads: their own cadence, as a setting;
  - the situation page leads: the page's write only when it changed.
  The target for U2: a tick's `work` under 10 s, and the lane steward under 10 percent of a core over one hour (ps). If one unit of 250 lines does not reach it, the rest is a follow-up goal, not a third unit.
- **D8 Where U1 publishes.** The tick line in `runner.log` carries every figure. The steward-tick component record keeps what it has: it is completed inside `RunTick` (`tick.go:637`), before any post-tick duty runs, so its `lastDurationMillis` is the tick phase alone and U1 does not change it. Nothing new goes to `health.json`: that record holds the health state and verdict (`internal/steward/health.go:306-309`), and revision 1 promised no field there. A record of the cycle's phases is rejected (section 9).

## 3. What does not change

- The cadence. `metasystem.steward.tick-seconds` stays the setting, default 600 (`runner.go:107-122`). The 15 s lane recheck stays a constant in U1 (`lane/agent.go:48`).
- The duties, their order, their `duties.json` record and their health roles (`duties.go:15-32`).
- The claim-quota ask: once per (ready, held) pair, owned by finding 448 and the-steward-acts.
- `steward-tick.json`, `health.json` and every component record (D8).
- Every ledger reader's own read. U1 counts reads; it shares none.
- The helm path: the same marks, fewer phases (`runner.go:385-391`).

## 4. Questions answered

1. Why does the lane tick every 120 s? It does not. It ticks every 600 s plus the last keeper step; "slept" leaves the keeper's wait steps out (section 1).
2. Can the line name a subprocess count per duty? No (D5). Proposed: reaped children's CPU per phase. Rejected: a counting wrapper around `exec.Command` in every package.
3. Does each duty load the ledger itself? Many do: section 1's inventory. Health and situation share within themselves, not with each other. The decision, rearm, recovery and revival read fresh.
4. Does U1 share one ledger read? No. Proposed: count and time first (D6), share in U2 only if the reads lead, keeping each owner's freshness (D7). Rejected: share now. Three reader shapes (fetch-first `ReadClaimableBudgetedWork`, offline `Project`, `ProjectAt` at a named tip) across ten owners, bought without a number.
5. Are 36 claim-quota notes sent per tick? No, once per pair (section 1). The role's per-tick cost is 36 lane reads and job scans; D7 lists it as a candidate.
6. Is census a phase? No: an included subtotal of the tick and of seat recovery (D1, D2). The fixture at `internal/steward/runner_idle_test.go:124-136` decides it: a 200 ms census inside a tick that runs 400 ms more is 600 ms of work, not 800.

## 5. Units

Production lines are additions plus deletions; a replaced line counts twice. Inventory from the call sites above, after the folds. Estimates run double in practice; the doubled figure stands beside each.

| Unit | Decisions | Production (estimate) per owner file | Tests (estimate) | Split seam | Step value |
|---|---|---|---|---|---|
| U1 the tick line times every phase | D1 to D6, D8 | about 143, doubled 286: `metasystem/internal/steward/tick_phases.go` 48 (new: recorder, exclusive marks, census and inner subtotals, wait steps and remainder, CPU deltas, the line); `metasystem/internal/steward/tick_cpu_unix.go` 14 (new: SELF and CHILDREN reader) and `metasystem/internal/steward/tick_cpu_other.go` 5 (new, zero reader); `metasystem/internal/steward/runner.go` 33 (13 marks; the wait's keeper steps and remainder 6; `logTick` replaced, 6 out and 6 in; the CPU reader dependency 2); `metasystem/internal/steward/tick.go` 17 (the `TickResult` field 2; 11 marks; the clock helper 4); `metasystem/cmd/metasystem/landing_agent.go` 10 (two timings); `metasystem/internal/landing/lane/agent.go` 2; `metasystem/internal/goal/ledgerreads.go` 8 (new: counter, clock, `LedgerReads`); `metasystem/internal/goal/validate.go` 6 (timing at `:99`, `:254`, `:691`) | about 170: steward 120, cmd 25, goal 25; the SELF-only helper in `cpu_time_unix_test.go` and its sibling replaced | declared, not taken now: U1a the runner's phases, the wait, the keeper halves and the CPU (`tick_phases.go`, `tick_cpu_*.go`, `runner.go`, `landing_agent.go`, `agent.go`; about 112); U1b the tick's inner phases and the ledger reads (`tick.go`, `ledgerreads.go`, `validate.go`; about 31) | the next tick line on every steward says which phase costs what, in wall time and CPU, and how many full ledger reads it made at what cost |
| U2 the fix U1 names | D7 | PLACEHOLDER: at most 125, doubled 250, owner files named when the steward fills D7 | PLACEHOLDER | none planned; over 125, a follow-up goal | the lane steward's tick costs seconds and the steward stops burning a core |

The doubled U1 figure is above 250. The seam is declared so a build that crosses 250 stops at it; two units stay the plan, and no split is made now (disposition, minor 5). U1 lands before U2 is written. Each brief gets BRIEF-CHECKLIST (items 17 and 18), `Read each round: yes`, and one Astra round. Reverse dependents run whole for U1: `metasystem/internal/steward`, `metasystem/internal/goal`, `metasystem/internal/landing/lane`, `metasystem/cmd/metasystem` by name.

## 6. Tests

Through the production loop `runLoopWithDependencies` with `deps.Now` and `deps.Sleep` injected, the production `RunTick` with `tickHealthNow` injected, and the production goal readers with the ledger-read clock injected (R-104-m1e). No test reads the wall clock or the real CPU.

U1:

- T1 `TestTheTickLineTimesEachPhase` (new, `metasystem/internal/steward/runner_idle_test.go`, beside `TestRunnerSleepsOnlyTheRemainder` at `:111`). The fixture tick takes a 200 ms census and 400 ms more, as the existing fixture does (`:124-136`); each fixture duty advances the clock by a distinct amount (7, 11, 13, ... ms), seat recovery with a 3 ms census inside. The line is asserted byte for byte: `tick 0.600 s [... census 0.200]`, `seat-recovery-requests ... [census 0.003]`, `work` the sum of the top-level phases only. Kills: census added as a phase of its own (work 800 ms); a phase measured from the tick's start instead of the previous mark; a duty left out; recovery's census shown inside the tick.
- T2 `TestRunnerSleepsOnlyTheRemainder` (`runner_idle_test.go:111`, want at `:161`) and `TestStewardRunPublicVerbSleepsAndLogsATick` (`metasystem/cmd/metasystem/steward_runner_idle_test.go:15`, want at `:67`) take the new line; in the first, normal mode reads `work 0.600 s (tick 0.600 s [health 0.100; census 0.200], ...)`, not 800 ms. Their modes (normal, helm, failed, overrun) and sleep assertions stay. Kills: the helm or the overrun path writing the old line; census added twice.
- T3 `TestTheWaitTimesTheKeeperSteps` (new, `metasystem/internal/steward/runner_lane_test.go`, beside `TestRunnerStepsAWaitingLaneBetweenCycles` at `:153`). Interval 600 s; the keeper fixture answers `AgentHeld` and advances the clock 50 s per step. The line says `slept` equal to the sleeps alone, `lane-keeper N steps` with N and the total, `wait other` 0, and the four parts sum to the cycle on the clock. Kills: the wait's steps left untimed (today's line); steps counted into `slept`.
- T4 `TestTheKeeperStepTimesItsHalves` (new, `metasystem/cmd/metasystem`, beside the `landingStewardTick` tests) with an injected clock: keeper 3 s, steward pass 5 s; `AgentRun` carries both; `AgentNotKept` carries the keeper's time and no steward time. Kills: the steward half timed when the lane is not kept; the halves swapped.
- T5 `TestTheTickNamesItsInnerPhases` (new, `metasystem/internal/steward/tick_test.go`), through `RunTick` on the duty bed (`duties_test.go:80-100`) with a `tickHealthNow` that advances 1 ms per call: every inner phase is present, in order, above zero. The double count is checked where the line is made: `TestTheLinePrintsHealthOnce` (new, `tick_phases_test.go`) hands the formatter a `TickResult` with inner health 30 ms and `healthElapsed` 30 ms and asserts the tick's bracket says `health 0.030` once and the bracket's sum is the tick's time. Kills: a mark missing or reordered; health added from both the inner mark and `healthElapsed`.
- T6 `TestPhaseCPUIsTheReadersDelta` (new, `tick_phases_test.go`): an injected reader returns scripted (self, children) pairs; the line prints the deltas and the cycle's totals; a phase under one second prints no `cpu`. Kills: absolute counters printed instead of deltas; self and children swapped; children read with `RUSAGE_SELF`.
- T7 `TestLedgerReadsCountEveryFullRead` (new, `metasystem/internal/goal/ledgerreads_test.go`, on the two-clone bed of `metasystem/internal/goal/fetchadvance_test.go:46`), the read clock advancing 1 ms per call. One offline `Project`, one `ProjectAt`, one `ValidateCommit` on the accepted tip, and one fresh fetch-first projection after the other clone published (the fetch validates the new commit, then projects it): `LedgerReads()` rises by exactly five, its time by the clock's steps. Kills: the counter in `loadTreeFor` instead of `ParseTreeFiles` (three of five: both validations missed); `ValidateTree` left untimed; a read counted twice.

U2: named when D7 is filled; at least one test through the tick that bounds the leading phase on the injected clock, and its mutation.

## 7. Rollout

1. U1 lands through the lane.
2. The lane steward runs the lane's own engine (`identity.json` `engineBuild`); it takes U1 at the lane's next engine refresh. m1e takes it at its next rearm.
3. The steward reads six tick lines from each `runner.log` (about an hour), fills D7 and the U2 row, and asks one Astra round on that amendment.
4. U2 lands. The steward checks the lane steward's CPU over one hour with ps and records it under finding 470.

## 8. Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| saying a cycle's cost | four words, the rest as "other" (`runner.go:322-326`) | exclusive phases with included subtotals, the wait's keeper steps and remainder timed (U1, D1 to D3; T1 to T3, T5) | `metasystem/internal/steward/runner.go:322-334`, `:521-574` (the new owner, `tick_phases.go`, does not exist yet) |
| the process CPU reader | a SELF-only test helper (`cpu_time_unix_test.go:11-18`) | production, SELF and CHILDREN, behind an injected reader (U1, D5; T6) | `metasystem/internal/steward/cpu_time_unix_test.go:11-18` (the new owner, `tick_cpu_unix.go`, does not exist yet) |
| knowing the ledger's read cost | a historical comment (`health_ledger.go:11-12`) | a counted and timed figure per cycle (U1, D6; T7) | `metasystem/internal/steward/health_ledger.go:11-12`, `metasystem/internal/goal/validate.go:99`, `:254`, `:691` (the new owner, `ledgerreads.go`, does not exist yet) |

U2's moved effects are named when D7 is filled.

## 9. Rejected alternatives

- A profile (pprof or `sample`) instead of the line. Rejected: it answers once, by hand; the line answers every cycle on every steward. A one-off profile stays open to the steward beside U1.
- A longer lane cadence. Rejected: the cadence is already 600 s; the cost is the wait's keeper steps and the duties, not the cadence.
- Sharing one ledger read in U1. Rejected for U1 (section 4 Q4).
- Counting forks with an `exec.Command` wrapper. Rejected (D5).
- Counting ledger reads at `loadTreeFor`. Rejected: it misses `ValidateCommit` and the fetch's validation (finding 1).
- Census as a phase beside the tick. Rejected: it runs inside the tick and inside seat recovery (finding 2).
- Changing the claim-quota ask here. Rejected: once per pair is already the code; its owner is the-steward-acts.
- Writing the cycle's phases into `duties.json`, `health.json` or the steward-tick component record. Rejected: the component record is completed before the duties run (`tick.go:637`), so it would need a second writer; no reader wants the figures; the log line is the measurement, and Wido reads it.

## Critique record

- Round 1, 2026-11-01 about 16:00: Astra, direct read, findings `agentic-tools-evidence/lane-test-20261009/designs/tick-cost-critique-1.findings.md`; 2 material, 5 minor; verdict accept after the folds. Dispositions `agentic-tools-evidence/lane-test-20261009/designs/tick-cost-page-r1-dispositions.md` (decision 127, every finding accepted). Folded in revision 2: finding 1 as D6, T7, section 1's read inventory and section 8's third row; finding 2 as D1, D2, T1, T2 and Q6; minors as D3 (wait setup, polling, probing), T5 (the double count at the line), D8 (where U1 publishes; no health.json field), D5 (reaped children, the count declined) and section 5 (re-estimate, seam declared, no split); the diagnosis note as section 1's likeliest sink. Closed; no round 2.
