# tier-from-severity-and-novelty

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a mis-tiered goal still gets critique at its tier; novelty 1: the derivation exists; exposure 3: every goal; accumulation 1: one formula"
- Tier: 2
- Intent: 75 percent of concluded goals are tier 3 because exposure 3 alone lifts the tier, as it lifted test depth before landing-runs-standard-deep-runs-at-cadence (process-rules.md section 5 item 3 of the delivery deep dive); this very goal had to be opened by a human with a tier override to be tier 2. DONE means: the tier is derived from severity and novelty only; exposure and accumulation scale cadence weight and the one-time deep run, exactly as that goal did for test depth; docs/orchestration.md and the classify path say so; proven by the tier probe on the current backlog showing under 40 percent tier 3. Goal 16 of plans/delivery-efficiency-plan.md. Tier 2 by Wido's choice 2026-09-11.
- Origin: human
- Next step: Read the tier derivation (the severity-tiered-rigor lineage and internal/testpolicy risk), change the formula and the docs, land.
- OpenedAt: 2026-09-11T15:50:34Z
- Revision: 2
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T15:50:34Z PGA3WGGB587Z9EJ8VSMBS1W4ZS-m1-c6925449 open actor=human:Wido targets=tier-from-severity-and-novelty reason=TierOverride: derived=3 set=2 why=Wido 2026-09-11: a formula change with the existing derivation; exposure alone does not make it tier 3, which is the point of this goal
- 2026-09-11T15:50:56Z 2GRVB10B9CHYJN2VG2N2T4N173-m1-c6925449 set-pin actor=human:Wido targets=tier-from-severity-and-novelty
Integrity: sha256=9af53ef7604e478686a2582b63abee50737d1c2b3f55c3582a5360ec8eda2749
