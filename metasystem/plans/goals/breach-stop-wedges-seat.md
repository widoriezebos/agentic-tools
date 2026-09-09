# breach-stop-wedges-seat

- State: approved
- Priority: 1
- Sequence: 7
- Intent: A breach-stopped claim wedges the whole machine: release is refused (only resume clears the fence, and resume is a human act), while the one-claim quota rejects every new claim as long as the stopped claim stands - so one budget breach freezes the seat until a human types resume, violating the standing order that a parked stream never prevents claiming the next item. DONE means a breach-stopped goal parks without holding the quota slot, or release becomes lawful on a stopped claim, with the fence on RESUMING that goal preserved
- Origin: main
- Next step: Appetite: 1h. Discovered live 2026-09-01 morning (records/misc/idle-loss-2026-09-01.md, the wedge specimen is in the ledger refusals at tips 4d8bff0e/88599665). CONSTRAINT: the budget law stays intact - only the quota interaction changes; a human word must still gate resuming the breached goal itself. Prove with a fixture: breach-stop a claim, then claim another goal successfully
- OpenedAt: 2026-09-01T06:49:32Z
- Revision: 6
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=a5613449ea270548e47fef27b8599ce8535d99b3cedbe90e7dc356ce2a6ecae1

History:
- 2026-09-01T06:49:32Z FP70QX8HBN6WY1V8PX52K60QHN-m3-a5da21ff open actor=m3+mac-m3 targets=breach-stop-wedges-seat
- 2026-09-01T20:26:21Z 7D9VYWG8J44X9MSVSGN4Q491G4-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-stop-wedges-seat
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=breach-stop-wedges-seat reason=sweep
- 2026-09-08T15:55:43Z NB2KYHBCNTG2QAHP37A03X2JVX-m1-7cd0bd60 set-priority actor=human:Wido targets=breach-stop-wedges-seat reason=priority-order subject=breach-stop-wedges-seat from=unranked to=1:7 requested-sequence=7
- 2026-09-08T17:03:35Z YZH9GDDZJGK4DB0ENK5CGSXADW-m1b-c6925449 done actor=human:Wido targets=account-provenance,actionable-metrics,backlog-ordered-by-priority,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,host-runtime-setup,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:7 to=1:6
- 2026-09-09T07:32:44Z MXJ2MBB9CW7M5FQMAK5B22KTC7-m1b-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=landing-receipt-survives-records-drift from=1:6 to=1:7 requested-sequence=3
Integrity: sha256=3f4b95c7ff06448c7593c85d8c48eb1e4d8937d2354e2201b776a036b0df5996
