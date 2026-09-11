# coordinator-context-stays-under-budget

- State: approved
- Priority: 1
- Sequence: 14
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: an over-bounded output hides evidence the seat needs; novelty 2: bounded verb output is a new discipline; exposure 3: every seat's every turn; accumulation 1: verbs and docs"
- Tier: 3
- Intent: Coordinator calls carried 386 to 507 thousand prompt tokens each, 5.33 billion cache-read tokens in five days, 19 compactions dropping 14.85 million tokens, while delegates ran efficiently at 76 to 126 thousand (claude-sessions.md sections 1a and 4b of the delivery deep dive); goal list prints 9 MB of JSON. DONE means: a coordinator turn's context stays under 150 thousand tokens by construction: tool output over a size bound lands in a file with a summary line, ledger and status verbs print bounded summaries with --json for detail, briefs and evidence are referenced by path, and the seat's instruction set says so; proven by a week of coordinator per-call prompt tokens with a median under 150 thousand and no compaction. Goal 14 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Measure which tool outputs dominate the context (goal list, watch, test logs, land output), bound them at the verb, write the instruction change, critique, build, land.
- OpenedAt: 2026-09-11T15:45:19Z
- Revision: 4
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T15:49:36Z revision=4 opid=FCRJCZR5R6QY88PSK1BR16ME9T-m1-c6925449 authority=proven digest=aef011606fae6fab2b48ef2aa1a89fdc8ec8a33a920666a2d07626b99a357f78

History:
- 2026-09-11T15:45:19Z 5F6TSCSSSW5B4H5JVZDBKEWHPE-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=coordinator-context-stays-under-budget
- 2026-09-11T15:48:17Z JY6QJN1S56GD8Q82K5KW1R50ZH-m1-c6925449 set-pin actor=human:Wido targets=coordinator-context-stays-under-budget
- 2026-09-11T15:48:21Z 8NDX8QPSQXDM5YE46TQMM40YKA-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,coordinator-context-stays-under-budget,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=coordinator-context-stays-under-budget from=unranked to=1:14 requested-sequence=14
- 2026-09-11T15:49:36Z FCRJCZR5R6QY88PSK1BR16ME9T-m1-c6925449 approve actor=human:Wido targets=coordinator-context-stays-under-budget
Integrity: sha256=291f887d1d38a5fda7d4bd3882398e50522b618a6e84c96babe1db0b40cfb1f6
