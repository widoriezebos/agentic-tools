# engine-owns-disk-lifetimes

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=2 basis="Deletes files on the local machine only; a wrong owner or lifetime could remove evidence or a live run folder, so deletion needs proof the owner is dead or finished; recurring machinery, so a defect repeats until fixed."
- Tier: 2
- Intent: The engine owns the lifetime of everything it writes to disk: every store is registered with an owner, a lifetime and a size cap; runs, delegates and goals release their stores when they end; a steward sweeper reclaims what escaped; one -trimpath Go cache per machine. The SSD never needs a manual cleanup pass again.
- Origin: human
- Next step: Write the design at plans/designs/engine-owns-disk-lifetimes.md from the 2026-09-27 disk census (125 GB private Go caches, 14 GB leaked test temp, 20 GB ad-hoc /private/tmp proof copies, cold rebuilds per source copy) and have Codex Astra critique it.
- OpenedAt: 2026-09-27T16:59:54Z
- Revision: 1
- Labels: disk, efficiency
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-27T16:59:54Z Q64R5EKH33MXEBGZ407C1DVE4S-m1e-c6925449 open actor=human:Wido targets=engine-owns-disk-lifetimes
Integrity: sha256=eb85299428b8bb502427c33eaa4ebb81e59eb19de4bec54b96d2832390c2849d
