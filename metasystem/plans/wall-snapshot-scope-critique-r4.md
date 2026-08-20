# Snapshot-scope critique, round 4 dispositions (the declared failsafe round)

Round 4 verdict: 5 material — 1 fixture-expressible, 2 requirement
failures, 2 shape. Under the pre-committed failsafe rule the
requirement failures and shape defects lawfully reopened prose; the
fixture-expressible finding became row content. All five adjudicated.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| WSS-R4-01 | ACCEPTED (requirement failure) | The pseudoref list missed REBASE_HEAD/REVERT_HEAD/BISECT_HEAD/AUTO_MERGE and multi-OID formats; a plumbing commit parked in REBASE_HEAD escaped every probe | The census is by CLASS: every *_HEAD file plus AUTO_MERGE under the git directory, all OIDs parsed, each accounted-or-reviewed or absent |
| WSS-R4-02 | ACCEPTED (requirement failure) | Linked worktrees carry private HEAD/index (live in this very repository: .git/worktrees/issue-9-run-seams-2); a detached worktree under ignored space is a complete unobserved carrier | The worktree census joins the transition fence: workspace and runner-recorded worktrees only; unrecorded creation violates outright |
| WSS-R4-03 | ACCEPTED (shape) | Recording the state-anchors tip in the payload it authenticates is a content-addressed self-reference (anchor.go:271/:343) | refMapPost omits self-owned publication refs; the fence authenticates them through the anchor machinery's existing independent verification |
| WSS-R4-04 | ACCEPTED (shape) | Repository and mission state share no transaction; a carrier can move while the writer waits on the state lock (state.go:978), so a pre-write probe is not a commit point | The acceptance write is two-phase: append with posture, post-publication re-capture, conclusion only on a clean match, mismatch tainting over the acceptance; post-conclusion motion is post-mission by timestamp |
| WSS-R4-05 | ACCEPTED (fixture-expressible) | The exact fence gave the checked-out candidate branch no lawful transition, refusing every commit rules 1-4 admit | The active branch must equal the capture's resolved HEAD; a same-tip detach or branch switch violates; folded into WSS-11 with positive fixtures |
