# lane-recovery-along-intent U1 correction 1 (Astra round 1): two material findings, both fixtures

Working Mode: Implement. Correction to the unit built from plans/lane-recovery-u1-resume-stop-restart.md (re-read it and the design's amended D1 and D5). Worktree uncommitted or committed as the top commit; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/, testing.json, go.mod or any metasystem.conf*.

1. (F1) Resume aborts a pending merge only when it is abandoned: the fix record's state is abandoned, or no job owns the lane checkout (the resolution job named by the fix record is not running — read its launch record state) and the checkout's writers have settled; "main moved: code change; rebuild" alone is not a reason. A merge a running job owns is left alone and reported ("the resolution job of G still runs (M min); give it back or wait"). Fixture: a resolution job running with unfinished edits and main moved → nothing aborted; the same with the job ended and the record abandoned → aborted.
2. (F2) `landing restart` performs the keeper's explicit continuation after recording the selection (the explicit step `landing run` takes), so the hold clears and execution starts in the same act. Fixture: two unchanged runs, waiting goals, no live proof → restart → the hold is gone and the agent started (observe the keeper state and the start seam), not merely the provenance recorded.

## Checks
as the unit's brief. Report both changes, their fixtures, the mutations (abort an owned merge → red; restart without the continuation → red) and every check's exit.
