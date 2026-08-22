# stop-message-truth

- State: queued
- Intent: The stop message reflects the actual state of the system: the live ledger projection (claims, real next steps) plus whether work is in flight right now — a stale snapshot that says nothing about activity must be impossible (Wido, 2026-08-20)
- Origin: main
- Next step: After cutover retargets the verdict to the new projection (that half is a backlog-git-sync cutover obligation), fold in the steward's status surface: the stop message names any live worker, in-flight continuation, or pending steward incident, so silence is never ambiguous; acceptance = the frozen-ledger staleness observed 2026-08-20 cannot recur.
- OpenedAt: 2026-08-20T16:51:00Z
- Revision: 1
- BlockedBy: backlog-git-sync, idle-watchdog

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=stop-message-truth
Integrity: sha256=1878bf2a5b725d612c692b2a921b47ef384cb398c2e6d9d44d34bd3e196530a8
