# acp-transport

- State: claimed
- Intent: ACP as the delegate transport, retiring the dangerous-mode waiver
- Origin: main
- Next step: Appetite: 8h, ~5h45 spent — RESUME ATTEMPTED 2026-08-23 after Wido's seal word, REFUSED: 'cohort resume refused: repetition 1 contract has no Approval line' (verbatim). The cohort target's contract is still the unsigned provision state — git log in targets/1 shows only 'Add unsigned bm-2d mission contract' / 'Provision benchmark bm-2d instruments', zero Approval lines in plans/mission-bm-2d.contract.md, no seal commit. THE SIGNING NEEDS WIDO AGAIN — likely the seal happened in another checkout or the commit/push step was missed. Exact sequence, in the VM (limactl shell metasystem-debian-amd64): cd /home/wido.guest/trials/cohorts/bm-2d-20260823t080725z-3548802/targets/1 && scripts/assert-mission.sh --seal --file plans/mission-bm-2d.contract.md (prints the contract hash; expect the ledgerNoGainBudget=5-below-cycles=8 warning to need its budget/fuse ruling first per issues #4/#8), then append 'Approval: name=...; date=...; contract-sha256=<printed hash>' to that file, git add + COMMIT + git push origin main (the target's sibling bare origin). Then any coordinator re-claims and resumes: METASYSTEM_TRIALS_ROOT=/home/wido.guest/trials benchmark/run-cohort.sh --resume bm-2d-20260823t080725z-3548802. Claim released meanwhile; machine m2 standing by.
- OpenedAt: 2026-08-20T00:25:00Z
- Revision: 13
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-23T08:39:50Z

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
Integrity: sha256=8453880d140d2ae36e9f3b0adc71c58f392ba86126c2b097b5dbc47b7ecf7736
