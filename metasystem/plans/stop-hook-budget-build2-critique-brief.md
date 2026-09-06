Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Review brief: the Stop hook budget, slice 1 re-issued onto main (chain shbo-build2-20260906)

FINDING IDS: chain-unique, SHB2-01, SHB2-02, ... never F-n.

Round budget: one focused round; this chain exists only to re-issue an
already reviewed change onto today's main, so the review is narrow.
Material only if it changes what gets built and names the artifact.

The change was reviewed twice in chain shbo-build1-20260906 (its
dispositions are
metasystem/plans/dispositions/stop-hook-budget-code-critique-r1.md and
metasystem/plans/dispositions/stop-hook-budget-code-critique-r2.md, the
second with zero material findings). The re-issue brief is
metasystem/plans/stop-hook-budget-build2-brief.md. The builder reports
the certified patch reversed cleanly from every file except the one it
had to reconcile (metasystem/cmd/metasystem/steward_verbs_test.go,
where main's three engine-rearm tests were kept and the certified
negative-elapsed test appended byte for byte).

Threat model: any behavioural difference from the certified change
(a hunk dropped, a hunk applied to the wrong place after main moved, a
test weakened while reconciling); a test of main's lost in the
reconciled file; a change outside the thirteen declared paths; the
fixture assertions from the certified rounds missing.

Out of scope: the design of the change itself (reviewed and closed in
the first chain); the live settings file at the repository root (Wido
landed it as d165cbec); the Go toolchain skew on this host.

Scope: the computed diff of the implementer job under review.
Contract: the three briefs of the first chain
(metasystem/plans/stop-hook-budget-build-brief.md,
metasystem/plans/stop-hook-budget-fix-brief.md,
metasystem/plans/stop-hook-budget-fold2-brief.md) and the goal record
metasystem/plans/goals/stop-hook-budget-is-ours.md.

# Mandate

1. The reviewed diff equals the certified round-3 change of the first
   chain everywhere except the reconciled test file; name any hunk that
   differs.
2. The reconciled test file carries every test main had plus the
   certified one, unchanged.
3. The five acceptance points of the first chain hold on this tree: the
   three registrations at sixty, the fifty-seven-second worker share,
   elapsed on every emission path with the exit-2 retry, pointer
   semantics with a measured zero kept, the fixture assertions (elapsed
   line, record field, deadline range 50 to 59).
4. Nothing outside the thirteen declared paths changed.

If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
shbo-build2-20260906.

# Gap Rule

stop and report a gap; never fill it silently.
