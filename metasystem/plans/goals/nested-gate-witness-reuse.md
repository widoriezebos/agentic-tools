# nested-gate-witness-reuse

- State: queued
- Intent: The battery's nested adoption gate re-runs the full 53-package go gate a second time inside the adopted copy - the witness machinery exists so a nested gate can REUSE the outer gate's proof for identical bytes; measured today: the double run is roughly a third of the ~40-minute battery (Wido 2026-08-29: are these long batteries really necessary)
- Origin: human
- Next step: Appetite: 4h — RESCOPED TO DESIGN-FIRST after the independent critique judged the built candidate UNSOUND (9 material, 2 critical: candidate code judged its own proof, violating the engine-of-record law; unsigned witnesses mintable by any agent). The build is reverted; the critique (artifacts/agents/critiques/witness-reuse/r1-output.md, archived on m1) is the design input. The design must answer: base-commit engine as the sole digest/policy judge; witness authentication (integrity-bound, provenance-proven, not permissions); engine identity binding actual binary bytes and toolchain; the full loud-fallback precondition table; audit-complete reuse provenance in the envelope. Design, critique, Wido, then build. The battery keeps its full nested gate until this lands lawfully
- OpenedAt: 2026-08-29T16:49:34Z
- Revision: 2

History:
- 2026-08-29T16:49:34Z R8RA5K8JWZKC0PQ9AA5VCBMVDA-m1-bf243850 open actor=human:wido targets=nested-gate-witness-reuse
- 2026-08-29T18:38:59Z 8EF9RQW3JMEAXWT637CV08RFRA-m1-bf243850 edit actor=m1+coordinator targets=nested-gate-witness-reuse
Integrity: sha256=2a4c9d79358f50048f4ff3e0f04746ee4327d1711189db1a60b28e1b93d6589e
