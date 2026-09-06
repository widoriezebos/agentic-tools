# seam-eventstream-error-surface

- State: approved
- Intent: EventStream.Next's false is overloaded (end-of-stream vs accessor-context expiry) — the seam contract's one recorded signature wart (acp-adapter-seam residue two)
- Origin: main
- Next step: Appetite: 2h. Amend the slice-one contract deliberately: either Next gains an error return or the contract documents a mechanical disambiguation (a Done()/Err() companion), with parity pins updated and both implementations (native driver spool, any future emulator stream) conforming in the same landing. Contract changes are seam-owned: design note first, one critique round minimum.
- OpenedAt: 2026-08-25T06:07:17Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=767b9f972161bdbc39b5e28bfd7e0a33387932299b4e91b1c727df0c29c95faf

History:
- 2026-08-25T06:07:17Z KYG7XWZN5R1KTDDRE1DQJV8HC8-m2-bc1be9cb open actor=m2+mac-coordinator targets=seam-eventstream-error-surface
- 2026-09-01T20:27:24Z 6JJEA3ZVX9RCRNPWXMZ20MTDM7-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=seam-eventstream-error-surface
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=seam-eventstream-error-surface reason=sweep
Integrity: sha256=444a14c85b1e3dcd1014545311663c573e092a68d949b335a373e0b577d7256f
