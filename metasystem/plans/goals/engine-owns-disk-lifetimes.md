# engine-owns-disk-lifetimes

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=2 basis="Deletes files on the local machine only; a wrong owner or lifetime could remove evidence or a live run folder, so deletion needs proof the owner is dead or finished; recurring machinery, so a defect repeats until fixed."
- Tier: 2
- Intent: What: The system takes care of everything it writes to disk: each store has an owner, a lifetime and a size limit, and it is cleaned up when its work ends or by a regular sweep. Almost all of this is built (shared build caches, the sweeper, release of run folders, the evidence limit, and the disk and evidence commands). What is left is shrinking old passing test logs inside kept failure evidence, and putting the design page into the repository. Why: In September the disk filled up with hundreds of gigabytes of caches and leftovers, and someone had to clean it by hand. Pros: The disk no longer needs manual cleanup, and old evidence takes less space. Cons: Shrinking evidence deletes bytes that are archived nowhere, so it must only ever drop output of tests that passed, which makes it the riskiest piece left.
- Origin: human
- Next step: Next: Build the two remaining compaction units from the approved second compaction design (drop only the output lines of passing tests in fully passing Go test logs; keep everything failed, incomplete or unclear), then copy the design page into plans/designs and conclude. Done when: The evidence command reports a compacted bundle with the bytes dropped, the tests for failed, skipped and appended runs show those logs kept whole, and the design page is on main.
- OpenedAt: 2026-09-27T16:59:54Z
- Revision: 4
- Labels: disk, efficiency
- BudgetExceptions: 0

History:
- 2026-09-27T16:59:54Z Q64R5EKH33MXEBGZ407C1DVE4S-m1e-c6925449 open actor=human:Wido targets=engine-owns-disk-lifetimes
- 2026-09-27T17:00:18Z XS74YE0T1SC6S3TEMA69CRBY0N-m1e-c6925449 approve actor=human:Wido targets=engine-owns-disk-lifetimes
- 2026-09-30T18:56:52Z HP2S3SJS4YAGMZ5ZH2TC021S2D-ui-bc2fda53 unapprove actor=human:Wido targets=engine-owns-disk-lifetimes reason=the backlog clean-up of 2026-09-30 rewrites the intent in plain English; approved again with the same box
- 2026-09-30T18:56:57Z 9ZB2WRE5WPH23WTSHP8Q0AATEX-ui-bc2fda53 edit actor=human:Wido targets=engine-owns-disk-lifetimes
Integrity: sha256=d4e5598c7c83772e0fa1ceea694b3b8d83134a063428615bdc093b2d23bc7ea9
