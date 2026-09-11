# delivery-receipt-stops-at-first-failure

- State: queued
- Priority: 1
- Sequence: 8
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the receipt is the landing judge; novelty 1: the runner loop exists and gains a purpose-dependent stop; exposure 3: every delivery attempt; accumulation 1: cadence unchanged"
- Tier: 3
- Intent: 31 failed attempts consumed 26 hours, 20.7 of them running groups that had already passed, because the runner continues after the first failure under ruling R-16 (proof-attempts.md finding 3 of the delivery deep dive). DONE means: a delivery-purpose attempt stops launching new groups at the first failed group, records the remaining groups as not-run with the reason and preserves the failure evidence; cadence-purpose attempts keep continue-and-collect; the retry launches only the failed and not-run groups and reuses the passed groups of the failed attempt; proven by a fixture with a failing second group and, in the field, by failed-attempt mean duration under 15 minutes over a week. The amendment of R-16 for the delivery purpose is Wido's to confirm on the design page. Goal 8 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read RunTestPlan in internal/proofrun/test_build.go and R-16 in memory/rulings.md, design the purpose-dependent stop and the retry projection, critique, build, land with its own battery.
- OpenedAt: 2026-09-11T15:45:11Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:11Z RKPY1HJBS8V1X9RAP3GX84EXYT-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=delivery-receipt-stops-at-first-failure
- 2026-09-11T15:47:30Z BBEGJX7GSGNED7D58P9FT29YSG-m1-c6925449 set-pin actor=human:Wido targets=delivery-receipt-stops-at-first-failure
- 2026-09-11T15:47:34Z BF0FVV403JF278KZJSGB8JWVAC-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,delivery-receipt-stops-at-first-failure,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=delivery-receipt-stops-at-first-failure from=unranked to=1:8 requested-sequence=8
Integrity: sha256=57cb0211d3ece11983e9765a7c57614e446019d115254a591a391ca91d960b7c
