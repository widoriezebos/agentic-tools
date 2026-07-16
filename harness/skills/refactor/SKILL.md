---
name: refactor
description: Restructure code without changing behavior. Covers readability work, extractions, moves, and de-duplication where the contract is that existing behavior is preserved. Use when the task is refactor-shaped or the user asks for cleanup or restructuring. Do not use for changes meant to alter behavior (that is implementation under the design docs) or for diagnosing failures (take-a-step-back).
---

# Refactor

In refactor work the outcome is structural and the proof is that behavior did not change. Refactors fail differently from feature work: the damage is quiet, spreads wide, and shows up late. These rules balance throughput against that risk.

## Contract and Trusted Baseline

- Name the acceptance gate before editing. The project's full proof that behavior is preserved (full suite, benchmark, golden run) is declared in `docs/project-rules.md`. Focused tests validate steps; only the gate accepts a candidate.
- The trusted baseline is the last commit that passed that gate. Record it with `scripts/refactor-baseline.sh record --gate "the gate command that passed"`, and commit the baseline file with the next checkpoint. Everything after the baseline is untrusted until the exact candidate HEAD passes the same gate.
- Before every new edit batch, run `scripts/refactor-baseline.sh check`. If it blocks (dirty worktree, diverged history, or the cadence backstop; defaults are owned by the script and tuned per project in `docs/project-rules.md`), stop editing. Gate the current candidate or return to the baseline. Do not argue with the check.

## Tests Before Restructuring

- Before moving or restructuring a behavior-heavy unit, its material behavior needs tests at production entrypoints: main path, material variable combinations, meaningful branches. After the refactor the same tests must pass unchanged. Needing to edit a test is a contract change to escalate, and no longer a refactor step.
- Inspect existing focused owner tests first; add tests only for material paths that are not already proven. If a valuable complex unit has weak coverage, coverage hardening is the first checkpoint: commit the tests, then refactor against them. Skip the unit only when it is low-value for the current objective.

## Risk-Sized Batches

- Default to throughput batches: a cohesive cluster sharing one owner or dependency layer. Several related classes or one coherent package slice, rather than a one-class loop. Choose units by behavioral risk and rollback cost, never by line or file count.
- Each unit in the cluster is one checkpoint commit that can be replayed on its own. The commit sequence is the replay queue if the cluster later fails.
- Shrink to one narrow unit at a time only for failure recovery, or when behavior risk or weak coverage demands it. Return to throughput batches once the failing unit is fixed, dropped, or parked.
- High-blast-radius surfaces (prompt and template text, serialization, ranking or scoring, budgets and limits, anything focused tests cannot fully prove) stay small enough to diagnose and revert as one unit, gate immediately, and are never batched into an ambiguous failure.

## Validation Ladder

Spend proof where the risk is:

1. Focused owner tests once at the cluster boundary; per checkpoint only when coverage is weak or a failure would be hard to attribute.
2. Compile or typecheck when signatures, package wiring, or generated sources change.
3. The full acceptance gate at cluster acceptance, for behavior-risk units, and when the cadence backstop is due. Never run it merely because a batch felt small. Order the gate so the fastest signal that can reject the candidate runs before the expensive remainder.

The gate accepts or rejects the exact candidate HEAD rather than the individual commits inside it. On acceptance, re-record the trusted baseline.

## Failure Handling

- When the gate rejects a candidate: at most three focused fix attempts. Then record what failed in the owning plan or handoff note, revert the whole unit, and continue with the next independent unit. Never stack more refactor work on a failed candidate.
- When a cluster fails: return to the trusted baseline and replay its checkpoint commits one at a time until the failing unit is isolated, fixed, dropped, or parked.
- Proof integrity: stale runs, partial runs, dirty-worktree runs, historical labels, and artifacts the gate did not produce prove nothing about the current HEAD.

Behavior-heavy refactor units track proof with the obligation matrix (`docs/design/design-obligation-gate.md`) in the owning plan. Low-risk clusters need no ledger: the checkpoint commits plus the focused test commands are the record.
