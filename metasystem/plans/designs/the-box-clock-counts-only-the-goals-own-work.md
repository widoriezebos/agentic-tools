# The box clock counts only the goal's own work

- Kind: design
- Id: the-box-clock-counts-only-the-goals-own-work
- Status: accepted
- Goals: the-box-clock-counts-only-the-goals-own-work
- Critique: round 1 (Astra, designs/astra-boxclock-r1.log; 9 material; build after the folds) folded in revision 2; the loop stopped there (decision 232)

Accepted 2026-11-08 by a1 in Wido's name (decision 232: most channel questions of 11-07/08 were box stops on waiting time).

Revision 2, written 2026-11-08 by opus-designer for a1. It folds Astra round 1 (9 material). Revision 1 stays at `the-box-clock-counts-only-the-goals-own-work.r1.md`. Code read at local main 370d69ee9e. Paths are relative to `metasystem/`. Rulings relied on: R-104-m1e (artificial clocks), R-35-m3 and R-107-m1e (no reservations between seats; the test slot cap exists by Wido's word), R-22-m1 (attempts spend mechanically). Program decisions 7 and 8 of 2026-10-19 (a breach waits for the unit boundary; the current unit gets its remaining jobs past the limit, with the overrun recorded).

## Problem

The elapsed box stops goals that are not working. Records:

- the-carry-check-classifies-a-flake (m1f). Its 4h box ran out at 4h31m waiting for a test slot (journal 11-06 18:04, `first-machinery-run-20261023/journal.md:1170`). Widened to 2d, it breach-stopped at 24h13m with 0 attempts, 18 h of it a collection wait (decision 213). Widened to 4d. On 11-08 it was parked: "box ran out three times" (decision 230).
- the-lane-batches-goals-and-proves-once (ui). Admission closed at 8h40m of its 1d box; raised to 4d/16 (decision 231). That most of those hours were a wait for the host's one test slot is the steward's reading. I did not verify it.
- the-lane-collects-its-resolution (m1e). Attempts 10 of 10, raised to 20, box 3d/20 (journal 11-08 04:45 and 05:53). That the attempts went mostly to audit and flake corrections is the steward's account; I did not verify it.
- Box questions were 14 of the 18 channel questions on 11-07/08 (decision 232). The week review counts 17 scripts named `goal budget` in one week (`designs/week-review-and-future-state-2026-11-07.md:214`).

What the code does today:

- Elapsed is wall time from the budget start, less idle gaps between own-pair claims (`internal/dispatch/budget.go:374-381`).
- Four kinds of wait then come off it as one union (`budget.go:985-986`): the lane stay (`:903-940`, `internal/goal/lane_budget.go`), provider waits (`:953`, `internal/dispatch/budget_provider.go:46-119`), person waits (`:958-968`, `internal/dispatch/budget_person_wait.go:22-104`), and the land-ready wait (`:972-984`). These are computed only for the authority lens, not for a working projection (`:902`).
- Nothing comes off for hours in which the goal has no job and no session. A night with nothing running counts in full. A collection wait counts unless a machine question of kind `uncollected-round` names the goal (`budget_person_wait.go:54`).
- The test slot wait happens inside a running launch. The unit check is a proof launch (`internal/launch/unit_run.go:662`). It takes the host slot from inside, in `metasystem work check` (`cmd/metasystem/intent_unit_check.go:158-169`). The wait is only a stderr line there (`:165-167`). While it waits, the acquisition holds a wait marker `wait-<priority>-<nonce>` in the host admission directory (`internal/proofrun/host_resources.go:722-750`, created at `:1122-1123`), and removes it when the acquisition returns, by success or by giving up (the deferred release at `:939-945`). The marker carries the owner's process identity and no goal (read at `host_admission_read.go:52-106`). `lease.Waited()` feeds test cost only (`cmd/metasystem/test.go:776`, `:980`). So no record says when a goal waited for a slot.
- A `metasystem test` run takes its slots before its suite process starts: the metadata lease at `test.go:770`, the attempt admission at `:842`, the native lease at `:974`, the suite launch at `:996`. A proof attempt's clock interval starts at its suite process (`internal/proofrun/record.go:194-218`). So a proof attempt never covers its own slot wait.
- A proof attempt that was reserved but not launched is not work: only a launched attempt becomes a clock dependency (`budget.go:670-684`).
- A unit reservation's job record is not a clock dependency; its launch record owns the runtime (`budget.go:435-438`). The reservation's `jobId` is the launch id (`internal/dispatch/unit_launch.go:78`).
- Governed obligations: a terminal attempt becomes a clock dependency from its durable state (`budget.go:783-788`). A live governed run is charged attempts, minutes and an active job, but adds no clock interval (`budget.go:800-870`; `excludeRunID` skips one run at `:810`).
- Environment stops already spend nothing else. An environment-caused unit launch spends no attempt and no minutes (`unit_launch.go:125-131`, `budget.go:554-560`); its retry spends an attempt but no minutes (`:634-636`). A red the engine classifies as a flake turns the step green (`internal/launch/unit_attribution.go:135-136`). Elapsed still counts an environment launch's hours.
- A seat session is a launch with no goal (`cmd/metasystem/steward_seat.go:149-150`). Its goal is on the seat record (`internal/steward/seat_start.go:93-112`), under `artifacts/agents/steward/seats` (`seat_start.go:118`, `internal/steward/runner.go:42-44`). This checkout holds no seat records today; where live seats write theirs I did not verify (hypothesis).
- An unreadable launch store makes the budget unknown today (`budget_provider.go:52-54`, returned at `budget.go:953-956`). So does every other required record: jobs, proof attempts, governed runs.
- The admission refusal (`internal/dispatch/admission.go:503-517`, reached from `EvaluateGoalAdmission` at `:93-165`), the breach state (`budget.go:1146-1156`) and the live stop reason (`internal/dispatch/stop.go:118-125`) all read `projection.Elapsed`.
- **Revision 1 was wrong on decisions 7 and 8.** Neither is built. A unit launch refuses every elapsed breach with no current-unit exception: `ReserveUnitLaunch` adds `budgetAdmissionBreaches` and `projection.Breaches` and refuses (`unit_launch.go:66-73`), and every build, check and read goes through it (`cmd/metasystem/intent_work.go:775`). So a build that crosses the box strands its own check and read. The breach stop does not wait for the unit either: `findBreachStopsWithReads` routes on `liveStopReason` with no eligibility test (`stop.go:405-469`). Both are owned by a-stop-waits-for-the-unit-boundary-2 (approved, not built; design `plans/designs/a-stop-waits-for-the-unit-boundary.md`, D9 and D10, units U1a.2 and U1b).
- The test environment clears inherited engine controls by name and prefix (`internal/testenv/testenv.go:53-80`, unset at `:556-563`). A new launch variable is not on that list.

## Decisions

- **D1 Elapsed is the goal's own work.** Elapsed is the union of the goal's own work intervals inside [budget start, now], less the existing pauses. Computed as `outage.Paused(work + pauses) - outage.Paused(pauses)`, the work time not under a pause (`internal/outage/waits.go:48-62`). The idle subtraction at `budget.go:374-381` is dropped on this path, since idle gaps hold no work. New file `internal/dispatch/budget_own_work.go`, wired where elapsed is final (`budget.go:985-986`), authority lens only. Rejected: a fifth pause "no work". It is the same set as a complement, and harder to read.
- **D2 What counts as own work.** Each of these, from its start to its end, or to now while it runs:
  - a launch of the goal (build, check, read, design), from the launch store `providerWaits` already lists (`budget_provider.go:51-72`), less its own slot waits (D5);
  - an executed job record of the goal that is not a unit reservation (`budget.go:433-453`; a pending reservation is not work);
  - a launched proof attempt and a terminal governed obligation attempt (`budget.go:684`, `:783-788`);
  - a live governed run of the goal, from its record's `startedAt` to now, skipping `excludeRunID` as the charge loop does (`budget.go:800-870`). A stuck run therefore spends the box while it runs;
  - a seat session whose seat record names the goal (D4).
- **D3 Concurrent work counts once.** Two own jobs at once count as one interval. Rejected: sum them. The box would then run faster for parallel work, and `activeJobLimit` and `reservedJobMinutesLimit` already bound what parallel work costs.
- **D4 A seat session is own work, as occupancy.** Start: `startedAt` on the seat record. End: the seat launch's `FinishedAt` when terminal, else `reapedAt`, else now. A session with no job still spends the box, so a session that loops is still stopped. Occupancy is weaker than executed work, in three rules:
  - it is cut by every confirmed slot wait of the goal (D5), whether recorded under the goal's launches or under the seat launch itself;
  - it is cut by the interval of every environment-excluded launch of the goal (D6);
  - it never cuts a person wait. `cutPersonWork` (`budget_person_wait.go:106-126`) and `providerWaits` take executed work only, so a seat waiting on a person's answer spends nothing.
  Independent work is never cut by these rules: a build of the goal that runs during a cut still counts on its own. The cost: a seat that thinks about something else while its check waits gets that time free. The seat blocks on `metasystem work check`, so this is bounded by the wait (hypothesis that it is rare). Rejected: count only engine jobs. A seat that idles for hours would never meet its box. A person's own pane session has no seat record and does not count; its spend is the person's call.
- **D5 A test slot wait is recorded, and comes off only its own execution.**
  - Writer. When a heavy acquisition first creates its wait marker (`host_resources.go:1122-1123`), it appends `{"wait":"start","marker":"wait-<priority>-<nonce>","pid","at"}` to `waits.jsonl` in its destination directory. The deferred release at `:939-945`, which runs on success and on every give-up, appends `{"wait":"end","marker","at"}` before it removes the marker. One hook point each.
  - Destination. The launch manager names its own state directory to its child in a new variable `METASYSTEM_LAUNCH_STATE`, beside `METASYSTEM_LAUNCH_KIND` (`internal/launch/launch.go:405`). A child launch's own value replaces an inherited one, as for the kind (`launch.go:26-30`). Only the public entry `AcquireHostResourcesWithWaitCheck` reads the variable; it passes the destination down to `acquireHostResourcesWith` as a parameter. Fixtures that call the inner function pass a `t.TempDir()` or nothing. Outside a launch nothing is written.
  - Test boundary. `METASYSTEM_LAUNCH_STATE` joins `inheritedControlNames` (`testenv.go:53`), so the test process unsets it at `:556-563` before any fixture runs. A builder's test can no longer append to the builder's real launch record.
  - Completed waits. A wait is credited only as a pair: a start and an end with the same marker, end not before start, clipped to its execution.
  - Open waits. A start with no end is credited from its start to now only while it is demonstrably waiting: the execution is running and the marker it names is still held in the admission directory (the held probe the reader at `host_admission_read.go:89-96` already uses, exported for dispatch). Otherwise it is not credited: the time counts and one `problem:` line names the record. A lost end line can therefore never make working time free.
  - Own waits only. Each execution's work is its interval less its own credited waits. That net interval is what enters the union and what `cutPersonWork` sees. So two checks waiting for the same slot do not make each other look busy, and a build that really runs during a check's wait still counts.
  - Seat join. Waits are read for each launch of the goal and for the seat launch named by each seat record of the goal (D4). A test that a seat runs directly writes under the seat launch, and is found there.
  - Rejected: record the wait in the unit check only (`intent_unit_check.go:165`). A builder's own `metasystem test` waits in the same owner. Rejected: read only the live markers. They carry no goal and are gone after the wait.
- **D6 An environment launch is not own work.** A unit launch whose reservation has `unitEnvironment: true` (`unit_launch.go:131`) is left out of D2, joined by id (the reservation's `jobId` is the launch id). Its interval also cuts seat occupancy (D4). A flake needs nothing new: its step passes and no round is spent. An `unclassified` red counts in full. Rejected: refund correction rounds by judgement. R-22-m1 keeps spending mechanical.
- **D7 Attempts do not change.** Environment launches are already free; flakes already spend no round. The lane-collects case is a classification or audit question, not a box rule.
- **D8 Required records stay required; one fallback for the new optional readers.**
  - Required readers keep today's unknown budget and its refusals: job records, the launch store, proof attempts, governed runs and obligation state (`budget.go:392-398`, `:953-956`, `:660-663`, `:774-777`, `:800-808`). Nothing new falls back for them.
  - New optional readers: seat records, and a job dependency with no readable start (which `providerWaits` passes through when no provider work exists, `budget_provider.go:76-81`). If one of these fails, elapsed is today's formula, which is complete because every required reader succeeded. One `problem:` line names the record (`projection.Problems`, printed at `cmd/metasystem/intent_goals.go:301-303`). A new field `ElapsedLegacy` says the fallback was used.
  - `waits.jsonl` never triggers the fallback. An unreadable or unpaired line credits nothing; its time counts with one problem line (D5).
  - A launch record that still says running with no live process counts to now. Every failure counts time; none hides it.
- **D9 Say it.** While a credited open wait exists, goal status and goal show print "waiting for a test slot since HH:MM, clock paused (Nh so far)", in the shape of `PersonWords` (`budget_person_wait.go:14-19`) and at its two sites (`cmd/metasystem/intent_goals.go:308`, `cmd/metasystem/goal_show_view.go:195`).
- **D10 The current unit gets its remaining jobs past the box (decision 8), at the unit launch.** `ReserveUnitLaunch` waives the elapsed breach, the closed state and the breach state, for a request whose unit run already holds an executed reservation of the goal in the job records. A run's first reservation needed an open box, so the run started before the box closed. The runner already bounds what follows: its read, and corrections only while `allowCorrection`, `MaxRounds` and the counted cap allow (`internal/launch/unit_run.go:328-340`, `unit_stop.go:283`, `unit_revise.go:178-185`). A new run has no reservation and is refused. The waiver never covers attempt or minute limits, or a stop fence (`unit_launch.go:71`). The admitted reservation records `elapsedOverrunMin`, the own-work minutes past the box at admission. Goal show and goal status print "past its box by Nm for run R" from the largest such value. Rejected: write the overrun into the goal file from admission. That is a ledger write inside a launch lock. This covers decision 8 at the unit launch only. Revision and goal admission and delegation evidence stay with a-stop-waits-for-the-unit-boundary-2 U1b, which loses this one site.
- **D11 The breach stop is unchanged.** It comes at box plus grace of own work (`budget.go:1148`, grace 50) and still cancels the goal's jobs mid-unit. Waiting for the unit boundary (decision 7) is a-stop-waits-for-the-unit-boundary-2 U1a.2. Admission still closes at the box (`admission.go:505`) for anything D10 does not waive.
- **D12 Box defaults do not change now.** Tier 2 stays 4h/6/720m/1/20 and tier 3 8h/10/1200m/1/20 (`metasystem.conf:29-30`). Decision 202 asked for box defaults with the clock. Rejected: lower them now. Own work hours have never been measured; set the defaults after a week of the new numbers.

## Units

Inventory from the owners above, counted production lines. Estimates run double (unit estimates memory of 10-25), so each row shows both. Ceiling 250 per unit by the plain double.

| Unit | Content | Owners | Counted | Doubled |
|---|---|---|---|---|
| U1 | D10 | `unit_launch.go:43-73` waiver predicate and elapsed-breach filter ~10, `elapsedOverrunMin` on the reservation ~4; `budget.go` set of runs with an executed reservation in the job loop ~4, largest overrun read ~4; one line at each of the two display sites ~4 | ~26 | ~52 |
| U2 | D1, D2, D3, D4, D6, D8 | new `budget_own_work.go` union, clip, pause subtraction ~35; seat reader (reads the seat JSON directly; dispatch must not import steward) and join to launch records ~30; environment set and seat cut ~8; `budget_provider.go` launch ids returned ~6; live governed-run intervals at `budget.go:800-870` ~6; wiring at `:374-381` and `:985-986` ~12; `ElapsedLegacy` ~3 | ~100 | ~200 |
| U3 | D5, D9 | new `internal/proofrun/host_wait_record.go` writer ~30; hooks at `host_resources.go:1123` and `:940-945` ~6; destination read at the public entry and passed down ~6; held probe exported ~8; `launch.go:405` variable ~3; `testenv.go:53` one name ~1; `budget_own_work.go` pair reader, open-wait check, own-wait subtraction, seat-launch read, seat cut ~45; net intervals into `cutPersonWork` ~3; Words and two cmd sites ~10 | ~112 | ~224 |

Each unit is usable alone, in order U1, U2, U3.

- U1 lets a unit that crosses the box finish its check, read and allowed corrections. It is useful today, with the wall clock.
- U2 stops nights and collection waits from spending, and keeps idle seats spending. Revision 1 split seats into their own unit; without them a seat working without jobs spent nothing (round 1 finding 6). They are one unit now. No seam: a split brings that hole back, and 100 counted is under the 125 line.
- U3 takes slot waits off. Seam if the brief's inventory passes 125 counted: the seat-launch wait read (~15) moves to a U4. Until U4, a seat's direct test waits count as work. That is safe: time counts, nothing is hidden.

Test churn is a risk of U2. 59 assertions on `.Elapsed` sit in 14 test files (largest `internal/dispatch/budget_test.go` 22, `budget_person_wait_channel_test.go` 10, `stop_test.go` 9, `budget_person_wait_test.go` 8). Beds that let wall time pass with no work and expect elapsed now see zero. Revision 1's one-line helper "the goal works throughout" is withdrawn: an executed launch cuts every person wait, so `TestPersonWaitReplyIsNotTheAct` (`budget_person_wait_channel_test.go:43-50`) would read six hours of work instead of two. Fixtures migrate by the behaviour they prove:

- a bed that proves a pause (person, lane, provider, land-ready) gets seat occupancy over its window, which spends but never cuts a pause;
- a bed that proves executed work overrides a wait gets an executed launch or job at the times it names;
- a bed that proves a breach or a closed box gets executed work worth the box.

The builder counts the beds before the build and lists each with its class in the brief.

## Tests

Through `ProjectBudget`, `ReserveUnitLaunch`, `EvaluateGoalAdmission` and the bounded admission bed (`internal/dispatch/stop_test.go:335-356`), with real job, launch, seat, proof and governed-run records and an injected `now`. No test reads the wall clock (R-104-m1e). Every bed asserts `ElapsedLegacy` is false unless the test is about the fallback.

U1:

- T1 Current unit past the box. A goal at its box; run R has an executed build reservation. R's check: admitted, `elapsedOverrunMin` recorded, goal show prints the overrun. R's correction round 2: admitted. A new run's build: refused `BUDGET_REFUSED` on `elapsedLimit`. R's check with the attempt limit reached: refused. R's check with a stop fence: refused. `FindBreachStops` at box plus grace still returns the elapsed route.

U2:

- T2 Night. Claim 18:00, one build 18:00-19:00, now 08:00 next day. Elapsed 1h; admission open.
- T3 Concurrent. Two builds 10:00-12:00 and 11:00-13:00. Elapsed 3h.
- T4 Pending. A job reserved 10:00, never started. Elapsed 0.
- T5 Environment. A unit launch 10:00-10:32 with `unitEnvironment`. Elapsed 0; the retry 11:00-11:30 counts 30m.
- T6 Pause outside work. A person question 09:00-17:00, one build 10:00-11:00 inside it. Elapsed 1h, never below 0.
- T7 Fallback. A local job with no start. Elapsed equals today's formula; one problem line; `ElapsedLegacy` true.
- T8 Required record. Unreadable launch store, an open person question and a running build. Budget unknown; `ReserveUnitLaunch` refuses `BUDGET_UNKNOWN`; no fallback.
- T9 Breach still stops. Builds totalling box plus grace. `liveStopReason` says elapsed limit; admission closed at the box for a new run.
- T10 Session. Seat record for the goal 09:00, launch finished 12:00, no jobs. Elapsed 3h. With no finish and `reapedAt` 11:00: 2h. Running: to now.
- T11 Other goal's session. Not counted.
- T12 Seat over an environment launch. Seat 10:00-11:00, an environment launch of the goal 10:00-10:32. Elapsed 28m. Add a build 10:10-10:20: still 28m, the build lies inside the seat.
- T13 Seat under a person wait. Seat 09:00-15:00, a reply-is-not-the-act question 11:00-15:00. Elapsed 2h.
- T14 Live governed run. A running governed run record 09:00, nothing else, box 4h, now 13:00. Elapsed 4h; `EvaluateGoalAdmission` refuses. With that run as `excludeRunID`: 0.
- T15 Fixture migration. `TestPersonWaitReplyIsNotTheAct` and the other pause beds keep their numbers on seat occupancy.

U3:

- T16 Wait written. `acquireHostResourcesWith` with a held slot in a test admission directory, a scripted clock option and a temp destination: one start line naming the marker, then one end line after release, and one end line after a give-up by context. No destination: no file.
- T17 Inherited variable. In the shape of `TestMainClearsInheritedLandingEnvironment` (`internal/testenv/landing_environment_test.go:11`): the parent sets `METASYSTEM_LAUNCH_STATE` to a directory; after the test environment starts it is unset, and a fixture acquisition with a held slot leaves that directory empty.
- T18 Wait read. A check launch 10:00-17:00 with a paired wait 10:05-16:05. Elapsed 1h.
- T19 Open wait, held. A start with no end, the marker held by a live fixture process, the check running, now 12:00. Credited to now; the status line shows.
- T20 Open wait, end lost. The writer fails on the end line; the acquisition succeeds, the marker is gone, the check keeps running. Not credited; the time counts; one problem line.
- T21 Work inside a wait. A build 11:00-12:00 during T18's wait. Elapsed 2h.
- T22 Two waiters. Two checks 10:00-17:00, each with its own wait 10:05-16:05. Elapsed 1h, not 7h.
- T23 Seat-backed wait. A seat for the goal 09:00-17:00, a paired wait 10:00-16:00 under the seat launch's state directory. Elapsed 2h. Add a build of the goal 11:00-12:00: 3h.
- T24 Broken line. An unreadable line credits nothing; one problem line.

Mutations, each failing the named test:

- (a) no waiver at the unit launch: T1. (b) waiver for a new run: T1. (c) waiver over the attempt limit or a stop fence: T1. (d) no overrun recorded: T1.
- (e) elapsed from wall time again: T2. (f) sum concurrent work: T3. (g) count non-executed jobs: T4. (h) drop the environment exclusion: T5.
- (i) subtract pauses from work without the union, `|W| - |P|`: T6.
- (j) zero elapsed on an undated source: T7. (k) fall back on a required record: T8.
- (l) the new clock bypasses the breach comparison: T9.
- (m) ignore seat sessions: T10. (n) ignore `reapedAt`: T10. (o) count any seat: T11.
- (p) seat not cut by environment launches: T12. (q) seat occupancy cuts person waits: T13 and T15.
- (r) no live governed-run interval: T14. (s) ignore `excludeRunID`: T14.
- (t) no start line: T16. (u) no end line on give-up: T16.
- (v) variable not cleared at the test boundary: T17.
- (w) wait not subtracted: T18. (x) open wait credited without the held marker: T20. (y) open wait not credited while held: T19.
- (z) cut by other executions' raw intervals: T22. (aa) wait cut not applied per execution, build inside the wait lost: T21.
- (ab) seat launch waits not read: T23. (ac) a broken line credits: T24.

## Acceptance

A person can run each of these on m1e.

1. A unit whose build crosses the box gets its check and read. `metasystem goal show G` prints "past its box by Nm for run R". A new unit of G is refused.
2. A goal waits 6 h for a test slot and spends no elapsed box for it. Start a goal's unit check while the lane's full proof holds the slot. During the wait, `metasystem goal show G` prints "waiting for a test slot since HH:MM, clock paused". After the check, elapsed has grown by the check's run time, not by the wait.
3. A goal with no session and no job overnight. `metasystem goal show G` at 18:00 and at 08:00 shows the same elapsed.
4. A goal that keeps building still stops: its breach stop comes at box plus grace of own work.

## Not changed

- Attempts, job minutes, active jobs, review rounds and their environment rules.
- The four existing pauses and their records: lane stay, provider wait, person wait, land-ready wait.
- The breach stop and its timing (D11). Decision 7 stays with a-stop-waits-for-the-unit-boundary-2.
- Revision admission and goal admission (`admission.go:93-165`, `:319-336`); D10 touches the unit launch only.
- Tier box defaults (D12).
- The test slot cap and its markers (R-107-m1e). Waits come off the clock; no slot is reserved for anyone (R-35-m3).
- Proof admission and the lane.
- No new verb and no settings key.

## Critique record

- 2026-11-08 15:12, round 1 (Astra, `designs/astra-boxclock-r1.log`; 9 material, 0 minor; verdict build after the folds). Folded in revision 2: 1 as D10, U1 and T1, with the false premise corrected in Problem; 2 as D5 (paired records, open waits only while the marker is held) and T19-T20; 3 as D2 and T14; 4 as D5 (test boundary, destination passed down) and T16-T17; 5 as D8 and T7-T8; 6 as U2 (seats merged into the clock unit); 7 as D4, D5 (seat join for waits) and T12, T23; 8 as D5 (own waits only) and T22; 9 as the fixture classes under Units, T13, T15 and the `ElapsedLegacy` assertion. The steward stops the loop here under Wido's rule (stop when findings no longer change what gets built materially) unless a fold is impossible.
