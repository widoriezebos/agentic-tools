---
name: improve
description: Chase a measured improvement goal against a benchmark or evaluation, raising a primary metric without regressing guard metrics. Use when the task is to improve something an on-demand evaluation can measure (benchmark scores, quality evals, latency or cost budgets). Do not use when no runnable evaluation exists (building one is then the first deliverable), for behavior-preserving restructuring (refactor), or for diagnosing failures (take-a-step-back).
---

# Improve

Benchmark-driven improvement is offense, not repair: a planned experiment loop against a measured goal. Its failure modes are chasing noise, losing the best-known state, overfitting the evaluation, and unfalsifiable multi-change experiments.

## Improvement Contract

Before the first run, write in the owning plan:

- The primary metric and the exact evaluation command, as declared in `docs/project-rules.md`.
- The baseline score, the noise floor (minimum meaningful delta), and the target or stop score.
- Guard metrics that must not regress, with their floors.
- The budget — runs, cost, wall-clock — and the non-goals.

If the evaluation cannot be run on demand, the first deliverable is the evaluation, not the improvement; renegotiate the goal with the human.

## Frontier Ledger

- The frontier is the best-known state: exact SHA, score, evaluation command, and run artifact. Manage it with `scripts/frontier.sh` (record, challenge, status).
- Record the baseline frontier before the first experiment. Where the evaluation's output is machine-parseable, wrap it in a project script that runs the eval and calls `scripts/frontier.sh record` with the parsed score — declaration becomes mechanical instead of remembered.
- When a run beats the frontier (`scripts/frontier.sh challenge` passes — more than the noise floor above the recorded score), stop other work and preserve that exact state first: commit, re-record the frontier, and follow the project's push/tag policy. Only then iterate further.
- A score without provenance — SHA, configuration, run artifact — is not a frontier. Never update the ledger from memory, a partial run, or a stale artifact.

## Experiment Cycles

- One mechanism per experiment. Pre-register the hypothesis, the expected signal size relative to the noise floor, and the cheapest signal able to reject it. Use the cycle contract and result classifications from `skills/take-a-step-back/SKILL.md` verbatim.
- Cheapest rejection first: run the canary or subset evaluation before the full suite whenever the project's evaluation supports it.
- A delta within the noise floor is noise, not progress; repeat the run or increase the effect before believing it. A guard-metric regression is never averaged away by a primary-metric gain.
- After a falsified experiment, revert the behavior and keep the learning in the plan. Never stack experiments on unreverted falsified changes.

## Anti-Overfitting

- The evaluation is a proxy; the contract is the user or production outcome. A gain needs a mechanism you can explain — an unexplainable gain is treated as noise or overfitting until independently reproduced.
- Do not tune against the same fixed evaluation cases indefinitely; rotate, extend, or hold out cases per the project's evaluation policy in `docs/project-rules.md`.
- Never modify the evaluation and the system under test in one change. An evaluation change re-baselines the frontier (`scripts/frontier.sh record --force`, with the reason recorded in the owning plan).

## Stop Conditions

Stop and report when any applies: the target is reached; the budget is exhausted; three consecutive experiments fail to beat the frontier beyond the noise floor; or guard metrics keep regressing. These conditions are additional to the take-a-step-back stop-loss — whichever stops earlier wins, and a `falsified-continue` classification does not extend the three-experiment limit. Hand over the preserved frontier, the experiment ledger with classifications, the exhausted mechanisms, and the next higher-level decision. Whatever the outcome, the best-known state must be exactly recoverable at the end.
