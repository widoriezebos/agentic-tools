# conformance-round-immutability

- State: queued
- Intent: Conformance review artifacts are append-only: a re-run writes a new round directory or refuses - it never overwrites a persisted review.json/diff.patch. The overwrite destroyed chain evidence once this period (the root-id re-run clobbering rounds/1) and was independently flagged by two critics (TB finding F2-7; watch chain WVFC set). IL-35.
- Origin: main
- Next step: INTENT: persisted review evidence survives every later invocation. CONSTRAINTS: the certified-output locator landed with two-bars reads all rounds/N directories already - the fix must keep that locator's contract; refusal message names the existing artifact and the lawful path (invoke with the follow-up round id). FREEDOMS: new-round-dir vs refuse; whether close-time consumes a specific round or the newest valid. Budget 4h (raise to 8h if bigger, split beyond - Wido 2026-09-01). TEST SHAPE: conformance twice on one root id leaves round 1 byte-identical; the second invocation either creates rounds/2 or refuses naming rounds/1.
- OpenedAt: 2026-09-01T20:28:48Z
- Revision: 2
- Budget: elapsedLimit=2d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-01T20:28:48Z HPJMQZ27J41VFN0CP4GC0PM16V-m0-c5dbf036 open actor=human:Wido targets=conformance-round-immutability
- 2026-09-01T20:28:56Z BZSVGNZA35BN8534VMS5KQXTTV-m0-c5dbf036 set-budget actor=human:Wido targets=conformance-round-immutability
Integrity: sha256=231b5da57c35b9209121d9d0f6bdbad53cc81f27e84954c154e3eaa741207587
