# adoption-comparison-runs-once

- State: done
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The adoption fixture bed runs the adoption comparison test once, not twice: scripts/adopt-fixtures.sh calls run_adoption_comparison at lines 641 and 743 and the retained log shows 143 s and 93 s inside one 531 s section. DONE means the second call is proven redundant (its inputs equal the first's, the condition the testing-contract design names) and dropped or narrowed to the registration-only assertion, saving 90 to 140 s per run of section/adoption-fixtures.
- Origin: human
- Next step: Slice 4 item 1 of plans/suite-speed-plan.md. Prove the inputs of both calls are identical before removing; if they differ, narrow the second to what differs. Code critique only.
- Concluded: Landed 39907ed2 (2026-09-11) by a human commit from the enrolled terminal in Wido's name, implemented by the coordinator on the m1e seat. The copied adoption target keeps its registration-only assertions and the engine build they need and drops the redundant delivery proof, whose inputs were the filled target's minus the covenant sections from the same frozen source, contract filling and engine. Verified once on the candidate tree through the engine: section/adoption-fixtures passed as a diagnostic proof (548 s with its canaries), filled leg proving delivery and reuse, copied leg registration only.
- OpenedAt: 2026-09-10T12:02:39Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:16Z revision=2 opid=MC08E4PDV9QGDMN2MQ527SXR88-m1-f47a9d40 authority=proven digest=b383163cee13ffc093686a7295118af489e45dd336f797d6210c0dbeb67d1cbb

History:
- 2026-09-10T12:02:39Z TKA5FBGJT2H45NMSW0N2NBZBMG-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=adoption-comparison-runs-once
- 2026-09-11T14:32:16Z MC08E4PDV9QGDMN2MQ527SXR88-m1-f47a9d40 approve actor=human:Wido targets=adoption-comparison-runs-once
- 2026-09-11T14:41:37Z SK50DHCQES2R6EDWP96H6HKH4D-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=adoption-comparison-runs-once
- 2026-09-11T14:52:37Z 4Z1518DMHXP5F6EC95M6PGT61G-m1-c6925449 done actor=human:Wido targets=adoption-comparison-runs-once displaced=m1e+main-1789030447-51011-5722fc@2026-09-11T14:41:37Z
Integrity: sha256=84c2a8d033060d7276d8a23f6c55762692446de367030612f726ab0f2ebc3579
