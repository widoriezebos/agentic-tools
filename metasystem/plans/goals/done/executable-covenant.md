# executable-covenant

- State: done
- Intent: The verification covenant is runnable: one battery entrypoint with a verdict file, one critique-round driver any agent can invoke
- Origin: main
- Next step: Build battery.sh (one entrypoint, verdict file) and the critique-round driver carrying the arc's stop mechanism; designed together with critique-stop-rule (D114). The driver REFUSES to start a critique loop without a declared failsafe round and stops the loop itself when a tier fires — the mechanism runs without any particular agent (Wido's instruction 2026-08-19).
- Concluded: battery.sh (one entrypoint, durable verdict codes, mechanical check) + critique-round.sh (rounds archived per chain under artifacts). The week's hand rituals are the repo's.
- OpenedAt: 2026-08-20T00:17:00Z
- Revision: 3
- Arc: covenant-patience

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=executable-covenant
- 2026-08-22T11:38:48Z XWHPR2FT823CZ29SCQ3NVADS81-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=critique-stop-rule,executable-covenant
- 2026-08-22T11:57:46Z 1VEER6KGTN52QAHD5RB8TGP64G-widos-m5-pro-bf243850 done actor=widos-m5-pro+coordinator targets=executable-covenant
Integrity: sha256=9924ec74fa6703806d62a36f6ca6ae4405afe643406666f8aaddca8446df9b50
