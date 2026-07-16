---
name: refactor
description: Restructure code without changing behavior, readability work, extractions, moves, and de-duplication where the contract is that existing behavior is preserved. Use when the task is refactor-shaped or the user asks for cleanup or restructuring. Do not use for changes meant to alter behavior (that is implementation under the design docs) or for diagnosing failures (take-a-step-back).
---

# Refactor

Refactor work is a distinct contract: the outcome is structural, and the proof is that behavior did not change. Refactors fail differently from features — silently, broadly, and late — so throughput and safety are balanced by discipline, not by caution alone.

## Contract and Trusted Baseline

- Name the acceptance gate before editing: the project's full proof that behavior is preserved (full suite, benchmark, golden run) is declared in `docs/project-rules.md`. Focused tests validate steps; only the gate accepts a candidate.
- The trusted baseline is the last commit that passed that gate. Record it with `scripts/refactor-baseline.sh record --gate "the gate command that passed"`, and commit the baseline file with the next checkpoint. Everything after the baseline is untrusted until the exact candidate HEAD passes the same gate.
- Before every new edit batch, run `scripts/refactor-baseline.sh check`. If it blocks — dirty worktree, diverged history, or the cadence backstop (defaults owned by the script; tuned per project in `docs/project-rules.md`) — stop editing: gate the current candidate or return to the baseline. Do not argue with the check.

## Tests Before Restructuring

- Before moving or restructuring a behavior-heavy unit, its material behavior needs tests at production entrypoints: main path, material variable combinations, meaningful branches. After the refactor the same tests must pass unchanged; needing to edit a test is a contract change to escalate, not a refactor step.
- Inspect existing focused owner tests first; add tests only for material paths not already proven. If a valuable complex unit has weak coverage, coverage hardening is the first checkpoint: commit the tests, then refactor against them. Skip the unit only when it is low-value for the current objective.

## Risk-Sized Batches

- Default to throughput batches: a cohesive cluster sharing one owner or dependency layer — several related classes or one coherent package slice, not a one-class loop. Choose units by behavioral risk and rollback cost, never by line or file count.
- Each unit in the cluster is one checkpoint commit, independently replayable. The commit sequence is the replay queue if the cluster later fails.
- Shrink to one narrow unit at a time only for failure recovery, or when behavior risk or weak coverage demands it; return to throughput batches once the failing unit is fixed, dropped, or parked.
- High-blast-radius surfaces — prompt/template text, serialization, ranking or scoring, budgets and limits, anything focused tests cannot fully prove — stay small enough to diagnose and revert as one unit, gate immediately, and are never batched into an ambiguous failure.

## Validation Ladder

Spend proof where the risk is:

1. Focused owner tests once at the cluster boundary; per checkpoint only when coverage is weak or a failure would be hard to attribute.
2. Compile/typecheck when signatures, package wiring, or generated sources change.
3. The full acceptance gate at cluster acceptance, for behavior-risk units, and when the cadence backstop is due — never merely because a batch felt small. Order the gate cheapest-rejection-first: run the fastest signal that can reject the candidate before spending the expensive remainder.

The gate accepts or rejects the exact candidate HEAD, not the commits inside it. On acceptance, re-record the trusted baseline.

## Failure Handling

- When the gate rejects a candidate: at most three focused fix attempts. Then record what failed in the owning plan or handoff note, revert the whole unit, and continue with the next independent unit. Never stack more refactor work on a failed candidate.
- When a cluster fails: return to the trusted baseline and replay its checkpoint commits one at a time until the failing unit is isolated, fixed, dropped, or parked.
- Proof integrity: stale runs, partial runs, dirty-worktree runs, historical labels, and artifacts the gate did not produce prove nothing about the current HEAD.

Behavior-heavy refactor units track proof with the obligation matrix (`docs/design/design-obligation-gate.md`) in the owning plan. Low-risk clusters need no ledger: the checkpoint commits plus the focused test commands are the record.
