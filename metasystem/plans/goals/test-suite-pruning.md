# test-suite-pruning

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Removing useful tests could permit regressions; the mechanisms are existing tests and fixtures; the suite runs on every development host and cost accumulates with repeated runs. This item records queued cleanup, without claiming execution or approving a budget."
- Tier: 2
- Intent: At Wido request, prune duplicate and non-beneficial tests as separate work from risk-based execution modes. Reduce recurring validation cost while preserving distinct defect detection.
- Origin: main
- Next step: Inventory the most expensive test groups; identify each candidate test unique behavior and failure signal, equivalent retained coverage and measured runtime. Propose deletions or consolidation with evidence before changing tests. Keep this separate from the common application test interface and coordinator-loop-prevention delivery. ADDENDUM (Fable second-opinion review, 2026-09-09 20:25 UTC): during coordinator-loop-prevention Codex produced scratch patches converting nineteen fixture scripts to per-scenario collection, about 2,600 patch lines, none applied: collection-boundaries/collection-boundaries.patch (validate-metasystem.sh composite sections and go-gate.sh), collection-leaf-fixtures/leaf-fixture-collection.patch (runtime-hook, suite-progress, supervision-go fixtures), collection-other-fixtures/collection.patch (adapter-deadline, conformance, checkout-execution-guard, path-class, supervision-hook fixtures), collection-legacy-fixtures/legacy-fixture-collection-DRAFT.patch (ten legacy fixtures) and adoption-collection/adoption-collection.patch (adopt-fixtures.sh as 34 private cases), all under /Users/wido/metasystem-evidence/agentic-tools/coordinator-loop-prevention/fable-second-opinion-20260909T1440Z. They stay parked: real bodies were largely unrun (8 of 44 in the five-script set, with one real failure, the supervision-hook digest-compatibility case at 15,541 characters against a 4,000-rune bound, left unexplained), eight of the ten legacy scripts belong to sections the delivery plan omits as outside impact, and metasystem/plans/coordinator-loop-prevention-testing-design.md already states that a group owns its internal setup and dependent checks. Use those patches only as an inventory of scenario boundaries when deciding what to prune; pruning comes before collecting. Add to the inventory: the coverage groups goal-full-coverage and missionrunner-full-coverage measured 574 and 455 seconds on 2026-09-09; the six per-fixture surfaces and the brain-seat surface added to metasystem/testing.json on 2026-09-09 make each fixture run whenever its script changes, so pruning a fixture must remove its surface too. Order after diagnostics-never-swallowed; do not start while coordinator-loop-prevention is in flight.
- OpenedAt: 2026-09-08T16:13:31Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-08T16:13:31Z MVQB2N9NB40E5JN0G531CG0793-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=test-suite-pruning
- 2026-09-09T20:13:19Z QK94VV2Y4T80PS4CEPH86Y2RVK-m1c-8d678ae8 edit actor=m1c+main-1788963308-60248-b019cb targets=test-suite-pruning
- 2026-09-09T20:16:39Z Q9Z6QEB598RFNBCF5MZ4AKFN5H-m1c-8d678ae8 edit actor=m1c+main-1788963308-60248-b019cb targets=test-suite-pruning
Integrity: sha256=39b632d8b41597f3aff77fa340574d65ae5e42996ba2eb60a0715447f8265ea0
