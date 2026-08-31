# stop-message-truth

- State: queued
- Intent: The stop message reflects the actual state of the system: the live ledger projection (claims, real next steps) plus whether work is in flight right now - a stale snapshot that says nothing about activity must be impossible (Wido 2026-08-20). ABSORBS open-work-scanner-blindspots (parked, R-33 merge): KI-34 - the scanner equates work with this-checkout dispatch job records, so cross-checkout worktree jobs and non-job in-flight processes (background critiques, verification runs) are invisible; it cries wolf or gets silenced by wording. One stop-hook honesty seam, one goal
- Origin: main
- Next step: After cutover retargets the verdict to the new projection (that half is a backlog-git-sync cutover obligation), fold in the steward's status surface: the stop message names any live worker, in-flight continuation, or pending steward incident, so silence is never ambiguous; acceptance = the frozen-ledger staleness observed 2026-08-20 cannot recur. Proposal from m2 (2026-08-24, for the primary coordinator's ratification): extend the idle-watchdog predicate to be ledger-aware — arm the watchdog on every enrolled checkout, and revive/notify when goal next names a tokened, unclaimed, unblocked item AND the machine holds no claim; that makes the distributed backlog itself the wake signal, so a machine with standing work never sits idle behind a stale stop message.
- OpenedAt: 2026-08-20T16:51:00Z
- Revision: 4
- BlockedBy: backlog-git-sync, idle-watchdog

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=stop-message-truth
- 2026-08-24T10:19:34Z W0J5R9MB25P2BN3GH0G6Z0QD6K-m2-bc1be9cb edit actor=m2+mac-coordinator targets=stop-message-truth
- 2026-08-24T10:20:23Z E0FHAVYVRQF8WQWHFB36W0RJVV-m2-bc1be9cb edit actor=m2+mac-coordinator targets=stop-message-truth
- 2026-08-31T18:04:18Z JA1NW3CDG1DMVJYD8H5AWSN1F5-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=stop-message-truth
Integrity: sha256=80adb4de92a3c838ef50ed0e6704a58bc25ad4ec7bedd0959b28186c8ee7a210
