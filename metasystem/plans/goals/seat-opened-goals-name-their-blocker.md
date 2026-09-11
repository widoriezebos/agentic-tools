# seat-opened-goals-name-their-blocker

- State: queued
- Priority: 1
- Sequence: 7
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a refused open is re-issued with its blocker; novelty 1: dependency edges exist; exposure 2: seat opens only; accumulation 1: one verb"
- Tier: 2
- Intent: Seats opened 165 goals in five days against 62 concluded, growing the backlog by 117 (ledger.md section 5 of the delivery deep dive). DONE means: goal open from a seat (origin main) requires --blocks naming the claimed goal it unblocks, records the dependency edge, and refuses otherwise with the ruling; human-origin opens are unchanged; docs/orchestration.md carries the ruling that a seat opens only the defect that blocks its current goal and every other goal is opened by a person. The ruling is Wido's to confirm on the design page; the seat builds it as stated unless he names another. Goal 6 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read OpenTiered in internal/goal/verbs.go and the open flags in cmd/metasystem/goalsync_mutations.go, add the flag, the edge and the refusal, the docs line, land.
- OpenedAt: 2026-09-11T15:45:05Z
- Revision: 6
- Pinned: m1e
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:05Z 5W0FM0WGJ00Q1A7X06ESJFVC4X-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=seat-opened-goals-name-their-blocker
- 2026-09-11T15:47:14Z X1PRNWZEXP7ZK7V0Q8NE34S5BY-m1-c6925449 set-pin actor=human:Wido targets=seat-opened-goals-name-their-blocker
- 2026-09-11T15:47:18Z CW7PT0773AXANF6XTDSKH1KX37-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,seat-opened-goals-name-their-blocker,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=seat-opened-goals-name-their-blocker from=unranked to=1:6 requested-sequence=6
- 2026-09-11T15:49:14Z D0Z826ZYBY6WV1HRZCFH6B9Q4C-m1-c6925449 approve actor=human:Wido targets=seat-opened-goals-name-their-blocker
- 2026-09-11T15:50:51Z M4GZC1Q99QFRH9T13HSY774HAR-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,budget-extends-by-consumption-and-breach-parks,burn-without-delivery-tripwire,capped-round-continues-instead-of-restarting,carried-landing-debt-and-cap,coordinator-context-stays-under-budget,coordinator-wakes-on-events-not-polls,critique-closes-on-folded-proof,deep-battery-under-ten-minutes,delegate-job-liveness,delegate-rounds-reuse-a-warm-gate,delivery-receipt-stops-at-first-failure,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,land-ready-work-lands-without-a-claim-slot,landing-receipt-survives-records-drift,landing-refuses-without-its-receipt-line,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-groups-detect-hangs-by-progress-not-the-clock,proof-harness-process-custody,retained-proof-reuse-crosses-claims-and-attempts,seat-opened-goals-name-their-blocker,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=landing-refuses-without-its-receipt-line from=1:6 to=1:7 requested-sequence=5
- 2026-09-11T16:13:11Z BDBGP0J34XP4KAE6VGDVG6HQYY-m1-c6925449 unapprove actor=human:Wido targets=seat-opened-goals-name-their-blocker reason=Wido 2026-09-11: Codex critique round 1 of the delivery-efficiency plan accepted; intent amended before re-approval
Integrity: sha256=c8ffbe5a00deb4afeb6ab16c6d6ed7865571f67efb7e90c68c58cee11318212e
