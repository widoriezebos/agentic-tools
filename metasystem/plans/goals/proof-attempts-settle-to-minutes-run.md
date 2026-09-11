# proof-attempts-settle-to-minutes-run

- State: queued
- Priority: 1
- Sequence: 7
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: over-charging starves goals of budget they did not spend but grants nothing; novelty 1: the delegated-job settlement is the pattern; exposure 2: every goal that runs proof attempts; accumulation 2: each ended proof attempt keeps its full reservation charged until the goal's revision changes"
- Tier: 2
- Intent: An ended testing-risk proof attempt (1b12f534) still charges its full reservation to the goal's reserved job-minutes, the same over-charge that dispatch-cap-necessity removed for delegated jobs; the settlement box named it as its own component (ProofReservationMinutes) rather than settle it. DONE means an ended proof attempt charges the minutes it ran, rounded up and clamped to its reservation, a live one its reservation, with the refusal line's proof=<n> following, and 1b12f534's own tests amended for exactly that.
- Origin: main
- Next step: One Sol round in internal/dispatch/budget.go's proof-attempt loop mirroring settledJobMinutes for proof attempts, with tests beside the sum-invariant test; one Opus review; land with --chain. Found by dispatch-cap-crit3-20260910 (DCN-09) on 2026-09-10. Wido approves at the terminal.
- OpenedAt: 2026-09-09T22:51:49Z
- Revision: 5
- Pinned: m1e
- BudgetExceptions: 0

History:
- 2026-09-09T22:51:49Z 7MWERD2ZYAKHY1N3KVQVR8TY8K-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=proof-attempts-settle-to-minutes-run
- 2026-09-10T05:48:27Z WEXFFS3GDSTNTEYCMDYSEM1GJA-m1-c6925449 approve actor=human:Wido targets=proof-attempts-settle-to-minutes-run
- 2026-09-11T08:01:57Z YVD6080SP15T0KF02T7S3MDJ8E-m1-c6925449 unapprove actor=human:Wido targets=proof-attempts-settle-to-minutes-run reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
- 2026-09-11T16:17:29Z 8M31V7VHZ6TFG60GMGZQZ9S09F-m1-c6925449 set-pin actor=human:Wido targets=proof-attempts-settle-to-minutes-run
- 2026-09-11T16:17:45Z 030HT9PVZGVHDS5RKBJJ1AFZZP-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,budget-extends-by-consumption-and-breach-parks,burn-without-delivery-tripwire,capped-round-continues-instead-of-restarting,carried-landing-debt-and-cap,coordinator-context-stays-under-budget,coordinator-wakes-on-events-not-polls,critique-closes-on-folded-proof,deep-battery-under-ten-minutes,delegate-job-liveness,delegate-rounds-reuse-a-warm-gate,delivery-receipt-stops-at-first-failure,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,land-ready-work-lands-without-a-claim-slot,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-attempts-settle-to-minutes-run,proof-groups-detect-hangs-by-progress-not-the-clock,proof-harness-process-custody,retained-proof-reuse-crosses-claims-and-attempts,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,tier-from-severity-and-novelty,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=proof-attempts-settle-to-minutes-run from=unranked to=1:7 requested-sequence=7
Integrity: sha256=9de0fe988411c163819825cbc6e7d497c7be7246039928fcdf9b9563c05cb33a
