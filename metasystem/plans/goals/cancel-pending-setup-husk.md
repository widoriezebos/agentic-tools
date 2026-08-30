# cancel-pending-setup-husk

- State: claimed
- Intent: An operator's cancel of a pending-setup reservation husk must stick — today the internal cancellation gate exits zero and the reservation later completes setup and launches, silently outrunning the human's stop; the public cancel verb separately fails while resolving the husk's absent runtime (codex contract ruling, cancellation delta review round 6, 2026-08-20)
- Origin: main
- Next step: Per the review ruling: public cancel routing that works before a runtime exists on the record; a lawful pending-setup-to-cancelled transition; atomic arbitration with RecordSetup so a marked husk cannot complete setup (the same only-forward-path-is-cancelled rule every live-record writer now enforces); fence cleanup for the cancelled reservation; and a create-cancel-setup/launch ordering test.
- OpenedAt: 2026-08-20T16:52:00Z
- Revision: 3
- Budget: elapsedLimit=3h attemptLimit=5 reservedJobMinutesLimit=45 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T04:30:18Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=cancel-pending-setup-husk
- 2026-08-30T04:30:02Z JW8MYZWGDF9P2XR47MK9HAPEGX-m2-bc1be9cb set-budget actor=human:wido targets=cancel-pending-setup-husk
- 2026-08-30T04:30:18Z AKNF26P5WA0F3H6SNPQ6M2ZSQT-m2-bc1be9cb claim actor=m2+mac-coordinator targets=cancel-pending-setup-husk
Integrity: sha256=1343c52a4d4b174032fa11382c512908dde98afdf0e78384af9c0118203952bf
