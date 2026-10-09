# Design brief: autonomy-grows-with-measured-agreement

Working Mode: Design
Proposed goal, opened 2026-10-09 by Wido via m1e. Source: Wido's conversation of 2026-10-08 with a fellow engineer about how an organisation hands work to agents safely, read against the metasystem by m1e on 2026-10-09. The transcript is not in the repository; this page carries everything the design needs.

## Intent, in one sentence

Autonomy is earned per class of work, not set once: the machinery records whether the person agreed with the reviewer on every finding and every verdict, reports the agreement per class, and the retro proposes a wider automatic lane for a class only after agreement held over several goals; the person changes the setting, the machine never does.

## Why

Wido's picture for handing back-office work to an agent is a ladder with three rungs. First the agent does the task in the background and a person compares; nothing depends on it. Then the agent does the task and the person says yes or no. Then the task runs without the person, and only for the tasks where the record shows the agent was right often enough. Confidence is measured, not assumed, and it is measured per task, so one task can be fully automatic while the next still waits for a word.

The metasystem has the middle rung and a static knob for the top one. The landing gate waits for a person at or above `landing.review.human-from-tier` and lands below it after `landing.review.auto-after` (`internal/goal/landgate.go:61`, `:251`). In the review room the reviewer recommends one decision per finding and one way for the verdict; an undecided finding follows the recommendation (`internal/ui/uitools/deposit.go:106`, `:117`). What is missing is the measurement that would justify moving the knob: nobody records whether the person agreed with the recommendation, nothing reports agreement per class, and no path turns sustained agreement into a proposal.

The paper already describes the ladder for rules: warnings, then refusal in isolation, then refusal for a limited class, then full power, each step advanced only while the false-alarm bound holds (`paper/12-learning-systems.md:53`, repository top level). This goal applies the same ladder to the review of landed work. Wido's rule of 2026-10-06 is the governing principle: a practice applied by hand is promoted to machinery once its measure holds on two goals.

## What is true today (read 2026-10-09; the design verifies every site)

1. The person's verdict on a goal is a history line on the goal, `reviewed verdict=clear-to-land by=NAME tip=... record=...`, and the gate reads the newest word at the tip (`internal/goal/landgate.go:276`).
2. Each finding in a review record carries the reviewer's layers: severity (blocks, fix, note), title, why, `Recommends:` with one of must-fix, fix-later, not-a-problem, accept, and the person's answer on the same finding (`internal/ui/uitools/deposit.go:99-119`; design `review-findings-read-as-decisions`, done).
3. A finding the person never touched follows the recommendation and the record says so; the verdict's recommended way is a rule (send it back whenever any effective decision is must-fix).
4. The retro reads receipts and the instruction ledger and proposes changes with a testable expected effect for the person's veto (`skills/retro/SKILL.md`). It does not read review records.
5. `landing.review.human-from-tier` and `auto-after` are two settings for the whole project, not per class. The tier is derived from severity and novelty at intake (goal `tier-from-severity-and-novelty`, done).
6. Changing a `landing.*` setting is a person's act; under `process.change=person` an agent's attempt becomes a question (goal `machinery-measures-its-own-process`, U2). NO HAL 9000 (Wido, 2026-10-03): the machine never overrules a person and never rewrites its own rules without one.
7. The lane is off until the plan's goals land (Wido, 2026-10-06). Landings happen by hand. Nothing records what the lane would have done, so switch-on will rest on the lane's tests alone and not on a comparison with the hand landings it replaces.

## What done looks like

- Every finding decision and every verdict is recorded as agreement or disagreement with the reviewer's recommendation, with the class it belongs to: the finding's severity, the goal's tier, and the kind of change (the design decides the kinds; code, records, interface and machinery are the obvious first cut). A finding the person left to the recommendation counts as agreement by default and is marked as such, so a person who nods through everything is visible as a person who nods through everything.
- `metasystem receipt status` reports agreement per class for the period: how many decisions, how many agreed, how many overrode, and in which direction (the person was stricter, or the person was more lenient).
- The retro gains one step: for a class whose agreement held above a bound over at least two goals and at least N decisions (the design fixes the bound and N, and they are settings a person owns), it proposes the one setting change that would widen the automatic lane for that class, with the expected effect written in the instruction ledger as every retro change is. The person applies or vetoes. The machine never applies it.
- The same step proposes the reverse when agreement falls: narrowing the lane for a class whose overrides rose, so the ladder goes both ways.
- Shadow rung for the lane: while the lane is off, every hand landing of a goal gets one record of what the lane would have decided for it (would it have proven, what gate, would it have landed or waited for a person, and why), written by the lane's own decision code run read-only against the branch. `landing status` shows the comparison for the period: hand landings the lane would have refused, lane landings the person refused. This is the evidence the switch-on trial reads.

## Measure

Agreement per class over a period, and the count of setting changes proposed and applied. The goal is proven when one class reaches the bound on real reviews and the retro proposes the change in words a person can act on, and when the shadow record exists for every hand landing of one week.

## Constraints and freedoms

- NO HAL 9000: the machine proposes, the person changes the setting. A grant or the helm does not turn a proposal into a change.
- The design principle against shadow modes (`docs/design/design-principles.md:42`) is about internal design corrections. This shadow rung sits at the external boundary where a person's word is replaced, which is exactly where that principle asks for an explicit rollback. The design states this in one paragraph; it does not argue around it.
- A default agreement is marked as a default; the measure must never hide that the person decided nothing.
- Small: at most five units of at most 250 production lines; one gate per goal. The smallest step one is the agreement record and its line in receipt status; the retro step and the shadow rung follow.
- Free: the class vocabulary, the bound, where the agreement record lives (the review record or the goal's history), and whether the shadow record is a history line or a file beside the landing record.

## Not in this goal

Changing the landing gate's semantics; a new review verb; per-class settings in the gate itself (the retro proposes a change to the existing settings; per-class gating is a later goal if the measure shows it is worth it); the autonomy of design critique (the critique stop rules own that).

## Neighbours to read first

`review-findings-read-as-decisions` (done: the recommendation and the four decisions), `machinery-measures-its-own-process` (the measures reader and `process.change`), `landing-takes-an-hour` (the landing clock), `switch-on-trial` (the shadow record is its evidence), `tier-from-severity-and-novelty` (done), `continuous-self-improvement` (a proposal queue; this goal's proposals go through the retro, not a second queue).
