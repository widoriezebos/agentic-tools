# lane-batching U1 correction 1 (Astra, factual): the seal is its own stamp, not BatchRunning

Working Mode: Implement. Correction from the design critique (read the amended D1). Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4.

BatchRunning is set during gate admission (batch.go ~758), so it cannot be the seal: the batch's `sealed_at` is stamped when the FULL proof starts (the `landing prove --wait` without --gate that the agent runs after a green gate; find where the full proof's attempt is recorded for the batch and stamp there). Until then the batch is open and newcomers join (re-merge, re-gate). Test: a hand-in after the gate but before the full proof joins; one after the full proof started does not.

## Checks
as the unit's brief.
