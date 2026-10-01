# critique-cap-is-five

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Two constants/defaults and skill text"
- Tier: 1
- Intent: What: the critique round cap becomes 5 for design and code review at tiers 2 and 3 (tier 1 keeps 0 review rounds); the cap is a backstop, the loop ends when no finding would change the implementation materially under the threat model. Why: Wido 2026-10-01: 'agreed we can have a cap; but not at 2' - 2 cut converging loops short; the real stop is materiality + smallest step + threat model. Pros: real findings get fixed; a loop still producing material findings at 5 is diverging and goes to the human. Cons: none of note.
- Origin: human
- Next step: Change designCritiqueRounds (cmd/metasystem/intent_delivery.go), the tier boxes' review-round defaults for tiers 2/3, and the round-budget text in skills/design-critique and skills/code-critique; one test per limit
- OpenedAt: 2026-10-01T16:01:35Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T16:01:43Z revision=2 opid=N4SVA8S0DEVEX8ED7F811Y5YH2-m1e-9c612d71 authority=proven digest=e1a92f56f3740cadefb9a78975c164cfeec07e1a79429a0cea46cb2a2dc504a1 episode=2

History:
- 2026-10-01T16:01:35Z FN5N6N3FVKV3GH6TFNK5EHSVVS-m1e-9c612d71 open actor=human:Wido targets=critique-cap-is-five
- 2026-10-01T16:01:43Z N4SVA8S0DEVEX8ED7F811Y5YH2-m1e-9c612d71 approve actor=human:Wido targets=critique-cap-is-five
Integrity: sha256=920ab2873e3c111e33bea26aacd2c2faf0594ecba994974d6935c02e1d29233d
