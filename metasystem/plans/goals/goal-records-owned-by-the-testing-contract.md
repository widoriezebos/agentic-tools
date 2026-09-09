# goal-records-owned-by-the-testing-contract

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: no records can land, so every goal's evidence trail stays untracked; novelty 1: one surface entry in an existing contract; exposure 3: every seat's every goal; accumulation 2: untracked records pile up in every checkout until it lands"
- Tier: 3
- Intent: Since 1b12f534 the testing contract (metasystem/testing.json) owns no plans/**, records/** or memory/rulings.md, so every register-carriage landing of a goal's briefs, dispositions and shared append-only records is refused at land.sh's test verify --purpose delivery with 'delivery impact is unresolved: no surface owns changed path'. DONE means a goal-records surface owns those paths with the same static groups as the instructions surface, a records-only carriage resolves as delivery again, and the seat's test-policy tests pin that plans/ and records/ paths are owned.
- Origin: main
- Next step: One Sol round adding the goal-records surface (metasystem/plans/**, metasystem/records/**, metasystem/memory/rulings.md, metasystem/memory/receipts.log; groups section/static-contract-audits) to metasystem/testing.json with a testpolicy test that a records-only change selects delivery; one Opus review; land with --chain. Found by m1 on 2026-09-10 00:25 local landing the dispatch-cap-necessity records; m1c messaged (held for approval). Wido approves at the terminal.
- OpenedAt: 2026-09-09T22:15:49Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T22:15:49Z 3DBVQYVHVWMX8YZQ4FQAGE6VHY-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=goal-records-owned-by-the-testing-contract
Integrity: sha256=9a7923be683eea596048a57f67ee911a2031e1bf81a3d6cd72d4b3cc54db98ad
