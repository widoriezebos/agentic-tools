# tier-from-severity-and-novelty

- State: queued
- Priority: 1
- Sequence: 16
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a mis-tiered goal still gets critique at its tier; novelty 1: the derivation exists; exposure 3: every goal; accumulation 1: one formula"
- Tier: 2
- Intent: 75 percent of concluded goals are tier 3 because exposure 3 alone lifts the tier, as it lifted test depth before landing-runs-standard-deep-runs-at-cadence (process-rules.md section 5 item 3 of the delivery deep dive); this very goal had to be opened by a human with a tier override to be tier 2. DONE means: the tier is derived from severity and novelty only; exposure and accumulation scale cadence weight and the one-time deep run, exactly as that goal did for test depth; docs/orchestration.md and the classify path say so; proven by the tier probe on the current backlog showing under 40 percent tier 3. Goal 16 of plans/delivery-efficiency-plan.md. Tier 2 by Wido's choice 2026-09-11.
- Origin: human
- Next step: Read the tier derivation (the severity-tiered-rigor lineage and internal/testpolicy risk), change the formula and the docs, land.
- OpenedAt: 2026-09-11T15:50:34Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T15:50:34Z PGA3WGGB587Z9EJ8VSMBS1W4ZS-m1-c6925449 open actor=human:Wido targets=tier-from-severity-and-novelty reason=TierOverride: derived=3 set=2 why=Wido 2026-09-11: a formula change with the existing derivation; exposure alone does not make it tier 3, which is the point of this goal
- 2026-09-11T15:50:56Z 2GRVB10B9CHYJN2VG2N2T4N173-m1-c6925449 set-pin actor=human:Wido targets=tier-from-severity-and-novelty
- 2026-09-11T15:50:59Z XMVFS3RNBYJQA0S9A89QGZ8XAS-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,capped-round-continues-instead-of-restarting,carried-landing-debt-and-cap,critique-closes-on-folded-proof,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,land-ready-work-lands-without-a-claim-slot,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,tier-from-severity-and-novelty,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=tier-from-severity-and-novelty from=unranked to=1:16 requested-sequence=16
Integrity: sha256=a86e4378e4373f647b5343c2ed2df94f6cc289e622a7d2032ce2dbf964cbf5d8
