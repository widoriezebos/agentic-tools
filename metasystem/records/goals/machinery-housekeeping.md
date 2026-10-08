# machinery-housekeeping

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Small, known fixes (novelty 1); a wrong fix fails a test, no damage (severity 2); gates and seats read the register and the stamp (exposure 2); several small units (accumulation 2)"
- Tier: 2
- Intent: Housekeeping bundled: the two load-fragile tests and the findings-store isolation fixed at the source first, then the flake kind of the register, the engine stamp and the engine inputs (plans/engine-inputs-design-brief.md), and the remaining small findings 14, 17, 18, 19, 21 of the plan
- Origin: main
- Next step: Design from plans/machinery-housekeeping-design-brief.md (units at most 250 production lines, at most 5), then build by hand with delegates; first unit the flakes at their source
- Concluded: Machinery housekeeping integrated at a5530e8c7: the three load-fragile tests fixed at their source (artificial deadlines, owned launcher cleanup, isolated steward registry), test-keyed flake facts on the shipped register, one repeat per tree with the failing tests output as evidence, and flake facts in test status and incident list; the engine stamp, inputs and findings 17-21 moved to plans/machinery-housekeeping-follow-up-design-brief.md
- OpenedAt: 2026-10-07T20:20:10Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T20:20:18Z revision=2 opid=03GBCMDY133PHN79N3SDKCFVVD-m1e-718ba0eb authority=proven digest=84b90eb77f392edd3346d9a77afd15766caa8403bbbcb65740c39fe6b8b0e079 episode=2

History:
- 2026-10-07T20:20:10Z K37MPJJD3G7YFH6KF371H7T7VF-m1e-718ba0eb open actor=human:Wido targets=machinery-housekeeping
- 2026-10-07T20:20:18Z 03GBCMDY133PHN79N3SDKCFVVD-m1e-718ba0eb approve actor=human:Wido targets=machinery-housekeeping
- 2026-10-08T04:44:24Z N8P4HX3H38ZE6GDX9R4WM71F50-m1e-718ba0eb done actor=human:Wido targets=machinery-housekeeping
Integrity: sha256=a8ce5c410ff60f233ad8efa6c9a401fd84e457d012cb3433c307eb287bae1de9
