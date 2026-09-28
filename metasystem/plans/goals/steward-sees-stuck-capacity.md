# steward-sees-stuck-capacity

- State: approved
- Risk: severity=2 novelty=2 exposure=1 accumulation=2 basis="Reports on and releases shared local resources; a wrong release could free a live holder, so every release needs proof of death; recurring machinery."
- Tier: 2
- Intent: The steward sees and names shared capacity that work is waiting on: a proof, run or job that has waited more than 10 minutes on a proof-admission lease, proof lock, landing queue, VM or suite lock, or a process that outlived its run is reported with what it waits on, who holds it, whether the holder is alive, and the exact command to settle it; provably dead holders are released by their owners; nothing waits silently.
- Origin: human
- Next step: Design from the 2026-09-27/28/29 incidents (dead heavy leases twice, the orphaned watchdog holding the VM suite lock, the proof-queue aliasing HM8-01, the 2026-09-29 disk-full with ~200 GB of unregistered private caches); include a machine-wide disk health role reporting the top consumers, registered and unregistered, with reclaim commands, kept on report-only at the helm, and language-generic across toolchain caches (brief additions: evidence steward-capacity-20260929/brief-additions.md); one Astra round; build after the lease-reclaim fix is on main.
- OpenedAt: 2026-09-28T08:23:19Z
- Revision: 3
- Labels: efficiency, steward
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- NormApproval: approvedRef=16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 minutes=1200 reviewRounds=3 goalRevision=1
- Approved: by=human:Wido at=2026-09-28T08:24:24Z revision=2 opid=16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 authority=proven digest=e0f7898e0bfcdbbe2efa440f32b4f43ddf3a886c5d65a07a258bcff4fde5fed8 episode=2

History:
- 2026-09-28T08:23:19Z 5JNGWCSGXH8ENQRMH2A3RPNG8D-m1e-c6925449 open actor=human:Wido targets=steward-sees-stuck-capacity
- 2026-09-28T08:24:24Z 16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 approve actor=human:Wido targets=landing-proof-records-are-written,steward-sees-stuck-capacity
- 2026-09-28T13:01:58Z AWEWACJW9HCJ61FV4W58GRYWZH-m1e-c6925449 edit actor=m1e+main-1788680071-18713-e76d5d targets=steward-sees-stuck-capacity
Integrity: sha256=568dae478b5239e4723a364a5c2f7918a9944377116ce0b97937ae296ad92a24
