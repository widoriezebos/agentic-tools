Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 15: the receipt-cutover land fixture builds the old engine's candidate from the new engine

Chain phd-build1-20260910. Round 14 passes the landing gate's fast
group, both packages plain and under the race detector, and the Linux
cross-compile. The landing receipt for tree
9162284fd003a5ad33f9f240d5af52585616c7bc then failed exactly one of
its forty-two groups: section/land-fixtures, scenario receipt-cutover,
with

    metasystem test worker: protected public-v1 probe emit-zero-tests failed: incomplete negative result=... err=json: unknown field "progressRule"

Cause, read from metasystem/scripts/agents/land-fixtures.sh: scenario 13
builds a pinned pre-cutover engine from commit 6bc19ba1c and installs it
in leg_local as the enrolled engine, so that "the pre-cutover engine is
real and keeps its own receipt intact". But the seed's
scripts/agents/go-build.sh stub (written in make_leg for workspace
receipt scenarios) execs a path baked at seed time,
`$leg_root/engine`, which is the NEW candidate engine built from the
tree under test. So the old engine's receipt in leg_local runs the new
engine as its test worker for the protected probe and its strict
reader (older than the tolerant probe reader landed today in
673f62f51) refuses the group record's new fields. The old engine's
receipt was never an exact old-engine receipt; it only looked like one
while the record shape stood still. Report `round` as 15. Do not fetch
or rebase the worktree.

## Decision D-R15-1 (the designer's rule; it amends revision 3)

In the land fixtures, a checkout's candidate engine is the engine
installed in that checkout: the seed's go-build stub execs the
checkout's own bin/metasystem, resolved from the stub's location
(`$(cd "$(dirname "$0")/../.." && pwd -P)/bin/metasystem`), never a
path baked at seed time. The stub's bytes are then identical in every
clone, so the cutover scenario's two candidate trees stay equal, while
leg_local (old engine) builds the old engine as its candidate and
leg_peer (new engine) builds the new one. The old engine's receipt is
an exact old-engine receipt, as the scenario says; the peer's receipt
carries the newer shape; the cross-reading rejection the scenario
proves is untouched. The pin at 6bc19ba1c stays.

## Mandate for this round

1. Implement D-R15-1 in metasystem/scripts/agents/land-fixtures.sh: the
   stub written at make_leg (the heredoc that prints `exec
   $candidate_fixture_engine_q`) execs the checkout-relative engine;
   remove the baked path if nothing else reads it. Keep every scenario
   that installs `$leg_root/engine` into the seed or a clone as it is:
   those checkouts resolve to the same engine as before.
2. Nothing else changes.

## Proof

bash scripts/agents/go-gate.sh --fast, green; bash
scripts/agents/land-fixtures.sh (all thirteen legs) from the metasystem
directory of your worktree with the candidate engine built into its
bin/metasystem (bash scripts/agents/go-build.sh --out bin/metasystem;
bin/ is ignored), green, and paste the receipt-cutover leg's lines as
evidence; if the sandbox cannot run the bed, say so and the seat runs
it. Report the round as your own.

## Constraints

Wall-clock budget: 25 minutes. Return per the implementer schema. Never
delete written work.
