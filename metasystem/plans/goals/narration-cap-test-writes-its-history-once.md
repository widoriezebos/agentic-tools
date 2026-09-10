# narration-cap-test-writes-its-history-once

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: TestNarrationCapsItsHistory in internal/steward/narrate_test.go builds its 2000-line history through 2025 Narrate calls, each rewriting the file through atomicfile with an fsync, and takes 41 s in governed-standard. DONE means the test writes the history to disk directly, then calls Narrate a handful of times to prove the cap, and governed-standard drops by about 40 s.
- Origin: human
- Next step: Slice 4 item 3 of plans/suite-speed-plan.md. The cap's behaviour under test must not weaken: the test still proves the cap at the boundary. Code critique only.
- OpenedAt: 2026-09-10T12:02:46Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-10T12:02:46Z DPGDD6PEGVCFSPJ6RCWKB9ZKY8-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=narration-cap-test-writes-its-history-once
Integrity: sha256=66ffd087bf1561afdbc165bc57f2785465e22e352c77c830e9520cd1126463f5
