# nested-gate-witness-reuse

- State: approved
- Priority: 3
- Sequence: 69
- Intent: The battery's nested adoption gate re-runs the full 53-package go gate a second time inside the adopted copy - the witness machinery exists so a nested gate can REUSE the outer gate's proof for identical bytes; measured today: the double run is roughly a third of the ~40-minute battery (Wido 2026-08-29: are these long batteries really necessary)
- Origin: human
- Next step: Appetite: 4h — RESCOPED TO DESIGN-FIRST after the independent critique judged the built candidate UNSOUND (9 material, 2 critical: candidate code judged its own proof, violating the engine-of-record law; unsigned witnesses mintable by any agent). The build is reverted; the critique (artifacts/agents/critiques/witness-reuse/r1-output.md, archived on m1) is the design input. The design must answer: base-commit engine as the sole digest/policy judge; witness authentication (integrity-bound, provenance-proven, not permissions); engine identity binding actual binary bytes and toolchain; the full loud-fallback precondition table; audit-complete reuse provenance in the envelope. Design, critique, Wido, then build. The battery keeps its full nested gate until this lands lawfully
- OpenedAt: 2026-08-29T16:49:34Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=4 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=c9c27b3a9a459bc0c6e4cdda0c3d4a75a473a6d49b3f09072b222b44cf83e3a4

History:
- 2026-08-29T16:49:34Z R8RA5K8JWZKC0PQ9AA5VCBMVDA-m1-bf243850 open actor=human:wido targets=nested-gate-witness-reuse
- 2026-08-29T18:38:59Z 8EF9RQW3JMEAXWT637CV08RFRA-m1-bf243850 edit actor=m1+coordinator targets=nested-gate-witness-reuse
- 2026-09-01T20:27:09Z N47EAGY2RSKV1CWFSGGT0FSX00-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=nested-gate-witness-reuse
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=nested-gate-witness-reuse reason=sweep
- 2026-09-08T16:03:10Z NT0F1MV5TFB7J2B9M4GJ7PJEF3-m1-7cd0bd60 set-priority actor=human:Wido targets=nested-gate-witness-reuse reason=priority-order subject=nested-gate-witness-reuse from=unranked to=3:69 requested-sequence=69
Integrity: sha256=5f829668d7519d95dab408ffea8ecee05e4d16a16f8c67896dd5439354be7400
