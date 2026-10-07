# Design brief: machinery-measures-its-own-process

Working Mode: Design
Proposed goal (not yet opened). Written by m1e on 2026-10-07 22:50 CEST after Wido asked what the steward needs to be self-aware enough to steer clear of the failure m1e showed that day: m1e added test runs and check-and-fix rounds to every unit, reactively, against two standing rules it held in memory ("expensive test once per batch", "impacted tests per slice"); it did not measure what the additions cost (1 to 2.5 hours of a 4 to 7 hour unit); when asked why work was slow it described the overhead as external; and its proposed fix added more of the same. Nothing in the machinery or in the agent forced a check against the rules, measured the cost per step, attributed the drift to the agent's own process changes, or stopped the agent from changing its own process.

## Intent, in one sentence

Self-awareness for the machinery is not judgment; it is four mechanisms: it measures its own cost per step against the estimate, it attributes a drift to its cause with its own process changes as a first-class cause, it cannot change how work runs without citing the rule or design stage the change instantiates (or asking a person), and it reports its own-caused cost to the person separately from external causes. The person holds the intent; the machine holds the measurements and the constraint.

## Wido's binding words

- "What you apply here should be the best solution. And if that proves to be true; that should be promoted to machinery" (10-06).
- "Why oh why are you adding this enormous overhead" and "are you lacking introspection and self awareness completely?" (10-07): a process that drifts without its operator noticing is the defect.
- Standing rules the drift broke: expensive test once per batch (09-19); impacted tests per slice, full at combine (09-19); fastest track, not a reactor (10-01); detectors share the defect they catch (09-20); NO HAL 9000 (10-03: the machine never overrules a person, and, read the other way, never rewrites its own rules without one).

## What is true today (read on 10-07; the design verifies each site on its tree)

1. The plan measures the process by hand: the practices table (`plans/machinery-open-findings-plan-2026-10-06.md`, "Practices tried by hand") names a measure per practice (hours per unit, corrections per unit, reds at integration, idle minutes) and nobody computes them; m1e computed hours per unit only when asked, from commit times.
2. Idle gaps between steps are a plan finding (finding 10: 59 gaps over ten minutes, 38 idle hours in four days) consumed by goal 4's driver; busy-but-wasted time (a suite run that finds nothing the integration gate would not) has no finding and no owner.
3. The stop rules (mechanism 2, `plans/designs/machinery-mechanisms.md`) measure the convergence of quality (material falls, red set shrinks, class repeat) and cap attempts; they do not measure cost, and they apply to units, rounds and lane loops, not to the steward's or coordinator's own interventions.
4. The cause vocabulary of a red (plan finding 1; `internal/landing/plain/cause.go`) is own, main, other, flake, environment, unclassified; "a change in how work is run" is not a cause anything can name.
5. How work runs is partly data (declarations `proof.cheap`, `proof.full`, `review.stop`, `landing.*`, read through `settings`) and partly text an agent edits freely (the COMMON block of the builder briefs, `plans/lane-policies-and-helm-u1-settings.md`, edited three times on 10-07, each time widening or narrowing every builder's check with no record of why and no measure attached). Goal 6 (briefs carry their rules: the scaffold) makes the brief composed; goal 2's declared-check makes the builder's check the declared one. Neither records or gates a change to the declaration itself.
6. The retro skill (`skills/retro`) mines records for patterns after the fact and proposes instruction changes for a person's veto; it runs on a cadence, not on a drift.
7. The budget box (finding 9: derived from the design's units and estimate) catches a goal over its allowance and raises or asks at 75 %; it does not say which step grew or why.

## What the design must deliver (units; re-cut allowed, intent not; each at most 250 production lines)

| Unit | Intent | Lines |
| --- | --- | ---: |
| U1 cost per step | Every unit round records the start and end of each step (build, check, read, correction, wait for a person, wait for a job) and the tokens the step reports, against the design's per-unit estimate; `goal status` and `unit status` show where the time went; the practices table's measures (hours per unit, corrections per unit, reds first seen at integration, suite minutes per unit) are derived from these records by one reader, never typed. | 200 |
| U2 a process change is an act | A change to how work runs (a `proof.*`, `review.*`, `landing.*` or `launch.*` declaration or setting; the brief scaffold's check block) is a recorded act carrying the rule or design stage it instantiates, the before and after, and the measure it expects to move; policy `process.change` (`auto` \| `person`, default `person`): under `person` an agent's change is a question with the one command and its impact; under `auto` a change with no citation is refused for an agent and never for a person. | 220 |
| U3 drift names its cause | A derived measure leaving its band (a unit over twice its estimate; reds first seen at integration above the goal's baseline; suite minutes per unit above the declared check's) raises a stop of loop `process` (mechanism 2) whose cause is classified like a red's: `process-change <act>` (the most recent recorded change since the measure was in band), `load`, `provider`, `design` (the estimate was wrong), `unclassified`; the printed first act for `process-change` is the revert of that act, so the remedy of first resort is removal, not addition. | 220 |
| U4 the steward's own class repeat | Every intervention by the steward or the coordinating agent on how work runs is classed (added a check, widened a check, added a round, changed a budget, changed a policy); a class repeating within the goal's window stops with a stop record and asks a person, the unit stop rule applied to the thing applying stop rules. | 150 |
| U5 own cost reported apart | The status a person reads (`goal status`, the fleet view, the message a seat sends) lists the process changes since the last report with the cost U1 measured for them, own-caused apart from external (load, provider limit, a red on main), and a drift stop names itself there before anything else. | 150 |

Build order: U1, U2, U3, U5, U4. Each unit: one build, at most two corrections, material must fall, a class repeat stops the unit. Each unit's acceptance has one test through a public verb.

## Not in this goal

Judgment about which process is best (the retro and the person); the driver (goal 4); the declared check itself (goal 2); the scaffold (goal 6); the budget derivation (goal-budget-follows-its-plan). This goal makes the process observable and its changes accountable; it decides nothing about the process.

## What would have caught 10-07

U1 shows a unit at 5 hours against 70 minutes with 2.5 hours in check steps. U3 raises a `process` stop with cause `process-change` naming the COMMON block edit of 12:23 and prints its revert. U2 refuses the 20:30 and 21:59 widenings for an agent (no rule cited; the cited rules say the opposite) and asks the person with the impact. U4 stops the third "added a check" intervention of the day. U5 puts "m1e widened every builder's check twice; cost 1 to 2.5 h per unit" at the top of the status Wido read at 17:14, which instead said "builders miss broad test failures".

## Questions the design may put to Wido

Only ones whose answer changes what is built. The default for `process.change` is proposed as `person`; the bands for U3 are proposed as twice the estimate and the goal's own baseline.

## Added 2026-10-07 23:30: the measures the day's levers need

U1's derived measures include, per unit and per goal: hours from build start to commit by step (build, attest, read, correction, wait); corrections and fix units per unit; reds first seen at the integration gate; merge conflicts at integration; suite minutes per run against the unloaded baseline; reruns per gate attributed to flakes; builds and suites refused or queued for load. Each lever in the plan's "Learned 2026-10-07" table names one of these as its measure; promotion of a lever follows the measure holding on two goals (Wido 10-06).
