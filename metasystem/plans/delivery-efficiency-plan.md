# delivery-efficiency

- Owner: m1e (Wido's word 2026-09-11: "create a design and plan for all these, slice it in goals, put these goals on the backlog and pin them to m1e, prioritise these above all else so m1e can execute them in order")
- Goal and current status: landed work per day goes up by cutting waiting, re-verification and ceremony together; the five-day audit of 6 to 11 September is the baseline. Status: designed, sliced, critiqued once (Codex, 2026-09-11, 23 findings, 21 applied, see section 6); twenty-one goals pinned to m1e at priority 1 in execution order; slices already landed today by the running m1e session: c08fe2cb (go groups bounded by consumption, goal 11's slice 1) and 7cbd1188 (deep battery groups side by side, goal 12's slice 2).
- In flight right now: nothing under this plan (m1e holds dispatcher-fixture-long-waits-bounded, a suite-speed slice-4 goal, when this plan was written).
- Decisions made (and who made them): Wido, 2026-09-11: the whole program is executed by m1e, in order, above everything else on the backlog. Wido, 2026-09-11 morning: approvals are deliberate per goal; the program's goals are approved in his name from the enrolled terminal under his four-day grant.
- Waiting on the human: six policy decisions under R-36-m3, each put to Wido on its goal's design page and built only after his word records it: goal 1 (whether an admission cap on top-level attempts supersedes R-35-m3), goal 6 (seats open only blockers), goal 8 (budget self-extension and power of attorney for tier 1 and 2 acts), goal 9 (delivery receipts stop at first failure, amending R-16 for the delivery purpose), goal 16 (fold-proven closure and the round-2 failsafe). Silence and execution approval are not ratification.
- Dead ends (do not retry without new evidence): lifting the same-goal reuse rule alone (saves about 75 minutes fleet-wide; identity churn is the binding constraint). Lengthening the Go timeout (rejected by Wido 2026-09-10: remove the wall clock, do not lengthen it). A host-wide slot cap that counts nested fixture receipts (deadlocks two batteries; Codex finding 1). An auto-park on breach-stop (the fenced-claim quota exemption already landed as breach-stop-wedges-seat).
- Next step: m1e claims goal 1 (receipt-admission-caps-concurrent-batteries) when its current claim ends, and proceeds down the rank.

Evidence: the audit report (artifact "Fleet Delivery Deep Dive", 2026-09-11) and its six source analyses under `metasystem/artifacts/reports/delivery-deep-dive-2026-09-11/` in the m1e checkout (ledger.md, git-records.md, proof-attempts.md, claude-sessions.md, codex-sessions.md, process-rules.md). Every number below is from those files.

## 1. What the audit found

Five days, 6 to 11 September: 1,948 commits on main, 67 code landings (56 pipeline repairs, 48 without a receipt), 62 goals concluded against 179 opened. Coordinator seats spent 302 of 360 session hours waiting and 5.33 billion tokens re-reading 400 to 500 thousand token contexts on polls. Codex delegates spent 45 of 146 active hours in fold-critique rounds and 31 of 182 jobs returned nothing. Proof took 3.1 attempts per goal that reached green; goal-full-coverage failed 16 attempts on Go's default ten-minute timeout, from 8 percent alone to 100 percent with three concurrent batteries; five attempts hung up to 27 hours; reuse fired for 3.6 percent of groups. Wido performed 43 set-budget acts, 26 repeat raises within a day, 11 overnight. The nominal tier 3 pipeline is 6 to 12 wall hours; 75 percent of goals are tier 3; critique yield 68 percent; 1 of 13 design chains ended on its own stop rule.

The verdict: the pipeline is waiting-bound and verification-bound, not generation-bound. The suite is the largest calendar blocker but not the largest sink.

## 2. Design

One principle above the cuts, and three cuts made together, each with the guarantee it must keep:

0. Runtime independence. The metasystem is agent-independent: every mechanism in this plan holds for every runtime the roster can host today (claude, codex, devin) and for future adapters such as opencode, and lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime's hook, harness, CLI or transcript format. A runtime's native facility (a Claude Code Stop hook, a background-task notification, a Codex or Devin session record) may accelerate a mechanism through its adapter but is never the mechanism, and a runtime without the facility still gets the guarantee through the verb. Every goal that touches a runtime path proves its DONE on at least two runtimes. Every goal's intent carries this clause.

1. Waiting. A seat that has nothing to do sleeps until an event; it never pays a full context to ask whether something finished, and the Stop hook never forces a turn on an unchanged condition. Guarantee kept: never-idle (a pending wait is not idleness; the seat resumes on the event), stop safety (a refusal still names a real condition).
2. Verification. A test result is computed once per identity and reused from the newest observation only; a delivery receipt stops at its first failure; load-caused failures are attributable and treated as patience defects under R-35-m3; hung attempts are reconciled by liveness, never by the clock. Guarantee kept: no landing without sufficient retained proof for every required group; cadence still sweeps the whole battery with fresh executions when it establishes trust; the judge is changed only in its own goal with its own battery and three fresh green cadence runs.
3. Ceremony. Tier follows severity and novelty and critique follows the tier (critique-always); small changes take a small lane; critique closes on folded proof through a certification record and design critique fails safe at round two with a fixture-obligation exit; built work lands without waiting for a claim slot; budgets extend once by measured progress. Guarantee kept: every backlog change is critiqued at its tier (Wido 2026-09-10); the final-tree certification gate stays; human authority over approve, resume, accept-risk and done; the ledger records every act.

Phase A (operations and rulings, small mechanisms) comes first because it removes failure modes that would corrupt the measurements of phases B and C. Phase B changes the proof engine. Phase C changes coordination. Within a phase the order is by measured cost.

Two items of the audit are operations, not goals: run three seats instead of five until goals 1 and 11 land, and cap concurrent receipts at two by hand until goal 1 enforces it. They are Wido's to apply today.

## 3. The slices, in execution order

Each goal's intent on the ledger carries its DONE line, evidence and proof; this table is the map. "New" goals were opened 2026-09-11 with human origin; "adopted" goals already existed and were pinned and ranked into the program.

| # | Goal | Phase | Source | Tier | Cut |
| --- | --- | --- | --- | --- | --- |
| 1 | receipt-admission-caps-concurrent-batteries | A | new; re-aimed after critique: attributable load first, a cap only under a superseding ruling | 3 | verification |
| 2 | steward-launches-resolve-their-model | A | new; re-aimed: the config's own fallback is the template | 2 | waiting (runtime) |
| 3 | hung-proof-attempts-end-at-their-deadline | A | new; narrowed to liveness reconciliation | 3 | verification |
| 4 | stop-hook-never-forces-an-empty-turn | A | new; idle-with-backlog path kept | 3 | waiting |
| 5 | landing-refuses-without-its-receipt-line | A | new | 2 | evidence |
| 6 | seat-opened-goals-name-their-blocker | A | new; opens and parks in one publish | 2 | ceremony |
| 7 | proof-attempts-settle-to-minutes-run | A | adopted; settlement must be right before budgets self-extend | 2 | ceremony (human gating) |
| 8 | budget-extends-by-consumption-and-breach-parks | A | new; auto-park dropped, allowance made exact | 3 | ceremony (human gating) |
| 9 | delivery-receipt-stops-at-first-failure | B | new; carries its own narrow retry reuse | 3 | verification |
| 10 | retained-proof-reuse-crosses-claims-and-attempts | B | adopted, intent amended twice | 3 | verification |
| 11 | proof-groups-detect-hangs-by-progress-not-the-clock | B | adopted from m1; slice 1 landed c08fe2cb, slice 2 remains | 3 | verification |
| 12 | deep-battery-under-ten-minutes | B | adopted; claimed by m1e, slice 2 landed 7cbd1188 | 3 | verification |
| 13 | delegate-rounds-reuse-a-warm-gate | B | new; baseline corrected | 3 | verification |
| 14 | coordinator-wakes-on-events-not-polls | C | new; measured by wake-ups and latency | 3 | waiting |
| 15 | coordinator-context-stays-under-budget | C | new; bounded by maximum, not median | 3 | waiting |
| 16 | critique-closes-on-folded-proof | C | new; certification record and fixture-obligation exit | 3 | ceremony |
| 17 | tier-from-severity-and-novelty | C | new | 2 | ceremony |
| 18 | critique-always | C | adopted; critique follows the tier, replacing the hazard-class exemption | 3 | ceremony |
| 19 | small-change-lane | C | adopted from m1b; DONE written; needs Wido's classification before approval | 0 | ceremony |
| 20 | land-ready-work-lands-without-a-claim-slot | C | new; narrowed to the slot and the accounting episode | 3 | ceremony |
| 21 | capped-round-continues-instead-of-restarting | C | adopted | 3 | verification (delegate) |

Dependencies that matter: 7 before 8 (settlement before self-extension). 9 carries its own narrow retry reuse and lands before 10, which widens identity. 3 and 11 share one liveness contract: 3 owns dead-launcher and orphan reconciliation, 11 owns elapsed-time patience; neither releases capacity on the clock. 11 before 12 (same engine). 14 and 15 are independent. 17 before 18 before 19: the lane is defined by tier and its read by critique-always. 20 changes no receipt acceptance; record-only tip moves stay with landing-receipt-survives-records-drift, which is land-ready on m1e and should land before 20.

## 4. Rules for m1e while executing

- One goal at a time, in rank order; a goal that cannot proceed is parked with the blocker named and the next goal is claimed, and the parked goal is the first to return.
- No new goals opened under this plan except a defect that blocks the current goal (the rule goal 6 makes law).
- Every goal that changes the judge (1 if the cap is built, 3, 9, 10, 11, 12, 13) lands in its own goal with its own battery and three fresh green cadence runs, executed with no reuse, before the next judge change starts.
- Design first for 1, 3, 4, 8, 9, 10, 13, 14, 15, 16, 20 (mechanisms); 2, 5, 6, 7, 17 go straight to build at their tier.
- Policy decisions inside 1, 6, 8, 9 and 16 are put to Wido on the design page with the evidence, per R-36-m3; the seat builds them only after his word records the decision. Everything else in those goals proceeds meanwhile.
- Each landing appends its receipt line in the same commit (the rule goal 5 enforces).
- No goal may be closed with a runtime-specific mechanism: a DONE that works only under Claude Code, only under Codex or only under Devin is not done (principle 0).

## 5. How progress is measured

Weekly, from the same sources as the audit, over a fixed cohort: every goal concluded in the week, every code landing on origin/main in the week (a commit touching .go, .sh, testing.json or metasystem.conf), every proof attempt started in the week, every Claude and Codex session started in the week. Missing data is reported as missing, never filled in.

- Code landings per day, split into capability and pipeline repair, plus post-landing rework: forward fixes for a main the landing left red, counted against the landing.
- Tokens per code landing, both runtimes, and the coordinator's model wake-ups per pending hour, tokens while pending, and event-to-resume latency.
- Proof: active hours per green goal, attempts per green goal, reused group share, attempts terminalized by liveness reconciliation, load-attributed failures.
- Delegate rounds per landing and redundant closing reads (a read after a clean read of the same tree).
- Human acts per landing, split into manual acts at the terminal and acts under a recorded authority; overnight budget raises with old and new values once goal 8 records them.
- Goals opened per goal concluded, by origin; hours from land-ready to landed for goals that reached done; fleet-red hours per day from the landings that left main red.

The baseline is the audit's table. The program succeeds when capability landings per day rise while tokens per landing and manual human acts per landing fall, three weeks running, with no rise in post-landing rework.

## 6. Critique record

Round 1, Codex (gpt-5.6-sol), 2026-09-11 16:10Z, read-only, 23 material findings. Applied: 1 (goal 1 re-aimed; the cap is a ruling question under R-35 and counts top-level attempts only), 2, 3, 4, 5 (goal 10's identity, newest-observation and fresh-cadence rules), 6 (goals 3 and 11 share one liveness contract), 7 (goal 4 keeps the idle path), 8 (goal 8 exact allowance; goal 7 adopted first), 9 (goal 6 opens and parks in one publish), 10 and 11 (goal 16 certification record and fixture-obligation exit, redundant-read metric), 12 (six decisions listed for Wido; goal 1 named), 13 (goal 9 carries its own retry reuse; numbers corrected), 14 (auto-park dropped), 15 (critique-always adopted as goal 18), 16 (goals 11 and 12 reconciled with today's landings), 17 (goal 20 narrowed), 18 (this section), 19 (goal 14 measures wake-ups), 20 (goal 15 bounded by maximum), 21 (goal 13 baseline corrected), 22 (goal 2 re-aimed at the configuration), 23 (small-change-lane DONE written). Not applied: none rejected outright; finding 13's proposal to reorder 10 before 9 was answered by giving 9 its own narrow retry reuse instead. Full text: the session scratchpad, codex-critique-r1.md.
