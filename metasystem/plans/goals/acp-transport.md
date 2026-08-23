# acp-transport

- State: claimed
- Intent: ACP as the delegate transport, retiring the dangerous-mode waiver
- Origin: main
- Next step: Appetite: 8h continues — PARKED AT WIDO'S SEAL (D88), 2026-08-23 08:15Z, exactly as designed. The genesis fix (9a86eb8) held: rep 1's target provisioned clean in the VM at matched HEAD 89b2509. Cohort bm-2d-20260823t080725z-3548802. THE SEAL NEEDS WIDO, in the VM (limactl shell metasystem-debian-amd64): (1) review /home/wido.guest/trials/cohorts/bm-2d-20260823t080725z-3548802/targets/1/plans/mission-bm-2d.contract.md; (2) note the standing warning — devin-host@1 carries ledgerNoGainBudget=5 below cycles=8 with no acceptBinaryGateFuse, and the contract WILL NOT SEAL until the budget is raised or the fuse acknowledged (issues #4/#8) — that ruling is part of the seal; (3) seal: cd targets/1 && scripts/assert-mission.sh --seal --file plans/mission-bm-2d.contract.md; (4) sign: add the Approval line with the printed hash, commit and push. THEN any coordinator resumes: benchmark/run-cohort.sh --resume bm-2d-20260823t080725z-3548802 (VM, METASYSTEM_TRIALS_ROOT=/home/wido.guest/trials). Machine 2 moves to fleet-menu follow-on work meanwhile; the claim holds for the resume.
- OpenedAt: 2026-08-20T00:25:00Z
- Revision: 9
- Claimed: machine=widos-macbook-pro lineage=mac-coordinator at=2026-08-23T08:06:42Z

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=acp-transport
- 2026-08-22T20:11:01Z 2A6NK72FWYE8BJVY14HAWCS6MK-widos-m5-pro-bf243850 unpark actor=human:wido targets=acp-transport
- 2026-08-22T20:11:05Z BYCBMSN6M9DSXA25WWFRXFXN68-widos-m5-pro-bf243850 edit actor=widos-m5-pro+coordinator targets=acp-transport
- 2026-08-22T20:28:53Z 46BBGXS24D3V5HFKYZE224T08Q-widos-macbook-pro-bc1be9cb claim actor=widos-macbook-pro+mac-coordinator targets=acp-transport
- 2026-08-22T23:29:03Z F3XJH1MECQ52E45A36Y9CKP3XG-widos-macbook-pro-bc1be9cb edit actor=widos-macbook-pro+mac-coordinator targets=acp-transport
- 2026-08-22T23:29:17Z KJ6T9MQS52A1FPB6ZR1YK8FRKA-widos-macbook-pro-bc1be9cb release actor=widos-macbook-pro+mac-coordinator targets=acp-transport
- 2026-08-23T07:55:43Z YP2PPK1WG52X99ACWMEVS5WZP4-widos-m5-pro-bf243850 edit actor=widos-m5-pro+coordinator targets=acp-transport
- 2026-08-23T08:06:42Z W5C3A9F7PA9ZMK6SSA4J6KTPEB-widos-macbook-pro-bc1be9cb claim actor=widos-macbook-pro+mac-coordinator targets=acp-transport
- 2026-08-23T08:09:04Z HZZHGM8GA47PE5EFEKZ4NAGHJX-widos-macbook-pro-bc1be9cb edit actor=widos-macbook-pro+mac-coordinator targets=acp-transport
Integrity: sha256=c0c9a4e0677b3ddf54a4128e5ba1e0fd157c2ea0b271cf5548919de74d0864d7
