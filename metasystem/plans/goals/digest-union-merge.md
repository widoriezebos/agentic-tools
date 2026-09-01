# digest-union-merge

- State: claimed
- Intent: Append-only fleet logs merge by union mechanically: the narrator digest (and any register the carriage allowlist marks append-only) stops producing hand-resolved git conflicts on every landing - roughly ten manual unions in one m0 session, every fleet landing colliding on the same file. IL-34.
- Origin: main
- Next step: INTENT: the union that every hand-resolution performed identically becomes the merge itself. CONSTRAINTS: union preserves both sides' line order and never drops a line (the append-only discipline the two-bars carriage class already enforces makes union semantically safe for these files exactly); scope is the gitattributes union driver or a land.sh rebase hook - whichever survives plain git pulls by other tools too. FREEDOMS: driver vs script; whether the allowlist file drives the covered set. Budget 4h (raise to 8h if bigger, split beyond - Wido 2026-09-01). TEST SHAPE: two branches appending different lines to the digest merge clean with both lines present; a REWRITE still conflicts loudly.
- OpenedAt: 2026-09-01T20:28:24Z
- Revision: 5
- Budget: elapsedLimit=2d attemptLimit=6 reservedJobMinutesLimit=480 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-01T20:30:53Z
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-01T20:36:27Z revision=5
- StopCapability: generation=5 revision=5 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-01T20:28:24Z BZDFE2CRMS385A37GYVTDFYQH8-m0-c5dbf036 open actor=human:Wido targets=digest-union-merge
- 2026-09-01T20:28:32Z Z043NQ37F93YBZX4JK5JYGZFP4-m0-c5dbf036 set-budget actor=human:Wido targets=digest-union-merge
- 2026-09-01T20:30:41Z 79T6VWWCW4CER5F6CJNMYQBSM1-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=digest-union-merge
- 2026-09-01T20:30:53Z JGZ0CJAWYRCJ6ATCZ56Y1JXPG1-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=digest-union-merge
- 2026-09-01T20:36:27Z X25JBCCZJPERW3D3NVC15REE67-m0-c5dbf036 set-budget actor=human:Wido targets=digest-union-merge
Integrity: sha256=a688ab2efb00265419982e25d7cd67202ee9b3c19b7ed2df60ba96b59c1c00f3
