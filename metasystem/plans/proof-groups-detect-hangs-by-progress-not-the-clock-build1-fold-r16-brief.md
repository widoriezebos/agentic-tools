Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 16: the fixture stub copies the minting checkout's engine

Chain phd-build1-20260910. Round 15 was right to stop: the stub runs
inside the materialized candidate worktree, which carries no
bin/metasystem and is removed before the emitted wrapper runs, so a
path resolved from the stub's location is dead on arrival. The seat
read the engine: the candidate build's environment
(cmd/metasystem/test.go, inheritedTestingEnvironment then
candidateEngineBuildEnvironment) keeps METASYSTEM_PROOF_CONTROL_ROOT,
which the receipt launcher sets to the checkout minting the receipt.
Report `round` as 16. Do not fetch or rebase the worktree.

## Decision D-R15-1, restated

In the land fixtures a checkout's candidate engine is the engine
installed in that checkout: the seed's go-build stub copies
`$METASYSTEM_PROOF_CONTROL_ROOT/bin/metasystem` into the requested
`--out` path (a real copy, executable, not an exec wrapper), and fails
loudly, naming the variable or the missing file, when either is absent.
The stub's bytes are identical in every clone, so the cutover
scenario's two candidate trees stay equal; leg_local (old engine) builds
the old engine as its candidate, leg_peer (new engine) builds the new
one, and every other workspace-receipt scenario resolves to the same
installed engine as before. The pin at 6bc19ba1c stays.

## Mandate for this round

1. Replace the round-15 stub body with the copy per the restated
   decision; keep the `--trimpath --out` argument check. Do not change
   Go code.
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
