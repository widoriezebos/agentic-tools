Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal dispatch-fixture-critic-close-register-fold)
Date: 2026-09-04

# Build brief: the dispatch scenario's critic-close leg follows the close law

Goal `dispatch-fixture-critic-close-register-fold` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`scripts/agents/dispatch-fixtures.sh`, scenario `dispatch`, once past its permission-envelope leg, fails at a chain-close leg with "cannot close a critic chain whose register is folded through round 1 while terminal round 2 exists" (`internal/dispatch/close.go` line 59). The material-stop landing (78018dd5) made close refuse a critic chain whose finding register was not advanced to its terminal round; the fixture's leg closes such a chain without advancing the register first. Seen seat-side on m2 on 2026-09-04 at 14:03Z; the leg was latent behind five earlier reds.

## What to build

Read the leg (search the script for the close call that follows a two-round critic chain) and decide from its intent: if it tests that a chain closes, advance the register first the way a seat does (`metasystem job critique-register-advance --root-job <root> --round-job <round>`) and then close; if it tests the refusal, assert the exact refusal text and continue. Then read on through the scenario for the next leg that would fail for the same reason and fix it too. Do not delete assertions.

## Verification

`bash -n` on the script. Your sandbox cannot run the suite (KI-15); the orchestrator runs `dispatch-fixtures.sh` seat-side and reports the next red, if any, back to you. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `scripts/agents/dispatch-fixtures.sh` only.
