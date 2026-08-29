# nested-gate-witness-reuse

- State: queued
- Intent: The battery's nested adoption gate re-runs the full 53-package go gate a second time inside the adopted copy - the witness machinery exists so a nested gate can REUSE the outer gate's proof for identical bytes; measured today: the double run is roughly a third of the ~40-minute battery (Wido 2026-08-29: are these long batteries really necessary)
- Origin: human
- Next step: Appetite: 2h — the nested gate accepts the outer witness when the adopted copy's judged bytes are digest-identical to the witnessed tree, recomputing only what adoption changed; the judge-is-not-candidate law holds (the witness itself came from a recomputation); target battery wall time 12-15 min together with missionrunner-suite-speed and timing-tests-synthetic-clock
- OpenedAt: 2026-08-29T16:49:34Z
- Revision: 1

History:
- 2026-08-29T16:49:34Z R8RA5K8JWZKC0PQ9AA5VCBMVDA-m1-bf243850 open actor=human:wido targets=nested-gate-witness-reuse
Integrity: sha256=a85e1ba5e0e7c7dd8c047e1c08ba59d13fdb724d97d130cf8162dd4cc4d50b22
