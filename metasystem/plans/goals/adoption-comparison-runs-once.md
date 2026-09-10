# adoption-comparison-runs-once

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The adoption fixture bed runs the adoption comparison test once, not twice: scripts/adopt-fixtures.sh calls run_adoption_comparison at lines 641 and 743 and the retained log shows 143 s and 93 s inside one 531 s section. DONE means the second call is proven redundant (its inputs equal the first's, the condition the testing-contract design names) and dropped or narrowed to the registration-only assertion, saving 90 to 140 s per run of section/adoption-fixtures.
- Origin: human
- Next step: Slice 4 item 1 of plans/suite-speed-plan.md. Prove the inputs of both calls are identical before removing; if they differ, narrow the second to what differs. Code critique only.
- OpenedAt: 2026-09-10T12:02:39Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-10T12:02:39Z TKA5FBGJT2H45NMSW0N2NBZBMG-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=adoption-comparison-runs-once
Integrity: sha256=04a0a8cd41b8a86741726b4f0620bcbb5f031d5e9684dc01225aba6b60e13fba
