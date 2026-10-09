# Design brief: lead-time-decides-parallelism

Working Mode: Design
Proposed goal, opened 2026-10-09 by Wido via m1e. Source: Wido's conversation of 2026-10-08 with a fellow engineer about delegating to agents, read against the metasystem by m1e on 2026-10-09. The transcript is not in the repository; this page carries everything the design needs.

## Intent, in one sentence

Lead time, from a goal's opening to its landing on main, with the wait for a person and the review rounds shown apart, is measured per goal and per period and is the number that decides how many goals run in parallel; throughput is never claimed without it.

## Why

The engineer in the conversation runs five agents in parallel and finds that the throughput looks good but the latency to anything a user can touch is poor: each change comes back as "okay", draws a round of comments, the fix draws another round, and the reviews pile up behind each other. He is faster alone on one change than the agent is, and he is not convinced the parallel version wins on impact. Wido's answer was that one person can run five at once; the engineer's answer was that the five all wait on review.

Both are measurable, and the metasystem measures neither yet. It has hours per unit, corrections per unit and the wait for a person per unit (goal `machinery-measures-its-own-process`, the measures reader) and the landing clock per landing (goal `landing-takes-an-hour`, U5). It has no number for the whole journey of a goal and no number for how many rounds of review a goal took before it landed. Without those, the WIP limit (Wido, 2026-09-18: at most three goals with coding started) and the parallel delegate count (Wido, 2026-10-08: as many as practical, CPU is the limit) rest on judgement. Wido's rule of 2026-09-19: measure before claiming.

## What is true today (read 2026-10-09; the design verifies every site)

1. The goal ledger's history records every act with its time and actor: open, approve, claim, hand-in, review verdicts, landed, done (`internal/goal/file.go:463`, actors `machine+lineage` or `human:<name>`).
2. The measures reader `processmeasure.Read` derives per-unit measures from retained run, launch, question and proof evidence, and reports unknowns as unknown, never zero (handoff `plans/handoff-agentic-tools-m1e-machinery-measures-its-own-process-machinery-measures-its-own-process.md`, section 1). Its status faces are `work status GOAL --work UNIT` and the goal status route.
3. The landing clock records merge, gate, fix rounds and push minutes per landing (`plans/landing-takes-an-hour-design-brief.md`, U5).
4. Critique rounds and send-backs are recorded on the design page's critique record, on the review record and as history lines; no reader counts them per goal.
5. `metasystem receipt status` lays out the period's numbers for the retro (`cmd/metasystem/receipt_verbs.go`).
6. The WIP limit is a rule in Wido's words and in briefs; no setting or verb holds it.

## What done looks like

- Per goal, one line in `goal show` and in the goal status route: lead time from open to landed on main, split into waiting for approval, waiting for a seat, building, waiting for review, waiting for a person's word, and landing. Each part is derived from recorded times; a part with no evidence says unknown.
- Per goal: review rounds, counted as critique rounds on the design plus code reads plus send-backs, each kind named, so a goal that took eight rounds is visible beside a goal that took one.
- Per period, in `receipt status`: median and worst lead time, the share of lead time spent waiting on a person, rounds per goal, and goals landed per week, side by side with how many goals were in flight at once. The retro reads these.
- A setting for the number of goals that may have coding started at once, read by claim, with the measured lead time printed beside it in `goal list` so the person sees the number the limit rests on. Changing it is a process change under `process.change`.
- Lead time measured by hand on the goals of 2026-10-06 to 2026-10-09 is the baseline, written into this brief's goal when the reader exists.

## Measure

Lead time per goal and per period, review rounds per goal, and the WIP setting with its evidence. The goal is proven when the retro can say, from the numbers, whether raising or lowering parallelism shortened lead time over two periods.

## Constraints and freedoms

- Derive, never type: every number comes from recorded acts; unknown stays unknown.
- Extend the measures reader; do not build a second reader or a totals database.
- Human-readable: the status line reads as a sentence a colleague would understand; the parts are named in words, not codes.
- Small: at most five units of at most 250 production lines; one gate. Step one is the per-goal lead time line.
- Free: the exact split of the journey, the period boundaries, whether rounds are read from records or from history lines.

## Not in this goal

Changing how work is scheduled; the landing clock itself; estimates and budgets (`goal-budget-follows-its-plan`); deciding the right parallelism (the measure informs the person and the retro).

## Neighbours to read first

`machinery-measures-its-own-process` (the reader this goal extends; this goal waits for it), `landing-takes-an-hour` (the landing clock), `goal-budget-follows-its-plan` (estimates against actuals), `machine-concurrency-governor` (CPU-side limits on dispatch).
