Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Design fold: revision 2 of the backlog ordering page, and the last one

Deliverable: revision 2 of
metasystem/plans/backlog-ordered-by-priority-design.md, edited in place.
Revision 1 was written on this lane an hour ago and is landed on trunk;
read it first. This round folds its one independent critique and closes
the page. No code, no fixtures, no other file.

The critique is job backlogorder-crit1 (design-critic, gpt-5.6-sol,
xhigh, read-only) and its disposition is
metasystem/records/misc/backlog-ordered-by-priority-critique-r1.md.
Three findings are accepted and one is recorded without action. The
contract remains the goal record
metasystem/plans/goals/backlog-ordered-by-priority.md and Wido's
sentence inside it.

## BOP-01, high, a design decision and the reason this round exists

The dense-position invariant is not closed over the supported
edit-then-reconcile way to conclude a goal. Sections around lines
145-159 of revision 1 specify compaction inside the done request, but
reconciliation has its own executor and its own batch behaviour, and
the page never names them. So concluding the middle ranked goal through
a lawful human reconcile leaves a hole, which the new validator then
refuses.

The critic also named the wrong answer, so do not walk into it: a naive
per-row fix can emit a change for a goal that a later row deletes, and
reconciliation's first-change-per-path fold then discards that
deletion.

Decide and write:

- how a multi-row reconcile applies rank compaction once, coherently,
  across every done row in the batch, given a fold that keeps the first
  change per path;
- what the resulting tree must look like after a batch that concludes
  two ranked goals in the same priority, and after a batch that
  concludes two in different priorities;

SCOPE CLARIFICATION, added 2026-09-08 after this round's first attempt
reported the gap: cover only the batch shapes reconciliation supports
TODAY. Do not expand reconciliation's grammar or its executor; that is
outside this goal's intent, which is ordering the backlog. If a mixed
batch that both concludes and reopens is unsupported, say so in the page
as a stated limitation, name the refusal a person would see, and treat
reopen as its own transaction. A limitation written down is a finished
design; an invented mechanism is not.
- a focused reconciliation fixture in section 6, with the smallest
  proving run and its independent observation, in the same shape as the
  rows already there.

The evidence the critic read: internal/goal/reconcilemap.go:270 maps a
hand-edited done state; internal/goal/reconcilepub.go:300 archives it
directly; the mutation loop at internal/goal/reconcile.go lines 95-126
applies several rows and keeps the first change per path. Read those
before deciding, and say in the return which file and line each part of
your decision rests on.

## BOP-02, medium, one canary correction

No named canary proves convergence when set-priority races goal claim.
Add deterministic compare-and-swap collisions in BOTH publication
orders, asserting that the claim coordinates and the new rank both
survive. The steady-state claimed-peer row and the two-seat claim race
row cannot expose one mutation overwriting fields the other published.

The seam exists and needs no sleeps: `BeforePush` at
internal/goal/txn.go:484, used at internal/goal/txn_test.go:170-249.

## BOP-03, medium, one canary correction

The approval-sweep row cannot fail when the sweep drops an existing
rank, because revision 1 assigns the first rank only after the sweep.
Rank at least one swept record BEFORE preview and publication, and make
the observation assert that its pair survives while the other records
stay unranked. `ApproveSweep` at internal/goal/approval.go:660-684
touches and re-renders every approved or ratified goal, so the current
ordering cannot discriminate.

## BOP-04, low, recorded and not actioned

Next can recommend the same unpinned goal to two free machines. The page
already says next is a read and that claim publication is the only
reservation, and turning next into a reservation would change what a
claim means, which is out of scope. Leave it as it is; if you want, make
that sentence explicit so the next reader does not re-raise it.

## Scope

Fold these three and nothing else. Do not restructure the page, do not
revisit decisions the critique did not challenge, and do not add a
mechanism the intent does not ask for. Keep the canary discipline: every
fixture row keeps its smallest proving run and its two-minute ceiling.

Mark the page revision 2 and say at the top which findings it folded.

This is the last design round for this page. After it, the build
proceeds behind the fixtures section 6 names. If something remains that
you believe must be settled in prose, say so plainly in the return
rather than writing a third revision.

Gap rule: stop and report a gap; never fill it silently.
