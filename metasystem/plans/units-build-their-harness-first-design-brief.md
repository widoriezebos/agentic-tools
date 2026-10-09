# Design brief: units-build-their-harness-first

Working Mode: Design
Proposed goal, opened 2026-10-09 by Wido via m1e. Source: Wido's conversation of 2026-10-08 with a fellow engineer about delegating to agents, read against the metasystem by m1e on 2026-10-09. The transcript is not in the repository; this page carries everything the design needs.

## Intent, in one sentence

A unit with a runnable surface names the command that exercises its change as its first task, the builder writes that harness before the change and runs its experiments through it, and the Proven walk of the review cites the same command, so the builder's feedback loop and the reviewer's proof are one entry point rather than two stories.

## Why

The engineer in the conversation described the delegation that worked best for him: before anything else he has the agent write a small harness that builds the feedback loop (build the image, run it, read the error), and then a context-free sub-agent runs experiment after experiment through it. He wanted to formalise it and had not. The metasystem has the idea in two places and in neither as a rule for building. The improve skill makes the evaluation the first deliverable when none exists (`skills/improve/SKILL.md:24`). The verify skill asks for the smallest real entrypoint that exercises the change and for the exact command and observed output (`skills/verify/SKILL.md`, "Choose the Surface"). Design pages carry a "Verification and box" section that names tests, not the one command a person could run. Build briefs do not ask for a harness, so a builder proves by tests and the reviewer reconstructs how to run the thing.

## What is true today (read 2026-10-09; the design verifies every site)

1. Build briefs are composed from a scaffold whose rules the goal `briefs-carry-their-rules` (approved, in design) makes declared rather than free text; a unit's check is the declared check (`work build --check`).
2. The verify skill is loaded at completion for any runnable surface; it is advice to the builder, not a field in the brief or the record.
3. The review room's Proven walk explains "how it was proven: the tests and the proof recorded for it, what each one holds, and what they all assume" (`internal/ui/partner/review.go:57`); it reads records, and no record holds a run command.
4. Fixture beds run diagnostic single-group runs through the engine (Wido's standing practice); they are the metasystem's own harnesses and are not named per unit.
5. The design page template has per-unit tables (unit, intent, lines) and a verification section per goal.

## What done looks like

- The design's unit row and the composed build brief carry one field, the harness: the command (or short script under the goal's evidence path) that builds what the unit changes and exercises the changed behaviour with realistic input, printing what a person would look at. For a unit with no runnable surface the field says so in words, and that is a valid answer.
- The builder's first commit on a unit is the harness, runnable before the change exists, so it can show the before state; the unit's check runs it after every correction, and its output is kept with the unit's evidence.
- The code read and the Proven walk cite the harness command and its recorded output; a reviewer can run the same command in the review room's Try it step.
- The scaffold rule is declared, not prose: the brief composer refuses a unit whose harness field is empty with the one sentence that says what to write.

## Measure

Share of units landed with a harness field and a recorded run, and the reviewer's wait: minutes between opening a review and the first run of the change. Proven when one goal lands with every unit carrying its harness and the Proven walk cites it.

## Constraints and freedoms

- The harness is the smallest real entrypoint, not a test suite and not a second test framework; a shell line or a Go test name with its args is enough.
- Scrooge on testing (Wido, 2026-10-07): the harness replaces nothing in the unit's check; it names the one run that shows the behaviour.
- Small: at most three units of at most 250 production lines; one gate. Step one is the field in the design row and the brief, with the composer's refusal.
- Free: the field's name, where the output is kept, how the review room offers the run.

## Not in this goal

Changing the unit check's test selection; the fixture beds themselves; the verify skill's content beyond pointing at the field.

## Neighbours to read first

`briefs-carry-their-rules` (the scaffold this rule lands in; this goal waits for it), `round-proof-feeds-the-next-brief`, `review-findings-read-as-decisions` (done: Try it), `goal-completion-verifies-observed-intent`.
