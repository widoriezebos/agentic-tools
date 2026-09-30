# steward-sees-stuck-capacity

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=2 basis="Reports on and releases shared local resources; a wrong release could free a live holder, so every release needs proof of death; recurring machinery."
- Tier: 2
- Intent: What: The steward, the background supervisor, notices when work has waited more than 10 minutes on a shared resource (a proof lease, the proof lock, the landing queue, the VM or suite lock, or a process left over from a finished run) and reports what waits, who holds it, whether the holder is alive, and the exact command to free it; holders that are provably dead are released by their owners. Why: between 2026-09-27 and 2026-09-29 work stalled silently several times: dead leases twice, an orphaned watchdog holding the VM suite lock, a queue mix-up, and a full disk. A disk report with a clean-up command already exists and is reused, not rebuilt. Pros: stalls show up within minutes with the fix attached. Cons: one more report to keep accurate, and releasing a lock wrongly could break a live run, so only provably dead holders are released.
- Origin: human
- Next step: Next: write a short design for the capacity-wait report that reuses the existing disk report, and give it one round with the design critic. Done when: the design is accepted after that round; the dead-lease fix it waited on is already on main, so building can start right after.
- OpenedAt: 2026-09-28T08:23:19Z
- Revision: 5
- Labels: efficiency, steward
- BudgetExceptions: 0

History:
- 2026-09-28T08:23:19Z 5JNGWCSGXH8ENQRMH2A3RPNG8D-m1e-c6925449 open actor=human:Wido targets=steward-sees-stuck-capacity
- 2026-09-28T08:24:24Z 16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 approve actor=human:Wido targets=landing-proof-records-are-written,steward-sees-stuck-capacity
- 2026-09-28T13:01:58Z AWEWACJW9HCJ61FV4W58GRYWZH-m1e-c6925449 edit actor=m1e+main-1788680071-18713-e76d5d targets=steward-sees-stuck-capacity
- 2026-09-30T18:57:41Z 27D0V8SAZJJTHTYCFV3RMQ0G16-ui-bc2fda53 unapprove actor=human:Wido targets=steward-sees-stuck-capacity reason=the backlog clean-up of 2026-09-30 rewrites the intent in plain English; approved again with the same box
- 2026-09-30T18:57:46Z JVEEVJRNMSPHJVTJA56JNRWQCB-ui-bc2fda53 edit actor=human:Wido targets=steward-sees-stuck-capacity
Integrity: sha256=8ad9d34193a4fd5052098e54127d2db7474e221c3c791d613c2375005473918a
