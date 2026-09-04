Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-fixture-answer-history-drift)
Date: 2026-09-04

# Build brief: the channel fixture asserts the history the product now writes

Goal `channel-fixture-answer-history-drift` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`scripts/agents/channel-fixtures.sh` fails silently (bare exit 1 under `set -e`, no message) at `grep -q 'answer actor=human:wido'` on the goal file's history after the fixture's human answers a budget-above-norm question with the token and a valid code. Since the status-post binding (4bdaa5ec) and the budget-answer landing (3615da7a), such an answer re-approves the goal as a verified channel answer, and the history lines written are not the one the fixture expects.

## What to build

Run the fixture's answer leg in your head against `internal/channel/poll.go` and `internal/goal/verbs.go` (the `Answer` and approval paths) and make the assertion match what is written now: the answer event with its actor, and the approval event with outcome `VERIFIED_CHANNEL_ANSWER`. Replace the bare `grep -q` with a check that prints the history and a one-line reason on failure. Do the same for the Telegram leg of the fixture if it asserts the same line.

## Verification

`bash -n` on the script. Your sandbox cannot run the suite (KI-15); the orchestrator runs `channel-fixtures.sh` seat-side. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `scripts/agents/channel-fixtures.sh` only.
