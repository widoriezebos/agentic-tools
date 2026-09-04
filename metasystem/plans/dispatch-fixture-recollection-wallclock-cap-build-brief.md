Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal dispatch-fixture-recollection-wallclock-cap)
Date: 2026-09-04

# Build brief: the recollection leg counts passes, not seconds

Goal `dispatch-fixture-recollection-wallclock-cap` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`scripts/agents/dispatch-fixtures.sh`, scenario `dispatch`, fails under load at "recollection did not conclude the delivered-then-lost critic (elapsed: 40s; scaled cap: 40s)": the leg waits on wall-clock for the steward's recollection to conclude a critic whose delivery was lost, against the repository's patience rule (attempts counted in observed passes, wall-clock only as a silence failsafe; see the census-wait rewrite earlier in the same script for the pattern).

## What to build

Rewrite the leg's wait to count observed recollection passes (the steward's census `scanSeq` or the recollection's own record advancing), with a bound in passes and a silence-only failsafe of thirty intervals, never a fixed forty seconds. Keep the assertion that the critic is concluded. Look for any other `scaled cap` wait in the dispatch scenario and convert it the same way in this round.

## Verification

`bash -n` on the script. Your sandbox cannot run the suite (KI-15); the orchestrator runs `dispatch-fixtures.sh` seat-side. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `scripts/agents/dispatch-fixtures.sh` only.
