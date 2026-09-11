# receipt-admission-caps-concurrent-batteries

- State: approved
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a wrong cap starves landings or lets batteries collide as today; novelty 2: attempt reservations exist per checkout but nothing is host-wide; exposure 3: every receipt on every seat; accumulation 1: the battery content is unchanged"
- Tier: 3
- Intent: Concurrent batteries on one host stop failing each other, within ruling R-35-m3 (no machine reservations between seats; failing when slow is the defect). Evidence 6 to 11 September (proof-attempts.md section 4): goal-full-coverage failed 8 percent with no overlapping attempt and 53, 67 and 100 percent with 1, 2 and 3 or more; attempt failure 32 percent alone against 57 percent at 3 or more overlaps; heavy groups up to two times slower under overlap. Since the audit, c08fe2cb bounds go test groups by consumption, not the clock, which removes the largest load failure. DONE means: (1) every attempt record carries host load and the count of overlapping attempts at start and end, so a load-caused failure is attributable in the record and the retry decision names it; (2) a load-caused failure of a group already bounded by consumption is filed as a defect in that group's patience, never as a reason to serialize seats; (3) an admission cap on TOP-LEVEL delivery attempts (nested fixture receipts inside a battery exempt, a slot released only on confirmed termination) is designed and proven deadlock-free with two full batteries and their nested receipts completing side by side, but is BUILT only if Wido supersedes R-35-m3 on the design page, which puts that question to him with the audit numbers. Absorbs the receipt-admission part of machine-concurrency-governor, which keeps dispatch parallelism as load-tolerant machinery. Goal 1 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Wido's word recorded 2026-09-11 (R-92-m1e): no admission cap for now; build the attribution (host load and overlapping-attempt count in every attempt record; load failures of consumption-bounded groups filed as patience defects) and put the cap question again with a week of post-c08fe2cb numbers. Next: claim when deep-battery-under-ten-minutes ends; design page names R-92-m1e.
- OpenedAt: 2026-09-11T15:44:50Z
- Revision: 8
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T16:18:44Z revision=7 opid=MR82975FP1C9JVF7ZS7YEBMQTS-m1-c6925449 authority=proven digest=31ab67526b103a10219722699bdf9042e95e9a1e8e84d0ae187755a7339bb028

History:
- 2026-09-11T15:44:50Z ATQJYESKCTSPAS92AEEAW0TKS3-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=receipt-admission-caps-concurrent-batteries
- 2026-09-11T15:46:40Z 3TKA7BCN8ABA87V8RE4843HSYM-m1-c6925449 set-pin actor=human:Wido targets=receipt-admission-caps-concurrent-batteries
- 2026-09-11T15:46:44Z X5BAA136JBXHCVE0A45HFHNACB-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,receipt-admission-caps-concurrent-batteries,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=receipt-admission-caps-concurrent-batteries from=unranked to=1:1 requested-sequence=1
- 2026-09-11T15:48:59Z DWMGQ992CQRS9HZD15292B01R0-m1-c6925449 approve actor=human:Wido targets=receipt-admission-caps-concurrent-batteries
- 2026-09-11T16:12:56Z Q4VDEEMPVWX68HSRRYZ8KBVXFM-m1-c6925449 unapprove actor=human:Wido targets=receipt-admission-caps-concurrent-batteries reason=Wido 2026-09-11: Codex critique round 1 of the delivery-efficiency plan accepted; intent amended before re-approval
- 2026-09-11T16:16:17Z SXAC0HTV3ETPBGR4CRYSW2XJ7Q-m1e-ab5421f5 edit actor=m1e+main-1789143377-71578-b9c0d2 targets=receipt-admission-caps-concurrent-batteries
- 2026-09-11T16:18:44Z MR82975FP1C9JVF7ZS7YEBMQTS-m1-c6925449 approve actor=human:Wido targets=receipt-admission-caps-concurrent-batteries
- 2026-09-11T20:15:37Z VJGH901E4AZJXM3TJDV3GKKNMB-m1e-892cdaec edit actor=m1e+main-1789030447-51011-5722fc targets=receipt-admission-caps-concurrent-batteries
Integrity: sha256=4f921c56421901725512af2f284edf2a82c5492c4b801fabd6cb5c1a074c39b0
