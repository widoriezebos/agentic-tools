# testing-contract-owns-record-paths

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: no seat can land a register, brief or ruling until this is fixed, which is the whole carriage of judgment; novelty 1: one surface entry and one fixture in a contract that already has twelve surfaces; exposure 3: every seat's every record landing; accumulation 1: nothing has been built on it yet"
- Tier: 3
- Intent: Since 1b12f534 (coordinator-loop-prevention, m1c) commit.sh consumes 'metasystem test verify' whenever testing.json exists, and the contract's surfaces own only code and scripts: a landing whose only change is a record under metasystem/plans, metasystem/records or metasystem/memory is refused with 'delivery impact is unresolved: no surface owns changed path ...', so every register carriage on every seat (critique registers, briefs, rulings, receipts) is now impossible. Observed 2026-09-09 23:55Z on m1b landing plans/goal-abandoned-with-a-reason-build1-read1-brief.md after steward arm re-authenticated the engine. DONE means the contract names a surface for the record trees (plans, records, memory, docs) whose selected groups are the static placeholder scan and nothing else, a record-only candidate resolves to that surface and lands on the fast gate, and a fixture proves a record-only tree resolves.
- Origin: main
- Next step: WIDENED 2026-09-10 01:30Z: not only records. The slice-1 candidate of goal-abandoned-with-a-reason (43 files, two closing reads clean, both shell beds green) cannot take a schema-2 receipt: 'landing test-receipt --mode auto' refuses 'delivery impact is unresolved: no surface owns changed path' for 12 code paths, among them metasystem/cmd/metasystem/goal.go, metasystem/cmd/metasystem/goalsync_mutations.go, metasystem/internal/channel/report.go (full list in m1b's report). Legacy schema-1 receipts are refused once the contract exists (tierone.go), so there is no lawful landing route for any change touching an unowned path, on any seat. DONE now also means: every path under metasystem/cmd, metasystem/internal and metasystem/scripts is owned by some surface (a catch-all surface with the full battery is acceptable as the first state), and a fixture proves the contract owns every tracked source path. Opened by m1b; the contract is m1c's (coordinator-loop-prevention, 1b12f534). Approve and rank first thing.
- OpenedAt: 2026-09-09T21:49:17Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T21:49:17Z HM3T2QVDJE4SEVNYC59QJ148Y3-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
- 2026-09-09T22:59:34Z PTANMVFV3XQ985PWAW2KRHW6FJ-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
Integrity: sha256=87c9228f6997431c10e303ed0437eb30b7edbd269b83453b7cf7fbc745356566
