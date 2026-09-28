# steward-sees-stuck-capacity

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=2 basis="Reports on and releases shared local resources; a wrong release could free a live holder, so every release needs proof of death; recurring machinery."
- Tier: 2
- Intent: The steward sees and names shared capacity that work is waiting on: a proof, run or job that has waited more than 10 minutes on a proof-admission lease, proof lock, landing queue, VM or suite lock, or a process that outlived its run is reported with what it waits on, who holds it, whether the holder is alive, and the exact command to settle it; provably dead holders are released by their owners; nothing waits silently.
- Origin: human
- Next step: Design from the 2026-09-27/28 incidents (dead heavy leases twice, the orphaned watchdog holding the VM suite lock, the batch proof-queue aliasing HM8-01) with the steward health roles as the home; one Astra round; build after the lease-reclaim fix lands.
- OpenedAt: 2026-09-28T08:23:19Z
- Revision: 1
- Labels: efficiency, steward
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-28T08:23:19Z 5JNGWCSGXH8ENQRMH2A3RPNG8D-m1e-c6925449 open actor=human:Wido targets=steward-sees-stuck-capacity
Integrity: sha256=3f610243c9fd71bffcf0860f95777b2cd38439aca2939f8cb26ef215f6b331ba
