# testing-contract-owns-record-paths

- State: approved
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: no seat can land a register, brief or ruling until this is fixed, which is the whole carriage of judgment; novelty 1: one surface entry and one fixture in a contract that already has twelve surfaces; exposure 3: every seat's every record landing; accumulation 1: nothing has been built on it yet"
- Tier: 3
- Intent: Since 1b12f534 (coordinator-loop-prevention, m1c) commit.sh consumes 'metasystem test verify' whenever testing.json exists, and the contract's surfaces own only code and scripts: a landing whose only change is a record under metasystem/plans, metasystem/records or metasystem/memory is refused with 'delivery impact is unresolved: no surface owns changed path ...', so every register carriage on every seat (critique registers, briefs, rulings, receipts) is now impossible. Observed 2026-09-09 23:55Z on m1b landing plans/goal-abandoned-with-a-reason-build1-read1-brief.md after steward arm re-authenticated the engine. DONE means the contract names a surface for the record trees (plans, records, memory, docs) whose selected groups are the static placeholder scan and nothing else, a record-only candidate resolves to that surface and lands on the fast gate, and a fixture proves a record-only tree resolves.
- Origin: main
- Next step: WIDENED AGAIN 2026-09-10 04:45Z: slice 3 of goal-abandoned-with-a-reason (the landing side) also touches metasystem/scripts/agents/landing-promotion.json, which no surface owns, so that chain is blocked too. Three chains on m1b wait on this one goal: gawr-build1 (slices 1 and 2, closed clean), the slice-3 chain (building), and the parked landing-receipt-survives-records-drift chain (one unowned path, static-reproof-fixtures.sh). A catch-all surface for every tracked source path under metasystem/, selecting the full battery, is the first acceptable state. Opened by m1b; the contract is m1c's. Approve and rank first thing.
- OpenedAt: 2026-09-09T21:49:17Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T07:29:14Z revision=4 opid=4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 authority=proven digest=3ba72d9bfc753d2638ad986ff5cb13e30a10404dbcad76e53800bdbef94dcb8b

History:
- 2026-09-09T21:49:17Z HM3T2QVDJE4SEVNYC59QJ148Y3-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
- 2026-09-09T22:59:34Z PTANMVFV3XQ985PWAW2KRHW6FJ-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
- 2026-09-10T02:33:25Z 8CKWTEFNT6FWDKQRK57R73K9HF-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
- 2026-09-10T07:29:14Z 4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 approve actor=human:Wido targets=carried-landing-debt-and-cap,enrollment-binds-by-the-skew-rule-not-the-tip,human-carried-landing-verb,receipt-beds-run-the-candidate-engine,skew-preflight-knows-a-receipt-worktree,testing-contract-owns-record-paths,the-metasystem-validates-itself-with-itself
Integrity: sha256=505b26fc139cb22ae47c88764c9365ddd89eee198946ba3636f763349b9c9030
