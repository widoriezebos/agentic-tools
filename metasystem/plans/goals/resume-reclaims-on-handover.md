# resume-reclaims-on-handover

- State: approved
- Intent: goal resume hands the claim to the RESUMING machine, which is the wrong default for the handover case it will most often serve: m3 resumed alert-escalation-channel on Wido's relayed word so that m0b could build it, and the resume re-claimed the goal to m3 (revision 21) - the same wedge under a new name - until m3 noticed and released. Found 2026-09-01 by m3 executing a directed cross-machine resume
- Origin: main
- Next step: Appetite: 2h. Design question first (Fable lane): should resume take an explicit claim disposition - claim-to-me, release-to-queue, or claim-to-<machine> - rather than defaulting to the resumer? The handover case (one machine clears a fence so ANOTHER can work) is at least as common as the self-resume case, and the current default silently produces a blocked successor. Whatever is chosen, the verb must SAY what it did to the claim in its typed outcome; m3's report is the specimen. Then implement (Sol) with a fixture proving both dispositions and the typed statement
- OpenedAt: 2026-09-01T12:44:34Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=5ab9ce90f2ed45846bf0b296ec102815db51c8c3c840b2ad05c27ca841d4560f

History:
- 2026-09-01T12:44:34Z JT0WA4WGVZRR51WQZP9V3N66M6-m1-bf243850 open actor=m1+coordinator targets=resume-reclaims-on-handover
- 2026-09-01T20:27:13Z C8DWDM1VG17N87XQMGKBKMWBQX-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=resume-reclaims-on-handover
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=resume-reclaims-on-handover reason=sweep
Integrity: sha256=661308d7154cc639fdf909561a0729aab27958845a40cdb050ac9f35bb7884c3
