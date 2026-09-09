Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal brain-summary-leads-the-stop-display)
Date: 2026-09-09

# Review brief: the brain summary leads the Stop display (chain brain-summary-build1-20260909)

FINDING IDS: chain-unique, BSL-01, BSL-02, ... never F-n. Report `round`
as 1 in your return: it is this job's own round.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Why: the bounded Stop display landed today in d533caf17 moved a brain
seat's summary lines from the head of the display to after the
actionable lines; two goal-cli scenarios (brain-stop-seeded,
brain-stop-corrupt) assert the old order and fail on main, and that bed
is in the full battery, so every full-width landing on the fleet is
refused until this lands.

Threat model: the brain lines placed first but counted against the
bound so a long summary trims the verdict line or the file line; the
brain lines trimmed by the bound at all (they are the seat's verdict
context and must survive whole); a non-brain seat's display changed in
any way; the renderer's earlier tests loosened rather than updated
(the 200-run shape, the actionable-first order on a non-brain seat, the
three-continuations rule, the trim notice); the two scenarios passing
by an assertion change instead of by the order change; the hook suite's
template scenarios regressed; any change outside the renderer, its
tests and the brain verdict tests.

Scope: the computed diff of the implementer job under review.
Contract: the build brief brain-summary-leads-the-stop-display-build-brief.md
in the plans directory (new, not yet committed) and the goal record
metasystem/plans/goals/brain-summary-leads-the-stop-display.md.

# Mandate

1. On a brain seat the display begins with the brain summary lines,
   never trimmed, then the verdict line and the rest of the bounded
   order; on a non-brain seat nothing changes.
2. The renderer's tests cover the brain case and keep the non-brain
   cases as they were; the brain verdict tests encode the leading order.
3. goal-cli-fixtures.sh brain-stop-seeded and brain-stop-corrupt pass
   for the order, and supervision-hook-fixtures.sh still passes.
4. Nothing outside the declared boundary changed.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
brain-summary-build1-20260909; if your sandbox cannot run it, read the
tree from the review record beside the diff and say so. The
orchestrator runs the goal package, the goal-cli bed and the hook suite
before your return lands.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
