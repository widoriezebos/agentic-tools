# acp-transport

- State: queued
- Intent: ACP as the delegate transport, retiring the dangerous-mode waiver
- Origin: main
- Next step: Appetite: 8h — RESERVED FOR MACHINE 2 (Wido's order 2026-08-22): run the dual-host benchmark bm-2d in the VM (run-cohort.sh, VM-only; push origin AND transport and confirm VM HEAD before trusting green). The run stops for Wido's seal per D88; a sealed green flips the ACP default (fix forward per D82). If the benchmark or the flip blows the appetite, stop and raise. The acp-adapter-seam goal starts only after the flip lands.
- OpenedAt: 2026-08-20T00:25:00Z
- Revision: 3

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=acp-transport
- 2026-08-22T20:11:01Z 2A6NK72FWYE8BJVY14HAWCS6MK-widos-m5-pro-bf243850 unpark actor=human:wido targets=acp-transport
- 2026-08-22T20:11:05Z BYCBMSN6M9DSXA25WWFRXFXN68-widos-m5-pro-bf243850 edit actor=widos-m5-pro+coordinator targets=acp-transport
Integrity: sha256=6d72cb897212b553fd49f4636d1e516a53cbcf8a6115da8b671a5b7ad5f1e8a0
