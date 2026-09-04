# finding-register-id-collision-across-chains

- State: queued
- Tier: 2
- Intent: The finding register (internal/dispatch/finding_register.go refuseCrossRootClassConflict) unions findings by their bare id across every chain root in the job records, so two unrelated reviews that both number a finding F-1 (the code-critic schema's example id) collide: on m2 2026-09-04 the part-three review of the tiering machinery (str-build3-cc1, F-1 severe) was refused against part one's review (str-build1c-cc1, F-1 bounded) with 'conflicting rigor classes ... waiting on the original critic or the human is the only remedy', which stalled the correction round until the critic re-issued its findings under chain-unique ids. DONE means finding identity in the register is scoped by the reviewed subject (chain root or reviewed tree), or ids are namespaced by chain at registration, so same-id findings from different subjects never conflict; a fixture proves two chains with F-1 of different classes both advance.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside an existing owner): build plus one code review; box 4h/6/720m/1/2. Waits for human approval for execution. Related: STR2-CRITIC-UNION-11 of the tiering design (the same-tree union), which is the lawful union this defect over-applies. SPECIMEN 2026-09-04 (m2): cancelling the stuck critic round (delegate --cancel str-build3-cc1) did not unblock the implementer follow-up (it still demands the register advance), and a critic follow-up to re-issue ids is refused because the fold itself fails; the only exit was a fresh implementer chain from a preserved branch (str-build3b) with a fresh critic. Until this lands, every critic brief must ask for chain-unique finding ids (STRxx-nn), never the schema's example F-n.
- OpenedAt: 2026-09-04T01:40:46Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2

History:
- 2026-09-04T01:40:46Z W6CNNVERCJ0WSECCHBFG386GGQ-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=finding-register-id-collision-across-chains
- 2026-09-04T01:43:35Z DJDSDV56G3BHTG89EPC037Y7K1-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=finding-register-id-collision-across-chains
- 2026-09-04T01:44:30Z ZHBHPZHE1CKNHRCQ6EEBBPRE5K-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=finding-register-id-collision-across-chains
Integrity: sha256=78105b8945fa421e4a9cc8f54fcbfe6b81d6a7ab42dc56ac41b7b33055312c42
