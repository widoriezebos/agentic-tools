Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-fixture-census-lifecycle-red)
Date: 2026-09-04

# Build brief: the census-lifecycle and idle-hook scenarios pass on a Mac

Goal `supervision-fixture-census-lifecycle-red` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`scripts/agents/supervision-fixtures.sh`: scenario `census-lifecycle` is red on plain main (m2, 2026-09-04): its enumerate-filter-resolve leg passes and a later leg fails; the run's evidence sits under the suite's `suite-failures` directory of that run (the orchestrator will paste the failing lines into the follow-up if you cannot see them). Scenario `idle-hook` was red on one baseline run and green on the next, so it is timing-sensitive.

## What to build

Read both scenarios and their failure evidence. For census-lifecycle, fix the cause where it lives: a fixture expectation that no longer matches the product's lawful behaviour is rewritten to assert the current behaviour; a product defect is reported, not patched around. For idle-hook, replace any wall-clock wait with patience counted in observed events (census passes or hook invocations), keeping a silence-only failsafe, so the scenario is deterministic on a loaded Mac.

## Verification

`bash -n` on the script. Your sandbox cannot run the suite (KI-15); the orchestrator runs `supervision-fixtures.sh` twice seat-side. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `scripts/agents/supervision-fixtures.sh` only; report a product defect rather than changing product code.
