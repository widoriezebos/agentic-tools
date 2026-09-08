# account-provenance cannot be resumed by any verb that exists

Recorded by m1b, 2026-09-08, after three separate attempts by Wido each hit a different door. Written here rather than in the goal's own record because editing another machine's claimed goal is itself a human act, and this note exists precisely to stop more human commands being spent on it.

## The state

The goal is breach-stopped. Its fence was recorded 2026-09-07T21:17:15Z with reason ELAPSED_LIMIT, at goal revision 19. Its capability has since advanced: StopCapability is now Generation 21, Revision 21, Machine m1d, ClaimEpoch 2, FenceEpoch 1. The stop batch at artifacts/agents/goal-stops/stop-account-provenance-r19-f1.json still binds goalRevision 19, capabilityGeneration 19 and machine null, with state COMPLETE.

## The three locked doors

- `goal resume` refuses: "stop batch stop-account-provenance-r19-f1 does not bind the exact stopped authority for goal account-provenance revision 21". VerifyStopBatchComplete at internal/goal/stop.go:249-259 requires batch.GoalRevision, CapabilityGeneration, Machine, ClaimEpoch, FenceEpoch and Reason all to equal the current capability. Three of the six differ and no verb changes them.
- `goal set-budget` refuses: "goal account-provenance revision 19 is breach-stopped by stop-account-provenance-r19-f1; only goal resume with its standing approved budget may reopen admission". So the budget cannot move while the fence stands.
- `job breach-stop --revision 21`, which would write a batch binding the current authority, refuses: "stop batch stop-account-provenance-r19-f1 contradicts the accepted fence". So the stale batch cannot be replaced.

`goal steal` refuses because only resume may replace claim authority on a breach-stopped goal, and `goal recover` only reconciles the invoking machine's own journal.

## The cause

The goal's capability advanced from 19 to 21 after the fence was recorded. Any breach-stopped goal whose capability moves afterwards strands its stop batch permanently, because resume verifies the batch against the current capability while nothing is allowed to update either side. That is a general defect, not a fact about this goal.

## What is not lost

Chain account-provenance-build1 is complete through round 4: reviewed tree 2a4460f7, the diff applies to main, the Go gate is green and both fixture beds were green on round 2. Only its closing critic and the landing remain. The work is intact; the goal is simply unclaimable.

## What would unblock it

A fix under goal breach-stop-wedges-seat, or a sibling of it: either let a stale stop batch be re-bound to the current capability, or let resume accept a fence whose batch is stale when nothing about the stop's reason has changed. Until one of those lands, spend no further human commands on this goal.
