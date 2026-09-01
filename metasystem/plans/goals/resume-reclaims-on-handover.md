# resume-reclaims-on-handover

- State: queued
- Intent: goal resume hands the claim to the RESUMING machine, which is the wrong default for the handover case it will most often serve: m3 resumed alert-escalation-channel on Wido's relayed word so that m0b could build it, and the resume re-claimed the goal to m3 (revision 21) - the same wedge under a new name - until m3 noticed and released. Found 2026-09-01 by m3 executing a directed cross-machine resume
- Origin: main
- Next step: Appetite: 2h. Design question first (Fable lane): should resume take an explicit claim disposition - claim-to-me, release-to-queue, or claim-to-<machine> - rather than defaulting to the resumer? The handover case (one machine clears a fence so ANOTHER can work) is at least as common as the self-resume case, and the current default silently produces a blocked successor. Whatever is chosen, the verb must SAY what it did to the claim in its typed outcome; m3's report is the specimen. Then implement (Sol) with a fixture proving both dispositions and the typed statement
- OpenedAt: 2026-09-01T12:44:34Z
- Revision: 1

History:
- 2026-09-01T12:44:34Z JT0WA4WGVZRR51WQZP9V3N66M6-m1-bf243850 open actor=m1+coordinator targets=resume-reclaims-on-handover
Integrity: sha256=2e26b49801fe77e9253b8d313205e36ad3a1aa49c018db9c088f27b5544b902c
