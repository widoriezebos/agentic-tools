---
name: improve
description: Chase a measured improvement goal against a benchmark or evaluation, raising a primary metric without regressing guard metrics. Use when the task is to improve something an on-demand evaluation can measure (benchmark scores, quality evals, latency or cost budgets). Do not use when no runnable evaluation exists (building one is then the first deliverable), for behavior-preserving restructuring (refactor), or for diagnosing failures (take-a-step-back).
---

# Improve

Improvement work is planned experimentation against a measured goal. Its failure modes are chasing noise, losing the best-known state, overfitting the evaluation, and experiments that change too much to teach anything.

## Improvement Contract

Before the first run, write in the owning plan:

- The primary metric and the exact evaluation command, as declared in `docs/project-rules.md`.
- The baseline score, the noise floor (minimum meaningful delta), and the target or stop score.
- Guard metrics that must not regress, with their floors.
- The budget (runs, cost, wall-clock) and the non-goals.

If the evaluation cannot be run on demand, the first deliverable is the evaluation itself. Renegotiate the goal with the human.

## Frontier Ledger

- The frontier is the best-known state: exact SHA, score, evaluation command, and run artifact. Manage it with `scripts/frontier.sh` (record, challenge, status).
- Record the baseline frontier before the first experiment. Where the evaluation's output is machine-parseable, wrap it in a project script that runs the eval and calls `scripts/frontier.sh record` with the parsed score, so declaration is mechanical instead of remembered.
- When a run beats the frontier (`scripts/frontier.sh challenge` passes: more than the noise floor above the recorded score), stop other work and preserve that exact state first. Commit, re-record the frontier, and follow the project's push and tag policy. Only then iterate further.
- A score without provenance (SHA, configuration, run artifact) is not a frontier. Never update the ledger from memory, a partial run, or a stale artifact.

## Experiment Cycles

- One mechanism per experiment. Pre-register the hypothesis, the expected signal size relative to the noise floor, and the cheapest signal able to reject it. Use the cycle contract and result classifications from `skills/take-a-step-back/SKILL.md` verbatim.
- Cheapest rejection first: run the canary or subset evaluation before the full suite whenever the project's evaluation supports it.
- A delta within the noise floor is noise. Do not count it as progress; repeat the run or increase the effect before believing it. A guard-metric regression is never averaged away by a primary-metric gain.
- After a falsified experiment, revert the behavior and keep the learning in the plan. Never stack experiments on unreverted falsified changes.

## Evaluation Types

The loop in this skill is the same for every optimization problem. What changes per problem type is how the evaluation must be built. Declare the type together with the evaluation in `docs/project-rules.md`.

- **Deterministic search** (design choices, algorithms, architectures validated by tests and benchmarks). The evaluation is the test and benchmark suite, and the noise floor is small. Explore candidates as separate branches or worktrees, one design decision per experiment, and let frontier challenges prune the losers.
- **Stochastic systems** (randomized load, chaos scenarios, sampled inputs). A single run proves nothing. The evaluation command must aggregate over enough seeded scenarios to produce a stable statistic. Measure the noise floor by re-running the unchanged baseline several times, and set guard metrics on the tail (worst case, high percentiles), never only on the average.
- **Hidden information** (security, recommendations, anything where real feedback is partial). Build the on-demand evaluation as a simulator or a replay of recorded traces, and add a standing guard: reconcile simulator results against live outcomes on a declared cadence. When simulator and reality diverge, stop optimizing; the evaluation is broken, and fixing it re-baselines the frontier.
- **Dynamic systems** (live traffic, changing conditions, evolving targets). Frontier scores expire when the environment shifts. Declare the measurement window when recording the baseline (`scripts/frontier.sh record --max-age-minutes M`); after that, `challenge` refuses to compare against an expired frontier, and continuing requires an explicit re-baseline (`record --force`) with the reason in the owning plan. Use offline or replay evaluation as the fast inner loop and live measurement as the slow outer confirmation. Anything that touches production runs under the reserved decisions in `docs/project-rules.md`, never as an unsupervised experiment loop.

## Anti-Overfitting

- The evaluation is a proxy; the contract is the user or production outcome. A gain needs a mechanism you can explain. An unexplainable gain is treated as noise or overfitting until independently reproduced.
- Do not tune against the same fixed evaluation cases indefinitely; rotate, extend, or hold out cases per the project's evaluation policy in `docs/project-rules.md`.
- Never modify the evaluation and the system under test in one change. An evaluation change re-baselines the frontier (`scripts/frontier.sh record --force`, with the reason recorded in the owning plan).

## Stop Conditions

Stop and report when any of these applies: the target is reached; the budget is exhausted; three consecutive experiments fail to beat the frontier beyond the noise floor; or guard metrics keep regressing. These conditions apply in addition to the take-a-step-back stop-loss. Whichever stops earlier wins, and a `falsified-continue` classification does not extend the three-experiment limit. Hand over the preserved frontier, the experiment ledger with classifications, the exhausted mechanisms, and the recommended next decision. Whatever the outcome, the best-known state must be exactly recoverable at the end.
