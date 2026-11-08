# The lane batches goals and proves once

- Kind: design
- Id: the-lane-batches-goals-and-proves-once
- Status: accepted
- Goals: the-lane-batches-goals-and-proves-once
- Critique: round 1 (Astra, designs/astra-batches-r1.log; 8 material; build after the folds) folded in revision 2; round 2 (designs/astra-batches-r2.log; 7 material; build after the folds) folded in revision 3

Accepted 2026-11-07 by a1 in Wido's name (decisions 222, 224, 226): the strategy is his, in his words: batch up multiple features that test only what they touch each, then one full suite when landing. The gathering defaults (2 goals or 3 hours) are proposed; U4b activates them only on his adoption (R-36-m3).

Revision 3, 2026-11-07, drafted on Opus 5.5 for the steward. It folds Astra's round 2 (designs/astra-batches-r2.findings.md, 7 material, 0 minor, verdict build after the folds) as a1 decided the folds. Revision 2 stays at designs/the-lane-batches-goals-and-proves-once.r2.md, revision 1 at designs/the-lane-batches-goals-and-proves-once.r1.md. Code read at main 17acb13cb2 in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem. Lane records read in /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing. Times are UTC with CET in brackets where a person reads them.

Authority: Wido's decision 222 (2026-11-07 19:4x, values class, memory wido-program-decisions-2026-10-19.md:377): "batch up multiple features that test only what they touch each. Then have a single full suite when landing. A single feature having run a full suite is a very costly way to do this." It restates his 2026-09-19 rule (memory wido-expensive-test-once-per-batch-2026-09-19.md) and R-3 (rulings.md:28): per-landing verification is fast tests; full runs serve batches. Decision 224 (memory :379) is a1's recorded reading of 222, not a further human grant. Decision 226 (memory :382) records what a1 implements under 222 and what waits for Wido's adoption.

This page amends the accepted pages plans/designs/lane-proves-at-the-batch-risk.md (R-149-m1e, rulings.md:208) and plans/designs/lane-batching.md (D2, the restart rule). Inputs: a1's analysis designs/analysis-testing-cost-2026-11-07.md and Astra's critique of it, designs/astra-testing-cost-r1.findings.md. Rulings relied on: R-3, R-35-m3 (no machine reservations; slowness is not failure), R-36-m3 (policy is adopted only by Wido's word, after critique), R-104-m1e (artificial clocks), R-149-m1e.

## Decisions for Wido

Two kinds, kept apart (Astra r2 F6).

**A. What a1 implements under decision 222.** 222 supplies the testing strategy and its priority. These follow from it, or are implementation choices. Wido overrules any of them.

| Decision | Where | Rejected |
|---|---|---|
| An executable batch has a full green on its final tree before its push. An impact green never lands a batch. | D1, U1a | Impact-depth landings for low tiers (R-149 as written). |
| Seat and resolution checks run named selections inside cmd/metasystem too; the batch's full proof is their net. This is the explicit amendment to decision 212. | D2, U3 | The whole command package per unit at the seat. |
| Once a batch's full proof is admitted, an ordinary newcomer waits for the next batch. | D5, U2 | The half-time restart (result 971: 19 minutes cancelled). |
| The keeper's idle cadence of main is off by default. `proof.trunk-every` takes `off` or at least 24h. | D6, U2 | The 4-hour clock: 42 standalone suites in 15 days, 2.1 lane-hours a day. |
| A valid records inheritance keeps the batch's executed full coverage, with no hour limit, and a published fold of it credits main's clock. | D8, U5 | Keep the hour limits: a records refresh then reruns the full suite (landing_depth_class_test.go:232 requires two). |
| Internal ownership, helpers, unit cut and order: U1a, then U2, then the rest. | Units | U2 before U1a (Astra r2: the clock overrides then expose the impact-only landing path). |

**B. What needs Wido's adoption under R-36-m3.** These shape standing delivery. a1 proposes; only his word adopts.

| Proposal | Evidence | What happens until he answers |
|---|---|---|
| Gathering defaults `landing.gather-goals=2`, `landing.gather-wait=3h`, as a measured initial trial with a two-week report (Measures). | The replay is a sensitivity study, not a forecast: 38 batch starts against 57 for seal-on-one, but it processes 54 submissions against 58 and omits eligibility, gate and merge cost and observed rework (D3). | U4b ships with gathering off (`gather-goals=1`, `gather-wait=0`). The trial is two default lines, or two keys he sets. |
| Key names `landing.gather-goals`, `landing.gather-wait`, and keeping `proof.trunk-every` rather than `landing.full-every`. | Naming is his call (memory wido-verb-naming-is-my-call). | Built with the proposed names. |

Not proposed: immediate person cancellation of a live proof. Today a person's reopen or restart during a proof is a pending request (D5). Making it cancel would be new behaviour and a new decision.

## Problem

**1. The day's numbers.** On 2026-11-07 (CET day) the lane started 11 full suites: 9 batch proofs and 2 main proofs (results.jsonl, distinct attempts with scope full). It landed 4 goals in 3 pushes (pushes.jsonl lines 78 to 80: a-verb-says-its-own-result, a-fixture-launch-counts-only-real-competitors, then an-alert-ends-with-its-situation with a-green-rounds-read-is-collected). a1's "seven for four" counted the window 06:28 to 18:22 CET: 7 full starts (6 batch, 1 main) and 3 landed goals. a-seat-moves-its-own-note was proven green once and did not land; its second proof went red. Astra r2 reproduced these counts.

Of the 9 batch suites, 4 were red. Three of those reds were the deadlock of finding 553, which the fixture fix ended. Two batch suites repeated a green proof of the same batch:

- 10:16Z (11:16) proved a-fixture-launch-counts-only-real-competitors again after its 09:49Z green. Reason recorded: "full proof overdue since 13:10" (results.jsonl:1124). That is the cadence clock, through `overdueBatch` (depth.go:47-76, which reads `fullProofDue`, wake.go:96), the forced depth at proof_admission.go:115-117 and the reuse refusal in `batchGreenReusable` (depth.go:78-90).
- 17:22Z (18:22) proved a-seat-moves-its-own-note again after its 15:45Z green. Reason recorded: "main's clock is due; this batch needs a full check" (results.jsonl:1138). That is `ClockFull`, set by the keeper (keeper_clock.go:128-145) and read first by `BatchProofDepth` (proof_admission.go:111-113).

**2. Two weeks.** From 2026-10-24 to 2026-11-07 the lane saw 58 hand-ins of 37 distinct goals. It ran 56 batch full proofs (38 green, 18 red) and 76 main full proofs (72 green). It made 33 pushes with 36 goals. That is 132 full suites for 36 landed goals: 3.7 per landed goal. Minutes are counted from the latest record of each attempt: 3,059 batch minutes and 3,493 main minutes, 182 lane minutes of full proof per landed goal. Cancelled attempts carry no scope and are counted apart: 3 attempts, 49 minutes (result 971 among them).

Of the 76 main proofs, 42 were the keeper's clock ("fresh full check requested", keeper_clock.go:176-192). They took about 1,904 minutes: 2.1 lane-hours a day, over 32 distinct trees. Not all were on an unchanged main. 22 main proofs carried "records-only batch has no full green on main under an hour old" (1,005 minutes). All 22 are on or before October 30. Main already holds the October 30 repair d4b0127a84 (a-records-only-move-keeps-mains-green U1), which takes the hour away for main's own records check (scope.go:141, `hourBound := !running.Trunk || d.standing != nil`). Those 1,005 minutes are not a saving this page can claim, and the main path is not rebuilt here (Astra r2, OPEN 3). The lane never ran a batch proof at impact depth: results.jsonl has no non-trunk result with scope impact.

**3. Why a batch proof is full today.** Not mainly the command rule. Of the 56 batch fulls: 29 by the share rule (27 at 61 to 64% of 173 packages, 2 at 97%), 24 by the clock (6 `ClockFull`, 18 overdue), 2 by tier 3, 1 by an explicit fresh request. The depth is chosen in `batchDepth` (test_impact.go:433-512): tier floor from `landing.full-from-tier` (:444-467), then the plan's share and the command rule through `impactCost` (:402-415).

**4. The member gate.** gates.jsonl holds 84 gate records since 10-10. 75 are skipped, and 38 of those carry "change selects cmd/metasystem: full". These are not full suites. `prepareGate` (gate_impact.go:34) runs the static group, records `Static` (:40-41), computes the plan, and on a not-cheap plan records `skipped` and stops (:81-88). A skipped gate took a median 4.5 minutes, at most 9.2. So the command rule makes the member gate static-only. The rule is the last line of `impactDepth` (cmd/metasystem/test_impact.go:367): a selection that includes cmd/metasystem is never cheap, named or whole. The same verdict feeds the seat's `--check` (test_impact.go:244-262), the unit and resolution path `committedImpactCheck` (:553) and the gate (`seams.ImpactCost`, intent_landing_prove.go:222-229).

**5. One goal per batch.** Batching exists. `selectDepthClass` returns every entry with one class label (depth.go:15-45). An automatic batch stays open during merges and gates, and `extendBatchLocked` admits arrivals (batch.go:530-567). Admission ends when the full proof's attempt is recorded (prove.go:782-806). The skill runs `landing prove` as soon as the last merge gate is green or skipped (skills/landing-agent/SKILL.md:96, :102). Nothing waits for a second goal. 58 hand-ins in 15 days is about 4 a day, rarely minutes apart (queue.jsonl).

**6. What makes cheap landings possible later.** `provenGreen` admits an impact green for the push when the batch recorded impact depth (push.go:391-413). Today no batch reaches impact depth (point 2). If the command rule is relaxed first, a low-tier batch could land on named tests only. So the order is: full before push first, then a cheaper member gate.

**7. Gathering meets the existing clocks and the restart rule.**

- Health marks a goal overdue after 60 minutes of total waiting age (health.go:389, :413) and raises cannot-heal after 120 minutes from the same `Minutes`, also while proving (health.go:741-745).
- The keeper's clock gives a candidate batch one wake per state and then starts a fresh main proof (keeper_clock.go:147-192). Its gate exception accepts only `gate.Result == Green` (keeper_clock.go:167). A skipped gate with a green static group is neither green nor missing, so the clock falls through to `Start` (:192). 75 of the 84 recorded gates were skipped (Astra r2 F2).
- The keeper turns any nonempty clock line into `AgentHeld` before it reads the wake (lane/agent.go:248-253). A clock "hold" stops merges, not only main's proof.
- An ordinary newcomer that arrives before half the expected full-proof time cancels the running full proof and reopens the batch (`SelectBatch` calls `restartBatchLocked`, batch.go:375-377; rule restart.go:226-230; cancel restart.go:275). Result 971 (2026-11-03 16:32 to 16:51Z) is such a cancel: 19 minutes for an ordinary newcomer.
- A person does not cancel a live proof today. Revision 2 said `landing batch reopen --with G` and `landing restart` "still cancel and reopen". That was false (Astra r2 F5). During a live proof `ActOnBatch` records the act as pending and says nothing acts on it until the proof ends; the person runs reopen again then (batch_verbs.go:88-98). `restartBatchLocked` returns early for a person-owned proof (restart.go:180), and its common restrictions (a person-selected batch, a fix already proving, a classification) come before a fix newcomer's preemption (restart.go:217-225, :243-247). The only cancel is `cancelProofLocked` (restart.go:275, :312), reached from the restart rule.

**8. The clock is policy, and the accepted page asks for it.** lane-proves-at-the-batch-risk.md D6 keeps idle trunk proofs ("With nothing waiting, the trunk proof runs as today"). `wakeReasons` adds `full-due` from `keeperFullProofDue` (wake.go:85-91). `clockDue` (wake.go:105-112) is due on a scoped push's hourly debt or when the last full main proof is `proof.trunk-every` old (default 4h, internal/config/defaults.go:86). `KeeperClock` then sets `Trunk` and `Fresh` and calls `Start` (keeper_clock.go:176-192). Removing `ClockFull` alone does not stop that. Separately, `RecoverMovedMain` (trunk_recovery.go:82-107) runs in the keeper's observation on every tick (cmd/metasystem/landing_agent.go:347) and starts a main proof when main's commit and tree have no applicable recorded proof (`applicableMainProof`, trunk_recovery.go:110-167). It has no timer. Its applicability check requires the newest recorded environment (:121-131).

**9. The records-reuse rules still spend a second full suite.** Revision 2 left them to a-push-folds-a-records-only-move and claimed the overdue refusal would fire "at most once a day". Both were wrong (Astra r2 F1).

- Cadence off: `decideScope` widens a batch's records check to full when the batch's own full proof is over an hour old (scope.go:158-160, "the batch's full proof is ... old, more than one hour"). A records-only batch reuses main's kept full green only when it is under an hour old (scope.go:141-143, reason at :113). Main's kept greens have no hour (scope.go:482 `kept`).
- Cadence on: while main's clock is due, `BatchProofDepth` forces full (proof_admission.go:115-117), and `batchGreenReusable` refuses any green whose scope is not `full` (depth.go:85-88), even when its full proof ran a minute ago. The retained `TestLandingFullDueLedgerRefreshAdapterRunsFullAndPaysClock` (cmd/metasystem/landing_depth_class_test.go:178) requires two full executions one minute apart (:232, `b.executions(t) != 2 || b.launches != 2`).
- The clock credit: `fullProofClock` credits a push only when the pushed tree's own result is a full execution (wake.go:151-152). An inherited fold (`inheritFold`, push.go:242-254, scope `inherited`) or a scoped records check of the batch's full green earns no credit, so main stays due and the daily clock starts another suite. `proofDebt` already treats a fold of a full green as owing nothing (wake.go:196, `foldOfFull`, push.go:258-260).
- Kept checks: executable paths since the full tree (scope.go:178-195), the environment (prove.go:1386-1410, "full check pending: the proof environment changed"), and a later unresolved red (scope.go:133-136).

A long cadence interval does not bound repeats while the same debt stays unpaid. So the repair belongs on this page (U5).

## Decisions

**D1. An executable batch has a full green on its final tree before its push.**

- Rule: a batch whose tree differs from main in an executable path is pushed only on a green of scope `full` on that exact tree, or a `scoped` or `inherited` green whose full base is that batch's executed full proof (records-only inheritance, scope.go:186-193 and push.go:242-255; D8 removes its hour). An impact green never satisfies a batch push.
- Mechanism: `BatchProofDepth` (proof_admission.go:110-131) returns `full` with the reason "the batch proves full before its push" for any batch, after its records-only early return at :124, which stays and loses its `seams.BatchDepth != nil` guard. The `depth == "impact"` branch of `proofScope` (:79-85) goes. `provenGreen` (push.go:378-415) replaces its impact branch (:391-413) with one refusal: "an impact check never lands a batch; the batch needs a full check". The impact-base refusal at push.go:160-161 goes with it. intent_landing_prove.go:317-320 no longer turns a decided depth into `seams.Impact`. The batch depth reader `batchDepth` and `batchDepthSeams` (test_impact.go:417-512), the `BatchDepth` seam (prove.go:206-207) and its two wirings (intent_landing.go:1356, intent_landing_prove.go:124) are removed (U1b).
- Kept: `landing prove --impact` stays an explicit check. It records scope `impact` with D1a's provenance and its gate reuse (prove.go:1371-1376), and never lands a batch.
- Rejected: impact-depth landings for low tiers (R-149 as written). With D2 a low-tier batch would land on named tests only. Rejected: a full member gate per goal. That is the per-goal suite decision 222 rules out.

**D2. Every consumer of the cheapness verdict runs named command selections; a whole-package selection stays static-only and the batch proof covers it.** (Decision 212 amended under 222.)

- Rule: the verdict treats `cmd/metasystem=TestA,TestB` as one selected package, cheap when the share stays under `landing.impact-max-share`. It treats a whole cmd/metasystem selection (the bare package, a contract group that lists it, or a package selection of `...`) as not cheap, with the reason "change selects cmd/metasystem whole; the batch proof covers it". A not-cheap gate keeps today's path: static group, `skipped`, the batch proof follows (gate_impact.go:81-88). A not-cheap seat or resolution check stays static-only (test_impact.go:249-261).
- One rule, three consumers: the seat's `--check` (test_impact.go:244), the unit and resolution path `committedImpactCheck` (:553) and the gate's `ImpactCost` (intent_landing_prove.go:222-229) read the same `impactCost` (:402). There is no seat-only exception.
- Named selection is a heuristic early check. When the selector finds named tests for a changed command file, it does not always widen to shared command wiring (goadapter/impact.go:159 widens only when no names are found). The batch's full proof is the net for what the names miss.
- Ownership: this page owns the cheapness change at test_impact.go:367 and the reason at :411-413 (U3). a-unit-check-runs-the-importers-too owns the selector: importer edges, source audits by path, the configuration-key literal addition for finding 548 (goadapter/impact.go:273; intent_work_commit.go:204). It builds on U3 and declares that dependency. Its next step (plans/goals/a-unit-check-runs-the-importers-too.md:10) drops "a production change in cmd/metasystem runs the package whole at the seat and is exempt from the static-only fallback". The steward makes that ledger edit before dispatch; this page does not.
- Order: D1 (U1a) lands before D2 (U3), so no batch lands on named tests alone.
- Rejected: the whole command package per unit at the seat (decision 212 as written). Rejected: drop the command clause wholesale. A whole command package then counts as one of about 173 packages and passes the share: a 30 to 40 minute member suite. Rejected: a gate-only verdict. Two rules for one question drift.

**D3. A batch gathers before its full proof: one readiness decision, sticky once reached.**

- Rule: an automatic batch is ready for its full proof when any of these holds:
  1. it holds `landing.gather-goals` eligible members (waiting, not held);
  2. its oldest eligible member's hand-in time plus `landing.gather-wait` has passed;
  3. a person acted: sealed it (`metasystem landing batch seal`, act time `SealedAt`, batch_verbs.go:39-45), selected or reopened it (`Batch.Person`, its `CheckedAt`, batch_verbs.go:112-118), or ran `landing prove` from an enrolled terminal (`Running.Person`, its `CheckedAt`, proof_admission.go:207-216);
  4. a member is the fix of an open main-red incident, by the existing identity test `incidentFix` (restart.go:166-170);
  5. the batch selector is not `auto` (a numeric cap or person policy).
  Otherwise it is gathering. The deadline counts from the queue's hand-in time (`Entry.At`), so it survives keeper wakes, restarts and new merges. A numeric `landing.batch` caps the count.
- Sticky: once a batch is ready, it stays ready for the life of that batch record. A member held or returned later does not make it gather again. The time readiness began is recorded on the batch (D4). A new batch record gathers afresh.
- One function decides it: `Gathering` in a new internal/landing/plain/gather.go. It takes the policy, the batch, the queue entries, the incidents and the recorded person acts. It returns gathering or ready, the count, the deadline and the end (D4). Admission, the wake, the clock and health all call it.
- Admission: in `admitExecutionLocked` (proof_admission.go:135), before the full-check policy, for a batch proof (not gate, not trunk) whose decided scope is `full` and with no person present. A gathering batch gets the refusal `LANE_BATCH_GATHERING`: "gathering: 1 of 2 goals; gathering ends at 17:46", next command `metasystem landing batch seal`. It does not go through `holdProofLocked` (:223), which seals the batch and writes a stop for a person (:224-252). The batch stays open; no stop line, no running record.
- Merging and gating continue while gathering. Arrivals join through `extendBatchLocked`; the agent merges and gates them; only the full proof waits.
- The clock never returns a hold for gathering, because lane/agent.go:248-253 turns any clock line into `AgentHeld` before the wake is read. Selection and preparation run on every tick as today (`Prepare`, landing_agent.go:323-328).
- The acceptable-gate predicate, shared by the wake and the clock (D6): the last gate is for HEAD, the current merge, and is green, or skipped with a green static group (gate_impact.go:40-41, :81-88); and no repair or classification obligation exists: no `candidateRepairPending` (fix.go:383), no result with `ClassificationPending`, no open fix round. A red, missing, stale or unfinished gate fails it. A gate red whose failure is attributed to the parent commit is still a red for the requested merge and fails it.
- The wake: `GatherWake`, called after `DrainWake` in the keeper's `AdmitWake` (landing_agent.go:300-306), removes `queued`, `proof-finished` and `full-due` only when the batch is gathering, every pending entry is a member already merged into the lane checkout's HEAD, and the acceptable-gate predicate holds. An unmerged arrival keeps the wake. At readiness the wake returns.
- Defaults: off (`landing.gather-goals=1`, `landing.gather-wait=0`) until Wido adopts trial values (Decisions for Wido, B). The timer bounds intentional gathering, not total landing time.
- Rejected: a1's 3 goals or 2 hours (same 47 starts as 2 goals or 2 hours on the trace, more wait). Rejected: a fixed hour a day; every goal waits up to a day. Rejected: gathering at selection (`selectBatchLocked`, batch.go:481); the window is already open there. Rejected: a hold through `holdProofLocked`; it seals the batch and asks a person. Rejected: a clock hold while gathering (r1); it stops the merges gathering needs. Rejected: readiness recomputed from current eligibility (r2); a held member would reopen gathering and extend its allowance (Astra r2 F4).

**Replay and the proposed trial values.** The replay (designs/batch-gather-replay-2026-11-07.py) is an arrival sensitivity study. It is not a delivery forecast. Input: the hand-in lines of queue.jsonl, 2026-10-24 to 2026-11-07, 58 hand-ins of 37 goals. Model: one lane, 50-minute proofs, free merges and gates, arrivals during a proof wait for the next batch. Its table reproduces (Astra r2). Its limits: seal-on-one processes 58 submissions but 2 goals or 3 hours processes 54, because a re-hand-in of a waiting goal collapses into the first and a goal is forgotten after its counterfactual green while the trace keeps its real repair and re-hand-in. It ignores eligibility and returns, merge and gate time, the cheap gate's baseline, recovery proofs, repair time and the restart policy. 57 synthetic starts against 56 recorded attempts does not show equivalent work.

| Rule | Batch full starts | Goals per start | Mean wait to proof start | Longest wait |
|---|---|---|---|---|
| seal on one (today) | 57 | 1.02 | 2 min | 48 min |
| 2 goals or 1 h | 52 | 1.12 | 50 min | 61 min |
| 2 goals or 2 h | 47 | 1.19 | 88 min | 121 min |
| 3 goals or 2 h (a1) | 47 | 1.19 | 111 min | 121 min |
| **2 goals or 3 h** | **38** | **1.42** | **100 min** | **181 min** |
| 2 goals or 4 h | 35 | 1.54 | 107 min | 241 min |
| 3 goals or 4 h | 32 | 1.59 | 180 min | 241 min |

So a1 proposes 2 goals or 3 hours as a measured initial trial. Wido adopts it or not (R-36-m3). If he does, the lane reports three measures from its own records after two weeks: full suites per landed goal, lane minutes per landed goal, and wait from hand-in to proof start. A better replay (conserved workload with submission identity and terminal outcomes, clean success kept apart from observed rework, measured gate and merge overhead, D5's and D6's policies) is the evidence before anyone raises the values. It is not a project he has to accept first.

**D4. Health tells gathering from delay by actionable age, from recorded facts.**

- Rule: health keeps two ages per goal. `Minutes` is the total waiting age, shown as today (health.go:389). A new `Actionable` is total age minus the gathering allowance. The allowance is the gathering end minus this member's hand-in, never below zero. While the batch still gathers, the allowance is now minus hand-in. `Overdue` (health.go:413) and `cannotHeal` (:743) read `Actionable`. The 60 and 120 minute bounds stay.
- The end is recorded, never recomputed. The batch record gets one field, `GatheredUntil`. The keeper's selection writes it under the queue lock (`SelectBatch`, batch.go:363) the first time `Gathering` reports ready. The value written is the end computed from recorded facts, not the time of writing: the earliest of the Nth eligible member's hand-in, the oldest member's deadline, the fix member's hand-in, the seal's `SealedAt`, `Batch.Person.CheckedAt`. A person's proof admission writes it too, under the same lock, from `Running.Person.CheckedAt` (proof_admission.go:213-215), when the field is empty. Once written it never moves. A health read has no caller `seams.Person`; it reads the field.
- Never the batch's `CreatedAt`. A reopen keeps the original `CreatedAt` (batch_verbs.go:108 sets state and seal only), so `CreatedAt` can backdate readiness. Never a later person act on a batch already ready: that would extend the allowance.
- A member that joins after the end has allowance zero. It joined a ready batch; its wait is lane delay and shows.
- Example: a lone goal handed in at 14:46:38 gathers until 17:46:38 and is overdue only if still unlanded at 18:46:38. Two goals handed in at 11:08:34 and 11:10:41: gathering ended at 11:10:41, so the first goal's allowance is 2 minutes 7 seconds. If the second goal is held at 11:30, the end stays 11:10:41.
- Words: health and status say "gathering until 17:46 (1 of 2 goals)" and, once ready, "eligible to prove since 17:46". Never "full check by": under R-35-m3 a proof cannot promise its end.
- Rejected: follow the gathering flag (the deadline transition would fire both rows at once). Rejected: always subtract the full deadline (it hides real delay when a second member or a seal ended gathering early). Rejected: compute the end at each read from current members and the caller's person (r2); the early person proof and the held member would both extend the allowance.

**D5. An admitted full batch proof is not restarted by an ordinary newcomer; person acts stay as they are today.** (Amends lane-batching.md D2.)

- Rule: once a batch's full proof is admitted, an ordinary newcomer waits for the next batch with the reason "G waits: the batch's full check has started; it joins the next batch". The batch already gathered on purpose (D3).
- Kept as today, and tested: a person's `landing batch reopen --with G` or `landing restart` during a live proof is recorded as a pending request and acts on nothing until the proof ends; the person runs it again then (batch_verbs.go:88-98). A person-owned proof is never restarted (restart.go:180). An incident-fix newcomer preempts only when none of the common restrictions holds: a person-selected batch, a fix already proving, a classification in progress (restart.go:217-225, :243-247). The clean cancel (restart.go:275, :312, lane-batching.md D3) and its safety stay.
- Mechanism: in `restartBatchLocked` the ordinary rule at restart.go:226-230 (one restart per batch, under half the expected time) becomes the one wait reason above. Nothing else in the function changes. `expectedFullMinutes` stays for the restart record (:279).
- Amendment line for lane-batching.md, appended by the steward: "D2 is replaced by the-lane-batches-goals-and-proves-once D5: an admitted full proof is not restarted by an ordinary newcomer; a person's reopen or restart during a proof stays a pending request; D3 and D4 stand."
- Not here: an immediate person cancel. That is new behaviour with its own implementation and authorization.
- Rejected: keep the half-time restart. It spends a second full start on a batch that gathered to save one.

**D6. Automatic full checks of main are off by default; readiness has priority over a due clock.**

- Rule: the keeper starts a full suite of main on a timer only when `proof.trunk-every` is set to a duration of at least 24h. The default is `off`. A full suite runs at a landing (D1), when main is genuinely unproved, and when a person or the agent runs `metasystem landing prove --trunk`. A known red stays recorded and governed by its incident; it is not retried because time passed.
- Mechanism, one reader for three paths. A cadence reader replaces the parse in `fullProofClock` (wake.go:115-122) and returns off or the interval.
  1. The wake: `clockDue` (wake.go:105-112) returns false when off. When on, it is due when the last full main green, full batch push or credited fold (wake.go:131-155, D8) is the interval old. The scoped push's hourly debt is no longer a trigger of its own. `wakeReasons` (:85-91) and `overdueBatch` (depth.go:51) read `clockDue`, so neither fires when off.
  2. The standalone launch: `KeeperClock` (keeper_clock.go:83-94) returns at once when off, calls `prepare` and records "automatic full check is off; main is proven at each landing".
  3. The hourly-debt fallback: the debt branch of `KeeperClock` (:179-191) runs only when the cadence is on and due.
- Readiness hands priority to the batch (Astra r2 F2). Under the daily setting, with a batch open, `KeeperClock` never reaches `Start` (:192) while the batch has its next step to take. In the candidate block (:147-175):
  1. While `Gathering` says gathering: record "clock due; batch B gathers until 17:46; main's check waits for it", call `prepare`, return no hold.
  2. When ready and a pending member is not yet merged into HEAD: one wake per pending set, "clock due; batch B is ready; G merges first".
  3. The gate exception at :167 uses the acceptable-gate predicate (D3), so a skipped gate with a green static group is a passed gate and gets its wake, "the batch's gate passed, its full check goes next".
  4. The full proof running holds as today (:105-108); a green candidate gets its push wake as today (:158-162).
  Each state still gets one wake keyed by its attempt (`state.Gate`, `state.Push`). A state that does not advance after its wake falls back to main's check as today (TestKeeperClockCandidatePushOpportunity, push refused; TestKeeperClockSelectionFallbackOnce, gate red). So a stuck batch cannot hold main's check forever.
- `RecoverMovedMain` stays as is. It proves main only when main's commit and tree have no applicable recorded proof (trunk_recovery.go:91, :110-167): a full green on that tree in the newest environment, a current records check, or a known red with its incident. A batch push leaves its full green on main's tree, so recovery starts nothing. It never reads the clock and never recreates cadence. A test proves it (U2).
- Settings: `proof.trunk-every` default `off` (internal/config/defaults.go:86). `config validate` accepts `off` or a duration of at least 24h (validate.go:670-673). An invalid value is reported by `config validate` and in status, and the keeper treats it as off; a bad value never holds the lane and never starts suites.
- Eligibility interval, stated alone: with a 24h setting the keeper may start a main check once the last full coverage of main is 24 hours old. That governs proof frequency, not machine reservations; R-35-m3 is untouched.
- Detection trade-off, stated alone: neither setting promises to find environment drift within a day. A due check waits for a gathering or ready batch, a repair or a slow proof. `applicableMainProof` reuses a proof only in the newest recorded environment (trunk_recovery.go:121-131), and a scoped run in a changed environment widens to full (prove.go:1386-1410). Both stay. With the cadence off, a changed toolchain or host is found at the next full proof that runs: a landing, a recovery or an explicit `landing prove --trunk`.
- The landing skill: case 8 (SKILL.md:218-220) applies only when woken with `full-due`, which needs the daily setting. SKILL.md:197 "`proof.trunk-every` schedules its next proof" becomes "main is proven at each landing; `proof.trunk-every`, when set, adds a daily check".
- `ClockFull` goes. Every executable batch is full under D1, so the mark only forced repeats (results.jsonl:1138). The mark (keeper_clock.go:130-142), its reader (proof_admission.go:111-113) and its field (batch.go:68) are removed.
- Rejected: keep the 4-hour clock (2.1 lane-hours a day). Rejected: keep the clock as a seal trigger; it seals a gathering batch every few hours and defeats D3. Rejected: drop only the gathering suppression and keep the old candidate fallback (r2); a skipped gate then hands the lane to main's check at readiness.

**D7. Ruling text: R-149-m1e is amended.** The register is append-only (rulings.md header), so the steward appends a new row, proposed id R-150-m1e, dated 2026-11-07, owner m1e:

> PROOF DEPTH: TARGETED UNTIL THE BATCH, FULL AT THE LANDING (Wido, decision 222, 2026-11-07 19:4x: "batch up multiple features that test only what they touch each. Then have a single full suite when landing. A single feature having run a full suite is a very costly way to do this"). Amends R-149-m1e and restores R-3's amortization: unit checks, resolution checks and member gates run what the change touches, named selections inside cmd/metasystem included (decision 212 amended); an executable batch has a full green on its final tree before its push; a valid records-only inheritance keeps the batch's executed full coverage with no hour limit and its publication credits main's clock; the goal's tier no longer sets proof depth (landing.full-from-tier retired); an automatic batch may gather by member count and oldest wait before its full proof, with values Wido sets, and a person's act or a main-red fix proves at once; an admitted full proof is not restarted by an ordinary newcomer, and a person's reopen or restart during a proof stays a pending request; the keeper's automatic full check of main is off unless proof.trunk-every sets at least 24h, and a ready batch goes before it. It does not authorize reusing one machine's full green on another machine.

Context column: "132 full suites for 36 landed goals, 2026-10-24 to 11-07, 42 of them the 4-hour clock; design plans/designs/the-lane-batches-goals-and-proves-once.md". Review condition: full suites per landed goal, lane minutes per landed goal, and wait from hand-in to proof start, two weeks after the gathering trial starts. docs/collaboration.md:39, which cites R-149's tier floor, is updated in U1c.

The accepted page lane-proves-at-the-batch-risk.md keeps status accepted with this amendment line (Astra r2 F7):

> Amended by the-lane-batches-goals-and-proves-once D1, D2 and D6. D1 is replaced: an executable batch has a full green before its push. D1a keeps its provenance rules (scope impact, base, tree, plan hash, environment fingerprint) for an explicit `landing prove --impact`; its fixture "a first low-tier landing with no prior full proof proves at impact depth and pushes" is retired, because an impact green never pushes a batch. D2 is replaced by new D2: a named selection inside cmd/metasystem is cheap under the share, a whole selection is not. D2a keeps its share and canary accounting; its clause "a change-driven selection naming any cmd/metasystem test is never cheap" is replaced for every consumer (seat check, resolution check, gate) by new D2. D4 stands for the explicit impact check only. D5 is retired with landing.full-from-tier. D6 is replaced by new D6: no idle trunk proof by default, and a ready batch goes before a due daily check. D1b, D2b and D3 stand.

**D8. What is retired, re-keyed or repaired.**

- `landing.full-from-tier` is retired (U1c). Its readers go: `batchDepth` (test_impact.go:444, gone in U1b) and `selectDepthClass` (depth.go:15-45) with the `DepthClass` field and its two writers (batch.go:67, :510-518, :557-563). The key moves from the defaults (internal/config/defaults.go:74) to the retired-key tombstones (internal/config/budget.go:50), so a conf that still sets it is reported by `config validate`. The committed metasystem.conf does not set it; local confs were not read (hypothesis: none set it).
- Records reuse keeps a batch's full coverage (U5, Astra r2 F1). A records inheritance is valid when the executable paths since the full tree are records only (scope.go:178-195), the environment matches (prove.go:1386-1410), and no later red waits for resolution (scope.go:133-136). Then:
  1. A batch's own full green has no hour limit for a records-only move (scope.go:158-160). Only a full time in the future stays a refusal.
  2. A records-only batch reuses main's full green with no hour limit (scope.go:141-143); its reason at :113 drops "under an hour old".
  3. While main's clock is due, `BatchProofDepth` forces full only when the batch has no executed full green whose `FullAt` is not before the overdue time (proof_admission.go:115-117).
  4. `batchGreenReusable` accepts a scoped or inherited green whose full coverage (`FullTree` set, `FullAt` not before the overdue time) is the batch's own executed full (depth.go:85-88).
  5. `fullProofClock` credits a pushed scoped or inherited green of an executed full at that full's `FullAt` (wake.go:151-152). A full push keeps its push-time credit. Rejected: credit the fold at push time; a records refresh would then extend a stale full's age.
  The prove-time reuse hour in `fullCurrent` (scope.go:482) stays. Its effect is a rerun of the records checks, not a full suite.
- a-push-folds-a-records-only-move is superseded for these reuse rules. Its remaining scope is its own intent: the push path folds a records-only move of main since the batch base into the proven batch with the records-only classifier's cheap check, not a second proof (finding 460). The steward edits its next step before dispatch; this page does not. How much of that intent `inheritFold` (push.go:242) already covers is not verified here (OPEN).
- The stale comment on `landing.batch` ("consumption deferred to lane-reads-its-policies", defaults.go:71) is corrected; that goal is done.

## Units

Each unit is usable alone. Counted lines are production lines added, edited or removed, from the inventory below. The bound is Wido's ceiling of 250 production lines per unit, checked against the plain double: the whole count doubled (memory unit-estimates-run-double; decision 56 caps the actual change, and the double is the forecast margin). Tests, the skill and docs are outside the production count. Changed-line allocation with tests: about 1,400 lines.

Order (Astra r2, ordering conclusion): U1a first. Then U2, the first measurable saving: it removes the clock's 42 standalone suites in 15 days. Then U5, U3, then U1b and U1c (cleanup, any time after U1a), then U4a, U4b, U4c. U2 must not precede U1a: removing the clock overrides would expose the still-supported impact-only landing path. U3 adds targeted early checks where gates now skip; it removes no full suite by itself.

| Unit | Decisions | Counted | Plain double |
|---|---|---|---|
| U1a | D1 | 44 | 88 |
| U2 | D5, D6 | 47 | 94 |
| U5 | D8 records reuse | 16 | 32 |
| U3 | D2 | 12 | 24 |
| U1b | D1 cleanup | 102 | 204 |
| U1c | D8 tier floor, D7 docs | 49 | 98 |
| U4a | D3 readiness, no setting | 90 | 180 |
| U4b | D3, D4, D6 handoff; gathering off | 116 | 232 |
| U4c | D4 status words | 14 | 28 |
| **Total** | | **490** | |

**U1a. Full before push (D1).**

| Owner | Change | Added or edited | Removed |
|---|---|---|---|
| proof_admission.go:110-131 `BatchProofDepth` | full for every batch after the records-only return | 6 | 0 |
| proof_admission.go:79-85 `proofScope` | impact-depth branch | 0 | 7 |
| push.go:391-413 `provenGreen` | impact branch becomes one refusal | 3 | 24 |
| push.go:160-161 | impact-base refusal | 0 | 2 |
| intent_landing_prove.go:317-320 | decided depth no longer sets Impact | 1 | 1 |
| **Total** | | **10** | **34** |

Counted 44; plain double 88.

**U2. A full suite runs only for a landing (D5, D6 first paths).**

| Owner | Change | Added or edited | Removed |
|---|---|---|---|
| wake.go:115-122 | cadence reader: off or at least 24h; invalid reads as off | 8 | 0 |
| wake.go:105-112 `clockDue` | off returns false; debt is no separate trigger | 4 | 0 |
| keeper_clock.go:83-94 | early return when off, with its reason | 5 | 0 |
| keeper_clock.go:130-142, proof_admission.go:111-113, batch.go:68 | `ClockFull` mark, reader, field | 1 | 17 |
| internal/config/defaults.go:86, validate.go:670-673 | default `off`; accept off or at least 24h | 7 | 0 |
| restart.go:226-230 | ordinary newcomer waits once the full proof is admitted | 2 | 3 |
| **Total** | | **27** | **20** |

Counted 47; plain double 94. Before U4b, under the daily setting, the clock keeps today's candidate fallback; the default is off. The skill lines (SKILL.md:197, :218-220) are outside the count.

**U5. Records reuse keeps the batch's full coverage (D8).**

| Owner | Change | Added or edited | Removed |
|---|---|---|---|
| scope.go:158-160 | no hour for a batch's own full green; future time still refused | 2 | 0 |
| scope.go:141-143, :113 | no hour for a records-only batch on main's full green; reason text | 2 | 0 |
| proof_admission.go:115-117 | overdue forces full only without a fresh executed full of the batch | 5 | 0 |
| depth.go:85-88 `batchGreenReusable` | accept a records inheritance of the batch's executed full | 1 | 0 |
| wake.go:151-152 `fullProofClock` | credit a pushed scoped or inherited green at its full's `FullAt` | 6 | 0 |
| **Total** | | **16** | **0** |

Counted 16; plain double 32.

**U3. Named command selections are cheap at every consumer (D2).** `impactDepth` (test_impact.go:323-367): mark whole command selections; new cheap expression: 10 added or edited. `impactCost` reason (:411-413): 2. Counted 12; plain double 24. a-unit-check-runs-the-importers-too declares this unit as its dependency.

**U1b. The batch depth reader goes (D1 cleanup).**

| Owner | Change | Added or edited | Removed |
|---|---|---|---|
| test_impact.go:417-512 and imports | `batchDepthSeams`, `batchDepth` | 0 | 98 |
| intent_landing.go:1356, intent_landing_prove.go:124, prove.go:206-207 | seam and wirings | 2 | 2 |
| **Total** | | **2** | **100** |

Counted 102; plain double 204.

**U1c. The tier floor is retired (D8 first bullet, D7 docs line).**

| Owner | Change | Added or edited | Removed |
|---|---|---|---|
| depth.go:15-45, batch.go:67, :510-518, :557-563 | `selectDepthClass`, `DepthClass` | 2 | 45 |
| internal/config/defaults.go:74, budget.go:50 | key to tombstone | 1 | 1 |
| **Total** | | **3** | **46** |

Counted 49; plain double 98.

**U4a. The readiness calculation, complete and tested, with no setting (D3, D4 end).** No consumer calls it and no key exists, so nothing operational changes (Astra r2 F3). The policy is an argument.

| Owner | Change | Added or edited |
|---|---|---|
| new internal/landing/plain/gather.go `Gathering` | five ready conditions, sticky readiness, count, deadline | 55 |
| gather.go `gatheringEnd` | end from recorded facts: Nth hand-in, deadline, fix hand-in, `SealedAt`, `Batch.Person.CheckedAt`, `Running.Person.CheckedAt`; never `CreatedAt` | 15 |
| gather.go `gateAcceptable`, `unmergedMembers` | the shared gate predicate; pending members not in HEAD | 20 |
| **Total** | | **90** |

Counted 90; plain double 180.

**U4b. Gathering connected: admission, wake, clock and health land together, gathering off (D3, D4, D6 handoff).** U4b is dispatched after U4a. It ships with `gather-goals=1` and `gather-wait=0`. The trial values are two default lines, changed only on Wido's word.

| Owner | Change | Added or edited |
|---|---|---|
| internal/config/defaults.go:71, validate.go | two keys with help text, off; validation; stale comment | 6 |
| gather.go | policy read from the keys | 6 |
| batch.go:55-70, `SelectBatch` (batch.go:363) | `GatheredUntil` field; written once under the lock at first readiness | 9 |
| proof_admission.go:135 `admitExecutionLocked` | gathering refusal; a person's admission writes `GatheredUntil` from `Running.Person.CheckedAt` | 14 |
| intent_landing_prove.go:201-211 `proofRefusal` | render the gathering refusal as held, with its next command | 6 |
| wake.go after :314 `GatherWake`; landing_agent.go:300-306 | drop queued, proof-finished and full-due only while gathering with nothing to merge or gate | 33 |
| keeper_clock.go:147-175 | gathering: no `Start`, reason, no hold; ready: merge wake, acceptable-gate wake | 20 |
| health.go:100-106, :389-413, :741-745 | `Actionable` from `GatheredUntil`; overdue and cannot-heal read it; gathering reason | 22 |
| **Total** | | **116** |

Counted 116; plain double 232. Split seam if the builder passes 250 production lines: the refusal render and health's gathering reason move to U4c. Admission, wake, clock and `Actionable` never split (Astra r2 F3: admission without wake suppression launches agents to receive the refusal; gathering without health raises false overdue rows; gathering without the clock handoff starts main's check). The skill gets two lines outside the count (SKILL.md:96, :102, :107-108): "`landing prove` that answers gathering ends your turn; the keeper wakes you when there is work or at the deadline".

**U4c. Status shows gathering (D4 words).** status.go:27 `Status`, `ReadStatus` (:211), the status text in intent_landing.go: a gathering field and the two lines. Counted 14; plain double 28.

Total production: about 490 counted lines over nine units, every unit at or under 232 by the plain double.

## Tests

All tests keep the production decision owners and stub only effects: Git, process launch and liveness, the clock (R-104-m1e). Keeper scenarios run through `newLandingAgentKeeper` (cmd/metasystem/landing_agent.go:262), including its `Observe` path with `RecoverMovedMain`, with an injected `now`, process observations (`probe`, `running`, `Alive`, `Launch`) and Git. Arrival times come from the recorded trace. Every mutation below fails the named test.

**U1a.**
- `TestBatchPushNeedsAFullGreen` (plain, push bed): seed a historical admission at impact depth for the batch base and an impact green on the tree. The push is refused with the D1 reason. A full green on the same tree pushes. A records-only fold of a full green (`inheritFold`) pushes. Mutation: restore the impact branch of `provenGreen`; the seeded admission makes it accept, and the refusal row fails.
- `TestBatchProofDepthIsFullForEveryExecutableBatch` (plain): tier 1, two-package plan: `full` with the D1 reason. Mutation: call the old depth reader for low tiers; fails.
- `TestBatchProofDepthKeepsRecordsOnlyInheritance` (plain): a records-only move on a batch with a full green under an hour stays `scoped`. Mutation: drop the early return at :124; fails.
- `TestLandingExplicitImpactGreenNeverLands` (cmd, beside `TestLandingProveReusesGateOnlyAtBatchBaseWithStaticAndEnvironment`, landing_gate_reuse_test.go:301): `landing prove --impact --wait` green, then `landing push` refused. Mutation: accept impact when the admission's depth is impact; fails.
- Changed: `TestLandingProveBatchDepthReasonsAndStatus` (landing_depth_test.go:66): every row proves full with one reason. The "below" row's full green pushes; its former impact green, seeded with an impact-depth admission, is refused. `TestSettingsBatchDepthDefaults` (:201) and `TestLandingStatusBatchDepthInFlight` (:215) follow. The accepted page's D1a fixture (first low-tier impact landing pushes) is retired with it (D7).

**U2.**
- `TestKeeperCadenceOffStartsNoSuite` (cmd, `newLandingAgentKeeper`, 48 ticks an hour apart on the injected clock): main's tree carries the batch push's full green, `proof.trunk-every` unset. Zero trunk launches, no `full-due` wake, clock reason "automatic full check is off". Row daily: `proof.trunk-every=24h`, one trunk launch at 24h and none before. Row debt: a scoped push older than an hour starts no suite while off. Row invalid: `proof.trunk-every=4h` is reported by `config validate`; the keeper reads it as off and the lane is not held. Row known red: main's tree red with its incident; no start across 48 hours. Mutations: restore the 4h default (launches every 4 hours; fails); keep the debt as a separate trigger (the debt row launches; fails).
- `TestRecoverMovedMainProvesOnlyWhatIsUnproved` (cmd, same keeper, cadence off): main moved by an executable commit outside the lane with no proof: one trunk start. Main's tree with a known red and its incident: no start. Main's tree proven by a batch push: no start across 48 hours. Mutation: let recovery read the clock and start when due; the third row fails under the daily setting.
- `TestRestartSelectionAndWaitReasons` (restart_test.go:101) changed: the "young" row (12 minutes) now waits with "the batch's full check has started; it joins the next batch". "main red" and "main red after restart" still restart. Cancellation-safety rows stay. Mutation: restore the half-time rule; the young row fails.
- `TestPersonActDuringAFullProofStaysPending` (cmd, through `landing batch reopen --with G` and `landing restart` from an enrolled terminal, live full proof on the injected prober): each act is recorded with `Pending` and the pending impact text (batch_verbs.go:98); running.json, the attempt and the cancel file are unchanged; the proof is not cancelled; after the proof ends, the same reopen admits G. A person-owned proof with an ordinary newcomer is not restarted (restart.go:180). Mutation: let a pending reopen call `cancelProofLocked`; the unchanged-attempt assertion fails. This replaces r2's "a person's `landing restart` row restarts", which asserted behaviour the code does not have.
- Changed: `TestARecordsOnlyMoveKeepsMainsFullGreen` (internal/landing/plain/records_kept_test.go:64). Its four-hour launch (:94-100) and its hourly-debt `full-due` (:136-139) fail at U2's default, so the change lands here. Under the default: no launch at 4h; the cheap batch proves full with the D1 reason, not "full proof overdue since" (:91); the standing debt raises no `full-due`. With `proof.trunk-every=24h`: one launch at 24h. The kept-records rows (:102-122) stay.
- Changed: `TestLandingKeeperClockWiringAndVerboseReason` (landing_keeper_clock_test.go:18) runs its trunk rows with `proof.trunk-every=24h` and gains an off row with zero trunk launches. `TestLandingFullDueBatchAdapterProofPushClock` sets 24h. `TestLandingFullDueLedgerRefreshAdapterRunsFullAndPaysClock` sets 24h and keeps its expectation until U5. keeper_clock_test.go: `TestKeeperClockFullDepthMarkSurvivesLaterCredit` (:447) is deleted; `TestKeeperClockAdmitsFullCandidateAndCreditsPush` (:391) and `TestKeeperClockCandidatePushOpportunity` (:105) drop the mark.

**U5.**
- Replaced: `TestLandingFullDueLedgerRefreshAdapterRunsFullAndPaysClock` (cmd/metasystem/landing_depth_class_test.go:178) becomes `TestLandingFullDueLedgerRefreshKeepsFullCoverageAndPaysClock`. With `proof.trunk-every=24h` and the clock due: the first proof runs full; a goal-ledger commit on main one minute later; the refresh is scoped with the first proof's `FullTree` and `FullAt`, runs only the affected groups, and the full executions stay at one (the second-full expectation at :232 goes). The push clears `full-due`. Its "production proof has no batch depth policy" assertion (:194-196) is already gone in U1b if U1b lands first; otherwise U1b drops it. Mutations: restore the overdue force at proof_admission.go:115 (a second full; fails); restore `result.Scope == "full"` at depth.go:87 (the green is refused and reruns; fails); drop the fold credit (`full-due` stays after the push; fails).
- `TestBatchRecordsRefreshAfterAnHourKeepsFullCoverage` (plain, scope bed, injected clock, cadence off): a batch full green at T; main gains a goal-record commit; the refresh at T+2h is scoped with `FullAt` T and no full execution. Rows: an executable change after the hour is full with "an executable input changed"; a changed environment is "full check pending: the proof environment changed" (prove.go:1410); a later unresolved red is "a later failed check still needs resolution" (scope.go:134); a records-only batch two hours after main's full green is scoped (scope.go:141). Mutations: restore the hour at scope.go:158 (the first row is full; fails); restore it at :142 (the records-only row is full; fails); drop the executable-path check (the executable row is scoped; fails).
- `TestInheritedFoldPushPaysTheDailyClock` (plain, push bed, `proof.trunk-every=24h`): a fresh full green; the push folds a goal-record move (`inheritFold`, scope `inherited`); `fullProofClock` returns that full's `FullAt`; no `full-due`; the keeper starts no trunk check before 24h after `FullAt`. Mutation: credit only scope `full` (wake.go:152 as today); `full-due` is raised after the push; fails.

**U3.**
- `TestNamedCommandSelectionIsCheapAtEveryEntrypoint` (cmd): one plan `cmd/metasystem=TestProcess` and one plan `cmd/metasystem` through the seat's `--check` (test_impact.go:244), `committedImpactCheck` (:553) and the member gate. Named: all three cheap; the gate runs the static group, the baseline and the named test, green at depth impact. Whole: all three static-only; the gate is skipped with "change selects cmd/metasystem whole; the batch proof covers it". The gate rows use `newMergeGateBed` without its `ImpactCost` stub (landing_merge_gate_test.go:42), so production `impactCost` (intent_landing_prove.go:222-229) reads the plan. Mutations: treat named as whole at :367 (named rows fail at all three); treat whole as named (whole rows fail).
- Changed: `TestLandingProveDepthShareBoundary` (landing_depth_test.go:19) row "command named" becomes cheap, "command whole" stays not cheap, a new row for a group listing cmd/metasystem; `TestLandingDepthCountsEverySelectedPackage` (:246) and `TestLandingDepthRealCanariesDoNotDecideCheapness` (:324) "process changes" rows become cheap; `TestLandingDepthNamedCommandReasonWithRealCanaries` (:396) becomes cheap, plus a whole row with the new reason; `TestLandingCheckCheapOrSkipGitAdapter` (landing_check_skip_test.go:25) mode "cmd-named" runs the selection, new mode "cmd-whole" stays static-only; the real-plan assertion at :204-206 becomes cheap; `TestLandingGateNamedCommandFullReason` (landing_gate_reuse_test.go:430) takes the new text.

**U1b.** Changed: depth_test.go `TestFullDueBatchDepthAndClock` (:105) and `TestFullDueBatchFreshGreenWithinSelectionSecond` (:259) drop the `BatchDepth` seam; the ledger-refresh test (landing_depth_class_test.go:194-196) drops its "production proof has no batch depth policy" assertion; `TestLandingFullDueBatchAdapterProofPushClock` (:142) drops its `BatchDepth` stub.

**U1c.**
- `TestConfigRetiredFullFromTier` (config): `landing.full-from-tier=3` in metasystem.conf is reported retired. Mutation: drop the tombstone; fails.
- Changed: `TestSelectBatchDepthClasses` (depth_test.go:18) and `TestSelectBatchDepthNeedsNoAncestrySplit` (:235) keep membership and drop the class; `TestSelectBatchDepthReadFailures` (:182) and `TestSelectBatchReadsGoalFileTier` (:198) are deleted; `TestLandingSelectDepthClassStatus` (landing_depth_class_test.go:104); `DepthClass` fixtures in landing_check_skip_test.go:240, open_batch_test.go:327, batch_status_test.go:67, restart_test.go:49, health_test.go:405-406.

**U4a.** Pure tests of gather.go on recorded data; no keeper, no keys.
- `TestGatherReadinessOnRecordedArrivals` (plain, injected clock): hand-ins of queue.jsonl:257-259 (11:08:34Z, 11:10:41Z, 14:46:38Z), policy 2 goals or 3 hours. At 11:10:40 one member: gathering, deadline 14:08:34. At 11:10:41 two members: ready, end 11:10:41. The lone 14:46:38 goal: gathering at 17:46:37, ready at 17:46:38. Rows: an incident-fix member is ready at once; a seal is ready with end `SealedAt`; a person selection is ready with end `Batch.Person.CheckedAt`; a person proof is ready with end `Running.Person.CheckedAt`; a held member does not count; a numeric cap of 1 caps the count; a batch ready and then a member held stays ready (sticky). Mutations: measure from the batch's `CreatedAt`; drop the incident condition; ignore the seal; recompute readiness from current members. Each fails its row.
- `TestGateAcceptablePredicate` (plain): accepted: gate for HEAD green; gate for HEAD skipped with green static. Refused: static red; red attributed to the parent; no gate for HEAD; a gate for an earlier HEAD; a gate still running; a pending repair; a classification pending. Mutation: accept `Skipped` without the static check; the static-red-skipped row fails.

**U4b.**
- `TestBatchProofWaitsWhileGatheringAndStaysOpen` (plain, admission through `Start`, keys set to 2 and 3h): the agent's `landing prove` on a gathering batch returns `LANE_BATCH_GATHERING`; the batch stays open with no `SealedAt`; stops.jsonl and running.json are unchanged; with `seams.Person` set the proof is admitted and `GatheredUntil` is that person's `CheckedAt`. Mutation: route through `holdProofLocked`; `SealedAt` is set and the test fails.
- `TestKeeperDoesNotLaunchWhileGathering` (cmd, `newLandingAgentKeeper`): suppressed rows: every member merged, gate for HEAD green; gate skipped with green static. Launch rows: every refused row of the predicate test, and an unmerged arrival. Mutations: drop the predicate (the red rows fail); drop the merged-member check (the arrival row fails); leave `full-due` out of the dropped reasons (the daily row below fails).
- `TestKeeperGathersAndMergesUnderADueClock` (cmd, `newLandingAgentKeeper`, `proof.trunk-every=24h` and due, keys 2 and 3h, injected clock one minute a tick). Every gate in it is skipped with a green static group. Transition one, second arrival: the first member is handed in at 11:08:34 unmerged; the agent launches, merges and gates it; the second arrives at 11:10:41 unmerged, so the batch is ready before its merge; the agent launches, merges and gates it; then one full batch proof, its green and its push. Transition two, deadline: a lone member handed in at 14:46:38, merged and gated at once; at 17:46:38 the batch is ready; one full batch proof, its green and its push. Asserts for both: zero standalone trunk launches from start to push; exactly one batch full proof; no barren count accumulates; `GatheredUntil` is 11:10:41 and 17:46:38. Mutations: keep the old gate exception (`gate.Result == Green` at keeper_clock.go:167; the deadline transition starts main's check; fails); suppress `Start` only while gathering (both transitions start main's check after readiness; fails); return the gathering line as a hold (the first merge never happens; fails).
- `TestKeeperClockWaitsForAGatheringBatch` (keeper_clock bed, injected clock, daily setting): clock due and a gathering batch: reason recorded, no `Start`, empty hold, `prepare` called. Clock due and no batch: the main proof starts. Clock due, ready batch, gate red: one wake, then main's check as today. Mutation: start the main proof while gathering; fails.
- `TestHealthTellsGatheringFromDelay` (health bed, injected clock, continuous fixtures, keys 2 and 3h, health read with no caller person):
  1. Lone goal at 14:46:38. At 17:46:37: total 180 minutes shown, not overdue. At 17:46:38 ready; proof starts 17:47. At 18:46:37 not overdue; at 18:46:39 overdue; at 19:46:39 cannot-heal.
  2. Two goals at 11:08:34 and 11:10:41; at 12:10:42 the first goal is overdue (allowance 2 minutes 7 seconds).
  3. Early person proof: lone goal at 14:46:38; a person runs `landing prove` at 15:00:00 from an enrolled terminal. At 16:00:01 the goal is overdue (allowance 13 minutes 22 seconds), read with no `seams.Person`.
  4. Reopened old batch: batch created 09:00:00; member A handed in 08:59:00, member B at 09:20:00, ready and recorded at 09:20:00; full proof 09:21 to 10:10; G handed in at 09:40:00 waits (D5); after the proof the person reopens with G at 10:30:00. A's end stays 09:20:00, not `CreatedAt` 09:00:00 and not 10:30:00. G's allowance is zero.
  5. Eligibility after readiness: the two goals of fixture 2; at 11:30:00 the second is held. The end stays 11:10:41; at 12:10:42 the first goal is overdue; admission does not answer gathering.
  Mutations: read total minutes (fixture 1 fires at 17:46:38; fails); always subtract the full deadline (fixture 2 never fires; fails); compute the end at read time from current members and the caller (fixtures 3 and 5 do not fire; fail); use `CreatedAt` or the latest person act (fixture 4 fails).
- Default off: with the shipped values `gather-goals=1`, `gather-wait=0`, the existing batch, keeper and health tests pass unchanged.

**U4c.**
- `TestLandingStatusShowsGathering` (cmd): "gathering until HH:MM (1 of 2 goals)" in local time, "eligible to prove since HH:MM" once ready, and the JSON field.
- Changed: `TestSkillLandingAgentGateStatesAndBatchDepth` (landing_gate_reuse_test.go:404) requires the gathering line.

## Questions answered

Revision 1 sent six questions with its critique prompt. Astra's answers, adopted, stood in round 2:

1. Is a named selection inside cmd/metasystem enough? It is a heuristic early check with the full batch as its net. The selector does not always widen to shared command wiring when it finds names (D2).
2. Do the named and whole assertions in the existing tests change? Yes, as listed under U3, with the wiring tests.
3. Where does readiness belong? Before proof execution admission, with the shared wake and health handling of D3 and D4.
4. Is a new ruling row needed? Yes, appended under Wido's authority (D7), amending cadence, restart and decision 212 explicitly.
5. Can the deploy record serve as a fleet full-proof certificate? No. It lacks full-proof coverage, the proven tree and environment evidence (deploy/record.go:37; proofrun/groups.go:314). A separate later page.
6. Does the September rule cover finding 548? No. It misses the configuration-literal specimen; the configuration-key fixture stays with a-unit-check-runs-the-importers-too.

## Acceptance a person can run

1. No idle suite. Leave the lane with nothing handed in for 24 hours under the default. results.jsonl gains no trunk full attempt; `landing status` shows "automatic full check is off". (After U2.)
2. A newcomer does not cancel a full proof. Hand in a goal 10 minutes into a batch's full proof. results.jsonl shows no cancelled attempt; status shows the newcomer waiting for the next batch. A person's `landing batch reopen --with G` during the proof shows "request pending" and the proof runs on. (After U2.)
3. A records-only move does not rerun a green batch's full proof. Move main by a goal-record commit two hours after a batch's full green. The batch's refresh is scoped; results.jsonl shows one full attempt for that batch id. With `proof.trunk-every=24h`, the inherited push clears `full-due` in `landing status`. (After U5.)
4. A member gate inside cmd/metasystem runs named tests in minutes. Hand in a goal whose change selects named cmd/metasystem tests. `landing status` shows `last_gate` green at depth impact; the gate log shows `-run` with those names; the gate's minutes, baseline included, are under 10 (target; hypothesis until measured). (After U3.)
5. A batch of two goals lands with one full suite, once Wido sets the trial values. Hand in two goals within three hours. `metasystem landing status` shows "gathering until HH:MM (1 of 2 goals)", then "proving at full depth". pushes.jsonl gets one line with both members; results.jsonl has one full attempt for that batch id; `metasystem landing health` shows no overdue goal while it gathered. (After U4c and his word.)

## What this page does not change

- The targeted selector: importer edges, source audits, the configuration-key literal addition and finding 548's fixture stay with a-unit-check-runs-the-importers-too, which builds on U3.
- The push path's fold of main's records-only move since the batch base (a-push-folds-a-records-only-move, its remaining scope, D8).
- Main's own records check, repaired on October 30 (d4b0127a84).
- Reuse of a full green across machines. A separate later page.
- Proof parallelism: the-proofs-critical-path-is-short.
- `RecoverMovedMain` and the explicit `landing prove --trunk`.
- A person's cancel of a live proof.

## Measures

Read from results.jsonl, pushes.jsonl, gates.jsonl and queue.jsonl. Latest record per attempt. Batch, main and cancelled attempts counted apart. These are reported, not forecast: after U2 and U5 for the clock rows, two weeks after the gathering trial for the rest.

| Measure | 2026-10-24 to 11-07 | Report |
|---|---|---|
| Full suites per landed goal | 132 / 36 = 3.7 (batch 1.56, main 2.1) | the same split, plus cancelled attempts |
| Lane full-proof minutes per landed goal | (3,059 + 3,493) / 36 = 182 | the same convention |
| Clock suites of main | 42, about 1,904 minutes | zero under the default |
| Repeat batch fulls on the same batch (overdue, `ClockFull`) | 24 | zero |
| Wait from hand-in to proof start | not measured | median and longest |
| Hand-in to landing (last hand-in to push) | median 95 min, 51 to 837 | median and longest |
| Member gate minutes (skipped, static only) | median 4.5, max 9.2 | named runs, baseline included |

The 22 historical records-only main proofs (1,005 minutes, all by October 30) are not counted as a saving of this page.

## Critique record

- Round 1, 2026-11-07 about 20:20 CET (findings written 20:21), log designs/astra-batches-r1.log, findings designs/astra-batches-r1.findings.md: 8 material, 0 minor, verdict build after the folds. Folded in r2: F1 as D6 and D7 (cadence off by default); F2 as D2 (decision 212 amended; this page owns test_impact.go:367); F3 as D3 (no clock hold; the shared gathering wake drops `full-due` too); F4 as D3's gate predicate (green or skipped with green static, no repair or classification obligation); F5 as D4 (actionable age; new status words); F6 as D5 (no ordinary restart of an admitted full proof); F7 as the replay paragraph and the Problem corrections (37 goals, clock 24 of 56, minutes by one convention); F8 as the Tests preamble and the corrected U1a, U3 and U4a tests. U1 split into U1a, U1b and U1c.
- Round 2, 2026-11-07, prompt 20:32 CET, findings written 20:48, log designs/astra-batches-r2.log, findings designs/astra-batches-r2.findings.md: 7 material, 0 minor, verdict build after the folds. Folded in r3, as a1 decided them: F1 as D8's records-reuse repair and new unit U5 (Problem point 9; a-push-folds-a-records-only-move superseded for the reuse rules; the ledger-refresh test's second full replaced); F2 as D6's readiness handoff with the shared acceptable-gate predicate and the extended keeper test (deadline and second arrival, skipped gates, zero main starts, one batch proof); F3 as the U4a/U4b/U4c cut (U4a pure, no setting; U4b connects admission, wake, clock and health together, gathering off); F4 as D3's sticky readiness and D4's recorded `GatheredUntil` from the actual person act, with three new health fixtures; F5 as D5 rewritten to today's pending requests, Problem point 7 corrected, and a pending-act test replacing the false restart row; F6 as the split Decisions for Wido (A under decision 222, B for his adoption under R-36-m3; U4b ships with gathering off until his word); F7 as the rewritten amendment line (D1a fixture retired, provenance kept; D2a clause replaced for every consumer, accounting kept; points at new D1, D2 and D6). Also folded: the ordering conclusion (U1a, then U2 as the first measurable saving), the eligibility interval and the detection trade-off stated apart, and OPEN 3 (the 1,005 historical minutes are not this page's saving).
