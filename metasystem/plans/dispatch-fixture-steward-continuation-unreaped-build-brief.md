Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal dispatch-fixture-steward-continuation-unreaped)
Date: 2026-09-04

# Build brief: the steward-continuation scenario reaps before it heals

Goal `dispatch-fixture-steward-continuation-unreaped` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`scripts/agents/dispatch-fixtures.sh`, scenario `steward-continuation`, fails on plain main with: "steward heal-first: notifier outage produced no launch: launched=false reason=the world changed before launch: worker provably dead, but a continuation is already open and unreaped". The scenario's setup leaves a continuation open before the heal-first launch it asserts, so the steward lawfully refuses to launch a second one. Seen seat-side on m2 on 2026-09-04, with and without the fixture-suite drift fix.

## What to build

Read the scenario's setup against the steward's heal-first law (`internal/steward`, the heal-first launch and its "continuation already open" reason) and make the fixture match the law: either reap the earlier continuation before the notifier-outage leg or arrange the setup so no continuation is open when the heal-first launch is expected. Do not weaken the assertion that a launch happens; if the law says no launch is correct here, assert that refusal text instead and say so in the return.

## Verification

`bash -n` on the script. Your sandbox cannot run the suite (KI-15); the orchestrator runs `dispatch-fixtures.sh` seat-side. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `scripts/agents/dispatch-fixtures.sh` only; report a product defect rather than changing product code.
