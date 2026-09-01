# seam-eventstream-error-surface

- State: queued
- Intent: EventStream.Next's false is overloaded (end-of-stream vs accessor-context expiry) — the seam contract's one recorded signature wart (acp-adapter-seam residue two)
- Origin: main
- Next step: Appetite: 2h. Amend the slice-one contract deliberately: either Next gains an error return or the contract documents a mechanical disambiguation (a Done()/Err() companion), with parity pins updated and both implementations (native driver spool, any future emulator stream) conforming in the same landing. Contract changes are seam-owned: design note first, one critique round minimum.
- OpenedAt: 2026-08-25T06:07:17Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-25T06:07:17Z KYG7XWZN5R1KTDDRE1DQJV8HC8-m2-bc1be9cb open actor=m2+mac-coordinator targets=seam-eventstream-error-surface
- 2026-09-01T20:27:24Z 6JJEA3ZVX9RCRNPWXMZ20MTDM7-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=seam-eventstream-error-surface
Integrity: sha256=b31c38d7bb1a9a1a02364a1dedbaeb69c788f561e328d644b59c406f0d6a9672
