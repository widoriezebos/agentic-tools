# hung-proof-attempts-end-at-their-deadline

- State: queued
- Priority: 1
- Sequence: 3
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a hung attempt holds a reservation and a seat's landing for a day; novelty 1: the deadline and watchdog exist and only their coverage changes; exposure 3: every attempt on every seat; accumulation 1: no new mechanism"
- Tier: 3
- Intent: Five m1b attempts hung 1.8 to 26.8 hours after a section died with exit 141 (proof-mtwiqdlq, proof-mtwlmbl5, proof-mtwp0iuu, proof-mtwvjs0d, proof-mtvdviee, 10 to 11 September); an m1e attempt stayed live 24.5 hours after its launcher died (proof-mtvl0alu); 23 process records still say running; 16 attempts ended launcher-killed without a terminal (proof-attempts.md section 6). Since the audit, c08fe2cb bounds go test groups by consumption, and proof-groups-detect-hangs-by-progress-not-the-clock owns elapsed-time patience; this goal owns liveness reconciliation, not clocks. DONE means: an attempt whose launcher pid is dead, whose section process ended without an end event, or whose supervisor verdict is dead or stopped is terminalized as failed with the cause within one watchdog tick; a reservation is released only after confirmed termination of every child process or a recorded custody transfer, never on elapsed time alone; metasystem status shows no live attempt whose launcher is dead; a fixture kills a section mid-run and the attempt ends with its evidence preserved and its reservation released. Goal 3 of plans/delivery-efficiency-plan.md. RUNTIME INDEPENDENCE (plan principle 0; architecture doctrine of 2026-08-16: correctness never depends on an accelerator): this holds for every runtime the roster can host today (claude, codex, devin) and for future adapters such as opencode; the mechanism lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime's hook, harness, CLI or transcript format; a native facility may accelerate it through its adapter, and an agent on any runtime with no accelerator gets the same guarantee from the records and the verb alone; where the goal touches a runtime path, DONE is proven on at least two runtimes.
- Origin: human
- Next step: Read the watchdog and launcher in internal/proofrun and the section adapter's end-event handling; agree the liveness contract with the landed consumption supervisor (c08fe2cb); design the three reconciliation paths; fixture; land with its own battery.
- OpenedAt: 2026-09-11T15:44:57Z
- Revision: 9
- Pinned: m1e
- BudgetExceptions: 0

History:
- 2026-09-11T15:44:57Z G00GBWKD64KA1ZT4J6WDGWFCSB-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=hung-proof-attempts-end-at-their-deadline
- 2026-09-11T15:46:56Z 7ZCH1E0KCPY6ZZ18JTDK66Q29T-m1-c6925449 set-pin actor=human:Wido targets=hung-proof-attempts-end-at-their-deadline
- 2026-09-11T15:46:59Z 57R7RXDCG69QZ86PM2WM8V8ZT1-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,hung-proof-attempts-end-at-their-deadline,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=hung-proof-attempts-end-at-their-deadline from=unranked to=1:3 requested-sequence=3
- 2026-09-11T15:49:06Z AQGP66AJX5BDEEGY8TYHP47DVY-m1-c6925449 approve actor=human:Wido targets=hung-proof-attempts-end-at-their-deadline
- 2026-09-11T16:13:04Z 7N37D1HZY6Q26JPNP2AE4XQBHR-m1-c6925449 unapprove actor=human:Wido targets=hung-proof-attempts-end-at-their-deadline reason=Wido 2026-09-11: Codex critique round 1 of the delivery-efficiency plan accepted; intent amended before re-approval
- 2026-09-11T16:16:24Z 1BXAQDCQF5NN6081ACS2HRQ5N4-m1e-ab5421f5 edit actor=m1e+main-1789143377-71578-b9c0d2 targets=hung-proof-attempts-end-at-their-deadline
- 2026-09-11T16:18:51Z 50WPMJCAAPEZKP261JRMKRA7JK-m1-c6925449 approve actor=human:Wido targets=hung-proof-attempts-end-at-their-deadline
- 2026-09-11T20:30:27Z 8FEM7KDHD01K2W2S8AV7PFSZGH-m1-c6925449 unapprove actor=human:Wido targets=hung-proof-attempts-end-at-their-deadline reason=Wido 2026-09-11: every program goal states runtime independence (claude, codex, devin, future adapters); intent amended before re-approval
- 2026-09-11T20:31:48Z J3QFJC5HS7B2P9CHG9RP64EKNE-m1e-3ca84c28 edit actor=m1e+main-1789158700-39729-b9c0d2 targets=hung-proof-attempts-end-at-their-deadline
Integrity: sha256=7a03b5b4fac0ab0da7d1d093175ede4bed914ec56bff80453840ab53c023fb3c
