Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 18: the checkout's PATH is the same for the receipt's producer and its consumers

Chain phd-build1-20260910. Round 17 showed the PATH channel reaches the
right engine and named the last contradiction exactly: PATH is a
selected testing input, so a receipt minted with the checkout's bin
first on PATH is rejected later by run_fixture_landing, which verifies
it under the original environment. Your proposed resolution is the
decision. Report `round` as 18. Do not fetch or rebase the worktree.

## Decision D-R15-1, final form

In the land fixtures, every engine invocation the fixture makes for a
checkout that mints, lands, or re-verifies a receipt runs with that
checkout's `bin` directory prepended to the PATH entry of the receipt
environment: take_fixture_receipt, run_fixture_landing, and any other
consumer of that receipt (the peer's receipt in the cutover scenario,
the cross-reading step) use the same per-checkout PATH, so the proof's
producer and consumer environments are identical. Implement it once as
a helper that takes the checkout and derives the environment, and call
it from every such site; the seed's go-build stub from round 17 (copy
of `$(command -v metasystem)`) stays. No Go code changes; the pin at
6bc19ba1c stays.

## Mandate for this round

1. Implement the final form above.
2. Nothing else changes.

## Proof

bash scripts/agents/go-gate.sh --fast, green; bash
scripts/agents/land-fixtures.sh (all thirteen legs) from your
worktree's metasystem directory with the candidate engine built into
its bin/metasystem, green; paste the receipt-cutover leg's lines. If
the sandbox cannot run the bed, say so and the seat runs it. Report the
round as your own.

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema. Never
delete written work.
