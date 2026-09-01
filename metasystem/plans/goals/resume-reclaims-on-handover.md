# resume-reclaims-on-handover

- State: queued
- Intent: goal resume hands the claim to the RESUMING machine, which is the wrong default for the handover case it will most often serve: m3 resumed alert-escalation-channel on Wido's relayed word so that m0b could build it, and the resume re-claimed the goal to m3 (revision 21) - the same wedge under a new name - until m3 noticed and released. Found 2026-09-01 by m3 executing a directed cross-machine resume
- Origin: main
- Next step: Appetite: 2h. Design question first (Fable lane): should resume take an explicit claim disposition - claim-to-me, release-to-queue, or claim-to-<machine> - rather than defaulting to the resumer? The handover case (one machine clears a fence so ANOTHER can work) is at least as common as the self-resume case, and the current default silently produces a blocked successor. Whatever is chosen, the verb must SAY what it did to the claim in its typed outcome; m3's report is the specimen. Then implement (Sol) with a fixture proving both dispositions and the typed statement
- OpenedAt: 2026-09-01T12:44:34Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-01T12:44:34Z JT0WA4WGVZRR51WQZP9V3N66M6-m1-bf243850 open actor=m1+coordinator targets=resume-reclaims-on-handover
- 2026-09-01T20:27:13Z C8DWDM1VG17N87XQMGKBKMWBQX-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=resume-reclaims-on-handover
Integrity: sha256=6aedc41c50a48ad75713f1bf32300603e0dc055afaa5399fd9edd818428ba555
