# goal-records-owned-by-the-testing-contract

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: no records can land, so every goal's evidence trail stays untracked; novelty 1: one surface entry in an existing contract; exposure 3: every seat's every goal; accumulation 2: untracked records pile up in every checkout until it lands"
- Tier: 3
- Intent: Since 1b12f534 the testing contract (metasystem/testing.json) owns no plans/**, records/** or memory/rulings.md, so every register-carriage landing of a goal's briefs, dispositions and shared append-only records is refused at land.sh's test verify --purpose delivery with 'delivery impact is unresolved: no surface owns changed path'. DONE means a goal-records surface owns those paths with the same static groups as the instructions surface, a records-only carriage resolves as delivery again, and the seat's test-policy tests pin that plans/ and records/ paths are owned.
- Origin: main
- Next step: One Sol round adding the goal-records surface (metasystem/plans/**, metasystem/records/**, metasystem/memory/rulings.md, metasystem/memory/receipts.log; groups section/static-contract-audits) to metasystem/testing.json with a testpolicy test that a records-only change selects delivery; one Opus review; land with --chain. Found by m1 on 2026-09-10 00:25 local landing the dispatch-cap-necessity records; m1c messaged (held for approval). Wido approves at the terminal.
- OpenedAt: 2026-09-09T22:15:49Z
- Revision: 25
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T09:58:34Z revision=13 opid=FMK6TTJWTWPG2B58S27RTBMSQR-m1-c6925449 authority=proven digest=4c3c0cd054441164af8c62d982c44dd8f37f4d6c9c2f46b8fd4e53a9d894edfc
- Sliced: machine=m1 lineage=main-1788940932-18533-7fa6c2 revision=3 at=2026-09-10T05:50:10Z
- AcceptedRisk: finding=HAS-05 chain=ha-surfacecrit2-20260910 by=Wido opid=W4SJTD2MHV2NWKA0KZWYR7G03V-m1-c6925449
- Claimed: machine=m1 lineage=main-1788940932-18533-7fa6c2 at=2026-09-10T13:26:50Z revision=25 accountingRevision=25
- StopCapability: generation=25 revision=25 machine=m1 claimEpoch=6 fenceEpoch=0

History:
- 2026-09-09T22:15:49Z 3DBVQYVHVWMX8YZQ4FQAGE6VHY-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T05:48:24Z J91KS4812YR2Z1GKMWPG86SSSM-m1-c6925449 approve actor=human:Wido targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T05:50:03Z YXSZJEFAXMCBVCM8S08PX6EWAW-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T05:50:10Z WYRSX5XGC356BGDWK8EGT95H5N-m1-1701c13c slice-start actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T07:16:33Z 9PA4YH32AB1KCKNJ8RYG36250C-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T07:21:10Z 3VCWDH2PNWMNC7Q8H7V5Y8KJ69-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T07:21:20Z W4MEW086C9BYAZAR6RPQWCWJC8-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T08:45:10Z 2MTH2ZAV9CA9KMB2XC4DCJ1741-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T08:46:53Z TV6AKYDMSMG6MED6BNSAFXV0BZ-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T08:53:44Z MMRYBQJK0NATPRCPQ1HXAVFHP3-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T09:53:24Z RADDFZ3EMWA9ANNTM2ZZG4BBQ7-m1-1701c13c done actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T09:54:13Z VFAP4RYH2G4NHH9ZDHS5D4D26T-m1-1701c13c reopen actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T09:58:34Z FMK6TTJWTWPG2B58S27RTBMSQR-m1-c6925449 approve actor=human:Wido targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T09:58:45Z Q8C7KXYJ624762CKB8EW0114YN-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T10:04:14Z G8J4YK0FVPE4J2301MERDHFZPV-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T10:18:36Z KG8RMV0W9S09Z2RT69J6011907-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T11:31:28Z FXH5E47YZVMWGZ2Z40RAB3GF1J-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T13:19:03Z P8KQDB2AB9QDK5MV3W2XB5P2FQ-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T13:19:07Z 3SJKDDZDSZA80DJS5JFBQ87546-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T13:23:17Z GYQJJQGKH4F6X6KER6RXKJA0P0-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T13:23:22Z W4SJTD2MHV2NWKA0KZWYR7G03V-m1-c6925449 accept-risk actor=human:Wido targets=goal-records-owned-by-the-testing-contract reason=Accepted for now: one subtest in internal/humanauthority (Terminal app login is the session leader) still skips outside Darwin, so a Linux seat cannot pass humanauthority-standard until goal skipped-test-fails-its-group-on-the-other-os lands; Linux seats already fail that group on main (three Darwin-only skips) and runtime-owner-standard for the same reason (HAS-06), so no seat is made worse; this Mac is the landing seat for the hp-terminal chain and its gate passes the package skip-free. Recorded under Wido's delegation of 2026-09-10 by the m1 seat.
- 2026-09-10T13:23:37Z STH1NVZBX3E5VE9NQ2PTN67A99-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T13:24:19Z SA4MKHZ2947H9Z88ZJCAYW7WPW-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T13:24:23Z 3BGA75MVCEEAHPM24T8XG6G51Y-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
- 2026-09-10T13:26:50Z VCNZJM02J8XPVY137NRBRCARNV-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
Integrity: sha256=22a6628864479c2c986982bb13df57b0117d91527fc65769d0619f1a746ebcf9
