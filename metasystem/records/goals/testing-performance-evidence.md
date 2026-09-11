# testing-performance-evidence

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Comparative evidence and test pruning can misstate savings or hide lost defect detection; existing owners suffice and no execution is authorized by opening this deferred item."
- Tier: 2
- Intent: Deferred from coordinator-loop-prevention at Wido direction: assess recurring test cost and prove worthwhile savings without blocking delivery correctness.
- Origin: main
- Next step: After coordinator-loop-prevention lands and human prioritization: inspect saved Astra R2 patches and cost audit on m1c, decide whether exact 30 percent targets remain useful, repair the comparison-only covenant payload mismatch if needed, reconstruct a common valid baseline, and measure both sides once with existing Go reports and temporary observation tools. Include descendant build, blocking census and copied-validation counts honestly. Keep test-suite-pruning separate; do not add instrumentation or run this comparison for the current delivery goal.
- Concluded: Absorbed by goal:deep-battery-under-ten-minutes in the 2026-09-11 backlog consolidation on Wido's word; Its ask (assess recurring test cost, prove savings) is answered by plans/suite-speed-plan.md section 4 (per-group medians, sharding experiment) and program 12 deep-battery-under-ten-minutes whose DONE is the measured cadence run under ten minutes with three green runs; the m1c Astra R2 patches and 3. Its specific requirement is appended to that goal's next step.
- OpenedAt: 2026-09-09T07:02:39Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-09T07:02:39Z HM0265CA42BEV4C7KJK8MRDJ7J-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=testing-performance-evidence
- 2026-09-11T22:06:52Z 8YKNM96BPEDHFJPZX35AMEG38M-m1-c6925449 done actor=human:Wido targets=testing-performance-evidence
Integrity: sha256=7a2c5e968b857000be711b7cc944418f27d4f2534a1b9c099da08f70ccda7782
