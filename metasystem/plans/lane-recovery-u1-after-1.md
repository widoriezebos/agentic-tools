# lane-recovery-along-intent U1 correction 1 (Astra round 1): two material findings, both fixtures

Working Mode: Implement. Correction to the unit built from plans/lane-recovery-u1-resume-stop-restart.md (re-read it and the design's amended D1 and D5). Worktree uncommitted or committed as the top commit; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/, testing.json, go.mod or any metasystem.conf*.

1. (F1) Resume aborts a pending merge only when it is abandoned: the fix record's state is abandoned, or no job owns the lane checkout (the resolution job named by the fix record is not running — read its launch record state) and the checkout's writers have settled; "main moved: code change; rebuild" alone is not a reason. A merge a running job owns is left alone and reported ("the resolution job of G still runs (M min); give it back or wait"). Fixture: a resolution job running with unfinished edits and main moved → nothing aborted; the same with the job ended and the record abandoned → aborted.
2. (F2) `landing restart` performs the keeper's explicit continuation after recording the selection (the explicit step `landing run` takes), so the hold clears and execution starts in the same act. Fixture: two unchanged runs, waiting goals, no live proof → restart → the hold is gone and the agent started (observe the keeper state and the start seam), not merely the provenance recorded.

## Checks
as the unit's brief. Report both changes, their fixtures, the mutations (abort an owned merge → red; restart without the continuation → red) and every check's exit.

## Added from Astra round 2
3. (LR-01) Abandonment is bound to the merge actually pending: abort only when MERGE_HEAD equals the abandoned fix record's tip, no job owns the checkout, and the writers have settled — all three, on every abort path. Fixture: an abandoned record for merge A while merge B is pending → B preserved, the run says so.
4. (LR-02) A barren hold with no selectable goal and no open incident clears only through the keeper's own remedy, a fresh trunk proof: resume requests it (the `landing prove --trunk` launch) in that case. Fixture: two unchanged runs, no eligible goals, no incidents → resume launches the trunk proof and reports "proving main to clear the hold".

## Added from Astra round 3
5. (LR-03b) The abort predicate binds the abandoned record to the pending merge by BOTH coordinates the resolver uses (resolve.go ~124, ~178): HEAD == the record's checkpoint Commit AND MERGE_HEAD == the record's Tip. Fixture: an abandoned record (A, T) while (B, T) is pending → no abort, the merge preserved.

## Added from Astra round 4
6. (LR-04) The real stale case (12:06 today) is a merge still pending with its record `resolving` (HEAD = checkpoint, MERGE_HEAD = tip) whose resolution job has ended (launch record terminal, writers settled) while main advanced by code: resume owns that transition — mark the record abandoned ("superseded by main's code move; rebuild"), abort the merge, let the lane rebuild on the current main. A job still running keeps its merge. Fixture: start from the naturally produced `resolving` state (not an injected abandoned record), end the job's record, advance main by a code commit → resume aborts and the record reads abandoned with that reason; the same with the job still running → untouched.

## Added from Astra round 5
7. (LR-05) The selection hold under `landing.batch=person` (SelectBatch prepares an unsigned person-policy batch and refuses, batch.go ~447/~41) is lifted by resume the same way as the barren hold: record the selection of every admissible waiting goal (oldest first) as the person's, then the explicit continuation. Fixture: batch policy person, one admissible waiting goal, no batch, no barren count → one resume records the selection and starts execution.

## Added from Astra round 6
8. (LR-07) `landing stop` invalidates a standing continuation a person recorded through a pause (StandingPause, batch.go ~145; the permission `landing run --goals` leaves for a paused lane) before it settles the session, so the next keeper cycle launches nothing. Fixture: pause, `landing run --goals G`, stop → the keeper's next cycle starts no agent; the session ended.
