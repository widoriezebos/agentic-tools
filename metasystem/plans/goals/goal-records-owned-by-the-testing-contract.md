# goal-records-owned-by-the-testing-contract

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: no records can land, so every goal's evidence trail stays untracked; novelty 1: one surface entry in an existing contract; exposure 3: every seat's every goal; accumulation 2: untracked records pile up in every checkout until it lands"
- Tier: 3
- Intent: Since 1b12f534 the testing contract (metasystem/testing.json) owns no plans/**, records/** or memory/rulings.md, so every register-carriage landing of a goal's briefs, dispositions and shared append-only records is refused at land.sh's test verify --purpose delivery with 'delivery impact is unresolved: no surface owns changed path'. DONE means a goal-records surface owns those paths with the same static groups as the instructions surface, a records-only carriage resolves as delivery again, and the seat's test-policy tests pin that plans/ and records/ paths are owned.
- Origin: main
- Next step: One Sol round adding the goal-records surface (metasystem/plans/**, metasystem/records/**, metasystem/memory/rulings.md, metasystem/memory/receipts.log; groups section/static-contract-audits) to metasystem/testing.json with a testpolicy test that a records-only change selects delivery; one Opus review; land with --chain. Found by m1 on 2026-09-10 00:25 local landing the dispatch-cap-necessity records; m1c messaged (held for approval). Wido approves at the terminal.
- OpenedAt: 2026-09-09T22:15:49Z
- Revision: 14
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T09:58:34Z revision=13 opid=FMK6TTJWTWPG2B58S27RTBMSQR-m1-c6925449 authority=proven digest=4c3c0cd054441164af8c62d982c44dd8f37f4d6c9c2f46b8fd4e53a9d894edfc
- Sliced: machine=m1 lineage=main-1788940932-18533-7fa6c2 revision=3 at=2026-09-10T05:50:10Z
- Claimed: machine=m1 lineage=main-1788940932-18533-7fa6c2 at=2026-09-10T09:58:45Z revision=14 accountingRevision=14
- StopCapability: generation=14 revision=14 machine=m1 claimEpoch=6 fenceEpoch=0

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
Integrity: sha256=d9f1d2b5b0c6d3409a8f2838483862f2dd5ed84c955220614e8db29eff249520
