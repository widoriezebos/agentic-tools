# seam-eventstream-error-surface

- State: queued
- Intent: EventStream.Next's false is overloaded (end-of-stream vs accessor-context expiry) — the seam contract's one recorded signature wart (acp-adapter-seam residue two)
- Origin: main
- Next step: Appetite: 2h. Amend the slice-one contract deliberately: either Next gains an error return or the contract documents a mechanical disambiguation (a Done()/Err() companion), with parity pins updated and both implementations (native driver spool, any future emulator stream) conforming in the same landing. Contract changes are seam-owned: design note first, one critique round minimum.
- OpenedAt: 2026-08-25T06:07:17Z
- Revision: 1

History:
- 2026-08-25T06:07:17Z KYG7XWZN5R1KTDDRE1DQJV8HC8-m2-bc1be9cb open actor=m2+mac-coordinator targets=seam-eventstream-error-surface
Integrity: sha256=07fc06cf50e53f25f9ca1ba7f80a294e2c54ac27642ecf58ec5a9f52944041c6
