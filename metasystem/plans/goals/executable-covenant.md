# executable-covenant

- State: claimed
- Intent: The verification covenant is runnable: one battery entrypoint with a verdict file, one critique-round driver any agent can invoke
- Origin: main
- Next step: Build battery.sh (one entrypoint, verdict file) and the critique-round driver carrying the arc's stop mechanism; designed together with critique-stop-rule (D114). The driver REFUSES to start a critique loop without a declared failsafe round and stops the loop itself when a tier fires — the mechanism runs without any particular agent (Wido's instruction 2026-08-19).
- OpenedAt: 2026-08-20T00:17:00Z
- Revision: 2
- Arc: covenant-patience
- Claimed: machine=widos-m5-pro lineage=coordinator at=2026-08-22T11:38:48Z

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=executable-covenant
- 2026-08-22T11:38:48Z XWHPR2FT823CZ29SCQ3NVADS81-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=critique-stop-rule,executable-covenant
Integrity: sha256=879412ad7f764481144df3ca677e02e6fe083f1575c2a98d337e13fe4964c279
