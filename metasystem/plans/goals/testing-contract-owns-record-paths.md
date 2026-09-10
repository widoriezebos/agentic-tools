# testing-contract-owns-record-paths

- State: claimed
- Priority: 1
- Sequence: 3
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: no seat can land a register, brief or ruling until this is fixed, which is the whole carriage of judgment; novelty 1: one surface entry and one fixture in a contract that already has twelve surfaces; exposure 3: every seat's every record landing; accumulation 1: nothing has been built on it yet"
- Tier: 3
- Intent: Since 1b12f534 (coordinator-loop-prevention, m1c) commit.sh consumes 'metasystem test verify' whenever testing.json exists, and the contract's surfaces own only code and scripts: a landing whose only change is a record under metasystem/plans, metasystem/records or metasystem/memory is refused with 'delivery impact is unresolved: no surface owns changed path ...', so every register carriage on every seat (critique registers, briefs, rulings, receipts) is now impossible. Observed 2026-09-09 23:55Z on m1b landing plans/goal-abandoned-with-a-reason-build1-read1-brief.md after steward arm re-authenticated the engine. DONE means the contract names a surface for the record trees (plans, records, memory, docs) whose selected groups are the static placeholder scan and nothing else, a record-only candidate resolves to that surface and lands on the fast gate, and a fixture proves a record-only tree resolves.
- Origin: main
- Next step: CLAIMED BY m1e 2026-09-10 11:10Z (Wido's direction: work the efficiency list; this gate blocks its first item, whose chain is closed and land-ready but refused for scripts/agents/static-reproof-fixtures.sh). DECISION: Select is a union of every matching surface, so a wildcard catch-all would make every landing pay the full battery; the remedy is a FALLBACK: contract field 'fallback' names a surface that owns any changed path no other surface matches (uncertainty raised only when no fallback is declared); the fallback surface has empty paths (owns by exclusion) and is the only surface allowed that; testing.json declares fallback=residual with standard/deep/critical equal to the union of every other surface's groups (the full battery by construction), pinned by a test over the committed contract; unit tests for unowned-path selection, missing-fallback refusal, and owned paths never affecting the fallback. Build dispatched (Sol, DESIGN-BEARING, brief in m1e's scratchpad tcorp/build-brief.md). THEN: closing read (Opus) in parallel with the receipt on the staged candidate; land; goal done with the journey chapter; then unpark and land landing-receipt-survives-records-drift. m1c owns the contract's design: this is the first acceptable state its goal record named, not a redesign.
- OpenedAt: 2026-09-09T21:49:17Z
- Revision: 8
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T07:29:14Z revision=4 opid=4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 authority=proven digest=3ba72d9bfc753d2638ad986ff5cb13e30a10404dbcad76e53800bdbef94dcb8b
- Sliced: machine=m1e lineage=main-1789030447-51011-5722fc revision=6 at=2026-09-10T10:59:18Z
- Claimed: machine=m1e lineage=main-1789030447-51011-5722fc at=2026-09-10T10:57:12Z revision=6 accountingRevision=6
- StopCapability: generation=6 revision=6 machine=m1e claimEpoch=1 fenceEpoch=0

History:
- 2026-09-09T21:49:17Z HM3T2QVDJE4SEVNYC59QJ148Y3-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
- 2026-09-09T22:59:34Z PTANMVFV3XQ985PWAW2KRHW6FJ-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
- 2026-09-10T02:33:25Z 8CKWTEFNT6FWDKQRK57R73K9HF-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=testing-contract-owns-record-paths
- 2026-09-10T07:29:14Z 4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 approve actor=human:Wido targets=carried-landing-debt-and-cap,enrollment-binds-by-the-skew-rule-not-the-tip,human-carried-landing-verb,receipt-beds-run-the-candidate-engine,skew-preflight-knows-a-receipt-worktree,testing-contract-owns-record-paths,the-metasystem-validates-itself-with-itself
- 2026-09-10T07:29:41Z K5YA955NH7PZHBG9N53NJHZN1B-m1b-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,governed-exhaustion-reprojection,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,testing-contract-owns-record-paths,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=testing-contract-owns-record-paths from=unranked to=1:3 requested-sequence=3
- 2026-09-10T10:57:12Z NZZJG6GSBN12VCFJ6AMSDSKBS4-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=testing-contract-owns-record-paths
- 2026-09-10T10:59:18Z 03B637Q4WTBFANWTJR2BSJEZ6M-m1e-892cdaec slice-start actor=m1e+main-1789030447-51011-5722fc targets=testing-contract-owns-record-paths
- 2026-09-10T10:59:50Z CHH92JCRQD1HFFBPS46CAATHSG-m1e-892cdaec edit actor=m1e+main-1789030447-51011-5722fc targets=testing-contract-owns-record-paths
Integrity: sha256=51ee9b9a177e74bcb0421521fdbe4b2eff333d9157ebb833ceb2b57e83dca0d9
