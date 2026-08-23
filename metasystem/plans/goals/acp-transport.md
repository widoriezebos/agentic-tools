# acp-transport

- State: queued
- Intent: ACP as the delegate transport, retiring the dangerous-mode waiver
- Origin: main
- Next step: Appetite: 8h, ~6h30 spent — REP 1 COMPLETE AND INVALID; banked and released per the failure rule. Cohort bm-2d-20260823t080725z-3548802, results in the VM at benchmark/results/89b2509.../bm-2d-20260823t080725z-3548802/1.{json,md}; cohort state phase=awaiting-approval with rep 2 provisioned at targets/2 awaiting ITS seal. Scorecard: Run validity INVALID on three gates (verbatim): fencesEnforced=no 'one or more observable fence limits were exceeded or unavailable'; rosterPinned=no 'job bm-2d-design-critic-2 effective model differs from requested model'; evidenceSetComplete=no (sourceOwner: KIT) 'missionState: $.admissionOrigins is not allowed; $.openTurn must match exactly one oneOf branch; $.schemaVersion is not one of the allowed values; turns..session-usage.* missing; turns.bm-2d-t1-c8e3.return: file is missing; lastCensus: $.scanSeq is not allowed'. READING: the probe's two hard mechanical gates PASSED (everyJobTerminal, everyChainClosed — dispatch worked, two critic jobs ran 619s/573s and closed); the host turn ran 90min, raised no implementer stream, produced NO return (protocolConformance 0.0), one no-progress cycle, devin orchestrator cost 98 units. TWO DISTINCT DEBTS BEFORE ANY RE-RUN CAN GO VALID: (1) KIT DEBT, mechanical, proposed Appetite 2h — benchmark/schemas/evidence/{mission-state,orchestrator}.schema.json predate WSS schema 4 (admissionOrigins, extended openTurn, schemaVersion 4) and census scanSeq; guaranteed evidenceSetComplete=invalid until aligned (the VM's Aug-18 stash bm-2d rescue contains a superseded start on exactly these files); (2) PROBE FINDINGS for Wido's read — devin substituting the critic's effective model (rosterPinned) and the host's empty return are the mechanical-health answers this probe exists to surface. DO NOT SEAL REP 2 until (1) lands, or its invalidity is guaranteed. FLIP stays approved-on-valid-green and is NOT met.
- OpenedAt: 2026-08-20T00:25:00Z
- Revision: 20

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
- 2026-08-23T08:10:06Z Z80JFBJM135TFZMMZESB4Y33G2-widos-macbook-pro-bc1be9cb edit actor=widos-macbook-pro+mac-coordinator targets=acp-transport
- 2026-08-23T08:10:14Z 2AF6R7255X4SK04JC5N5CX34HX-widos-macbook-pro-bc1be9cb release actor=widos-macbook-pro+mac-coordinator targets=acp-transport
- 2026-08-23T08:39:50Z K4DDXC6FTJYR4C8GY10JNR417Z-m2-bc1be9cb claim actor=m2+mac-coordinator targets=acp-transport
- 2026-08-23T08:40:49Z 2YV0TVZYX3M0YZ752V8536D5VT-m2-bc1be9cb edit actor=m2+mac-coordinator targets=acp-transport
- 2026-08-23T08:40:57Z EF4PF87ZVFJHDH97BFQR320T2G-m2-bc1be9cb release actor=m2+mac-coordinator targets=acp-transport
- 2026-08-23T08:42:52Z 1K9TBJR793ERYF90GJXME6XTEJ-m2-bc1be9cb edit actor=m2+mac-coordinator targets=acp-transport
- 2026-08-23T08:45:40Z EC7FY6DK4Y5PFP9VG8EX4PYFYN-widos-m5-pro-bf243850 edit actor=widos-m5-pro+coordinator targets=acp-transport
- 2026-08-23T13:41:42Z 1N3Z87ZMWAFX3H4DTBNWPF3HDG-m2-bc1be9cb claim actor=m2+mac-coordinator targets=acp-transport
- 2026-08-23T13:41:51Z RG3FG1NB4KRX9RCW57B9C4T6C9-m2-bc1be9cb edit actor=m2+mac-coordinator targets=acp-transport
- 2026-08-23T15:13:01Z WHMH0NDJ9JFGTHGCC72PM1Z6YH-m2-bc1be9cb edit actor=m2+mac-coordinator targets=acp-transport
- 2026-08-23T15:13:09Z KJYY737RSPEGT6RFHKC3WSWT8E-m2-bc1be9cb release actor=m2+mac-coordinator targets=acp-transport
Integrity: sha256=bd0c3ef2bd33cce75b0a42f7d3151a05930de32ed4e9d1d41a862818f79c0ef0
