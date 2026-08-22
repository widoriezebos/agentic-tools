# lease-acquire-atomicity

- State: done
- Intent: Lease acquisition and stale-lease cleanup are mutually exclusive: no second launcher can misclassify a mid-publication lease and mint two runners (KI-38)
- Origin: main
- Next step: KI-38: one flock over lease classification-and-removal and marker-and-record publication, plus a two-process witness; its wait (the wall landing) is satisfied.
- Concluded: One bounded flock over both critical sections (lease.LockBounded on mission-dir lease.lock): acquire covers marker-through-records, cleanup covers classify-through-remove; two-sided refusal witness plus full-record publication proof. KI-38 closed.
- OpenedAt: 2026-08-20T00:22:00Z
- Revision: 3

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=lease-acquire-atomicity
- 2026-08-22T10:02:32Z 01YD1XKYEHZBS0G12PB9HHTX4Y-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=lease-acquire-atomicity
- 2026-08-22T10:27:44Z 556FD9Y83622FJFTACDNKW73AJ-widos-m5-pro-bf243850 done actor=widos-m5-pro+coordinator targets=lease-acquire-atomicity
Integrity: sha256=0e3f14925ba4adb814238b85668dcd9edfdce172e522a42f4b000dbfb6521537
