# resume-reclaims-on-handover

- State: queued
- Priority: 3
- Sequence: 7
- Intent: goal resume hands the claim to the RESUMING machine, which is the wrong default for the handover case it will most often serve: m3 resumed alert-escalation-channel on Wido's relayed word so that m0b could build it, and the resume re-claimed the goal to m3 (revision 21) - the same wedge under a new name - until m3 noticed and released. Found 2026-09-01 by m3 executing a directed cross-machine resume
- Origin: main
- Next step: Appetite: 2h. Design question first (Fable lane): should resume take an explicit claim disposition - claim-to-me, release-to-queue, or claim-to-<machine> - rather than defaulting to the resumer? The handover case (one machine clears a fence so ANOTHER can work) is at least as common as the self-resume case, and the current default silently produces a blocked successor. Whatever is chosen, the verb must SAY what it did to the claim in its typed outcome; m3's report is the specimen. Then implement (Sol) with a fixture proving both dispositions and the typed statement
- OpenedAt: 2026-09-01T12:44:34Z
- Revision: 5
- BudgetExceptions: 0

History:
- 2026-09-01T12:44:34Z JT0WA4WGVZRR51WQZP9V3N66M6-m1-bf243850 open actor=m1+coordinator targets=resume-reclaims-on-handover
- 2026-09-01T20:27:13Z C8DWDM1VG17N87XQMGKBKMWBQX-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=resume-reclaims-on-handover
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=resume-reclaims-on-handover reason=sweep
- 2026-09-08T15:59:29Z N4892V7GJXTB532E59RY5TYE5M-m1-7cd0bd60 set-priority actor=human:Wido targets=resume-reclaims-on-handover reason=priority-order subject=resume-reclaims-on-handover from=unranked to=3:7 requested-sequence=7
- 2026-09-11T08:02:41Z 4CSE65RT60WKW3TR3XVFGDF5MP-m1-c6925449 unapprove actor=human:Wido targets=resume-reclaims-on-handover reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
Integrity: sha256=c8940ddd40a9530ef896749a59b7edd22087aeef95364919087e98dfe13cd7ce
