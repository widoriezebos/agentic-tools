Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Review brief: the second rostered read, after the fold of WVB-40, WVB-41 and WVB-42

FINDING IDS: chain-unique; WVB-40 to WVB-47 are taken. A finding of
round 1 that the fold resolved is returned under its OWN identifier with
`material: false` (that is how the register closes it); one that still
stands keeps its identifier with `material: true`; a new defect gets
WVB-48 onward. Never report a resolution as prose inside another finding.

Why this review exists: round 1 of this chain (job
wait-member1-codecritic-1) found WVB-40, WVB-41 and WVB-42 material on
round 5 of the build. Wido's word this morning: fold them, then this
read, then the landing. Round 6 of the build chain wait-member1-build-1
folded them on the seat's decisions (the brief is a repository file after
the landing; its decisions: a saved source terminal 0-4 replays and never
renews, a saved 124, 130, 6 or 65 renews only with an explicit --timeout
as a recorded renewal under the same key with the original cursor; a
landing wait refuses at registration with exit 4 and a named reason when
the code destination differs from the ledger branch; the actionable check
computes the open-work signature from the plan files alone, spawns no
process, and returns within the wait's context). The tree to read is the
chain's worktree as round 6 left it (the dispatcher snapshots it).

Round budget: 1 focused round. A finding is material only if an
implementer must change the code or tests before this lands, and it
names the artifact it would change.

Contract: the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md
(sections 2, 4, 5, 6) as landed, and the goal record's DONE (read it with
`bin/metasystem goal show --id wait-verb-returns-on-recorded-events`; the
JSON field Intent).

# Mandate

1. For each of WVB-40, WVB-41 and WVB-42: is the decision implemented as
   stated and tested? Return each under its identifier.
2. Attack what round 6 changed: the renewal path (can a renewal lose the
   original cursor or reuse a spent nonce? can a plain resume of a
   renewable result loop forever?), the landing destination check (does
   it read the right configured branch on this repository? does remote
   mode still accept?), the plan-only signature (does it equal the stop
   gate's digest for the same plan files? can it drift from the gate's?).
3. Nothing else: the earlier findings WVB-43 to WVB-47 were not material
   and stay as they are.

If nothing material remains, say so; that closes the chain and the
member lands.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
