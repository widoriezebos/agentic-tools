# lease-acquire-atomicity

- State: queued
- Intent: Lease acquisition and stale-lease cleanup are mutually exclusive: no second launcher can misclassify a mid-publication lease and mint two runners (KI-38)
- Origin: main
- Next step: KI-38: one flock over lease classification-and-removal and marker-and-record publication, plus a two-process witness; its wait (the wall landing) is satisfied.
- OpenedAt: 2026-08-20T00:22:00Z
- Revision: 1

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=lease-acquire-atomicity
Integrity: sha256=fd313e9d927d7c0732de494e0361e11e82ee33e743feb5d9db25166e2a4cc2e7
