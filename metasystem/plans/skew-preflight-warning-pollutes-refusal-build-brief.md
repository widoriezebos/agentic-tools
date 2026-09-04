Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal skew-preflight-warning-pollutes-refusal)
Date: 2026-09-04

# Build brief: the engine-skew preflight is silent unless it refuses

Goal `skew-preflight-warning-pollutes-refusal` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`scripts/agents/dispatch.sh`, function `engine_script_skew_preflight` (landed this morning as 269e4cdb), prints a warning to stderr whenever it allows a dispatch without a comparison: no build stamp, a `dev` stamp, or a stamp commit unknown to the repository (every fixture repository, since the stamp is a checkout commit). The delegate verb folds the dispatcher's stderr into the refusal detail, so the dispatch fixture suite's permission-envelope leg (`scripts/agents/dispatch-fixtures.sh` line 2083, which expects the detail `permission roots must be arrays` exactly) fails, and any other leg comparing a refusal detail exactly would too.

## What to build

The preflight writes nothing to stderr when it allows: drop the four warning lines (unknown stamp, dev stamp, missing stamp, comparison failure) or route them to the dispatcher's own log file if one exists at that point in the script; the refusal path keeps its full message naming both commits and the remedy. Add one assertion to the dispatch fixture's skew leg that a dispatch allowed on an unknown stamp produces no stderr line from the preflight.

## Verification

`bash -n` on both scripts; run the preflight function directly with an unknown stamp and show empty stderr and status 0, and with a stale known stamp show the refusal. Your sandbox cannot run the suite (KI-15); the orchestrator runs `dispatch-fixtures.sh` seat-side and expects the permission-envelope leg to pass. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `scripts/agents/dispatch.sh` and `scripts/agents/dispatch-fixtures.sh` only.
