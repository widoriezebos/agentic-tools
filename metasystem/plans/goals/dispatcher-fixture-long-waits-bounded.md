# dispatcher-fixture-long-waits-bounded

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The dispatcher fixture section has three long gaps that are waits, not work (46 s after the cannot-mirror cancel-husk step, 19 s and 16 s elsewhere). DONE means what each gap waits for in scripts/agents/dispatch-fixtures.sh and dispatch.sh is named and bounded, saving up to 80 s per run of section/dispatcher-adapter-and-mission-runner-fixtures.
- Origin: human
- Next step: Slice 4 item 5 of plans/suite-speed-plan.md. Measure first (CENSUS-WAIT-MEASUREMENT lines in the retained log), name the wait, then bound it without weakening the fixture's proof. Code critique only.
- OpenedAt: 2026-09-10T12:02:53Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-10T12:02:53Z PNVBM3MW0J0PTVC03C145AF6XC-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=dispatcher-fixture-long-waits-bounded
Integrity: sha256=3a997a983bfa6c3ebf946fc9a4a2a47a534462f36bf42ac9d87465ab03e45a71
