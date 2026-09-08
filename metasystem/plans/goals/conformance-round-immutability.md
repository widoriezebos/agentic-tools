# conformance-round-immutability

- State: approved
- Priority: 3
- Sequence: 55
- Intent: Conformance review artifacts are append-only: a re-run writes a new round directory or refuses - it never overwrites a persisted review.json/diff.patch. The overwrite destroyed chain evidence once this period (the root-id re-run clobbering rounds/1) and was independently flagged by two critics (TB finding F2-7; watch chain WVFC set). IL-35.
- Origin: main
- Next step: DONE, landed 1ec89183 by m0 (account Wido@M0): a conformance re-invocation that would overwrite a persisted review refuses naming the artifact and the lawful follow-up path; byte-identical re-runs succeed idempotently; the certified-output locator's contract preserved and re-tested. IL-35's effect armed: the overwrite class never recurs.
- OpenedAt: 2026-09-01T20:28:48Z
- Revision: 8
- Budget: elapsedLimit=2d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=7 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=903286dc9be8283467b99593357aab719a2d36bac079c8b23539f8bb6f2538cb
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-01T20:47:24Z

History:
- 2026-09-01T20:28:48Z HPJMQZ27J41VFN0CP4GC0PM16V-m0-c5dbf036 open actor=human:Wido targets=conformance-round-immutability
- 2026-09-01T20:28:56Z BZSVGNZA35BN8534VMS5KQXTTV-m0-c5dbf036 set-budget actor=human:Wido targets=conformance-round-immutability
- 2026-09-01T20:46:57Z E3MZX3WJ65YAW95QRZGXWR07EM-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-round-immutability
- 2026-09-01T20:47:24Z WVMR2AYV990RRZ3KAPGP5ARS1N-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-round-immutability
- 2026-09-01T20:54:59Z SPSEXJ1M02GVEKCQ2MMWTSKJNW-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-round-immutability
- 2026-09-01T20:55:03Z Y7GHDR6M3DJ365FRKBHYCZAGTJ-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-round-immutability
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=conformance-round-immutability reason=sweep
- 2026-09-08T16:02:21Z G02P7W9PAAQGFG1ZXKMB891WZ9-m1-7cd0bd60 set-priority actor=human:Wido targets=conformance-round-immutability reason=priority-order subject=conformance-round-immutability from=unranked to=3:55 requested-sequence=55
Integrity: sha256=56f64ad5632df2cbd0330068428e806068b73e034483906317fab490c37f321c
