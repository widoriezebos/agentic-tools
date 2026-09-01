# conformance-round-immutability

- State: claimed
- Intent: Conformance review artifacts are append-only: a re-run writes a new round directory or refuses - it never overwrites a persisted review.json/diff.patch. The overwrite destroyed chain evidence once this period (the root-id re-run clobbering rounds/1) and was independently flagged by two critics (TB finding F2-7; watch chain WVFC set). IL-35.
- Origin: main
- Next step: INTENT: persisted review evidence survives every later invocation. CONSTRAINTS: the certified-output locator landed with two-bars reads all rounds/N directories already - the fix must keep that locator's contract; refusal message names the existing artifact and the lawful path (invoke with the follow-up round id). FREEDOMS: new-round-dir vs refuse; whether close-time consumes a specific round or the newest valid. Budget 4h (raise to 8h if bigger, split beyond - Wido 2026-09-01). TEST SHAPE: conformance twice on one root id leaves round 1 byte-identical; the second invocation either creates rounds/2 or refuses naming rounds/1.
- OpenedAt: 2026-09-01T20:28:48Z
- Revision: 4
- Budget: elapsedLimit=2d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-01T20:47:24Z
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-01T20:46:57Z revision=3
- StopCapability: generation=3 revision=3 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-01T20:28:48Z HPJMQZ27J41VFN0CP4GC0PM16V-m0-c5dbf036 open actor=human:Wido targets=conformance-round-immutability
- 2026-09-01T20:28:56Z BZSVGNZA35BN8534VMS5KQXTTV-m0-c5dbf036 set-budget actor=human:Wido targets=conformance-round-immutability
- 2026-09-01T20:46:57Z E3MZX3WJ65YAW95QRZGXWR07EM-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-round-immutability
- 2026-09-01T20:47:24Z WVMR2AYV990RRZ3KAPGP5ARS1N-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-round-immutability
Integrity: sha256=27e399aa3ee022efaaf13cf25815678b1190d608135f13fcf64435d5393defb0
