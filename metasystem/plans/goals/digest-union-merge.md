# digest-union-merge

- State: queued
- Intent: Append-only fleet logs merge by union mechanically: the narrator digest (and any register the carriage allowlist marks append-only) stops producing hand-resolved git conflicts on every landing - roughly ten manual unions in one m0 session, every fleet landing colliding on the same file. IL-34.
- Origin: main
- Next step: INTENT: the union that every hand-resolution performed identically becomes the merge itself. CONSTRAINTS: union preserves both sides' line order and never drops a line (the append-only discipline the two-bars carriage class already enforces makes union semantically safe for these files exactly); scope is the gitattributes union driver or a land.sh rebase hook - whichever survives plain git pulls by other tools too. FREEDOMS: driver vs script; whether the allowlist file drives the covered set. Budget 4h (raise to 8h if bigger, split beyond - Wido 2026-09-01). TEST SHAPE: two branches appending different lines to the digest merge clean with both lines present; a REWRITE still conflicts loudly.
- OpenedAt: 2026-09-01T20:28:24Z
- Revision: 1

History:
- 2026-09-01T20:28:24Z BZDFE2CRMS385A37GYVTDFYQH8-m0-c5dbf036 open actor=human:Wido targets=digest-union-merge
Integrity: sha256=a231b13a5e0200b80ab8b2970e481c64c57d8ce38b2edd5006399d2f8506a53a
