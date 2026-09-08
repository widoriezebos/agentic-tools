Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Design critique: a reviewed chain recertifies after its base moves, revision 1

FINDING IDS: chain-unique, CLB-01, CLB-02, ... never F-n.

Read metasystem/plans/chain-landing-after-base-move-recertifies-design.md,
revision 1, written by Astra this hour. This is its one independent
critique. After the findings are disposed the build proceeds behind the
fixtures the page names, so raise now anything that would change what
gets built.

The contract is
metasystem/plans/goals/chain-landing-after-base-move-recertifies.md.
Judge the design against its DONE definition, not against your own
preference for how recertification should work.

# What the page decides

A mechanical no-overlap proof, not a merge-only review round. The
coordinator merges main into the stopped chain's worktree, conformance
recomputes against that main tree into a separate immutable
recertification record, the critic's original reviewed tree and round
artifacts are untouched, and the landing compares against the
mechanically derived merged tree. A proof failure refuses and parks: no
critic is dispatched, no register reopens, no review allowance is
acquired.

# Mandate

1. **Does the proof actually prove enough?** The page is explicit that
   it establishes preservation of two textual changes, not their
   semantic independence. Is the remaining risk acceptable given what
   still runs afterwards, and is every case it excludes genuinely
   excluded: binary on both sides, structural conflicts, ambiguous
   provenance, mode changes? Name any input that would pass the proof
   and still land something no critic ever read.
2. **Is the binding re-aimed or weakened?** After recertification the
   landing compares against the merged tree. Construct the attack: a
   candidate that quietly changes a certified file, or a forged or
   stale recertification record, or a record for a different target.
   Does each still refuse? The page claims a refusal twin proves this;
   check the claim rather than the sentence.
3. **The accounting hole it says it avoids.** No merge-only critic lane
   means no exception to `reviewRoundLimit` and `criticRoundsConsumed`
   (metasystem/internal/dispatch/finding_register.go:22-23). Verify
   nothing else in the design lets an ordinary review round escape the
   box, and that the parked path cannot be used to reset accounting.
4. **Overlap, defined tightly enough.** Two implementations must agree.
   Read the definition against the real diff machinery and say whether
   insertion boundaries, adjacent consuming hunks, unterminated final
   lines and hostile diff configuration are settled or merely
   mentioned.
5. **The headless case.** An unattended node must never bypass the
   evaluator. Does the failure contract give it exactly one lawful
   move, and is a park distinguishable from a stall by a reader who was
   not there?
6. **The canaries.** Three are named with ceilings. Can each fail for
   the right reason? The previous two designs in this program each had
   a canary that could not fail, and one of them shipped, so check
   rather than assume. Say plainly if a named observation is
   unreachable in the current tree.
7. **Scope.** The page must not redesign the landing evaluator, the
   critique register, or what a claim or approval means, and must not
   widen what an agent may do. Flag any place it does.

# Constraints

Wall-clock budget: 45 minutes. Read-only: write no file other than your
return. Return per the design-critic schema. Say for each finding
whether it changes what gets built; if it does not, mark it
material=false and keep it short. Gap rule: stop and report a gap;
never fill it silently.
