# Backlog ordered by priority: design critique round 1

Critic backlogorder-crit1 (design-critic, codex gpt-5.6-sol, xhigh, read-only) on revision 1 of plans/backlog-ordered-by-priority-design.md, written by Astra (codex gpt-6-astra, xhigh) the same hour. Three material findings and one note. Two of the three are defects in the canaries the page names, which is the class the brief asked about specifically: a canary that cannot fail for the right reason is worse than none.

## BOP-01 - high - material=True

CLAIM: The dense-position invariant is not closed over the supported edit-then-reconcile way to conclude goals. Revision 1 must cover reconciliation's independent done application, define how compaction is coalesced across multiple done rows, and name a focused reconciliation fixture. Otherwise concluding the middle ranked goal through a lawful human reconcile leaves a hole and is rejected by the new validator. A per-row fix is also unsafe: an earlier compaction can emit a change for a goal that a later row deletes, while reconciliation's first-change-per-path fold can discard that later deletion.

EVIDENCE: Revision 1 lines 145-159 specify compaction in the done request but never name reconciliation's separate executor or its batch behaviour. internal/goal/reconcilemap.go:270 maps a hand-edited done state and internal/goal/reconcilepub.go:300 archives it directly. Reconcile's mutation loop at lines 95-126 applies several rows and keeps the first change encountered for a path. The proposed lifecycle fixture at design lines 389-390 exercises direct done and reopen, not reconciliation.

## BOP-02 - medium - material=True

CLAIM: No named canary proves convergence when set-priority races goal claim. The race fixtures need deterministic compare-and-swap collisions in both publication orders, asserting that the claim coordinates and the new rank both survive. The steady-state claimed-peer test and the separate two-seat claim race cannot expose one mutation overwriting fields published by the other.

EVIDENCE: Revision 1 lines 191-200 claim an intervening claim does not invalidate a rank edit. The race fixture at lines 390-391 covers two rank mutations and a concurrent removal; claimed-peer at line 388 starts after a claim already exists; claim-race at line 392 concerns two seats reading and claiming. The existing BeforePush seam at internal/goal/txn.go:484, used at internal/goal/txn_test.go:170-249, can force the missing rank-versus-claim interleavings without sleeps.

## BOP-03 - medium - material=True

CLAIM: The approval-sweep canary cannot fail when the sweep drops an existing rank, because it assigns the first rank only after the sweep. At least one swept record must be ranked before preview and publication, and the observation must assert that its pair survives while the other records stay unranked.

EVIDENCE: Revision 1 line 394 says to run the existing sweep over unranked records and assign one priority afterward. ApproveSweep at internal/goal/approval.go:660-684 touches and re-renders every approved or ratified goal, so that ordering cannot discriminate rank preservation through the sweep.

## BOP-04 - low - material=False

CLAIM: Next can recommend the same unpinned goal to two free machines, but this does not change the build under the stated contract: next is a read rather than a reservation, claim publication admits one winner, and the loser must reselect.

EVIDENCE: The design states that next is a read and that claim publication is the only reservation.

## Coordinator's reading (m1b, 2026-09-08)

All three material findings accepted. BOP-04 is recorded and not
actioned: the design already says next is a read, and turning it into a
reservation would change what a claim means, which the brief forbids.

Revision 2 is the LAST design round for this page. BOP-01 needs a design
decision rather than a fixture, because how compaction is coalesced
across a multi-row reconcile is a choice with a wrong answer that the
critic already named: the naive per-row fix can emit a change for a goal
a later row deletes, and reconcile's first-change-per-path fold then
discards the deletion. That decision belongs to the design lane, not to
the builder and not to the coordinator.

BOP-02 and BOP-03 are corrections to section 6 and are folded in the
same round because they are one edit each.

After revision 2 the build proceeds behind those fixtures whatever a
further read would say. This is design cycle two, and the exit criterion
is stated rather than assumed: the loop has no natural end, so a third
design cycle does not open on the coordinator's own authority. If
revision 2 draws material findings, the coordinator brings Wido the list
graded as build-changing or not, with a land-or-fold recommendation.
