# examine-evidence-before-retesting

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Severity 3: wrongly carrying proof can authorize an unverified candidate. Novelty 2: extend existing result composition and independent examination with applicability judgment. Exposure 3: normal test and delivery workflows share this boundary. Accumulation 2: repeated broad reruns and partial-run evidence loss create material ongoing waste. Preserve proof obligations and test both safe reuse and invalidation."
- Tier: 3
- Intent: What: Before testing again, the system checks which existing test results still apply to the change and runs only what is missing. Why: Full reruns cost time when most results are still valid. A design exists, but it was written before the new command names and before the lane learned to reuse passed test groups. Pros: Faster checking with the same safety. Cons: Deciding which evidence still applies is subtle; a wrong reuse could hide a red.
- Origin: human
- Next step: Next: Update the design (plans/designs/examine-evidence-before-retesting.md) to today's commands and to the lane's reuse of passed groups by identity, then ask you to accept it; no build before that. Done when: you have accepted the revised design.
- OpenedAt: 2026-09-26T07:27:53Z
- Revision: 4
- Labels: evidence, testing
- BudgetExceptions: 0

History:
- 2026-09-26T07:27:53Z C9BN513AHRQ3N275GPMB12P9Q4-m1e-a2ccc4e1 open actor=human:Wido targets=examine-evidence-before-retesting
- 2026-09-26T07:30:49Z J68MHT0GAFFVA7CK7VQSW49M2G-m1e-a2ccc4e1 edit actor=m1e+main-1790272787-5030-13dde3 targets=examine-evidence-before-retesting
- 2026-09-26T07:54:49Z 81SG609AE41TA8BZETF9T1Z5HQ-m1e-a2ccc4e1 edit actor=m1e+main-1790272787-5030-13dde3 targets=examine-evidence-before-retesting
- 2026-09-30T18:48:59Z FZJS1MQ7XWX3KJ5XGV8TFGPQ22-m1e-b6a4eb0a edit actor=human:wido targets=examine-evidence-before-retesting
Integrity: sha256=fbc0c2ebb83da993afa0062ba21986043732065a15557653f87e95a11517070a
