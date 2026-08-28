---
name: take-a-step-back
description: Diagnose and redesign work that is stuck, repeatedly failing, expensive, drifting, or accumulating abstractions without a current consumer. Use when the user asks to step back; patches or runs are not converging; debugger, benchmark, or model-heavy work needs stop-loss; progress is unclear; or a proposed support layer may be unnecessary.
---

# Take a Step Back

Choose `analysis-only`, `design-only`, or `implement` from the user's request. A stop-loss trigger overrides implement mode.

## Freeze the Frame

Record the exact symptom, impact, reproduction, repository SHA and local changes, artifact/source parity, inputs/config/model, existing run ids and artifacts, success criteria, non-goals, prior attempts, and a wall-clock/run/cost budget.

Check existing plans, ledgers, logs, tests, and artifacts before creating theories. Do not re-prove a diagnosis already supported.

## Challenge the Premise

Before adding an abstraction or support layer, name:

- the current production caller and outcome;
- the existing owner;
- direct change, deletion, reuse, operational mitigation, and no-change alternatives;
- the first end-to-end slice and code replaced now;
- total lifecycle complexity: production code, state, migration, recovery, tests, monitoring, and operations.

No current consumer means stop or park unless the user explicitly requested an extension surface.

## Build Falsifiable Theories

Keep two to four theories. For each, record supporting and contradicting evidence, decisive missing evidence, cheapest check, and expected owner if true. Separate anchor symptoms from consequences. Rank explanatory fit before convenience.

Trace the owning boundary before patching a broken invariant, transition, selection/closure gate, or ownership handoff. Name the owner, required facts, decision, and focused invariant test.

## Contract Every Cycle

Before a patch, test, debug run, benchmark, or model-heavy run, write:

```markdown
### Cycle <id>
- Mechanism/question:
- Novel decision-relevant fact:
- Command/artifact/files:
- Contract signal for progress:
- Budget and stop condition:
- Recoverable checkpoint:
- Expected classification and next action:
```

Do not run a cycle that only promises cleaner logs, broader coverage, nicer output, or a duplicate fact.

Immediately classify the result:

- `contract-improved`: the user contract or named invariant became true.
- `falsified-continue`: a leading theory is ruled out and a new viable owner/mechanism is identified.
- `falsified-dead-end`: the mechanism is exhausted or no owner has the required facts; stop.
- `no-progress`: neither contract nor theory set improved. One such result blocks another attempt in the same mechanism; a second no-progress cycle anywhere triggers stop-loss.
- `unresolved`: a valid measured result whose delta lies inside a declared noise floor; the run executed and produced an interpretable measurement that neither confirms nor refutes. Illegitimate without a declared noise floor; a run with no interpretable measurement is `no-progress`. Never counts toward the no-progress trigger.
- `invalid-run`: parity/environment/timeout prevents interpretation; repair validity, not behavior.

Only `contract-improved` and `falsified-continue` authorize another cycle without user direction. An `unresolved` result authorizes one only while a declared `- No-gain budget: N` ledger line is unexhausted: `scripts/assert-stop-loss.sh` blocks once N trailing cycles pass without a `contract-improved`.

One dead end deserves its own name: a capability ceiling, where the model, tool, or dependency cannot do what the mechanism requires. Prompt and parameter variations cannot cross it; stop iterating the moment evidence isolates one. Record it in `memory/known-issues.md` with the evidence, the realistic cost when it bites, and the named escalation lever (a stronger resource tier, an upstream fix, a design change); the lever is usually a reserved decision in `docs/project-rules.md`. Do not retry without new evidence. When the same issue bites again, append the occurrence to its entry and raise its priority instead of reopening the investigation.

## Stop-Loss

Record every cycle and its classification in the ledger, and run `scripts/assert-stop-loss.sh --file <ledger>` before contracting a new cycle. It blocks on the machine-checkable triggers below (a dead end, two no-progress cycles, the declared cycle budget); the remaining triggers stay your judgment. Do not argue with the check.

Stop when any applies:

- two no-progress cycles;
- two failed attempts in one mechanism family;
- one expensive no-progress run;
- one falsified dead end;
- two preparatory checkpoints without a production-contract change;
- no novel fact or budget exhausted.

State `STOP-LOSS TRIGGERED`, preserve learning, remove or stash failed behavior, list exhausted mechanisms, and report the next higher-level decision. Do not evade the stop by renaming the same mechanism.

## Preserve Progress

Before expensive proof, make the exact state recoverable. After contract improvement, checkpoint before more edits. After falsification, preserve useful learning separately from behavior that should be reverted. A materially best-known frontier must be preserved exactly before further change; the frontier ledger and its rules are owned by `skills/improve/SKILL.md` and `scripts/frontier.sh`.

## Design and Proof

Write a short learning memo: supported diagnosis, rejected mechanisms, required owner/facts/invariant, and constraints the design must preserve. Apply the repository design principles and obligation gate. Implement only evidence-backed decisions, run focused owner tests first, and inspect primary runtime artifacts before rerunning.

Use [references/investigation-ledger.md](references/investigation-ledger.md) for the task ledger shape. A filled example lives at `docs/examples/step-back-ledger.md` from the metasystem root.
