Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 17: the minting checkout's engine reaches the stub through PATH

Chain phd-build1-20260910. Round 16 was right to stop: the control-root
variable is set only for the proof-suite child, and the pinned old
engine of the cutover scenario cannot be taught anything new anyway.
The seat checked the one channel both engines keep: PATH survives the
old engine's testingEnvironment allow-list (commit 6bc19ba1c, test.go
line 1117) and its candidateEngineBuildEnvironment filter, and the
fixture owns the receipt's environment (receipt_env_run is `env -i`
over receipt_environment). Report `round` as 17. Do not fetch or rebase
the worktree.

## Decision D-R15-1, restated once more

In the land fixtures a checkout's candidate engine is the engine
installed in that checkout, delivered through PATH: take_fixture_receipt
runs its `landing test-receipt` with the minting checkout's `bin`
directory prepended to the PATH entry of the receipt environment for
that call only; the seed's go-build stub copies `$(command -v
metasystem)` into the requested `--out` path (a real copy, executable)
and fails loudly when no `metasystem` is on PATH. The stub's bytes stay
identical in every clone, so the cutover scenario's two candidate trees
stay equal; leg_local (old engine) builds the old engine as its
candidate, leg_peer (new engine) builds the new one, and the other
three workspace-receipt legs resolve to the same installed engine as
before. No Go code changes; the pin at 6bc19ba1c stays.

## Mandate for this round

1. Replace the round-16 stub body per the restated decision (keep the
   `--trimpath --out` argument check) and prepend `$checkout/bin` to
   PATH in take_fixture_receipt for that receipt's environment.
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
